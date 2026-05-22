package rules

// dispatcher.go provides Dispatcher — the per-file rule dispatcher.
// It classifies rules once at construction (by Needs/NodeTypes), then
// walks each parsed file's flat AST a single time and routes matching
// nodes to node-targeted rules. Project-scope rule families
// (cross-file, module-aware, manifest, resources, icons, gradle) are
// stored here but invoked by the main pipeline once their indexes are
// built.

import (
	"context"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

var ruleProfileLabelsEnabled atomic.Bool

// SetRuleProfileLabels enables pprof labels around individual rule callbacks.
// It returns a restore function so tests or embedded callers can scope the
// setting. The CLI enables this while --cpuprofile is active so pprof can rank
// samples by krit_rule / krit_rule_family labels.
func SetRuleProfileLabels(enabled bool) func() {
	previous := ruleProfileLabelsEnabled.Swap(enabled)
	return func() {
		ruleProfileLabelsEnabled.Store(previous)
	}
}

func runWithRuleProfileLabel(ruleID, family string, fn func()) {
	if !ruleProfileLabelsEnabled.Load() {
		fn()
		return
	}
	pprof.Do(context.Background(), pprof.Labels(
		"krit_rule", ruleID,
		"krit_rule_family", family,
	), func(context.Context) {
		fn()
	})
}

// Dispatcher runs per-file rule execution directly against api.Rule
// values. It classifies rules once in the constructor and keeps them
// in parallel slices indexed by FlatNode.Type for O(1) node dispatch,
// using the scanner's flat-type index.
//
// Cross-file and module-aware rules are stored but NOT invoked inside
// Run — they require indexes that are only available once all files
// have been parsed. The main pipeline is expected to iterate
// CrossFileRules()/ModuleAwareRules() after the per-file pass.
type Dispatcher struct {
	// flatTypeRules is indexed by FlatNode.Type for O(1) flat dispatch.
	flatTypeRules [][]*api.Rule
	// allNodeRules is the set of node rules that declare nil NodeTypes
	// — they receive every node during the walk.
	allNodeRules []*api.Rule
	// lineRules run on file.Lines after the AST walk completes.
	lineRules []*api.Rule
	// crossFileRules and moduleAwareRules are exposed to callers via
	// CrossFileRules()/ModuleAwareRules() and not invoked from Run.
	crossFileRules   []*api.Rule
	moduleAwareRules []*api.Rule
	// manifestRules / resourceRules / iconRules / gradleRules / aggregateRules
	// are also NOT invoked from Run — they require project-level
	// context that is assembled by the main pipeline. They are stored
	// here so ensureFlatTypeIndex and collectAllRules can see them for
	// re-index purposes.
	manifestRules       []*api.Rule
	resourceRules       []*api.Rule
	resourceSourceRules []*api.Rule
	iconRules           []*api.Rule
	gradleRules         []*api.Rule
	aggregateRules      []*api.Rule
	// nodeDispatchRules are rules with non-empty NodeTypes. They live in
	// flatTypeRules indexed by FlatNode.Type, but scanner.NodeTypeTable
	// is populated lazily as files are parsed — so at construction time
	// many of these rules' node types may not yet have IDs. We keep the
	// full list here so ensureFlatTypeIndex can re-index them once the
	// table has grown after some files have been parsed.
	nodeDispatchRules []*api.Rule

	typeResolver      typeinfer.TypeResolver
	libraryFacts      *librarymodel.Facts
	javaSemanticFacts *javafacts.Facts
	// fileFacts memoizes per-file derived facts (imports, declarations,
	// references) shared by all rules running in this dispatcher. Lifetime
	// is bounded by the dispatcher; one cache per analysis run is the
	// expected use.
	fileFacts *filefacts.Cache

	flatTypeIndexSize int
	mu                sync.RWMutex

	// reportOnce guards ReportMissingCapabilities so repeat calls on the
	// same dispatcher (e.g. a shared instance across CLI + LSP) emit the
	// diagnostic log only once per run.
	reportOnce sync.Once

	// languageExcluded caches, per scanner.Language, the set of rule IDs
	// that do NOT apply to that language. Rules are static after
	// construction so the map per language is computed once and reused
	// for every file of that language.
	languageExcluded sync.Map // scanner.Language -> map[string]bool
}

// NewDispatcher constructs a Dispatcher from the supplied v2 rules.
// An optional TypeResolver is wired through to any rule declaring
// NeedsResolver via that rule's SetResolverHook.
func NewDispatcher(rules []*api.Rule, resolver ...typeinfer.TypeResolver) *Dispatcher {
	d := &Dispatcher{fileFacts: filefacts.NewCache()}
	setSharedFileFacts(d.fileFacts)
	if len(resolver) > 0 && resolver[0] != nil {
		d.typeResolver = resolver[0]
	}

	// Validate RelatedRules references against the full registry so
	// dangling cross-links panic at startup rather than producing dead
	// links in MCP output or surprising --disable-related behavior. The
	// full registry is used (not just the active subset) so a project
	// disabling some rules does not mask a typo in another's metadata.
	if err := api.ValidateRelations(api.Registry); err != nil {
		panic("rules: invalid RelatedRules registration: " + err.Error())
	}

	// Apply RunAfter ordering so any rule that declares a dependency
	// runs after the rule(s) it depends on across every per-file scope
	// bucket. Rules without RunAfter keep their original relative order.
	rules = SortByRunAfter(rules)

	// Classify each rule by its Scope (set explicitly on the rule, or
	// derived from Needs + NodeTypes via RuleScope).
	for _, r := range rules {
		if r == nil {
			continue
		}
		switch RuleScope(r) {
		case api.ScopeCrossFile:
			d.crossFileRules = append(d.crossFileRules, r)
		case api.ScopeModuleIndex:
			d.moduleAwareRules = append(d.moduleAwareRules, r)
		case api.ScopeManifest:
			d.manifestRules = append(d.manifestRules, r)
		case api.ScopeResourceSource:
			d.resourceSourceRules = append(d.resourceSourceRules, r)
		case api.ScopeResource:
			d.resourceRules = append(d.resourceRules, r)
		case api.ScopeIcons:
			d.iconRules = append(d.iconRules, r)
		case api.ScopeGradle:
			d.gradleRules = append(d.gradleRules, r)
		case api.ScopeAggregate:
			d.aggregateRules = append(d.aggregateRules, r)
		case api.ScopeLinePass:
			d.lineRules = append(d.lineRules, r)
		case api.ScopePerFileNode:
			// Indexed by flat type ID in buildFlatTypeIndex. Also
			// stored in nodeDispatchRules so ensureFlatTypeIndex can
			// re-index them later once NodeTypeTable has grown.
			d.nodeDispatchRules = append(d.nodeDispatchRules, r)
		case api.ScopePerFileAllNodes:
			// nil NodeTypes + no scope flag → receive every node.
			d.allNodeRules = append(d.allNodeRules, r)
		}
	}

	// Second pass: populate flatTypeRules for rules with explicit node types.
	for _, r := range rules {
		if r == nil {
			continue
		}
		if isDeferredFromPerFileDispatch(r) {
			continue
		}
		if len(r.NodeTypes) == 0 {
			continue
		}
		// Placement into flatTypeRules happens inside buildFlatTypeIndex so
		// resizing NodeTypeTable (lazy intern of node types) is handled
		// uniformly.
	}

	d.buildFlatTypeIndex(rules)
	return d
}

// SetLibraryFacts wires project-wide library semantics into contexts created
// by this dispatcher.
func (d *Dispatcher) SetLibraryFacts(facts *librarymodel.Facts) {
	if d == nil {
		return
	}
	d.libraryFacts = facts
}

// SetJavaSemanticFacts wires optional javac-backed semantic facts into
// contexts created by this dispatcher.
func (d *Dispatcher) SetJavaSemanticFacts(facts *javafacts.Facts) {
	if d == nil {
		return
	}
	d.javaSemanticFacts = facts
}

// buildFlatTypeIndex populates flatTypeRules from the supplied rules.
// Called from the constructor and from ensureFlatTypeIndex when the
// NodeTypeTable has grown since the last build.
func (d *Dispatcher) buildFlatTypeIndex(rules []*api.Rule) {
	d.mu.Lock()
	defer d.mu.Unlock()

	size := scanner.NodeTypeTableSize()
	if size <= 0 {
		size = 1
	}
	d.flatTypeRules = make([][]*api.Rule, size)

	ensureSize := func(need int) {
		if need <= len(d.flatTypeRules) {
			return
		}
		grown := make([][]*api.Rule, need)
		copy(grown, d.flatTypeRules)
		d.flatTypeRules = grown
	}

	for _, r := range rules {
		if r == nil {
			continue
		}
		if isDeferredFromPerFileDispatch(r) {
			continue
		}
		if len(r.NodeTypes) == 0 {
			continue
		}
		for _, t := range r.NodeTypes {
			if typeID, ok := scanner.LookupFlatNodeType(t); ok {
				ensureSize(int(typeID) + 1)
				d.flatTypeRules[typeID] = append(d.flatTypeRules[typeID], r)
			}
		}
	}

	d.flatTypeIndexSize = len(d.flatTypeRules)
}

// RuleScope returns the dispatcher bucket a rule lands in. When the
// rule sets r.Scope explicitly that value is honoured; otherwise the
// scope is derived from the rule's Capabilities and NodeTypes. The
// derivation matches the historical classifier ordering and is the
// authoritative mapping until every registration sets Scope explicitly.
func RuleScope(r *api.Rule) api.Scope {
	if r == nil {
		return api.ScopeUnset
	}
	if r.Scope != api.ScopeUnset {
		return r.Scope
	}
	switch {
	case r.Needs.Has(api.NeedsCrossFile):
		return api.ScopeCrossFile
	case r.Needs.Has(api.NeedsModuleIndex):
		return api.ScopeModuleIndex
	case r.Needs.Has(api.NeedsManifest):
		return api.ScopeManifest
	case isResourceBackedSourceRule(r):
		return api.ScopeResourceSource
	case r.Needs.Has(api.NeedsResources):
		return api.ScopeResource
	case rulesNeedsIcons(r):
		return api.ScopeIcons
	case r.Needs.Has(api.NeedsGradle):
		return api.ScopeGradle
	case r.Needs.Has(api.NeedsAggregate):
		return api.ScopeAggregate
	case r.Needs.Has(api.NeedsLinePass):
		return api.ScopeLinePass
	case r.Needs.Has(api.NeedsParsedFiles):
		return api.ScopeParsedFiles
	case len(r.NodeTypes) > 0:
		return api.ScopePerFileNode
	case r.NodeTypes == nil:
		return api.ScopePerFileAllNodes
	}
	return api.ScopeUnset
}

func isResourceBackedSourceRule(r *api.Rule) bool {
	if r == nil || !r.Needs.Has(api.NeedsResources) || len(r.NodeTypes) == 0 {
		return false
	}
	for _, lang := range api.RuleLanguages(r) {
		if lang != scanner.LangXML {
			return true
		}
	}
	return false
}

func isDeferredFromPerFileDispatch(r *api.Rule) bool {
	return r.Needs.Has(api.NeedsCrossFile) ||
		r.Needs.Has(api.NeedsModuleIndex) ||
		r.Needs.Has(api.NeedsManifest) ||
		r.Needs.Has(api.NeedsResources) ||
		rulesNeedsIcons(r) ||
		r.Needs.Has(api.NeedsGradle) ||
		r.Needs.Has(api.NeedsAggregate) ||
		r.Needs.Has(api.NeedsLinePass)
}

func rulesNeedsIcons(r *api.Rule) bool {
	return r != nil && AndroidDataDependency(r.AndroidDeps)&AndroidDepIcons != 0
}

func ruleMatchesLexicalCalleeNames(r *api.Rule, file *scanner.File, idx uint32) bool {
	if r == nil || len(r.LexicalCalleeNames) == 0 {
		return true
	}
	if file == nil || file.FlatType(idx) != "call_expression" {
		return true
	}
	for _, name := range r.LexicalCalleeNames {
		if flatCallExpressionNameEquals(file, idx, name) {
			return true
		}
	}
	return false
}

// ensureFlatTypeIndex returns the flat-type rule index, rebuilding it if
// NodeTypeTable has grown since the last build.
func (d *Dispatcher) ensureFlatTypeIndex(rules []*api.Rule) [][]*api.Rule {
	d.mu.RLock()
	idx := d.flatTypeRules
	builtSize := d.flatTypeIndexSize
	d.mu.RUnlock()

	needSize := scanner.NodeTypeTableSize()
	if needSize <= 0 {
		needSize = 1
	}
	if idx != nil && builtSize >= needSize {
		return idx
	}

	d.buildFlatTypeIndex(rules)
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.flatTypeRules
}

// Run executes all per-file rules and returns findings in columnar form.
// Rule panics are logged to stderr.
func (d *Dispatcher) Run(file *scanner.File) scanner.FindingColumns {
	columns, stats := d.RunColumnsWithStats(file)
	for _, e := range stats.Errors {
		reporter().Warnf("%s\n", e.Error())
	}
	return columns
}

// RunWithStats executes all per-file rules on a file and returns both
// findings in columnar form and coarse timing for each execution bucket.
func (d *Dispatcher) RunWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	return d.RunColumnsWithStats(file)
}

// RunColumnsWithStats runs all per-file rules emitting findings directly
// into a FindingCollector, bypassing the intermediate []scanner.Finding
// accumulation. Rules emit via ctx.Emit which routes straight to the
// collector; the result is returned as columnar data.
//
//nolint:gocyclo // Hot path: per-rule dispatch is inlined twice (posting-list vs full-walk path) to avoid closure allocations and preserve Go inlining; extracting into helpers measurably regresses small-file dispatch.
func (d *Dispatcher) RunColumnsWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	stats := RunStats{
		DispatchRuleNsByRule: make(map[string]int64),
		RuleStatsByRule:      make(map[string]RuleExecutionStat),
	}
	if file == nil {
		return scanner.FindingColumns{}, stats
	}

	flatTypeRules := d.ensureFlatTypeIndex(d.collectAllRules())

	// Reuse the SuppressionFilter built in pipeline.Parse when present;
	// otherwise (LSP/MCP ParseSingle path) build one lazily so single-
	// file callers keep the same @Suppress semantics. Either way the
	// annotation index walks the flat tree only once per file.
	start := time.Now()
	filter := file.Suppression
	if filter == nil {
		filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
		file.Suppression = filter
		file.SuppressionIdx = filter.Annotations()
	}
	stats.SuppressionIndexMs += time.Since(start).Milliseconds()

	excludedRules := d.buildExcludedSet(file.Path)
	for id := range d.excludedForLanguage(file.Language) {
		excludedRules[id] = true
	}

	collector := scanner.NewFindingCollector(0)
	javaFileFacts, javaSourceIndex := javaContextFactsForFile(file)

	// Single-pass AST walk.
	//
	// Two paths share the same per-rule dispatch logic (inlined here, not
	// a closure — closures would defeat the compiler's inlining and add
	// allocation overhead per Run call):
	//
	//   1. Posting-list path: iterate the CSR posting list
	//      (tree.NodeTypeOffsets / NodeTypeIndices) for each typeID that
	//      has at least one subscribed rule. Visits ONLY indices of
	//      subscribed types, skipping the bulk of the tree.
	//
	//   2. Full walk path: a single pass over every node. Used when
	//      there is at least one rule registered with nil NodeTypes
	//      (d.allNodeRules), since those need to see every node.
	//
	// In the common case (no allNodeRules) the posting-list path is
	// strictly less work than the old walk-everything-and-dispatch-by-type
	// approach: most nodes have no subscribed handlers, so we save the
	// per-node loop body entirely.
	start = time.Now()
	if tree := file.FlatTree; tree != nil && tree.Len() > 0 {
		if len(d.allNodeRules) == 0 {
			// Posting-list path.
			//
			// The per-rule body below is duplicated in the full-walk path
			// at line ~493 and in the allNodeRules block at line ~516.
			// Keep all three in sync — extracting to a helper measurably
			// regresses small-file dispatch (closures defeat inlining).
			// Hoist the CSR arrays into locals so the compiler keeps
			// them in registers across the loop and can prove the
			// two-index slice expression in-bounds from the n clamp.
			offsets := tree.NodeTypeOffsets
			postingIndices := tree.NodeTypeIndices
			n := len(offsets) - 1
			if n < 0 {
				n = 0
			}
			if n > len(flatTypeRules) {
				n = len(flatTypeRules)
			}
			for typeID := 0; typeID < n; typeID++ {
				handlers := flatTypeRules[typeID]
				if len(handlers) == 0 {
					continue
				}
				indices := postingIndices[offsets[typeID]:offsets[typeID+1]]
				if len(indices) == 0 {
					continue
				}
				for _, flatIdx := range indices {
					flatNode := tree.Node(flatIdx)
					for _, r := range handlers {
						if excludedRules[r.ID] {
							continue
						}
						if !ruleMatchesLexicalCalleeNames(r, file, flatIdx) {
							continue
						}
						resolver, ok := d.resolveForRule(r)
						if !ok {
							continue
						}
						t := time.Now()
						runWithRuleProfileLabel(r.ID, "dispatch", func() {
							safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats, resolver, d.libraryFacts, javaFileFacts, javaSourceIndex, d.javaSemanticFacts, d.fileFacts)
						})
						elapsed := time.Since(t).Nanoseconds()
						stats.DispatchRuleNs += elapsed
						stats.DispatchRuleNsByRule[r.ID] += elapsed
						stats.recordRule(r.ID, "dispatch", elapsed)
					}
				}
			}
		} else {
			// Full walk path: every node must be visited for allNodeRules.
			// Per-rule body kept in sync with the posting-list path above.
			n := tree.Len()
			for i := 0; i < n; i++ {
				flatIdx := uint32(i)
				flatNode := tree.Node(flatIdx)
				var handlers []*api.Rule
				if int(flatNode.Type) < len(flatTypeRules) {
					handlers = flatTypeRules[flatNode.Type]
				}
				for _, r := range handlers {
					if excludedRules[r.ID] {
						continue
					}
					if !ruleMatchesLexicalCalleeNames(r, file, flatIdx) {
						continue
					}
					resolver, ok := d.resolveForRule(r)
					if !ok {
						continue
					}
					t := time.Now()
					runWithRuleProfileLabel(r.ID, "dispatch", func() {
						safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats, resolver, d.libraryFacts, javaFileFacts, javaSourceIndex, d.javaSemanticFacts, d.fileFacts)
					})
					elapsed := time.Since(t).Nanoseconds()
					stats.DispatchRuleNs += elapsed
					stats.DispatchRuleNsByRule[r.ID] += elapsed
					stats.recordRule(r.ID, "dispatch", elapsed)
				}
				for _, r := range d.allNodeRules {
					if excludedRules[r.ID] {
						continue
					}
					resolver, ok := d.resolveForRule(r)
					if !ok {
						continue
					}
					t := time.Now()
					runWithRuleProfileLabel(r.ID, "dispatch", func() {
						safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats, resolver, d.libraryFacts, javaFileFacts, javaSourceIndex, d.javaSemanticFacts, d.fileFacts)
					})
					elapsed := time.Since(t).Nanoseconds()
					stats.DispatchRuleNs += elapsed
					stats.DispatchRuleNsByRule[r.ID] += elapsed
					stats.recordRule(r.ID, "dispatch", elapsed)
				}
			}
		}
	}
	stats.DispatchWalkMs += time.Since(start).Milliseconds()

	// Line-pass rules.
	start = time.Now()
	for _, r := range d.lineRules {
		if excludedRules[r.ID] {
			continue
		}
		resolver, ok := d.resolveForRule(r)
		if !ok {
			continue
		}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					stats.Errors = append(stats.Errors, DispatchError{RuleName: r.ID, FilePath: filePathOrEmpty(file), PanicValue: rec})
				}
			}()
			ctx := &api.Context{File: file, Rule: r, DefaultConfidence: api.ConfidenceMedium, Collector: collector, LibraryFacts: d.libraryFacts, JavaFacts: javaFileFacts, JavaSourceIndex: javaSourceIndex, JavaSemanticFacts: d.javaSemanticFacts, Facts: d.fileFacts}
			if r.Needs.Has(api.NeedsResolver) {
				ctx.Resolver = resolver
			}
			t := time.Now()
			runWithRuleProfileLabel(r.ID, "line", func() {
				r.Check(ctx)
			})
			stats.recordRule(r.ID, "line", time.Since(t).Nanoseconds())
		}()
	}
	stats.LineRuleMs += time.Since(start).Milliseconds()

	columns := *collector.Columns()

	// Suppression filter — one call covers annotations, config excludes,
	// and inline comments. Baseline filtering is still applied as a
	// post-collect step because it needs the full Finding struct.
	start = time.Now()
	if columns.Len() > 0 {
		filtered := columns.FilterRows(func(row int) bool {
			return !filter.IsSuppressed(columns.RuleAt(row), columns.RuleSetAt(row), columns.LineAt(row))
		})
		columns = filtered
	}
	stats.SuppressionFilterMs += time.Since(start).Milliseconds()

	for _, e := range stats.Errors {
		reporter().Warnf("%s\n", e.Error())
	}
	return columns, stats
}

// safeCheckV2NodeColumnar invokes a rule with a Collector attached so
// ctx.Emit routes findings directly into columnar storage.
func safeCheckV2NodeColumnar(r *api.Rule, idx uint32, node *scanner.FlatNode, file *scanner.File, collector *scanner.FindingCollector, stats *RunStats, typeResolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaFileFacts *javafacts.JavaFileFacts, javaSourceIndex *javafacts.SourceIndex, javaSemanticFacts *javafacts.Facts, fileFacts *filefacts.Cache) {
	defer func() {
		if rec := recover(); rec != nil {
			line := 0
			if node != nil {
				line = int(node.StartRow) + 1
			} else if file != nil {
				line = file.FlatRow(idx) + 1
			}
			stats.Errors = append(stats.Errors, DispatchError{
				RuleName:   r.ID,
				FilePath:   filePathOrEmpty(file),
				Line:       line,
				PanicValue: rec,
			})
		}
	}()
	ctx := &api.Context{
		File:              file,
		Node:              node,
		Idx:               idx,
		Rule:              r,
		DefaultConfidence: api.ConfidenceVeryHigh,
		Collector:         collector,
		LibraryFacts:      libraryFacts,
		JavaFacts:         javaFileFacts,
		JavaSourceIndex:   javaSourceIndex,
		JavaSemanticFacts: javaSemanticFacts,
		Facts:             fileFacts,
	}
	if r.Needs.Has(api.NeedsResolver) {
		ctx.Resolver = typeResolver
	}
	r.Check(ctx)
}

// fallbackAware is implemented by resolver wrappers (notably
// *oracle.CompositeResolver) that layer an oracle on top of a source-
// level resolver. When present, Fallback() returns just the source-level
// backend — handed to rules that declared TypeInfo.PreferBackend =
// PreferResolver so they skip the oracle IPC per call.
type fallbackAware interface {
	Fallback() typeinfer.TypeResolver
}

// resolveForRule returns the TypeResolver to populate in ctx.Resolver
// for rule r, honoring r.TypeInfo.PreferBackend when both backends are
// available. The second return is false when the rule should be skipped
// (preferred backend unavailable AND TypeInfo.Required=false).
//
// The zero-value hint (PreferAny, Required=false) preserves the pre-
// hint behaviour: whatever the caller handed NewDispatcher is passed
// through untouched. Rules that don't declare NeedsResolver never ask
// this helper — their ctx.Resolver stays nil as before.
func (d *Dispatcher) resolveForRule(r *api.Rule) (typeinfer.TypeResolver, bool) {
	base := d.typeResolver
	hint := r.TypeInfo
	switch hint.PreferBackend {
	case api.PreferResolver:
		if fb, ok := base.(fallbackAware); ok {
			// Composite is wired: hand out just the source-level leg.
			return fb.Fallback(), true
		}
		// Base is nil or a bare source-level resolver. A non-nil
		// non-composite resolver is itself a resolver, so it satisfies
		// the preference. A nil base means no backend → respect
		// Required: skip silently unless the rule opted into fall-
		// through with Required=true.
		if base != nil {
			return base, true
		}
		if hint.Required {
			return nil, true
		}
		return nil, false
	case api.PreferOracle:
		if _, ok := base.(fallbackAware); ok {
			// Composite present → oracle available.
			return base, true
		}
		// No composite wired → oracle isn't available. Honor Required:
		// true falls through to whatever base is (including nil);
		// false skips the rule silently.
		if hint.Required {
			return base, true
		}
		return nil, false
	default: // PreferAny
		return base, true
	}
}

func filePathOrEmpty(file *scanner.File) string {
	if file == nil {
		return ""
	}
	return file.Path
}

func javaContextFactsForFile(file *scanner.File) (*javafacts.JavaFileFacts, *javafacts.SourceIndex) {
	facts := javafacts.SourceFactsForFile(file)
	if facts == nil {
		return nil, nil
	}
	return facts, javafacts.SourceIndexForFiles([]*scanner.File{file})
}

// allRuleExcludes returns a snapshot of every rule's exclude globs. Used
// by the dispatcher's lazy SuppressionFilter build on the LSP/MCP path
// (where pipeline.Parse has not pre-populated file.Suppression). The
// snapshot is cheap — excludes rarely exceed a handful of entries — and
// avoids threading a new parameter through the api.Dispatcher API.
func allRuleExcludes() map[string][]string {
	return GetAllRuleExcludes()
}

// buildExcludedSet returns rule IDs that should be skipped for filePath
// based on YAML "excludes" glob patterns. Lookup is performed through the
// package-level exclusion map so config remains a single source of truth.
func (d *Dispatcher) buildExcludedSet(filePath string) map[string]bool {
	excluded := make(map[string]bool)
	check := func(r *api.Rule) {
		if excludes := GetRuleExcludes(r.ID); len(excludes) > 0 && IsFileExcluded(filePath, excludes) {
			excluded[r.ID] = true
		}
	}
	for _, bucket := range d.flatTypeRules {
		for _, r := range bucket {
			check(r)
		}
	}
	for _, r := range d.allNodeRules {
		check(r)
	}
	for _, r := range d.lineRules {
		check(r)
	}
	for _, r := range d.crossFileRules {
		check(r)
	}
	for _, r := range d.moduleAwareRules {
		check(r)
	}
	for _, r := range d.manifestRules {
		check(r)
	}
	for _, r := range d.resourceRules {
		check(r)
	}
	for _, r := range d.resourceSourceRules {
		check(r)
	}
	for _, r := range d.iconRules {
		check(r)
	}
	for _, r := range d.gradleRules {
		check(r)
	}
	for _, r := range d.aggregateRules {
		check(r)
	}
	return excluded
}

// collectAllRules returns every rule the dispatcher knows about, used
// when rebuilding the flat-type index after NodeTypeTable grows.
func (d *Dispatcher) collectAllRules() []*api.Rule {
	out := make([]*api.Rule, 0, len(d.nodeDispatchRules)+len(d.allNodeRules)+len(d.lineRules)+len(d.crossFileRules)+len(d.moduleAwareRules)+len(d.manifestRules)+len(d.resourceRules)+len(d.resourceSourceRules)+len(d.iconRules)+len(d.gradleRules)+len(d.aggregateRules))
	seen := make(map[*api.Rule]bool)
	addAll := func(rs []*api.Rule) {
		for _, r := range rs {
			if r == nil || seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
	}
	// Include node-dispatch rules first from the authoritative slice —
	// flatTypeRules may be empty at construction time because NodeTypeTable
	// is populated lazily as files are parsed.
	addAll(d.nodeDispatchRules)
	for _, bucket := range d.flatTypeRules {
		addAll(bucket)
	}
	addAll(d.allNodeRules)
	addAll(d.lineRules)
	addAll(d.crossFileRules)
	addAll(d.moduleAwareRules)
	addAll(d.manifestRules)
	addAll(d.resourceRules)
	addAll(d.resourceSourceRules)
	addAll(d.iconRules)
	addAll(d.gradleRules)
	addAll(d.aggregateRules)
	return out
}

// Stats returns per-family rule counts for logging.
func (d *Dispatcher) Stats() (dispatched, lineRules, crossFile, moduleAware int) {
	seen := make(map[*api.Rule]bool)
	for _, bucket := range d.flatTypeRules {
		for _, r := range bucket {
			if !seen[r] {
				seen[r] = true
				dispatched++
			}
		}
	}
	dispatched += len(d.allNodeRules)
	return dispatched, len(d.lineRules), len(d.crossFileRules), len(d.moduleAwareRules)
}

// ReportMissingCapabilities emits one diagnostic line per rule whose
// declared capabilities cannot be satisfied by the dispatcher's current
// wiring: NeedsResolver without a resolver, or an explicit KAA consumer when
// the caller indicates no oracle is configured.
//
// The log format is:
//
//	verbose: skipped rule <ID>: NeedsResolver declared but no resolver configured
//	verbose: skipped rule <ID>: NeedsOracle declared but no oracle configured
//	verbose: skipped rule <ID>: oracle metadata declared but no oracle configured
//
// A sync.Once guard inside the dispatcher ensures that even if multiple
// callers share the instance (CLI + LSP), the diagnostic is emitted at
// most once per run. Non-verbose paths pass a nil logger and stay silent.
func (d *Dispatcher) ReportMissingCapabilities(oracleAvailable bool, logger func(format string, args ...any)) {
	if logger == nil {
		return
	}
	d.reportOnce.Do(func() {
		missingResolver := d.typeResolver == nil
		missingOracle := !oracleAvailable
		if !missingResolver && !missingOracle {
			return
		}
		for _, r := range d.collectAllRules() {
			if r == nil {
				continue
			}
			if missingResolver && r.Needs.Has(api.NeedsResolver) {
				logger("verbose: skipped rule %s: NeedsResolver declared but no resolver configured\n", r.ID)
			}
			if missingOracle && RuleNeedsKotlinOracle(r) {
				reason := "oracle metadata declared"
				if r.Needs.HasAny(api.NeedsOracle) {
					reason = "NeedsOracle declared"
				}
				logger("verbose: skipped rule %s: %s but no oracle configured\n", r.ID, reason)
			}
		}
	})
}

// GradleRules returns the Gradle rules stored on this dispatcher. The
// main pipeline invokes these via RunGradle once per parsed Gradle file.
func (d *Dispatcher) GradleRules() []*api.Rule { return d.gradleRules }

// ManifestRules returns the manifest rules stored on this dispatcher.
func (d *Dispatcher) ManifestRules() []*api.Rule { return d.manifestRules }

// ResourceRules returns the resource rules stored on this dispatcher.
func (d *Dispatcher) ResourceRules() []*api.Rule { return d.resourceRules }

// ResourceSourceRules returns source AST rules that need the Android
// ResourceIndex. The Android phase invokes these after resource scanning.
func (d *Dispatcher) ResourceSourceRules() []*api.Rule { return d.resourceSourceRules }

// IconRules returns Android icon rules stored on this dispatcher.
func (d *Dispatcher) IconRules() []*api.Rule { return d.iconRules }

// RunGradle runs every registered Gradle rule against a single parsed
// Gradle build script. The file argument carries path/content with
// Language == LangGradle; cfg is the parsed BuildConfig. Findings are
// filtered by the per-rule YAML excludes and the Languages filter.
// Panics are recovered and surfaced via stderr to match Run().
func (d *Dispatcher) RunGradle(file *scanner.File, cfg *android.BuildConfig) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.gradleRules, func(ctx *api.Context) {
		ctx.GradlePath = file.Path
		ctx.GradleContent = string(file.Content)
		ctx.GradleConfig = cfg
	})
}

// RunManifest runs every registered manifest rule against a parsed
// AndroidManifest.xml. The api.Context.Manifest field is typed as
// interface{} to avoid an import cycle from the api package back into
// rules; the dispatcher itself takes a typed *manifest.Manifest.
func (d *Dispatcher) RunManifest(file *scanner.File, m *manifest.Manifest) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.manifestRules, func(ctx *api.Context) {
		ctx.Manifest = m
	})
}

// RunResource runs every registered resource rule against a merged
// ResourceIndex for a single res/ directory.
func (d *Dispatcher) RunResource(file *scanner.File, idx *android.ResourceIndex) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.resourceRules, func(ctx *api.Context) {
		ctx.ResourceIndex = idx
	})
}

// RunIcons runs every registered icon rule against an IconIndex for a
// single res/ directory.
func (d *Dispatcher) RunIcons(file *scanner.File, idx *android.IconIndex) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.iconRules, func(ctx *api.Context) {
		ctx.IconIndex = idx
	})
}

// RunResourceSource runs source AST rules that need the merged Android
// ResourceIndex. These rules are not part of the hot per-file dispatch phase
// because the resource index is assembled later in the Android phase.
func (d *Dispatcher) RunResourceSource(file *scanner.File, idx *android.ResourceIndex) scanner.FindingColumns {
	if file == nil || idx == nil || len(d.resourceSourceRules) == 0 {
		return scanner.FindingColumns{}
	}

	filter := file.Suppression
	if filter == nil {
		filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases())
		file.Suppression = filter
		file.SuppressionIdx = filter.Annotations()
	}

	excludedRules := d.buildExcludedSet(file.Path)
	for id := range d.excludedForLanguage(file.Language) {
		excludedRules[id] = true
	}

	rulesByType := make(map[uint16][]*api.Rule)
	for _, r := range d.resourceSourceRules {
		if excludedRules[r.ID] {
			continue
		}
		for _, nodeType := range r.NodeTypes {
			if typeID, ok := scanner.LookupFlatNodeType(nodeType); ok {
				rulesByType[typeID] = append(rulesByType[typeID], r)
			}
		}
	}
	if len(rulesByType) == 0 {
		return scanner.FindingColumns{}
	}

	stats := RunStats{RuleStatsByRule: make(map[string]RuleExecutionStat)}
	collector := scanner.NewFindingCollector(0)
	javaFileFacts, javaSourceIndex := javaContextFactsForFile(file)
	if tree := file.FlatTree; tree != nil && tree.Len() > 0 {
		// Resource-source rules are all node-typed (rulesByType only).
		// Iterate per-type via the posting list so we visit only the
		// indices for types with subscribers, skipping the bulk of the
		// tree.
		for typeID, handlers := range rulesByType {
			indices := tree.NodesOfType(typeID)
			for _, flatIdx := range indices {
				flatNode := tree.Node(flatIdx)
				for _, r := range handlers {
					if !ruleMatchesLexicalCalleeNames(r, file, flatIdx) {
						continue
					}
					resolver, ok := d.resolveForRule(r)
					if !ok {
						continue
					}
					t := time.Now()
					runWithRuleProfileLabel(r.ID, "resource-source", func() {
						safeCheckV2ResourceNodeColumnar(r, flatIdx, &flatNode, file, idx, collector, &stats, resolver, d.libraryFacts, javaFileFacts, javaSourceIndex, d.javaSemanticFacts, d.fileFacts)
					})
					stats.recordRule(r.ID, "resource-source", time.Since(t).Nanoseconds())
				}
			}
		}
	}

	columns := *collector.Columns()
	if columns.Len() > 0 {
		columns = columns.FilterRows(func(row int) bool {
			return !filter.IsSuppressed(columns.RuleAt(row), columns.RuleSetAt(row), columns.LineAt(row))
		})
	}
	for _, e := range stats.Errors {
		reporter().Warnf("%s\n", e.Error())
	}
	return columns
}

// runProjectRuleSet is the shared driver for RunGradle/RunManifest/RunResource.
// It applies config excludes + language filtering, invokes each rule's
// Check with a fresh Context populated by the supplied closure, stamps
// the base confidence, and returns aggregated findings in columnar form.
func (d *Dispatcher) runProjectRuleSet(file *scanner.File, ruleSet []*api.Rule, populate func(*api.Context)) scanner.FindingColumns {
	if file == nil {
		return scanner.FindingColumns{}
	}
	excluded := d.buildExcludedSet(file.Path)
	langExcluded := d.excludedForLanguage(file.Language)
	collector := scanner.NewFindingCollector(0)
	for _, r := range ruleSet {
		if excluded[r.ID] || langExcluded[r.ID] {
			continue
		}
		cols := d.runProjectRule(r, file, populate)
		collector.AppendColumns(&cols)
	}
	return *collector.Columns()
}

// runProjectRule invokes a project-level rule's Check function with a
// freshly constructed Context, recovering from panics. Returns findings
// in columnar form.
func (d *Dispatcher) runProjectRule(r *api.Rule, file *scanner.File, populate func(*api.Context)) (cols scanner.FindingColumns) {
	defer func() {
		if rec := recover(); rec != nil {
			reporter().Warnf("krit: panic in rule %s on %s: %v\n", r.ID, file.Path, rec)
			cols = scanner.FindingColumns{}
		}
	}()
	collector := scanner.NewFindingCollector(0)
	javaFileFacts, javaSourceIndex := javaContextFactsForFile(file)
	ctx := &api.Context{File: file, Rule: r, DefaultConfidence: api.ConfidenceMedium, Collector: collector, LibraryFacts: d.libraryFacts, JavaFacts: javaFileFacts, JavaSourceIndex: javaSourceIndex, JavaSemanticFacts: d.javaSemanticFacts, Facts: d.fileFacts}
	if r.Needs.Has(api.NeedsResolver) {
		if resolver, ok := d.resolveForRule(r); ok {
			ctx.Resolver = resolver
		}
	}
	if populate != nil {
		populate(ctx)
	}
	r.Check(ctx)
	return *collector.Columns()
}

func safeCheckV2ResourceNodeColumnar(r *api.Rule, idx uint32, node *scanner.FlatNode, file *scanner.File, resourceIndex *android.ResourceIndex, collector *scanner.FindingCollector, stats *RunStats, typeResolver typeinfer.TypeResolver, libraryFacts *librarymodel.Facts, javaFileFacts *javafacts.JavaFileFacts, javaSourceIndex *javafacts.SourceIndex, javaSemanticFacts *javafacts.Facts, fileFacts *filefacts.Cache) {
	defer func() {
		if rec := recover(); rec != nil {
			line := 0
			if node != nil {
				line = int(node.StartRow) + 1
			} else if file != nil {
				line = file.FlatRow(idx) + 1
			}
			stats.Errors = append(stats.Errors, DispatchError{
				RuleName:   r.ID,
				FilePath:   filePathOrEmpty(file),
				Line:       line,
				PanicValue: rec,
			})
		}
	}()
	ctx := &api.Context{
		File:              file,
		Node:              node,
		Idx:               idx,
		Rule:              r,
		DefaultConfidence: api.ConfidenceVeryHigh,
		ResourceIndex:     resourceIndex,
		Collector:         collector,
		LibraryFacts:      libraryFacts,
		JavaFacts:         javaFileFacts,
		JavaSourceIndex:   javaSourceIndex,
		JavaSemanticFacts: javaSemanticFacts,
		Facts:             fileFacts,
	}
	if r.Needs.Has(api.NeedsResolver) {
		ctx.Resolver = typeResolver
	}
	r.Check(ctx)
}

// excludedForLanguage returns the set of rule IDs that do NOT apply to
// the given file language. The result is cached per language — rules
// are static after construction, so we amortize the collectAllRules +
// filter scan across every file of that language.
func (d *Dispatcher) excludedForLanguage(lang scanner.Language) map[string]bool {
	if cached, ok := d.languageExcluded.Load(lang); ok {
		return cached.(map[string]bool)
	}
	m := make(map[string]bool)
	for _, r := range d.collectAllRules() {
		if !api.RuleAppliesToLanguage(r, lang) {
			m[r.ID] = true
		}
	}
	if actual, loaded := d.languageExcluded.LoadOrStore(lang, m); loaded {
		return actual.(map[string]bool)
	}
	return m
}

// CrossFileRules returns the cross-file rules stored on this dispatcher.
// The main pipeline invokes these after building the code index.
func (d *Dispatcher) CrossFileRules() []*api.Rule {
	return d.crossFileRules
}

// ModuleAwareRules returns the module-aware rules stored on this dispatcher.
// The main pipeline invokes these after building the per-module index.
func (d *Dispatcher) ModuleAwareRules() []*api.Rule {
	return d.moduleAwareRules
}
