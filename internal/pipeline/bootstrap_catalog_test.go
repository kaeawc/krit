package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

const catalogRequestSource = `package com.example

import androidx.activity.ComponentActivity
import androidx.activity.result.contract.ActivityResultContracts

class A : ComponentActivity() {
    val launcher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { }
}
`

const catalogRequestToml = "[versions]\nfragment = \"1.2.0\"\n\n[libraries]\n" +
	"androidx-fragment = { module = \"androidx.fragment:fragment\", version.ref = \"fragment\" }\n"

// catalogRequestProject writes a module whose build file depends on libs.androidx.fragment, without a version catalog
// yet, and returns the source path.
func catalogRequestProject(t *testing.T) (root, src string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.gradle.kts", "include(\":app\")\n")
	write("app/build.gradle.kts", "dependencies {\n    implementation(libs.androidx.fragment)\n}\n")
	src = filepath.Join(root, "app/src/main/java/com/example/A.kt")
	write("app/src/main/java/com/example/A.kt", catalogRequestSource)
	return root, src
}

func addCatalog(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gradle", "libs.versions.toml"), []byte(catalogRequestToml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fragmentRule(t *testing.T) []*api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "InvalidFragmentVersionForActivityResult" {
			return []*api.Rule{r}
		}
	}
	t.Fatal("InvalidFragmentVersionForActivityResult is not registered")
	return nil
}

func analyzeCatalogBuffer(t *testing.T, a *SingleFileAnalyzer, src string) int {
	t.Helper()
	cols, _, err := a.AnalyzeBufferColumns(context.Background(), src, []byte(catalogRequestSource))
	if err != nil {
		t.Fatal(err)
	}
	return cols.Len()
}

// A long-lived analyzer (LSP, daemon buffer analysis) must see a version catalog added between two requests, as a
// fresh analyzer does (#1784). The catalog discovery memo is scoped to one request there, not to the analyzer.
func TestSingleFileAnalyzerSeesCatalogAddedBetweenRequests(t *testing.T) {
	root, src := catalogRequestProject(t)
	active := fragmentRule(t)
	a := NewSingleFileAnalyzer(active, nil)
	if got := analyzeCatalogBuffer(t, a, src); got != 0 {
		t.Fatalf("without a catalog: got %d findings, want 0", got)
	}
	addCatalog(t, root)
	got := analyzeCatalogBuffer(t, a, src)
	// Built after the second request, so it cannot lend that request its memo.
	want := analyzeCatalogBuffer(t, NewSingleFileAnalyzer(active, nil), src)
	if want != 1 {
		t.Fatalf("fresh analyzer with the catalog: got %d findings, want 1", want)
	}
	if got != want {
		t.Fatalf("same analyzer after adding the catalog: got %d findings, fresh analyzer gets %d", got, want)
	}
	// AnalyzeFileColumns, which the LSP uses for cached files, is a request too.
	file, err := ParseSingle(context.Background(), src, []byte(catalogRequestSource))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "gradle", "libs.versions.toml")); err != nil {
		t.Fatal(err)
	}
	if cols := a.AnalyzeFileColumns(file); cols.Len() != 0 {
		t.Fatalf("AnalyzeFileColumns after removing the catalog: got %d findings, want 0", cols.Len())
	}
}

// The daemon builds a dispatcher for each project run while its buffer analyzer lives on. A buffer request must read
// its own analyzer's facts, not the memo of whichever dispatcher was built last.
func TestSingleFileAnalyzerIgnoresALaterDispatchersCatalogMemo(t *testing.T) {
	root, src := catalogRequestProject(t)
	active := fragmentRule(t)
	a := NewSingleFileAnalyzer(active, nil)
	projectRun := rules.NewDispatcher(active, nil)
	file, err := ParseSingle(context.Background(), src, []byte(catalogRequestSource))
	if err != nil {
		t.Fatal(err)
	}
	if cols := projectRun.Run(file); cols.Len() != 0 {
		t.Fatalf("project run without a catalog: got %d findings, want 0", cols.Len())
	}
	addCatalog(t, root)
	if got := analyzeCatalogBuffer(t, a, src); got != 1 {
		t.Fatalf("buffer request after the catalog was added: got %d findings, want 1", got)
	}
}
