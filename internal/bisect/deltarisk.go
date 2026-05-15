package bisect

import (
	"fmt"
	"math"
	"sort"

	"github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/snapshot"
)

// DeltaRiskInput scores a structural delta against historical events.
// SnapshotsRoot, FromSHA, and ToSHA are required; HistoricalEvents is
// the ledger to compare against (typically the full event store).
type DeltaRiskInput struct {
	SnapshotsRoot    string
	FromSHA          string
	ToSHA            string
	HistoricalEvents []breakage.Event
	MaxMatches       int
}

// DeltaRiskMatch is one historical event whose delta vector resembles
// the current one. Cosine is the similarity in [0, 1]; higher means
// more similar.
type DeltaRiskMatch struct {
	EventID     string  `json:"event_id"`
	FailureKind string  `json:"failure_kind"`
	CommitSHA   string  `json:"commit_sha"`
	Module      string  `json:"module"`
	Cosine      float64 `json:"cosine"`
}

// DeltaRiskResult is the scored delta plus its top historical matches.
// Score is the max cosine across matches — the bigger it is, the more
// the current delta looks like a previously-recorded breakage.
type DeltaRiskResult struct {
	From    string             `json:"from"`
	To      string             `json:"to"`
	Score   float64            `json:"score"`
	Vector  map[string]float64 `json:"vector"`
	Matches []DeltaRiskMatch   `json:"matches,omitempty"`
}

// ScoreDelta computes the per-module delta vector between two captured
// snapshots and scores it against historical events.
//
// Vector key form: "<module>::<metric>". Metrics included: loc, files,
// symbols, public_symbols, cyclomatic, fan_in, fan_out. Modules absent
// from either side contribute their populated side as a signed delta.
func ScoreDelta(in DeltaRiskInput) (*DeltaRiskResult, error) {
	if in.SnapshotsRoot == "" {
		return nil, fmt.Errorf("deltarisk: SnapshotsRoot required")
	}
	if in.FromSHA == "" || in.ToSHA == "" {
		return nil, fmt.Errorf("deltarisk: FromSHA and ToSHA required")
	}
	d, err := snapshot.Diff(in.SnapshotsRoot, in.FromSHA, in.ToSHA)
	if err != nil {
		return nil, fmt.Errorf("deltarisk: diff: %w", err)
	}
	vec := vectorFromDiff(d)
	result := &DeltaRiskResult{From: in.FromSHA, To: in.ToSHA, Vector: vec}
	if len(vec) == 0 || len(in.HistoricalEvents) == 0 {
		return result, nil
	}

	// For every historical event whose CommitSHA differs from FromSHA,
	// compute the diff from its parent (using snapshot.LoadManifests to
	// find the nearest earlier captured sha) to the event's sha, and
	// score cosine similarity against the current vector.
	manifests, err := snapshot.LoadManifests(in.SnapshotsRoot)
	if err != nil {
		return nil, fmt.Errorf("deltarisk: load manifests: %w", err)
	}
	ordered := orderManifests(manifests)
	parentOf := buildParentMap(ordered)

	var matches []DeltaRiskMatch
	seen := make(map[string]bool)
	for _, ev := range in.HistoricalEvents {
		if seen[ev.CommitSHA] {
			continue
		}
		seen[ev.CommitSHA] = true
		parent, ok := parentOf[ev.CommitSHA]
		if !ok {
			continue
		}
		hd, err := snapshot.Diff(in.SnapshotsRoot, parent, ev.CommitSHA)
		if err != nil || hd == nil {
			continue
		}
		histVec := vectorFromDiff(hd)
		if len(histVec) == 0 {
			continue
		}
		cos := cosine(vec, histVec)
		if cos <= 0 {
			continue
		}
		matches = append(matches, DeltaRiskMatch{
			EventID:     ev.ID,
			FailureKind: ev.FailureKind,
			CommitSHA:   ev.CommitSHA,
			Module:      ev.Module,
			Cosine:      cos,
		})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Cosine > matches[j].Cosine })
	if in.MaxMatches > 0 && len(matches) > in.MaxMatches {
		matches = matches[:in.MaxMatches]
	}
	if len(matches) > 0 {
		result.Score = matches[0].Cosine
	}
	result.Matches = matches
	return result, nil
}

// vectorFromDiff projects ModuleMetrics into a sparse string-keyed
// vector. Zero-deltas are dropped so cosine similarity isn't diluted
// by metrics that didn't move.
func vectorFromDiff(d *snapshot.DiffResult) map[string]float64 {
	if d == nil {
		return nil
	}
	out := make(map[string]float64)
	for mod, mm := range d.ModuleMetrics {
		for metric, md := range mm {
			if md.Delta == 0 {
				continue
			}
			out[mod+"::"+metric] = md.Delta
		}
	}
	return out
}

// buildParentMap maps each captured sha to its immediate predecessor
// in capture order. This is a crude stand-in for the git parent and
// works whenever snapshots have been captured in commit order (the
// common case for backfill / install-hook).
func buildParentMap(ordered []snapshot.Manifest) map[string]string {
	out := make(map[string]string, len(ordered))
	for i := 1; i < len(ordered); i++ {
		out[ordered[i].CommitSHA] = ordered[i-1].CommitSHA
	}
	return out
}

// cosine computes cosine similarity between two sparse vectors. The
// result is in [-1, 1]; negative means the vectors point in opposite
// directions (one shrunk loc where the other grew loc, etc.).
func cosine(a, b map[string]float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for k, va := range a {
		magA += va * va
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	for _, vb := range b {
		magB += vb * vb
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}
