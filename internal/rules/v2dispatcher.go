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
		// Fire SetResolverHook for rules that advertise NeedsResolver.
		if d.typeResolver != nil && r.Needs.Has(v2.NeedsResolver) && r.SetResolverHook != nil {
			r.SetResolverHook(d.typeResolver)
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

	size := len(scanner.NodeTypeTable)
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

	needSize := len(scanner.NodeTypeTable)
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

// Run executes all per-file rules and returns findings.
// Rule panics are logged to stderr, matching v1 Dispatcher.Run behavior.
func (d *V2Dispatcher) Run(file *scanner.File) []scanner.Finding {
	findings, stats := d.RunWithStats(file)
	for _, e := range stats.Errors {
		fmt.Fprintln(os.Stderr, e.Error())
	}
	return findings
}

// RunWithStats executes all per-file rules on a file and returns both
// findings and coarse timing for each execution bucket. The stats
// layout matches the v1 Dispatcher.RunStats.
func (d *V2Dispatcher) RunWithStats(file *scanner.File) ([]scanner.Finding, RunStats) {
	stats := RunStats{
		DispatchRuleNsByRule: make(map[string]int64),
	}
	if file == nil {
		return nil, stats
	}

	// Rebuild the flat index if NodeTypeTable has grown since construction.
	// We need the rule list again — gather it from our internal slices.
	flatTypeRules := d.ensureFlatTypeIndex(d.collectAllRules())

	// Build suppression index from @Suppress annotations (same semantics as v1).
	start := time.Now()
	suppressIndex := scanner.BuildSuppressionIndexFlat(file.FlatTree, file.Content)
	stats.SuppressionIndexMs += time.Since(start).Milliseconds()

	// Determine per-file rule exclusions once.
	excludedRules := d.buildExcludedSet(file.Path)

	// Fold in the per-language excluded set so the hot-path
	// walkDispatch/line/legacy loops only need one map lookup. The
	// language-to-excluded-IDs map is cached on the dispatcher and
	// reused across files — rules are static after construction.
	for id := range d.excludedForLanguage(file.Language) {
		excludedRules[id] = true
	}

	var findings []scanner.Finding

	// Single-pass AST walk for all node-dispatch rules.
	start = time.Now()
	d.walkDispatch(file, &findings, excludedRules, &stats, flatTypeRules)
	stats.DispatchWalkMs += time.Since(start).Milliseconds()

	// Line-pass rules.
	start = time.Now()
	for _, r := range d.lineRules {
		if excludedRules[r.ID] {
			continue
		}
		results := d.runLineRule(r, file, &stats)
		setV2RuleConfidence(results, r, 0.75)
		findings = append(findings, results...)
	}
	stats.LineRuleMs += time.Since(start).Milliseconds()

	// Legacy / catch-all rules.
	start = time.Now()
	for _, r := range d.legacyRules {
		if excludedRules[r.ID] {
			continue
		}
		results := d.runLegacyRule(r, file, &stats)
		setV2RuleConfidence(results, r, 0.50)
		findings = append(findings, results...)
	}
	stats.LegacyRuleMs += time.Since(start).Milliseconds()

	// Suppression filter (same semantics as v1 Dispatcher).
	start = time.Now()
	if len(findings) > 0 {
		filtered := findings[:0]
		for _, f := range findings {
			byteOffset := 0
			if f.Line > 0 {
				byteOffset = file.LineOffset(f.Line - 1)
			}
			if !suppressIndex.IsSuppressed(byteOffset, f.Rule, f.RuleSet) {
				filtered = append(filtered, f)
			}
		}
		findings = filtered
	}
	stats.SuppressionFilterMs += time.Since(start).Milliseconds()

	return findings, stats
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

	start := time.Now()
	suppressIndex := scanner.BuildSuppressionIndexFlat(file.FlatTree, file.Content)
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
						t := time.Now()
						safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats)
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
				t := time.Now()
				safeCheckV2NodeColumnar(r, flatIdx, &flatNode, file, collector, &stats)
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
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					stats.Errors = append(stats.Errors, DispatchError{RuleName: r.ID, FilePath: filePathOrEmpty(file), PanicValue: rec})
				}
			}()
			ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.75, Collector: collector}
			r.Check(ctx)
			if len(ctx.Findings) > 0 {
				stampV2Findings(ctx.Findings, r, file)
				collector.AppendAll(ctx.Findings)
			}
		}()
	}
	stats.LineRuleMs += time.Since(start).Milliseconds()

	// Legacy / catch-all rules.
	start = time.Now()
	for _, r := range d.legacyRules {
		if excludedRules[r.ID] {
			continue
		}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					stats.Errors = append(stats.Errors, DispatchError{RuleName: r.ID, FilePath: filePathOrEmpty(file), PanicValue: rec})
				}
			}()
			ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.50, Collector: collector}
			r.Check(ctx)
			if len(ctx.Findings) > 0 {
				stampV2Findings(ctx.Findings, r, file)
				collector.AppendAll(ctx.Findings)
			}
		}()
	}
	stats.LegacyRuleMs += time.Since(start).Milliseconds()

	columns := *collector.Columns()

	// Suppression filter.
	start = time.Now()
	if columns.Len() > 0 {
		filtered := columns.FilterRows(func(row int) bool {
			line := columns.LineAt(row)
			byteOffset := 0
			if line > 0 {
				byteOffset = file.LineOffset(line - 1)
			}
			return !suppressIndex.IsSuppressed(byteOffset, columns.RuleAt(row), columns.RuleSetAt(row))
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
// ctx.Emit routes findings directly into columnar storage. Any residual
// ctx.Findings mutations (rules not yet using ctx.Emit) are also drained.
func safeCheckV2NodeColumnar(r *v2.Rule, idx uint32, node *scanner.FlatNode, file *scanner.File, collector *scanner.FindingCollector, stats *RunStats) {
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
	r.Check(ctx)
	if len(ctx.Findings) > 0 {
		stampV2Findings(ctx.Findings, r, file)
		collector.AppendAll(ctx.Findings)
	}
}

// walkDispatch performs a single pass over the flat tree, invoking each
// node rule on every matching node with panic recovery and per-rule
// timing.
func (d *V2Dispatcher) walkDispatch(file *scanner.File, findings *[]scanner.Finding, excludedRules map[string]bool, stats *RunStats, flatTypeRules [][]*v2.Rule) {
	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		return
	}

	for idx := range file.FlatTree.Nodes {
		flatIdx := uint32(idx)
		flatNode := file.FlatTree.Nodes[idx]

		// Type-indexed rules.
		if int(flatNode.Type) < len(flatTypeRules) {
			if handlers := flatTypeRules[flatNode.Type]; handlers != nil {
				for _, r := range handlers {
					if excludedRules[r.ID] {
						continue
					}
					start := time.Now()
					results := safeCheckV2Node(r, flatIdx, &flatNode, file, stats)
					elapsed := time.Since(start).Nanoseconds()
					stats.DispatchRuleNs += elapsed
					stats.DispatchRuleNsByRule[r.ID] += elapsed
					setV2RuleConfidence(results, r, 0.95)
					*findings = append(*findings, results...)
				}
			}
		}

		// Rules that want every node.
		for _, r := range d.allNodeRules {
			if excludedRules[r.ID] {
				continue
			}
			start := time.Now()
			results := safeCheckV2Node(r, flatIdx, &flatNode, file, stats)
			elapsed := time.Since(start).Nanoseconds()
			stats.DispatchRuleNs += elapsed
			stats.DispatchRuleNsByRule[r.ID] += elapsed
			setV2RuleConfidence(results, r, 0.95)
			*findings = append(*findings, results...)
		}
	}
}

// safeCheckV2Node invokes the v2 rule's Check function with a freshly
// constructed Context and recovers from panics the same way
// safeCheckFlatNode does for v1.
func safeCheckV2Node(r *v2.Rule, idx uint32, node *scanner.FlatNode, file *scanner.File, stats *RunStats) (results []scanner.Finding) {
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
			results = nil
		}
	}()

	ctx := &v2.Context{
		File:              file,
		Node:              node,
		Idx:               idx,
		Rule:              r,
		DefaultConfidence: 0.95,
	}
	r.Check(ctx)
	stampV2Findings(ctx.Findings, r, file)
	return ctx.Findings
}

// runLineRule invokes a line-pass rule's Check function once with an
// empty Node/Idx. Line rules scan file.Lines themselves via ctx.File.
func (d *V2Dispatcher) runLineRule(r *v2.Rule, file *scanner.File, stats *RunStats) (results []scanner.Finding) {
	defer func() {
		if rec := recover(); rec != nil {
			stats.Errors = append(stats.Errors, DispatchError{
				RuleName:   r.ID,
				FilePath:   filePathOrEmpty(file),
				Line:       0,
				PanicValue: rec,
			})
			results = nil
		}
	}()
	ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.75}
	r.Check(ctx)
	stampV2Findings(ctx.Findings, r, file)
	return ctx.Findings
}

// runLegacyRule invokes a catch-all rule once per file. Legacy rules
// in the v2 world are rules that are neither node-indexed nor
// line-pass — they operate on ctx.File directly.
func (d *V2Dispatcher) runLegacyRule(r *v2.Rule, file *scanner.File, stats *RunStats) (results []scanner.Finding) {
	defer func() {
		if rec := recover(); rec != nil {
			stats.Errors = append(stats.Errors, DispatchError{
				RuleName:   r.ID,
				FilePath:   filePathOrEmpty(file),
				Line:       0,
				PanicValue: rec,
			})
			results = nil
		}
	}()
	ctx := &v2.Context{File: file, Rule: r, DefaultConfidence: 0.50}
	r.Check(ctx)
	stampV2Findings(ctx.Findings, r, file)
	return ctx.Findings
}

// stampV2Findings fills in Rule/RuleSet/Severity/File fields that a rule
// forgot to populate. Mirrors v2.v1compat.stampFindings but operates on
// a slice header rather than needing a Context.
func stampV2Findings(findings []scanner.Finding, r *v2.Rule, file *scanner.File) {
	path := filePathOrEmpty(file)
	for i := range findings {
		if findings[i].Rule == "" {
			findings[i].Rule = r.ID
		}
		if findings[i].RuleSet == "" {
			findings[i].RuleSet = r.Category
		}
		if findings[i].Severity == "" {
			findings[i].Severity = string(r.Sev)
		}
		if findings[i].File == "" && path != "" {
			findings[i].File = path
		}
	}
}

func filePathOrEmpty(file *scanner.File) string {
	if file == nil {
		return ""
	}
	return file.Path
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
func (d *V2Dispatcher) RunGradle(file *scanner.File, cfg *android.BuildConfig) []scanner.Finding {
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
func (d *V2Dispatcher) RunManifest(file *scanner.File, manifest interface{}) []scanner.Finding {
	return d.runProjectRuleSet(file, d.manifestRules, func(ctx *v2.Context) {
		ctx.Manifest = manifest
	})
}

// RunResource runs every registered resource rule against a merged
// ResourceIndex for a single res/ directory.
func (d *V2Dispatcher) RunResource(file *scanner.File, idx *android.ResourceIndex) []scanner.Finding {
	return d.runProjectRuleSet(file, d.resourceRules, func(ctx *v2.Context) {
		ctx.ResourceIndex = idx
	})
}

// runProjectRuleSet is the shared driver for RunGradle/RunManifest/RunResource.
// It applies config excludes + language filtering, invokes each rule's
// Check with a fresh Context populated by the supplied closure, stamps
// the base confidence, and returns aggregated findings.
func (d *V2Dispatcher) runProjectRuleSet(file *scanner.File, ruleSet []*v2.Rule, populate func(*v2.Context)) []scanner.Finding {
	if file == nil {
		return nil
	}
	excluded := d.buildExcludedSet(file.Path)
	langExcluded := d.excludedForLanguage(file.Language)
	var findings []scanner.Finding
	for _, r := range ruleSet {
		if excluded[r.ID] || langExcluded[r.ID] {
			continue
		}
		results := d.runProjectRule(r, file, populate)
		setV2RuleConfidence(results, r, 0.75)
		findings = append(findings, results...)
	}
	return findings
}

// runProjectRule invokes a project-level rule's Check function with a
// freshly constructed Context, recovering from panics the same way
// safeCheckV2Node does for per-node dispatch.
func (d *V2Dispatcher) runProjectRule(r *v2.Rule, file *scanner.File, populate func(*v2.Context)) (results []scanner.Finding) {
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "krit: panic in rule %s on %s: %v\n", r.ID, file.Path, rec)
			results = nil
		}
	}()
	ctx := &v2.Context{File: file}
	if populate != nil {
		populate(ctx)
	}
	r.Check(ctx)
	stampV2Findings(ctx.Findings, r, file)
	return ctx.Findings
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

// setV2RuleConfidence applies a rule's declared base confidence to any
// findings that haven't set their own, falling back to the family
// default. Mirrors setRuleConfidence for v1 rules but reads the Rule
// struct's Confidence field directly.
func setV2RuleConfidence(findings []scanner.Finding, r *v2.Rule, fallback float64) {
	ApplyV2Confidence(findings, r, fallback)
}

// ApplyV2Confidence is the exported form of setV2RuleConfidence for call
// sites outside the dispatcher (cmd/krit post-per-file passes). Per-finding
// overrides still win — the rule's base confidence is only applied to
// findings whose Confidence field is zero.
func ApplyV2Confidence(findings []scanner.Finding, r *v2.Rule, fallback float64) {
	confidence := r.Confidence
	if confidence == 0 {
		confidence = fallback
	}
	for i := range findings {
		if findings[i].Confidence == 0 {
			findings[i].Confidence = confidence
		}
	}
}
