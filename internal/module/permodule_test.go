package module

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestBuildPerModuleIndex_AssignsFilesToModules(t *testing.T) {
	// Create a temp directory simulating a multi-module project.
	root := t.TempDir()

	// Module :app
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")
	if err := os.MkdirAll(appSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	appFile := filepath.Join(appSrc, "Main.kt")
	if err := os.WriteFile(appFile, []byte("fun main() { println(greet()) }"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Module :lib
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")
	if err := os.MkdirAll(libSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	libFile := filepath.Join(libSrc, "Greet.kt")
	if err := os.WriteFile(libFile, []byte("fun greet(): String = \"hello\""), 0o644); err != nil {
		t.Fatal(err)
	}

	// A file at the root (not in any module)
	rootFile := filepath.Join(root, "build.kt")
	if err := os.WriteFile(rootFile, []byte("val x = 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build module graph
	graph := NewModuleGraph(root)
	graph.Modules[":app"] = &Module{
		Path: ":app",
		Dir:  filepath.Join(root, "app"),
	}
	graph.Modules[":lib"] = &Module{
		Path: ":lib",
		Dir:  filepath.Join(root, "lib"),
	}

	// Parse files
	allPaths := []string{appFile, libFile, rootFile}
	var allFiles []*scanner.File
	for _, p := range allPaths {
		f, err := scanner.ParseFile(p)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", p, err)
		}
		allFiles = append(allFiles, f)
	}

	// Build per-module index
	pmi := BuildPerModuleIndex(graph, allFiles, 2)

	// Verify file assignment
	if len(pmi.ModuleFiles[":app"]) != 1 {
		t.Errorf("expected 1 file in :app, got %d", len(pmi.ModuleFiles[":app"]))
	}
	if len(pmi.ModuleFiles[":lib"]) != 1 {
		t.Errorf("expected 1 file in :lib, got %d", len(pmi.ModuleFiles[":lib"]))
	}
	if len(pmi.ModuleFiles["root"]) != 1 {
		t.Errorf("expected 1 file in root bucket, got %d", len(pmi.ModuleFiles["root"]))
	}
	if pmi.ModuleIndex["root"] == nil {
		t.Fatal("expected root bucket to have an index")
	}

	// Verify per-module indexes contain only that module's symbols
	libIdx := pmi.ModuleIndex[":lib"]
	if libIdx == nil {
		t.Fatal("expected :lib to have an index")
	}
	foundGreet := false
	for _, sym := range libIdx.Symbols {
		if sym.Name == "greet" {
			foundGreet = true
		}
		if sym.Name == "main" {
			t.Error(":lib index should not contain 'main' from :app")
		}
	}
	if !foundGreet {
		t.Error(":lib index should contain 'greet'")
	}

	appIdx := pmi.ModuleIndex[":app"]
	if appIdx == nil {
		t.Fatal("expected :app to have an index")
	}
	foundMain := false
	for _, sym := range appIdx.Symbols {
		if sym.Name == "main" {
			foundMain = true
		}
	}
	if !foundMain {
		t.Error(":app index should contain 'main'")
	}

	// Verify global index contains all symbols
	if pmi.GlobalIndex == nil {
		t.Fatal("expected GlobalIndex to be non-nil")
	}
	globalSymNames := make(map[string]bool)
	for _, sym := range pmi.GlobalIndex.Symbols {
		globalSymNames[sym.Name] = true
	}
	if !globalSymNames["greet"] || !globalSymNames["main"] {
		t.Error("global index should contain both 'greet' and 'main'")
	}
}
