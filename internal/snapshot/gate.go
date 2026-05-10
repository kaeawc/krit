package snapshot

import (
	"errors"
	"math"
	"sort"
)

// GateConstraint identifies which limit a GateThreshold imposed.
type GateConstraint string

const (
	ConstraintMaxAbsolute    GateConstraint = "max_absolute"
	ConstraintMaxIncrease    GateConstraint = "max_increase"
	ConstraintMaxIncreasePct GateConstraint = "max_increase_pct"
)

// GateThreshold expresses one constraint on a metric. Empty Module
// targets the repo-scope reading; a non-empty Module targets that
// module's reading from DiffResult.ModuleMetrics. At least one of
// MaxAbsolute / MaxIncrease / MaxIncreasePct must be set.
type GateThreshold struct {
	Module         string
	Metric         string
	MaxAbsolute    *float64
	MaxIncrease    *float64
	MaxIncreasePct *float64
}

type GateViolation struct {
	// Module is empty for repo-scope thresholds; otherwise the module
	// path the violation applies to (e.g. ":feature:checkout").
	Module     string         `json:"module,omitempty"`
	Metric     string         `json:"metric"`
	Constraint GateConstraint `json:"constraint"`
	Limit      float64        `json:"limit"`
	Got        float64        `json:"got"`
	From       float64        `json:"from"`
	To         float64        `json:"to"`
}

type GateResult struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	Violations []GateViolation `json:"violations,omitempty"`
}

type GateOptions struct {
	Root       string
	FromSHA    string
	ToSHA      string
	Thresholds []GateThreshold
}

func Gate(opts GateOptions) (*GateResult, error) {
	if len(opts.Thresholds) == 0 {
		return nil, errors.New("snapshot: gate requires at least one threshold")
	}
	d, err := Diff(opts.Root, opts.FromSHA, opts.ToSHA)
	if err != nil {
		return nil, err
	}
	out := &GateResult{From: d.From.CommitSHA, To: d.To.CommitSHA}
	for _, t := range opts.Thresholds {
		m, ok := lookupMetric(d, t.Module, t.Metric)
		if !ok {
			continue
		}
		if t.MaxAbsolute != nil && m.To > *t.MaxAbsolute {
			out.Violations = append(out.Violations, GateViolation{
				Module: t.Module, Metric: t.Metric, Constraint: ConstraintMaxAbsolute,
				Limit: *t.MaxAbsolute, Got: m.To, From: m.From, To: m.To,
			})
		}
		if t.MaxIncrease != nil && m.Delta > *t.MaxIncrease {
			out.Violations = append(out.Violations, GateViolation{
				Module: t.Module, Metric: t.Metric, Constraint: ConstraintMaxIncrease,
				Limit: *t.MaxIncrease, Got: m.Delta, From: m.From, To: m.To,
			})
		}
		if t.MaxIncreasePct != nil {
			pct := percentChange(m.From, m.Delta)
			if pct > *t.MaxIncreasePct {
				out.Violations = append(out.Violations, GateViolation{
					Module: t.Module, Metric: t.Metric, Constraint: ConstraintMaxIncreasePct,
					Limit: *t.MaxIncreasePct, Got: pct, From: m.From, To: m.To,
				})
			}
		}
	}
	sort.Slice(out.Violations, func(i, j int) bool {
		if out.Violations[i].Module != out.Violations[j].Module {
			return out.Violations[i].Module < out.Violations[j].Module
		}
		if out.Violations[i].Metric != out.Violations[j].Metric {
			return out.Violations[i].Metric < out.Violations[j].Metric
		}
		return out.Violations[i].Constraint < out.Violations[j].Constraint
	})
	return out, nil
}

// lookupMetric resolves a (module, metric) pair against a DiffResult.
// Empty module reads RepoMetrics; a non-empty module reads
// ModuleMetrics. Missing module/metric pairs return false so callers
// can silently skip thresholds against snapshots that don't carry the
// requested target (mirrors the pre-existing repo-scope behaviour).
func lookupMetric(d *DiffResult, module, metric string) (MetricDelta, bool) {
	if module == "" {
		m, ok := d.RepoMetrics[metric]
		return m, ok
	}
	mm, ok := d.ModuleMetrics[module]
	if !ok {
		return MetricDelta{}, false
	}
	m, ok := mm[metric]
	return m, ok
}

// percentChange handles the from=0 case explicitly: any positive delta
// is treated as +∞%, so a brand-new file/symbol/module count violates
// a percent-cap rather than slipping through.
func percentChange(from, delta float64) float64 {
	if from == 0 {
		if delta > 0 {
			return math.Inf(1)
		}
		return 0
	}
	return (delta / from) * 100
}
