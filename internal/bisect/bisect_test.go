package bisect

import (
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/snapshot"
)

// captureBlob persists a Blob + Metrics + Manifest under root and
// returns the sha. Helper for fixture timelines.
func captureBlob(t *testing.T, root, sha string, capturedAt int64, modules []snapshot.Module, files []snapshot.File, symbols []snapshot.Symbol, metrics *snapshot.Metrics) {
	t.Helper()
	blob := &snapshot.Blob{
		SchemaVersion: snapshot.SchemaVersion,
		CommitSHA:     sha,
		CapturedAt:    capturedAt,
		Modules:       modules,
		Files:         files,
		Symbols:       symbols,
	}
	if _, err := snapshot.Save(root, blob); err != nil {
		t.Fatalf("Save blob: %v", err)
	}
	if metrics != nil {
		metrics.SchemaVersion = snapshot.MetricsSchemaVersion
		metrics.CommitSHA = sha
		metrics.CapturedAt = capturedAt
		if _, err := snapshot.SaveMetrics(root, metrics); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}
	m := &snapshot.Manifest{
		SchemaVersion: snapshot.ManifestSchemaVersion,
		CommitSHA:     sha,
		CapturedAt:    capturedAt,
		BlobSchema:    snapshot.SchemaVersion,
		Files:         len(files),
		Modules:       len(modules),
		Symbols:       len(symbols),
	}
	if metrics != nil {
		m.MetricsSchema = snapshot.MetricsSchemaVersion
	}
	if _, err := snapshot.SaveManifest(root, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}
}

func TestRunStackFrameRanksTopOne(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	const goodSHA = "1111111111111111111111111111111111111111"
	const badSHA = "2222222222222222222222222222222222222222"

	captureBlob(t, root, goodSHA, 1,
		[]snapshot.Module{{Path: ":app"}, {Path: ":core"}},
		[]snapshot.File{{Path: "core/Order.kt", Module: ":core"}, {Path: "app/Main.kt", Module: ":app"}},
		[]snapshot.Symbol{{FQN: "com.acme.Order", File: "core/Order.kt"}},
		nil)
	captureBlob(t, root, badSHA, 2,
		[]snapshot.Module{{Path: ":app"}, {Path: ":core"}},
		[]snapshot.File{{Path: "core/Order.kt", Module: ":core"}, {Path: "core/Repo.kt", Module: ":core"}, {Path: "app/Main.kt", Module: ":app"}},
		[]snapshot.Symbol{{FQN: "com.acme.Order", File: "core/Order.kt"}, {FQN: "com.acme.Repo", File: "core/Repo.kt"}},
		nil)

	event := breakage.Event{
		ID:          "evt1",
		CommitSHA:   badSHA,
		FailureKind: breakage.KindRuntimeFailure,
		Frames:      []string{"com.acme.Repo.fetch(Repo.kt:42)"},
	}
	res, err := Run(Input{
		SnapshotsRoot: root,
		FromSHA:       goodSHA,
		ToSHA:         badSHA,
		Event:         event,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Candidates) == 0 {
		t.Fatalf("no candidates")
	}
	top := res.Candidates[0]
	if top.Module != ":core" {
		t.Errorf("top module = %q, want :core", top.Module)
	}
	if top.Confidence < 0.95 {
		t.Errorf("top confidence = %g, want >= 0.95", top.Confidence)
	}
	containsTier := func(c Candidate, want Tier) bool {
		for _, t := range c.Tiers {
			if t == want {
				return true
			}
		}
		return false
	}
	if !containsTier(top, TierStackFrame) {
		t.Errorf("top tiers missing stack-frame tier: %+v", top.Tiers)
	}
}

func TestRunDiffTouchesRanksTopThree(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	const goodSHA = "1111111111111111111111111111111111111111"
	const midSHA = "2222222222222222222222222222222222222222"
	const badSHA = "3333333333333333333333333333333333333333"

	captureBlob(t, root, goodSHA, 1,
		[]snapshot.Module{{Path: ":app"}, {Path: ":core"}},
		[]snapshot.File{{Path: "core/A.kt", Module: ":core"}},
		nil, nil)
	captureBlob(t, root, midSHA, 2,
		[]snapshot.Module{{Path: ":app"}, {Path: ":core"}, {Path: ":lib"}},
		[]snapshot.File{{Path: "core/A.kt", Module: ":core"}, {Path: "lib/B.kt", Module: ":lib"}},
		nil, nil)
	captureBlob(t, root, badSHA, 3,
		[]snapshot.Module{{Path: ":app"}, {Path: ":core"}, {Path: ":lib"}},
		[]snapshot.File{{Path: "core/A.kt", Module: ":core"}, {Path: "lib/B.kt", Module: ":lib"}, {Path: "app/C.kt", Module: ":app"}},
		nil, nil)

	event := breakage.Event{
		ID:          "evt2",
		CommitSHA:   badSHA,
		FailureKind: breakage.KindTestFailure,
		Signature:   "x",
	}
	res, err := Run(Input{
		SnapshotsRoot: root,
		FromSHA:       goodSHA,
		ToSHA:         badSHA,
		Event:         event,
		MaxResults:    3,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Candidates) == 0 {
		t.Fatalf("no candidates")
	}
	wantModules := map[string]bool{":lib": false, ":app": false}
	for _, c := range res.Candidates {
		if _, ok := wantModules[c.Module]; ok {
			wantModules[c.Module] = true
		}
	}
	for m, found := range wantModules {
		if !found {
			t.Errorf("expected module %q in top-3, got %+v", m, res.Candidates)
		}
	}
}

func TestScoreDeltaCosine(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	const a = "1111111111111111111111111111111111111111"
	const b = "2222222222222222222222222222222222222222"

	captureBlob(t, root, a, 1,
		[]snapshot.Module{{Path: ":app"}},
		[]snapshot.File{{Path: "app/A.kt", Module: ":app"}},
		nil,
		&snapshot.Metrics{Modules: []snapshot.ModuleMetrics{{Path: ":app", LOC: 100, Cyclomatic: 5, FanIn: 1}}, Files: []snapshot.FileMetrics{{Path: "app/A.kt", Module: ":app", LOC: 100, Cyclomatic: 5}}})
	captureBlob(t, root, b, 2,
		[]snapshot.Module{{Path: ":app"}},
		[]snapshot.File{{Path: "app/A.kt", Module: ":app"}},
		nil,
		&snapshot.Metrics{Modules: []snapshot.ModuleMetrics{{Path: ":app", LOC: 200, Cyclomatic: 15, FanIn: 4}}, Files: []snapshot.FileMetrics{{Path: "app/A.kt", Module: ":app", LOC: 200, Cyclomatic: 15}}})

	res, err := ScoreDelta(DeltaRiskInput{SnapshotsRoot: root, FromSHA: a, ToSHA: b})
	if err != nil {
		t.Fatalf("ScoreDelta: %v", err)
	}
	if len(res.Vector) == 0 {
		t.Fatalf("vector empty")
	}
	if _, ok := res.Vector[":app::loc"]; !ok {
		t.Errorf("vector missing :app::loc, got %+v", res.Vector)
	}
}
