package snapshot

import (
	"path/filepath"
	"testing"
)

func TestTimelineRepoLOCSortedByCapturedAt(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	cases := []struct {
		sha string
		at  int64
		loc int
	}{
		{"1111111111111111111111111111111111111111", 1700000003000, 30},
		{"2222222222222222222222222222222222222222", 1700000001000, 10},
		{"3333333333333333333333333333333333333333", 1700000002000, 20},
	}
	for _, c := range cases {
		blob := &Blob{SchemaVersion: SchemaVersion, CommitSHA: c.sha, CapturedAt: c.at}
		if _, err := Save(root, blob); err != nil {
			t.Fatalf("Save: %v", err)
		}
		m := &Metrics{
			SchemaVersion: MetricsSchemaVersion,
			CommitSHA:     c.sha,
			CapturedAt:    c.at,
			Files:         []FileMetrics{{Path: "a.kt", LOC: c.loc}},
		}
		if _, err := SaveMetrics(root, m); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}

	points, err := Timeline(root, TimelineQuery{Scope: ScopeRepo, Metric: "loc"})
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	wantOrder := []float64{10, 20, 30}
	for i, p := range points {
		if p.Value != wantOrder[i] {
			t.Fatalf("point %d: value %v, want %v (full: %+v)", i, p.Value, wantOrder[i], points)
		}
	}
}

func TestTimelineModuleScopeSparse(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	full := &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     "1111111111111111111111111111111111111111",
		CapturedAt:    1,
		Modules:       []ModuleMetrics{{Path: ":app", LOC: 100, FanIn: 2}},
	}
	missing := &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     "2222222222222222222222222222222222222222",
		CapturedAt:    2,
		Modules:       []ModuleMetrics{{Path: ":core", LOC: 50}},
	}
	for _, m := range []*Metrics{full, missing} {
		if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: m.CommitSHA, CapturedAt: m.CapturedAt}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if _, err := SaveMetrics(root, m); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}

	points, err := Timeline(root, TimelineQuery{Scope: ScopeModule, Target: ":app", Metric: "loc"})
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(points) != 1 || points[0].Value != 100 {
		t.Fatalf("expected single :app point with 100, got %+v", points)
	}

	if _, err := Timeline(root, TimelineQuery{Scope: ScopeModule}); err == nil {
		t.Fatal("expected error when target missing")
	}
}
