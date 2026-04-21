package rules

// v2dispatcher.go provides V2Dispatcher — a rule dispatcher that
// operates directly on *v2.Rule values instead of the six v1 interface
// families (flat-dispatch rule, line rule, aggregate rule, cross-file rule,
// module-aware rule, legacy Rule). The public API intentionally mirrors
// the v1 Dispatcher so it can be swapped in at call sites (main.go,
// LSP, MCP) without rewriting pipeline code.
//
// This file is purely additive — the existing v1 Dispatcher in
// dispatch.go is left untouched so comparison tests can exercise both
// side-by-side.

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// V2Dispatcher runs per-file rule execution directly against v2.Rule
// values. It classifies rules once in the constructor and keeps them
// in parallel slices indexed by FlatNode.Type for O(1) node dispatch,
// matching the v1 Dispatcher's flat-type index strategy.
//
// Cross-file and module-aware rules are stored but NOT invoked inside
// Run — they require indexes that are only available once all files
// have been parsed. The main pipeline is expected to iterate
// CrossFileRules()/ModuleAwareRules() after the per-file pass.
type V2Dispatcher struct {
	// flatTypeRules is indexed by FlatNode.Type for O(1) flat dispatch.
	flatTypeRules [][]*v2.Rule
	// allNodeRules is the set of node rules that declare nil NodeTypes
	// — they receive every node during the walk.
	allNodeRules []*v2.Rule
	// lineRules run on file.Lines after the AST walk completes.
	lineRules []*v2.Rule
	// legacyRules have nil NodeTypes and !NeedsLinePass — in v2 these
	// are treated as "no hot-path dispatch" rules whose Check function
	// is invoked once per file with a bare Context.File. Kept separate
	// from allNodeRules so they are not called per-node.
	legacyRules []*v2.Rule
	// crossFileRules and moduleAwareRules are exposed to callers via
	// CrossFileRules()/ModuleAwareRules() and not invoked from Run.
	crossFileRules   []*v2.Rule
	moduleAwareRules []*v2.Rule
	// manifestRules / resourceRules / gradleRules / aggregateRules
	// are also NOT invoked from Run — they require project-level
	// context that is assembled by the main pipeline. They are stored
	// here so ensureFlatTypeIndex and collectAllRules can see them for
	// re-index purposes.
	manifestRules  []*v2.Rule
	resourceRules  []*v2.Rule
	gradleRules    []*v2.Rule
	aggregateRules []*v2.Rule
	// nodeDispatchRules are rules with non-empty NodeTypes. They live in
	// flatTypeRules indexed by FlatNode.Type, but scanner.NodeTypeTable
	// is populated lazily as files are parsed — so at construction time
	// many of these rules' node types may not yet have IDs. We keep the
	// full list here so ensureFlatTypeIndex can re-index them once the
	// table has grown after some files have been parsed.
	nodeDispatchRules []*v2.Rule

	typeResolver typeinfer.TypeResolver

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

// NewV2Dispatcher constructs a V2Dispatcher from the supplied v2 rules.
// An optional TypeResolver is wired through to any rule declaring
// NeedsResolver via that rule's SetResolverHook.
func NewV2Dispatcher(rules []*v2.Rule, resolver ...typeinfer.TypeResolver) *V2Dispatcher {
	d := &V2Dispatcher{}
	if len(resolver) > 0 && resolver[0] != nil {
		d.typeResolver = resolver[0]
	}

	// Classify each rule by its declared Needs + NodeTypes.
	//
	// Classification order:
	//   1. NeedsCrossFile  → crossFileRules (deferred to pipeline)
	//   2. NeedsModuleIndex → moduleAwareRules (deferred to pipeline)
	//   3. NeedsLinePass   → lineRules (per-file, line-pass)
	//   4. Non-empty NodeTypes → flatTypeRules / allNodeRules
	//   5. Anything else    → legacyRules
	for _, r := range rules {
		if r == nil {
			continue
		}
		switch {
		case r.Needs.Has(v2.NeedsCrossFile):
			d.crossFileRules = append(d.crossFileRules, r)
		case r.Needs.Has(v2.NeedsModuleIndex):
			d.moduleAwareRules = append(d.moduleAwareRules, r)
		case r.Needs.Has(v2.NeedsManifest):
			d.manifestRules = append(d.manifestRules, r)
		case r.Needs.Has(v2.NeedsResources):
			d.resourceRules = append(d.resourceRules, r)
		case r.Needs.Has(v2.NeedsGradle):
			d.gradleRules = append(d.gradleRules, r)
		case r.Needs.Has(v2.NeedsAggregate):
			d.aggregateRules = append(d.aggregateRules, r)
		case r.Needs.Has(v2.NeedsLinePass):
			d.lineRules = append(d.lineRules, r)
		case len(r.NodeTypes) > 0:
			// Indexed by flat type ID in buildFlatTypeIndex. Also
			// stored in nodeDispatchRules so ensureFlatTypeIndex can
			// re-index them later once NodeTypeTable has grown.
			d.nodeDispatchRules = append(d.nodeDispatchRules, r)
		case r.NodeTypes == nil:
			// nil NodeTypes + no other flag → treat as a "receive every
			// node" rule. This matches v1 flat-dispatch rule semantics
			// where NodeTypes()==nil means "all nodes".
			//
			// However, a rule that explicitly returns an empty slice
			// (len==0, non-nil) was routed to legacyRules above. This
			// preserves the intent that empty-but-non-nil means "opt
			// out of node dispatch entirely".
			d.allNodeRules = append(d.allNodeRules, r)
		default:
			d.legacyRules = append(d.legacyRules, r)
		}
	}

	// Second pass: populate flatTypeRules for rules with explicit node types.
	for _, r := range rules {
		if r == nil {
			continue
		}
		if r.Needs.Has(v2.NeedsCrossFile) || r.Needs.Has(v2.NeedsModuleIndex) || r.Needs.Has(v2.NeedsLinePass) {
			continue
		}
		if len(r.NodeTypes) == 0 {
			continue
		}
		// Placement into flatTypeRules happens inside buildFlatTypeIndex so
		// resizing NodeTypeTable (lazy intern of node types) is handled
		// uniformly with the v1 Dispatcher.
	}

	d.buildFlatTypeIndex(rules)
	return d
}

// buildFlatTypeIndex populates flatTypeRules from the supplied rules.
// Called from the constructor and from ensureFlatTypeIndex when the
// NodeTypeTable has grown since the last build.
func (d *V2Dispatcher) buildFlatTypeIndex(rules []*v2.Rule) {
	d.mu.Lock()
	defer d.mu.Unlock()

	size := scanner.NodeTypeTableSize()
	if size <= 0 {
		size = 1
	}
	d.flatTypeRules = make([][]*v2.Rule, size)

	ensureSize := func(need int) {
		if need <= len(d.flatTypeRules) {
			return
		}
		grown := make([][]*v2.Rule, need)
		copy(grown, d.flatTypeRules)
		d.flatTypeRules = grown
	}

	for _, r := range rules {
		if r == nil {
			continue
		}
		if r.Needs.Has(v2.NeedsCrossFile) || r.Needs.Has(v2.NeedsModuleIndex) || r.Needs.Has(v2.NeedsLinePass) {
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

// ensureFlatTypeIndex returns the flat-type rule index, rebuilding it
// if NodeTypeTable has grown since the last build. This mirrors the
// behavior of the v1 dispatcher's ensureFlatTypeIndex.
func (d *V2Dispatcher) ensureFlatTypeIndex(rules []*v2.Rule) [][]*v2.Rule {
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
// Rule panics are logged to stderr, matching v1 Dispatcher.Run behavior.
func (d *V2Dispatcher) Run(file *scanner.File) scanner.FindingColumns {
	columns, stats := d.RunColumnsWithStats(file)
	for _, e := range stats.Errors {
		fmt.Fprintln(os.Stderr, e.Error())
	}
	return columns
}

// RunWithStats executes all per-file rules on a file and returns both
// findings in columnar form and coarse timing for each execution bucket.
func (d *V2Dispatcher) RunWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	return d.RunColumnsWithStats(file)
}


// RunColumnsWithStats runs all per-file rules emitting findings directly
// into a FindingCollector, bypassing the intermediate []scanner.Finding
// accumulation. Rules emit via ctx.Emit which routes straight to the
// collector; the result is returned as columnar data.
func (d *V2Dispatcher) RunColumnsWithStats(file *scanner.File) (scanner.FindingColumns, RunStats) {
	stats := RunStats{
		DispatchRuleNsByRule: make(map[string]int64),
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
		filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "")
		file.Suppression = filter
		file.SuppressionIdx = filter.Annotations()
	}
	stats.SuppressionIndexMs += time.Since(start).Milliseconds()

	excludedRules := d.buildExcludedSet(file.Path)
	for id := range d.excludedForLanguage(file.Language) {
		excludedRules[id] = true
	}

	collector := scanner.NewFindingCollector(0)

	// Single-pass AST walk.
	start = time.Now()
	if file.FlatTree != nil && len(file.FlatTree.Nodes) > 0 {
		for idx := range file.FlatTree.Nodes {
			flatIdx := uint32(idx)
			flatNode := file.FlatTree.Nodes[idx]
			if int(flatNode.Type) < len(flatTypeRules) {
				if handlers := flatTypeRules[flatNode.Type]; handlers != nil {
					for _, r := range handlers {
						if excludedRules[r.ID] {
							continue
						}
						resolver, ok := d.resolveForRule(r)
						if !ok {
							continue
						}
						t := time.Now()
						safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats, resolver)
						elapsed := time.Since(t).Nanoseconds()
						stats.DispatchRuleNs += elapsed
						stats.DispatchRuleNsByRule[r.ID] += elapsed
					}
				}
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
				safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats, resolver)
				elapsed := time.Since(t).Nanoseconds()
				stats.DispatchRuleNs += elapsed
				stats.DispatchRuleNsByRule[r.ID] += elapsed
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
			ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.75, Collector: collector}
			if r.Needs.Has(v2.NeedsResolver) {
				ctx.Resolver = resolver
			}
			r.Check(ctx)
		}()
	}
	stats.LineRuleMs += time.Since(start).Milliseconds()

	// Legacy / catch-all rules.
	start = time.Now()
	for _, r := range d.legacyRules {
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
			ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.50, Collector: collector}
			if r.Needs.Has(v2.NeedsResolver) {
				ctx.Resolver = resolver
			}
			r.Check(ctx)
		}()
	}
	stats.LegacyRuleMs += time.Since(start).Milliseconds()

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
		fmt.Fprintln(os.Stderr, e.Error())
	}
	return columns, stats
}

// safeCheckV2NodeColumnar invokes a rule with a Collector attached so
// ctx.Emit routes findings directly into columnar storage.
func safeCheckV2NodeColumnar(r *v2.Rule, idx uint32, node *scanner.FlatNode, file *scanner.File, collector *scanner.FindingCollector, stats *RunStats, typeResolver typeinfer.TypeResolver) {
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
	ctx := &v2.Context{
		File:              file,
		Node:              node,
		Idx:               idx,
		Rule:              r,
		DefaultConfidence: 0.95,
		Collector:         collector,
	}
	if r.Needs.Has(v2.NeedsResolver) {
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
// hint behaviour: whatever the caller handed NewV2Dispatcher is passed
// through untouched. Rules that don't declare NeedsResolver never ask
// this helper — their ctx.Resolver stays nil as before.
func (d *V2Dispatcher) resolveForRule(r *v2.Rule) (typeinfer.TypeResolver, bool) {
	base := d.typeResolver
	hint := r.TypeInfo
	switch hint.PreferBackend {
	case v2.PreferResolver:
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
	case v2.PreferOracle:
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

// allRuleExcludes returns a snapshot of every rule's exclude globs. Used
// by the dispatcher's lazy SuppressionFilter build on the LSP/MCP path
// (where pipeline.Parse has not pre-populated file.Suppression). The
// snapshot is cheap — excludes rarely exceed a handful of entries — and
// avoids threading a new parameter through the v2.Dispatcher API.
func allRuleExcludes() map[string][]string {
	return GetAllRuleExcludes()
}

// buildExcludedSet returns rule IDs that should be skipped for filePath
// based on YAML "excludes" glob patterns. Lookup is performed via the
// v1 GetRuleExcludes helper so the exclusion config remains a single
// source of truth across both dispatchers.
func (d *V2Dispatcher) buildExcludedSet(filePath string) map[string]bool {
	excluded := make(map[string]bool)
	check := func(r *v2.Rule) {
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
	for _, r := range d.legacyRules {
		check(r)
	}
	for _, r := range d.crossFileRules {
		check(r)
	}
	for _, r := range d.moduleAwareRules {
		check(r)
	}
	return excluded
}

// collectAllRules returns every rule the dispatcher knows about, used
// when rebuilding the flat-type index after NodeTypeTable grows.
func (d *V2Dispatcher) collectAllRules() []*v2.Rule {
	out := make([]*v2.Rule, 0, len(d.nodeDispatchRules)+len(d.allNodeRules)+len(d.lineRules)+len(d.legacyRules)+len(d.crossFileRules)+len(d.moduleAwareRules))
	seen := make(map[*v2.Rule]bool)
	addAll := func(rs []*v2.Rule) {
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
	addAll(d.legacyRules)
	addAll(d.crossFileRules)
	addAll(d.moduleAwareRules)
	addAll(d.manifestRules)
	addAll(d.resourceRules)
	addAll(d.gradleRules)
	addAll(d.aggregateRules)
	return out
}

// Stats returns the same count tuple as v1 Dispatcher.Stats, so
// downstream loggers can consume either dispatcher without branching.
// The "aggregate" count is always 0 because v2 has no separate
// aggregate family — rules that need whole-file state use
// NeedsParsedFiles or simply aggregate inside a per-file closure.
func (d *V2Dispatcher) Stats() (dispatched, aggregate, lineRules, crossFile, moduleAware, legacy int) {
	seen := make(map[*v2.Rule]bool)
	for _, bucket := range d.flatTypeRules {
		for _, r := range bucket {
			if !seen[r] {
				seen[r] = true
				dispatched++
			}
		}
	}
	dispatched += len(d.allNodeRules)
	return dispatched, 0, len(d.lineRules), len(d.crossFileRules), len(d.moduleAwareRules), len(d.legacyRules)
}

// ReportMissingCapabilities emits one diagnostic line per rule whose
// declared capabilities cannot be satisfied by the dispatcher's current
// wiring: NeedsResolver without a resolver, or NeedsOracle when the
// caller indicates no oracle is configured.
//
// The log format is:
//
//	verbose: skipped rule <ID>: NeedsResolver declared but no resolver configured
//	verbose: skipped rule <ID>: NeedsOracle declared but no oracle configured
//
// A sync.Once guard inside the dispatcher ensures that even if multiple
// callers share the instance (CLI + LSP), the diagnostic is emitted at
// most once per run. Non-verbose paths pass a nil logger and stay silent.
func (d *V2Dispatcher) ReportMissingCapabilities(oracleAvailable bool, logger func(format string, args ...any)) {
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
			if missingResolver && r.Needs.Has(v2.NeedsResolver) {
				logger("verbose: skipped rule %s: NeedsResolver declared but no resolver configured\n", r.ID)
			}
			if missingOracle && r.Needs.Has(v2.NeedsOracle) {
				logger("verbose: skipped rule %s: NeedsOracle declared but no oracle configured\n", r.ID)
			}
		}
	})
}

// GradleRules returns the Gradle rules stored on this dispatcher. The
// main pipeline invokes these via RunGradle once per parsed Gradle file.
func (d *V2Dispatcher) GradleRules() []*v2.Rule { return d.gradleRules }

// ManifestRules returns the manifest rules stored on this dispatcher.
func (d *V2Dispatcher) ManifestRules() []*v2.Rule { return d.manifestRules }

// ResourceRules returns the resource rules stored on this dispatcher.
func (d *V2Dispatcher) ResourceRules() []*v2.Rule { return d.resourceRules }

// RunGradle runs every registered Gradle rule against a single parsed
// Gradle build script. The file argument carries path/content with
// Language == LangGradle; cfg is the parsed BuildConfig. Findings are
// filtered by the per-rule YAML excludes and the Languages filter.
// Panics are recovered and surfaced via stderr to match Run().
func (d *V2Dispatcher) RunGradle(file *scanner.File, cfg *android.BuildConfig) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.gradleRules, func(ctx *v2.Context) {
		ctx.GradlePath = file.Path
		ctx.GradleContent = string(file.Content)
		ctx.GradleConfig = cfg
	})
}

// RunManifest runs every registered manifest rule against a parsed
// AndroidManifest.xml. manifest is typed as interface{} to avoid an
// import cycle — callers in the rules package pass *rules.Manifest; the
// underlying rule Check functions type-assert through ctx.Manifest.
func (d *V2Dispatcher) RunManifest(file *scanner.File, manifest interface{}) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.manifestRules, func(ctx *v2.Context) {
		ctx.Manifest = manifest
	})
}

// RunResource runs every registered resource rule against a merged
// ResourceIndex for a single res/ directory.
func (d *V2Dispatcher) RunResource(file *scanner.File, idx *android.ResourceIndex) scanner.FindingColumns {
	return d.runProjectRuleSet(file, d.resourceRules, func(ctx *v2.Context) {
		ctx.ResourceIndex = idx
	})
}

// runProjectRuleSet is the shared driver for RunGradle/RunManifest/RunResource.
// It applies config excludes + language filtering, invokes each rule's
// Check with a fresh Context populated by the supplied closure, stamps
// the base confidence, and returns aggregated findings in columnar form.
func (d *V2Dispatcher) runProjectRuleSet(file *scanner.File, ruleSet []*v2.Rule, populate func(*v2.Context)) scanner.FindingColumns {
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
func (d *V2Dispatcher) runProjectRule(r *v2.Rule, file *scanner.File, populate func(*v2.Context)) (cols scanner.FindingColumns) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "krit: panic in rule %s on %s: %v\n", r.ID, file.Path, rec)
			cols = scanner.FindingColumns{}
		}
	}()
	collector := scanner.NewFindingCollector(0)
	ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.75, Collector: collector}
	if r.Needs.Has(v2.NeedsResolver) {
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

// excludedForLanguage returns the set of rule IDs that do NOT apply to
// the given file language. The result is cached per language — rules
// are static after construction, so we amortize the collectAllRules +
// filter scan across every file of that language.
func (d *V2Dispatcher) excludedForLanguage(lang scanner.Language) map[string]bool {
	if cached, ok := d.languageExcluded.Load(lang); ok {
		return cached.(map[string]bool)
	}
	m := make(map[string]bool)
	for _, r := range d.collectAllRules() {
		if !v2.RuleAppliesToLanguage(r, lang) {
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
func (d *V2Dispatcher) CrossFileRules() []*v2.Rule {
	return d.crossFileRules
}

// ModuleAwareRules returns the module-aware rules stored on this dispatcher.
// The main pipeline invokes these after building the per-module index.
func (d *V2Dispatcher) ModuleAwareRules() []*v2.Rule {
	return d.moduleAwareRules
}

