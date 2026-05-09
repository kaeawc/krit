package snapshot

import (
	"fmt"
	"sort"
)

// TimelineScope selects which slice of a Metrics rollup contributes a
// value at each captured commit.
type TimelineScope string

const (
	ScopeRepo   TimelineScope = "repo"
	ScopeModule TimelineScope = "module"
	ScopeFile   TimelineScope = "file"
)

// TimelineQuery describes a single timeline read across all captured
// snapshots under a repo's snapshots root.
type TimelineQuery struct {
	Scope  TimelineScope
	Target string // module path (":app") or repo-relative file path; ignored for ScopeRepo
	Metric string // see metricByName
}

// TimelinePoint is one (commit, value) reading. Sorted by CapturedAt
// ascending in the result of Timeline.
type TimelinePoint struct {
	CommitSHA  string
	CapturedAt int64
	Value      float64
}

// Timeline loads every metrics rollup under root, projects the requested
// scalar, and returns the points sorted by capture time. Snapshots that
// have no matching target (e.g. a module that did not exist at that sha)
// are skipped — callers see a sparse series rather than zero-filled
// readings, mirroring git history.
func Timeline(root string, q TimelineQuery) ([]TimelinePoint, error) {
	if q.Scope == "" {
		q.Scope = ScopeRepo
	}
	if q.Metric == "" {
		q.Metric = "loc"
	}
	if (q.Scope == ScopeModule || q.Scope == ScopeFile) && q.Target == "" {
		return nil, fmt.Errorf("snapshot: timeline scope=%s requires --target", q.Scope)
	}

	entries, err := List(root)
	if err != nil {
		return nil, err
	}
	out := make([]TimelinePoint, 0, len(entries))
	for _, e := range entries {
		m, err := LoadMetrics(root, e.CommitSHA)
		if err != nil {
			continue
		}
		v, ok := projectMetric(m, q)
		if !ok {
			continue
		}
		out = append(out, TimelinePoint{CommitSHA: e.CommitSHA, CapturedAt: m.CapturedAt, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CapturedAt < out[j].CapturedAt })
	return out, nil
}

// projectMetric pulls a single scalar out of a Metrics rollup according
// to scope/target/metric. Returns ok=false when target is absent at this
// snapshot — callers turn that into a sparse series.
func projectMetric(m *Metrics, q TimelineQuery) (float64, bool) {
	switch q.Scope {
	case ScopeFile:
		for _, fm := range m.Files {
			if fm.Path == q.Target {
				return fileMetricValue(fm, q.Metric)
			}
		}
		return 0, false
	case ScopeModule:
		for _, mm := range m.Modules {
			if mm.Path == q.Target {
				return moduleMetricValue(mm, q.Metric)
			}
		}
		return 0, false
	default:
		return repoMetricValue(m, q.Metric)
	}
}

func fileMetricValue(fm FileMetrics, metric string) (float64, bool) {
	switch metric {
	case "loc":
		return float64(fm.LOC), true
	case "bytes":
		return float64(fm.Bytes), true
	case "symbols":
		return float64(fm.Symbols), true
	case "public_symbols":
		return float64(fm.PublicSymbols), true
	case "cyclomatic":
		return float64(fm.Cyclomatic), true
	}
	return 0, false
}

func moduleMetricValue(mm ModuleMetrics, metric string) (float64, bool) {
	switch metric {
	case "loc":
		return float64(mm.LOC), true
	case "files":
		return float64(mm.Files), true
	case "symbols":
		return float64(mm.Symbols), true
	case "public_symbols":
		return float64(mm.PublicSymbols), true
	case "cyclomatic":
		return float64(mm.Cyclomatic), true
	case "fan_in":
		return float64(mm.FanIn), true
	case "fan_out":
		return float64(mm.FanOut), true
	}
	return 0, false
}

func repoMetricValue(m *Metrics, metric string) (float64, bool) {
	switch metric {
	case "loc":
		total := 0
		for _, fm := range m.Files {
			total += fm.LOC
		}
		return float64(total), true
	case "files":
		return float64(len(m.Files)), true
	case "symbols":
		total := 0
		for _, fm := range m.Files {
			total += fm.Symbols
		}
		return float64(total), true
	case "public_symbols":
		total := 0
		for _, fm := range m.Files {
			total += fm.PublicSymbols
		}
		return float64(total), true
	case "cyclomatic":
		total := 0
		for _, fm := range m.Files {
			total += fm.Cyclomatic
		}
		return float64(total), true
	case "modules":
		return float64(len(m.Modules)), true
	}
	return 0, false
}
