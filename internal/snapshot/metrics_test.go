package snapshot

import (
	"path/filepath"
	"testing"
)

func TestComputeMetricsAggregatesFromBlob(t *testing.T) {
	blob := &Blob{
		SchemaVersion: SchemaVersion,
		CommitSHA:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CapturedAt:    1700000000000,
		Modules: []Module{
			{Path: ":app", Dir: "app", Dependencies: []ModuleDep{{Path: ":core", Configuration: "implementation"}}},
			{Path: ":core", Dir: "core", Consumers: []string{":app"}},
		},
		Files: []File{
			{Path: "app/Main.kt", Module: ":app", Language: "kotlin", Lines: 20, Bytes: 400},
			{Path: "app/Util.kt", Module: ":app", Language: "kotlin", Lines: 5, Bytes: 100},
			{Path: "core/Lib.kt", Module: ":core", Language: "kotlin", Lines: 10, Bytes: 200},
		},
		Symbols: []Symbol{
			{Name: "main", Kind: "function", Visibility: "public", File: "app/Main.kt"},
			{Name: "helper", Kind: "function", Visibility: "private", File: "app/Util.kt"},
			{Name: "Greet", Kind: "class", Visibility: "public", File: "core/Lib.kt"},
		},
	}

	got := computeMetrics(blob, nil)
	if got.SchemaVersion != MetricsSchemaVersion {
		t.Fatalf("SchemaVersion: %d", got.SchemaVersion)
	}
	if got.CommitSHA != blob.CommitSHA {
		t.Fatalf("CommitSHA mismatch: %s", got.CommitSHA)
	}
	if len(got.Files) != 3 {
		t.Fatalf("expected 3 file metrics, got %d", len(got.Files))
	}
	mainMetrics := got.Files[0]
	if mainMetrics.Path != "app/Main.kt" || mainMetrics.LOC != 20 || mainMetrics.Symbols != 1 || mainMetrics.PublicSymbols != 1 {
		t.Fatalf("Main.kt metrics: %+v", mainMetrics)
	}
	utilMetrics := got.Files[1]
	if utilMetrics.Path != "app/Util.kt" || utilMetrics.PublicSymbols != 0 {
		t.Fatalf("Util.kt metrics: %+v", utilMetrics)
	}

	if len(got.Modules) != 2 {
		t.Fatalf("expected 2 module metrics, got %d", len(got.Modules))
	}
	var appMod, coreMod ModuleMetrics
	for _, m := range got.Modules {
		switch m.Path {
		case ":app":
			appMod = m
		case ":core":
			coreMod = m
		}
	}
	if appMod.Files != 2 || appMod.LOC != 25 || appMod.Symbols != 2 || appMod.PublicSymbols != 1 || appMod.FanIn != 0 || appMod.FanOut != 1 {
		t.Fatalf(":app rollup: %+v", appMod)
	}
	if coreMod.Files != 1 || coreMod.LOC != 10 || coreMod.FanIn != 1 || coreMod.FanOut != 0 {
		t.Fatalf(":core rollup: %+v", coreMod)
	}
}

func TestSaveLoadMetricsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	in := &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CapturedAt:    1700000000000,
		Files:         []FileMetrics{{Path: "a.kt", Module: ":app", Language: "kotlin", LOC: 10, Symbols: 2, Cyclomatic: 3}},
		Modules:       []ModuleMetrics{{Path: ":app", Files: 1, LOC: 10, Symbols: 2, Cyclomatic: 3}},
	}
	if _, err := SaveMetrics(root, in); err != nil {
		t.Fatalf("SaveMetrics: %v", err)
	}
	got, err := LoadMetrics(root, in.CommitSHA)
	if err != nil {
		t.Fatalf("LoadMetrics: %v", err)
	}
	if len(got.Files) != 1 || got.Files[0].Cyclomatic != 3 {
		t.Fatalf("metrics round-trip mismatch: %+v", got)
	}
}

func TestFileCyclomaticHandlesEmptyAndComments(t *testing.T) {
	if got := fileCyclomatic(nil); got != 1 {
		t.Fatalf("empty cyclomatic: got %d, want 1", got)
	}
	if got := fileCyclomatic([]string{"// if (true) return"}); got != 1 {
		t.Fatalf("comment-only cyclomatic: got %d, want 1", got)
	}
	got := fileCyclomatic([]string{
		"fun greet(x: Int) {",
		"  if (x > 0 && x < 10) return",
		"  for (i in 0..x) println(i)",
		"}",
	})
	if got < 4 {
		t.Fatalf("expected cyclomatic >= 4, got %d", got)
	}
}
