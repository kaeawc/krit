package rules

import (
	"os"
	"sort"
	"sync"

	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/manifest"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RunManifestSource runs every ScopeManifestSource rule once against the
// project's parsed manifests and source files. A rule may report into a
// source file or into a manifest, so each finding passes through the filter
// of the file it lands in: @Suppress / @SuppressLint / krit:ignore for source
// files, tools:ignore for manifests, and per-path config excludes for both.
// The bool is false when a rule panic was recovered.
func (d *Dispatcher) RunManifestSource(manifests []*manifest.Manifest, files []*scanner.File) (scanner.FindingColumns, bool) {
	if len(d.manifestSourceRules) == 0 || len(manifests) == 0 {
		return scanner.FindingColumns{}, true
	}
	manifests = append([]*manifest.Manifest(nil), manifests...)
	sort.SliceStable(manifests, func(i, j int) bool { return manifests[i].Path < manifests[j].Path })
	var javaSourceIndex *javafacts.SourceIndex
	for _, f := range files {
		if f != nil && f.Language == scanner.LangJava {
			javaSourceIndex = javafacts.SourceIndexForFiles(files)
			break
		}
	}
	populate := func(ctx *api.Context) {
		ctx.Manifests = manifests
		ctx.ParsedFiles = files
		ctx.JavaSourceIndex = javaSourceIndex
	}
	// Each rule sees every file at once, so NeedsConcurrent rules run in parallel. Results are merged in rule order,
	// which keeps the output independent of goroutine scheduling.
	type ruleResult struct {
		cols  scanner.FindingColumns
		ok    bool
		stats RunStats
	}
	results := make([]ruleResult, len(d.manifestSourceRules))
	var wg sync.WaitGroup
	for i, r := range d.manifestSourceRules {
		run := func() {
			res := &results[i]
			res.cols, res.ok = d.runProjectRule(r, nil, populate, &res.stats)
		}
		if r.Needs.Has(api.NeedsConcurrent) {
			wg.Go(run)
		} else {
			run()
		}
	}
	wg.Wait()
	collector := scanner.NewFindingCollector(0)
	cacheable := true
	var errs []DispatchError
	for i := range results {
		cacheable = cacheable && results[i].ok
		collector.AppendColumns(&results[i].cols)
		errs = append(errs, results[i].stats.Errors...)
	}
	d.projectStatsMu.Lock()
	d.projectStats.Errors = append(d.projectStats.Errors, errs...)
	d.projectStatsMu.Unlock()
	return d.filterManifestSourceFindings(*collector.Columns(), manifests, files), cacheable
}

// filterManifestSourceFindings applies the suppression of whichever file each
// finding lands in.
func (d *Dispatcher) filterManifestSourceFindings(cols scanner.FindingColumns, manifests []*manifest.Manifest, files []*scanner.File) scanner.FindingColumns {
	if cols.Len() == 0 {
		return cols
	}
	excludedByPath := map[string]map[string]bool{}
	cols = cols.FilterRows(func(row int) bool {
		path := cols.FileAt(row)
		excluded, ok := excludedByPath[path]
		if !ok {
			excluded = d.buildExcludedSet(path)
			excludedByPath[path] = excluded
		}
		return !excluded[cols.RuleAt(row)]
	})
	sources := make(map[string]*scanner.File, len(files))
	for _, f := range files {
		if f != nil {
			sources[f.Path] = f
		}
	}
	cols = cols.FilterRows(func(row int) bool {
		file := sources[cols.FileAt(row)]
		if file == nil {
			return true
		}
		filter := file.Suppression
		if filter == nil {
			filter = scanner.BuildSuppressionFilter(file, nil, allRuleExcludes(), "").WithRuleAliases(AllSuppressionAliases()).WithExcludedRules(ScriptPolicyExcludes(file))
			file.Suppression = filter
			file.SuppressionIdx = filter.Annotations()
		}
		return !filter.IsSuppressedAtColumn(cols.RuleAt(row), cols.RuleSetAt(row), cols.LineAt(row), cols.ColumnAt(row))
	})
	manifestPaths := make(map[string]bool, len(manifests))
	for _, m := range manifests {
		if m != nil && m.Path != "" {
			manifestPaths[m.Path] = true
		}
	}
	// tools:ignore on the element or any ancestor silences a manifest finding.
	cols = filterXMLToolsIgnored(cols, func(path string) bool { return manifestPaths[path] }, func(path string) []byte {
		data, _ := os.ReadFile(path)
		return data
	})
	return cols
}
