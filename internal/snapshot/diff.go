package snapshot

import (
	"errors"
	"fmt"
	"sort"
)

// DiffResult is the structural delta between two captured snapshots.
type DiffResult struct {
	From DiffSide `json:"from"`
	To   DiffSide `json:"to"`

	AddedFiles   []FileRef `json:"added_files,omitempty"`
	RemovedFiles []FileRef `json:"removed_files,omitempty"`

	AddedSymbols   []SymbolRef `json:"added_symbols,omitempty"`
	RemovedSymbols []SymbolRef `json:"removed_symbols,omitempty"`

	AddedModules   []string `json:"added_modules,omitempty"`
	RemovedModules []string `json:"removed_modules,omitempty"`

	AddedEdges   []ModuleEdge `json:"added_edges,omitempty"`
	RemovedEdges []ModuleEdge `json:"removed_edges,omitempty"`

	RepoMetrics   map[string]MetricDelta            `json:"repo_metrics,omitempty"`
	ModuleMetrics map[string]map[string]MetricDelta `json:"module_metrics,omitempty"`

	// FindingsByRule is populated only when both snapshots carry a
	// findings sidecar AND share a RuleSetHash. Cross-rule-set
	// comparisons stay nil so callers don't mistake an apples-to-oranges
	// delta for a real drift signal.
	FindingsByRule map[string]MetricDelta `json:"findings_by_rule,omitempty"`
	// FindingsRuleSetMismatch is set when both sides have findings
	// sidecars whose RuleSetHash differs. Consumers should refuse to
	// report a findings delta in that case.
	FindingsRuleSetMismatch bool `json:"findings_rule_set_mismatch,omitempty"`
}

// DiffSide identifies one end of a diff and tags it with the krit and
// blob schema versions so consumers can guard against incomparable
// reads.
type DiffSide struct {
	CommitSHA     string `json:"commit_sha"`
	CapturedAt    int64  `json:"captured_at"`
	KritVersion   string `json:"krit_version,omitempty"`
	BlobSchema    int    `json:"blob_schema"`
	MetricsSchema int    `json:"metrics_schema"`
}

// FileRef names a file in the diff. Module is the gradle path, "" when
// the file is outside any discovered module.
type FileRef struct {
	Path     string `json:"path"`
	Module   string `json:"module,omitempty"`
	Language string `json:"language,omitempty"`
}

// SymbolRef names a symbol in the diff. FQN + Signature is the join
// key; symbols with the same FQN but different overloads diff
// independently.
type SymbolRef struct {
	FQN       string `json:"fqn"`
	Signature string `json:"signature,omitempty"`
	Kind      string `json:"kind,omitempty"`
	File      string `json:"file,omitempty"`
}

// ModuleEdge is one outgoing dependency edge (`from` depends on `to`).
type ModuleEdge struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Configuration string `json:"configuration"`
}

// MetricDelta carries before/after scalars and their difference.
type MetricDelta struct {
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Delta float64 `json:"delta"`
}

// Diff loads the two captured snapshots and returns the structural
// delta. Both blobs and metrics are read; either side may have a
// metrics rollup absent without aborting the diff.
func Diff(root, fromSHA, toSHA string) (*DiffResult, error) {
	if fromSHA == "" || toSHA == "" {
		return nil, errors.New("snapshot: diff requires both shas")
	}
	from, err := Load(root, fromSHA)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	to, err := Load(root, toSHA)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	if from.SchemaVersion != to.SchemaVersion {
		return nil, fmt.Errorf("snapshot: incompatible blob schemas (from=%d, to=%d)", from.SchemaVersion, to.SchemaVersion)
	}

	result := &DiffResult{
		From: DiffSide{CommitSHA: from.CommitSHA, CapturedAt: from.CapturedAt, KritVersion: from.KritVersion, BlobSchema: from.SchemaVersion},
		To:   DiffSide{CommitSHA: to.CommitSHA, CapturedAt: to.CapturedAt, KritVersion: to.KritVersion, BlobSchema: to.SchemaVersion},
	}

	result.AddedFiles, result.RemovedFiles = diffFiles(from.Files, to.Files)
	result.AddedSymbols, result.RemovedSymbols = diffSymbols(from.Symbols, to.Symbols)
	result.AddedModules, result.RemovedModules = diffModuleSet(from.Modules, to.Modules)
	result.AddedEdges, result.RemovedEdges = diffModuleEdges(from.Modules, to.Modules)

	fromMetrics, _ := LoadMetrics(root, fromSHA)
	toMetrics, _ := LoadMetrics(root, toSHA)
	if fromMetrics != nil && toMetrics != nil {
		result.From.MetricsSchema = fromMetrics.SchemaVersion
		result.To.MetricsSchema = toMetrics.SchemaVersion
		result.RepoMetrics = diffRepoMetrics(fromMetrics, toMetrics)
		result.ModuleMetrics = diffModuleMetrics(fromMetrics, toMetrics)
	}

	fromFindings, _ := LoadFindings(root, fromSHA)
	toFindings, _ := LoadFindings(root, toSHA)
	if fromFindings != nil && toFindings != nil {
		if fromFindings.RuleSetHash != "" && toFindings.RuleSetHash != "" && fromFindings.RuleSetHash != toFindings.RuleSetHash {
			result.FindingsRuleSetMismatch = true
		} else {
			result.FindingsByRule = diffFindingsByRule(fromFindings, toFindings)
		}
	}

	return result, nil
}

func diffFindingsByRule(from, to *Findings) map[string]MetricDelta {
	rules := make(map[string]bool, len(from.ByRule)+len(to.ByRule))
	for r := range from.ByRule {
		rules[r] = true
	}
	for r := range to.ByRule {
		rules[r] = true
	}
	out := make(map[string]MetricDelta, len(rules))
	for r := range rules {
		fv := float64(from.ByRule[r])
		tv := float64(to.ByRule[r])
		if fv == 0 && tv == 0 {
			continue
		}
		out[r] = MetricDelta{From: fv, To: tv, Delta: tv - fv}
	}
	return out
}

func diffFiles(from, to []File) (added, removed []FileRef) {
	fromIndex := make(map[string]File, len(from))
	for _, f := range from {
		fromIndex[f.Path] = f
	}
	toIndex := make(map[string]File, len(to))
	for _, f := range to {
		toIndex[f.Path] = f
	}
	for _, f := range to {
		if _, ok := fromIndex[f.Path]; !ok {
			added = append(added, FileRef{Path: f.Path, Module: f.Module, Language: f.Language})
		}
	}
	for _, f := range from {
		if _, ok := toIndex[f.Path]; !ok {
			removed = append(removed, FileRef{Path: f.Path, Module: f.Module, Language: f.Language})
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].Path < added[j].Path })
	sort.Slice(removed, func(i, j int) bool { return removed[i].Path < removed[j].Path })
	return added, removed
}

func diffSymbols(from, to []Symbol) (added, removed []SymbolRef) {
	key := func(s Symbol) string { return s.FQN + "\x00" + s.Signature }
	fromIndex := make(map[string]Symbol, len(from))
	for _, s := range from {
		fromIndex[key(s)] = s
	}
	toIndex := make(map[string]Symbol, len(to))
	for _, s := range to {
		toIndex[key(s)] = s
	}
	for _, s := range to {
		if _, ok := fromIndex[key(s)]; !ok {
			added = append(added, SymbolRef{FQN: s.FQN, Signature: s.Signature, Kind: s.Kind, File: s.File})
		}
	}
	for _, s := range from {
		if _, ok := toIndex[key(s)]; !ok {
			removed = append(removed, SymbolRef{FQN: s.FQN, Signature: s.Signature, Kind: s.Kind, File: s.File})
		}
	}
	sortSymbolRefs(added)
	sortSymbolRefs(removed)
	return added, removed
}

func sortSymbolRefs(refs []SymbolRef) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].FQN != refs[j].FQN {
			return refs[i].FQN < refs[j].FQN
		}
		return refs[i].Signature < refs[j].Signature
	})
}

func diffModuleSet(from, to []Module) (added, removed []string) {
	fromSet := make(map[string]bool, len(from))
	for _, m := range from {
		fromSet[m.Path] = true
	}
	toSet := make(map[string]bool, len(to))
	for _, m := range to {
		toSet[m.Path] = true
	}
	for path := range toSet {
		if !fromSet[path] {
			added = append(added, path)
		}
	}
	for path := range fromSet {
		if !toSet[path] {
			removed = append(removed, path)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func diffModuleEdges(from, to []Module) (added, removed []ModuleEdge) {
	collect := func(mods []Module) map[ModuleEdge]bool {
		out := make(map[ModuleEdge]bool)
		for _, m := range mods {
			for _, d := range m.Dependencies {
				out[ModuleEdge{From: m.Path, To: d.Path, Configuration: d.Configuration}] = true
			}
		}
		return out
	}
	fromEdges := collect(from)
	toEdges := collect(to)
	for e := range toEdges {
		if !fromEdges[e] {
			added = append(added, e)
		}
	}
	for e := range fromEdges {
		if !toEdges[e] {
			removed = append(removed, e)
		}
	}
	sortEdges(added)
	sortEdges(removed)
	return added, removed
}

func sortEdges(edges []ModuleEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Configuration < edges[j].Configuration
	})
}

// repoMetricNames lists every scalar projected from a Metrics rollup at
// repo scope. Kept in sync with repoMetricValue in timeline.go.
var repoMetricNames = []string{"loc", "files", "symbols", "public_symbols", "cyclomatic", "modules"}

// moduleMetricNames lists every scalar projected at module scope. Kept
// in sync with moduleMetricValue in timeline.go.
var moduleMetricNames = []string{"loc", "files", "symbols", "public_symbols", "cyclomatic", "fan_in", "fan_out"}

func diffRepoMetrics(from, to *Metrics) map[string]MetricDelta {
	out := make(map[string]MetricDelta, len(repoMetricNames))
	for _, name := range repoMetricNames {
		fromV, _ := repoMetricValue(from, name)
		toV, _ := repoMetricValue(to, name)
		if fromV == 0 && toV == 0 {
			continue
		}
		out[name] = MetricDelta{From: fromV, To: toV, Delta: toV - fromV}
	}
	return out
}

func diffModuleMetrics(from, to *Metrics) map[string]map[string]MetricDelta {
	moduleSet := make(map[string]bool)
	fromIndex := make(map[string]ModuleMetrics, len(from.Modules))
	for _, m := range from.Modules {
		fromIndex[m.Path] = m
		moduleSet[m.Path] = true
	}
	toIndex := make(map[string]ModuleMetrics, len(to.Modules))
	for _, m := range to.Modules {
		toIndex[m.Path] = m
		moduleSet[m.Path] = true
	}
	out := make(map[string]map[string]MetricDelta, len(moduleSet))
	for modPath := range moduleSet {
		fm := fromIndex[modPath]
		tm := toIndex[modPath]
		mod := make(map[string]MetricDelta, len(moduleMetricNames))
		for _, name := range moduleMetricNames {
			fromV, _ := moduleMetricValue(fm, name)
			toV, _ := moduleMetricValue(tm, name)
			if fromV == 0 && toV == 0 {
				continue
			}
			mod[name] = MetricDelta{From: fromV, To: toV, Delta: toV - fromV}
		}
		if len(mod) > 0 {
			out[modPath] = mod
		}
	}
	return out
}
