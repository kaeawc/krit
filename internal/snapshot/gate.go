package snapshot

import (
	"errors"
	"sort"
)

// GateThreshold expresses one constraint on a repo-scope metric. At
// least one of MaxAbsolute / MaxIncrease / MaxIncreasePct must be set.
type GateThreshold struct {
	Metric         string
	MaxAbsolute    *float64
	MaxIncrease    *float64
	MaxIncreasePct *float64
}

// GateViolation describes a single threshold breach. Limit is the
// configured cap; Got is the actual value (absolute, delta, or
// percent depending on Constraint).
type GateViolation struct {
	Metric     string  `json:"metric"`
	Constraint string  `json:"constraint"`
	Limit      float64 `json:"limit"`
	Got        float64 `json:"got"`
	From       float64 `json:"from"`
	To         float64 `json:"to"`
}

// GateResult aggregates every violation found while evaluating
// thresholds against a Diff. An empty Violations slice means the gate
// passed.
type GateResult struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	Violations []GateViolation `json:"violations,omitempty"`
}

// GateOptions pairs the diff inputs with the threshold list.
type GateOptions struct {
	Root       string
	FromSHA    string
	ToSHA      string
	Thresholds []GateThreshold
}

// Gate runs Diff(from, to) and returns every threshold violation. The
// caller decides what to do with violations; callers wanting a non-zero
// exit status check len(result.Violations) > 0.
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
		m, ok := d.RepoMetrics[t.Metric]
		if !ok {
			continue
		}
		if t.MaxAbsolute != nil && m.To > *t.MaxAbsolute {
			out.Violations = append(out.Violations, GateViolation{
				Metric: t.Metric, Constraint: "max_absolute",
				Limit: *t.MaxAbsolute, Got: m.To, From: m.From, To: m.To,
			})
		}
		if t.MaxIncrease != nil && m.Delta > *t.MaxIncrease {
			out.Violations = append(out.Violations, GateViolation{
				Metric: t.Metric, Constraint: "max_increase",
				Limit: *t.MaxIncrease, Got: m.Delta, From: m.From, To: m.To,
			})
		}
		if t.MaxIncreasePct != nil && m.From > 0 {
			pct := (m.Delta / m.From) * 100
			if pct > *t.MaxIncreasePct {
				out.Violations = append(out.Violations, GateViolation{
					Metric: t.Metric, Constraint: "max_increase_pct",
					Limit: *t.MaxIncreasePct, Got: pct, From: m.From, To: m.To,
				})
			}
		}
	}
	sort.Slice(out.Violations, func(i, j int) bool {
		if out.Violations[i].Metric != out.Violations[j].Metric {
			return out.Violations[i].Metric < out.Violations[j].Metric
		}
		return out.Violations[i].Constraint < out.Violations[j].Constraint
	})
	return out, nil
}
