package snapshot

import (
	"path/filepath"
	"testing"
)

func TestDiffSurfacesAddedRemovedFilesSymbolsModules(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	from := &Blob{
		SchemaVersion: SchemaVersion,
		CommitSHA:     "1111111111111111111111111111111111111111",
		CapturedAt:    1,
		Modules: []Module{
			{Path: ":app", Dependencies: []ModuleDep{{Path: ":core", Configuration: "implementation"}}},
			{Path: ":core"},
		},
		Files: []File{{Path: "app/A.kt"}, {Path: "core/B.kt"}},
		Symbols: []Symbol{
			{FQN: "demo.A", Signature: "demo.A", File: "app/A.kt", Kind: "class"},
			{FQN: "demo.B", Signature: "demo.B", File: "core/B.kt", Kind: "class"},
		},
	}
	to := &Blob{
		SchemaVersion: SchemaVersion,
		CommitSHA:     "2222222222222222222222222222222222222222",
		CapturedAt:    2,
		Modules: []Module{
			{Path: ":app", Dependencies: []ModuleDep{{Path: ":core", Configuration: "api"}}},
			{Path: ":core"},
			{Path: ":lib"},
		},
		Files: []File{{Path: "app/A.kt"}, {Path: "core/B.kt"}, {Path: "lib/C.kt"}},
		Symbols: []Symbol{
			{FQN: "demo.A", Signature: "demo.A", File: "app/A.kt", Kind: "class"},
			{FQN: "demo.C", Signature: "demo.C", File: "lib/C.kt", Kind: "class"},
		},
	}
	for _, b := range []*Blob{from, to} {
		if _, err := Save(root, b); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	d, err := Diff(root, from.CommitSHA, to.CommitSHA)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if d.From.CommitSHA != from.CommitSHA || d.To.CommitSHA != to.CommitSHA {
		t.Fatalf("ends mismatch: %+v / %+v", d.From, d.To)
	}
	if len(d.AddedFiles) != 1 || d.AddedFiles[0].Path != "lib/C.kt" {
		t.Fatalf("AddedFiles: %+v", d.AddedFiles)
	}
	if len(d.RemovedFiles) != 0 {
		t.Fatalf("RemovedFiles: %+v", d.RemovedFiles)
	}
	if len(d.AddedSymbols) != 1 || d.AddedSymbols[0].FQN != "demo.C" {
		t.Fatalf("AddedSymbols: %+v", d.AddedSymbols)
	}
	if len(d.RemovedSymbols) != 1 || d.RemovedSymbols[0].FQN != "demo.B" {
		t.Fatalf("RemovedSymbols: %+v", d.RemovedSymbols)
	}
	if len(d.AddedModules) != 1 || d.AddedModules[0] != ":lib" {
		t.Fatalf("AddedModules: %+v", d.AddedModules)
	}
	// Edge configuration changed (implementation -> api): old edge
	// removed, new edge added.
	wantAdded := ModuleEdge{From: ":app", To: ":core", Configuration: "api"}
	wantRemoved := ModuleEdge{From: ":app", To: ":core", Configuration: "implementation"}
	if len(d.AddedEdges) != 1 || d.AddedEdges[0] != wantAdded {
		t.Fatalf("AddedEdges: %+v", d.AddedEdges)
	}
	if len(d.RemovedEdges) != 1 || d.RemovedEdges[0] != wantRemoved {
		t.Fatalf("RemovedEdges: %+v", d.RemovedEdges)
	}
}

func TestDiffSurfacesMetricDeltas(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	from := &Blob{SchemaVersion: SchemaVersion, CommitSHA: "1111111111111111111111111111111111111111", CapturedAt: 1}
	to := &Blob{SchemaVersion: SchemaVersion, CommitSHA: "2222222222222222222222222222222222222222", CapturedAt: 2}
	for _, b := range []*Blob{from, to} {
		if _, err := Save(root, b); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	fromMetrics := &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     from.CommitSHA,
		Files:         []FileMetrics{{Path: "a.kt", LOC: 10}, {Path: "b.kt", LOC: 20}},
		Modules:       []ModuleMetrics{{Path: ":app", LOC: 30, FanIn: 0}},
	}
	toMetrics := &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     to.CommitSHA,
		Files:         []FileMetrics{{Path: "a.kt", LOC: 15}, {Path: "b.kt", LOC: 20}, {Path: "c.kt", LOC: 5}},
		Modules:       []ModuleMetrics{{Path: ":app", LOC: 40, FanIn: 1}},
	}
	for _, m := range []*Metrics{fromMetrics, toMetrics} {
		if _, err := SaveMetrics(root, m); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}

	d, err := Diff(root, from.CommitSHA, to.CommitSHA)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	loc := d.RepoMetrics["loc"]
	if loc.From != 30 || loc.To != 40 || loc.Delta != 10 {
		t.Fatalf("repo loc delta: %+v", loc)
	}
	files := d.RepoMetrics["files"]
	if files.Delta != 1 {
		t.Fatalf("repo files delta: %+v", files)
	}
	if mm := d.ModuleMetrics[":app"]["loc"]; mm.Delta != 10 {
		t.Fatalf("module :app loc delta: %+v", mm)
	}
	if fanIn := d.ModuleMetrics[":app"]["fan_in"]; fanIn.Delta != 1 {
		t.Fatalf("module :app fan_in delta: %+v", fanIn)
	}
}

func TestDiffRequiresBothShas(t *testing.T) {
	if _, err := Diff(t.TempDir(), "", "x"); err == nil {
		t.Fatal("expected error for empty fromSHA")
	}
}

func TestDiffRefusesIncompatibleSchemas(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: "1111111111111111111111111111111111111111"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion + 99, CommitSHA: "2222222222222222222222222222222222222222"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := Diff(root, "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"); err == nil {
		t.Fatal("expected schema mismatch error")
	}
}
