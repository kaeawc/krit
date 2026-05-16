package bisect

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/snapshot"
)

const (
	labelBreakage = "breakage"
	labelBenign   = "benign"

	// corpusSize is the per-class commit count. 8 + 8 gives 64 pairwise
	// comparisons for AUC, enough resolution that AUC steps in 1/64
	// increments — fine for an 0.85 floor without being slow.
	corpusSize = 8

	// minAUC is the rank-quality floor delta-risk must clear. Random
	// baseline is 0.5; 0.85 leaves margin for the cosine score to be
	// noisy without going flaky.
	minAUC = 0.85
)

// TestScoreDeltaBeatsRandomOnLabeledCorpus is the acceptance baseline
// the bisect-structure PRD calls out: on a synthetic corpus of labeled
// breakage and benign commits, delta-risk's cosine score against the
// historical event ledger must rank breakages above benign deltas
// measurably better than a coin flip.
//
// The corpus models the failure mode the design sketch motivates:
// breakage commits share the same fan-in / cyclomatic-growth shape on
// the same module (':core'), benign commits perturb unrelated modules
// in unrelated directions. Random would deliver AUC ~= 0.5; we require
// AUC >= minAUC and the top-ranked event to be labeled "breakage."
func TestScoreDeltaBeatsRandomOnLabeledCorpus(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	// Every breakage / benign commit diffs against this baseline so the
	// historical-event delta vector and the candidate delta vector are
	// directly comparable.
	const baseSHA = "0000000000000000000000000000000000000000"
	captureBlob(t, root, baseSHA, 1,
		[]snapshot.Module{{Path: ":core"}, {Path: ":app"}, {Path: ":util"}},
		[]snapshot.File{
			{Path: "core/A.kt", Module: ":core"},
			{Path: "app/B.kt", Module: ":app"},
			{Path: "util/C.kt", Module: ":util"},
		},
		nil,
		&snapshot.Metrics{Modules: []snapshot.ModuleMetrics{
			{Path: ":core", LOC: 100, Cyclomatic: 10, FanIn: 4, FanOut: 2},
			{Path: ":app", LOC: 100, Cyclomatic: 10, FanIn: 0, FanOut: 3},
			{Path: ":util", LOC: 100, Cyclomatic: 10, FanIn: 2, FanOut: 0},
		}})

	rng := rand.New(rand.NewSource(0xBEEF))

	var historical []breakage.Event
	for i := 0; i < corpusSize; i++ {
		sha := fmt.Sprintf("b%039d", i+1)
		captureBreakageCommit(t, root, sha, int64(100+i), jitter(rng))
		historical = append(historical, breakage.Event{
			ID:          fmt.Sprintf("evt-break-%d", i),
			CommitSHA:   sha,
			FailureKind: breakage.KindTestFailure,
			Module:      ":core",
			Signature:   "fanin-blowup",
			OccurredAt:  int64(100 + i),
		})
	}

	var samples []labeledSample
	for i := 0; i < corpusSize; i++ {
		sha := fmt.Sprintf("c%039d", i+1)
		captureBreakageCommit(t, root, sha, int64(200+i), jitter(rng))
		samples = append(samples, labeledSample{sha: sha, label: labelBreakage})
	}
	for i := 0; i < corpusSize; i++ {
		sha := fmt.Sprintf("d%039d", i+1)
		captureBenignCommit(t, root, sha, int64(300+i), rng)
		samples = append(samples, labeledSample{sha: sha, label: labelBenign})
	}

	for i := range samples {
		res, err := ScoreDelta(DeltaRiskInput{
			SnapshotsRoot:    root,
			FromSHA:          baseSHA,
			ToSHA:            samples[i].sha,
			HistoricalEvents: historical,
			MaxMatches:       1,
		})
		if err != nil {
			t.Fatalf("ScoreDelta %s: %v", samples[i].sha, err)
		}
		samples[i].score = res.Score
	}

	if auc := computeAUC(samples); auc < minAUC {
		t.Fatalf("AUC = %.3f, want >= %.2f (random baseline is 0.5)", auc, minAUC)
	}

	bestLabel, bestScore := samples[0].label, samples[0].score
	for _, s := range samples[1:] {
		if s.score > bestScore {
			bestScore = s.score
			bestLabel = s.label
		}
	}
	if bestLabel != labelBreakage {
		t.Fatalf("top-ranked sample = %q, want %q (score=%.3f)", bestLabel, labelBreakage, bestScore)
	}
}

// jitter returns symmetric noise in [-2, 2). Breakage growth deltas
// stay positive even at the floor: with base +20 LOC / +8 cyclomatic /
// +6 fan-in, jitter cannot flip a delta to zero or negative.
func jitter(rng *rand.Rand) float64 {
	return rng.Float64()*4 - 2
}

// captureBreakageCommit lays down a snapshot whose module-delta vector
// matches the historical "fan-in blowup on :core" pattern.
func captureBreakageCommit(t *testing.T, root, sha string, capturedAt int64, jitter float64) {
	t.Helper()
	captureBlob(t, root, sha, capturedAt,
		[]snapshot.Module{{Path: ":core"}, {Path: ":app"}, {Path: ":util"}},
		[]snapshot.File{
			{Path: "core/A.kt", Module: ":core"},
			{Path: "app/B.kt", Module: ":app"},
			{Path: "util/C.kt", Module: ":util"},
		},
		nil,
		&snapshot.Metrics{Modules: []snapshot.ModuleMetrics{
			{Path: ":core", LOC: 100 + int(math.Round(20+jitter)), Cyclomatic: 10 + int(math.Round(8+jitter)), FanIn: 4 + int(math.Round(6+jitter)), FanOut: 2},
			{Path: ":app", LOC: 100, Cyclomatic: 10, FanIn: 0, FanOut: 3},
			{Path: ":util", LOC: 100, Cyclomatic: 10, FanIn: 2, FanOut: 0},
		}})
}

// captureBenignCommit perturbs an unrelated module in an unrelated
// direction. Cosine against the historical pattern should be ~0.
func captureBenignCommit(t *testing.T, root, sha string, capturedAt int64, rng *rand.Rand) {
	t.Helper()
	mods := []snapshot.ModuleMetrics{
		{Path: ":core", LOC: 100, Cyclomatic: 10, FanIn: 4, FanOut: 2},
		{Path: ":app", LOC: 100, Cyclomatic: 10, FanIn: 0, FanOut: 3},
		{Path: ":util", LOC: 100, Cyclomatic: 10, FanIn: 2, FanOut: 0},
	}
	if rng.Intn(2) == 0 {
		mods[1].LOC += 30
		mods[1].FanOut -= 2
	} else {
		mods[2].LOC -= 15
		mods[2].Cyclomatic -= 4
	}
	captureBlob(t, root, sha, capturedAt,
		[]snapshot.Module{{Path: ":core"}, {Path: ":app"}, {Path: ":util"}},
		[]snapshot.File{
			{Path: "core/A.kt", Module: ":core"},
			{Path: "app/B.kt", Module: ":app"},
			{Path: "util/C.kt", Module: ":util"},
		},
		nil,
		&snapshot.Metrics{Modules: mods})
}

// labeledSample is one labeled candidate commit used in the corpus test.
type labeledSample struct {
	sha   string
	label string
	score float64
}

// computeAUC returns the area under the ROC curve treating "breakage"
// as the positive class. 1.0 is perfect separation, 0.5 is random.
func computeAUC(samples []labeledSample) float64 {
	var pos, neg int
	for _, s := range samples {
		if s.label == labelBreakage {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0
	}
	var concordant, ties float64
	for _, p := range samples {
		if p.label != labelBreakage {
			continue
		}
		for _, n := range samples {
			if n.label == labelBreakage {
				continue
			}
			switch {
			case p.score > n.score:
				concordant++
			case p.score == n.score:
				ties += 0.5
			}
		}
	}
	return (concordant + ties) / float64(pos*neg)
}
