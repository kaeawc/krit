package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

const gradleFactsSource = `package com.example

fun log() {
    if (BuildConfig.DEBUG) println("debug")
}
`

const (
	gradleFactsApplication = "plugins {\n    id(\"com.android.application\")\n}\n"
	gradleFactsLibrary     = "plugins {\n    id(\"com.android.library\")\n}\n"
)

// gradleFactsProject writes an application module and returns its build script and source paths.
func gradleFactsProject(t *testing.T) (buildScript, src string) {
	t.Helper()
	root := t.TempDir()
	buildScript = filepath.Join(root, "lib", "build.gradle.kts")
	src = filepath.Join(root, "lib", "src", "main", "java", "com", "example", "Log.kt")
	for path, content := range map[string]string{
		filepath.Join(root, "settings.gradle.kts"): "include(\":lib\")\n",
		buildScript: gradleFactsApplication,
		src:         gradleFactsSource,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return buildScript, src
}

// BuildConfigDebugInLibrary fires only in an Android library module, which it learns from the module's build script
// through a directory-keyed Gradle fact.
func buildConfigDebugInLibraryRule(t *testing.T) []*api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == "BuildConfigDebugInLibrary" {
			return []*api.Rule{r}
		}
	}
	t.Fatal("BuildConfigDebugInLibrary is not registered")
	return nil
}

func analyzeGradleFactsBuffer(t *testing.T, a *SingleFileAnalyzer, src string) int {
	t.Helper()
	cols, _, err := a.AnalyzeBufferColumns(context.Background(), src, []byte(gradleFactsSource))
	if err != nil {
		t.Fatal(err)
	}
	return cols.Len()
}

func writeBuildScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A long-lived analyzer (LSP, daemon buffer analysis, MCP) must see a build-script edit made between two requests,
// as a fresh analyzer does (#1788): module Gradle facts are scoped to one request there.
func TestSingleFileAnalyzerSeesBuildScriptEditBetweenRequests(t *testing.T) {
	buildScript, src := gradleFactsProject(t)
	active := buildConfigDebugInLibraryRule(t)
	a := NewSingleFileAnalyzer(active, nil)
	if got := analyzeGradleFactsBuffer(t, a, src); got != 0 {
		t.Fatalf("application module: got %d findings, want 0", got)
	}
	writeBuildScript(t, buildScript, gradleFactsLibrary)
	got := analyzeGradleFactsBuffer(t, a, src)
	want := analyzeGradleFactsBuffer(t, NewSingleFileAnalyzer(active, nil), src)
	if want != 1 {
		t.Fatalf("fresh analyzer on the library module: got %d findings, want 1", want)
	}
	if got != want {
		t.Fatalf("same analyzer after the module became a library: got %d findings, fresh analyzer gets %d", got, want)
	}
	// AnalyzeFileColumns, which the LSP uses for cached files, is a request too.
	file, err := ParseSingle(context.Background(), src, []byte(gradleFactsSource))
	if err != nil {
		t.Fatal(err)
	}
	writeBuildScript(t, buildScript, gradleFactsApplication)
	if cols := a.AnalyzeFileColumns(file); cols.Len() != 0 {
		t.Fatalf("AnalyzeFileColumns after the module became an application again: got %d findings, want 0", cols.Len())
	}
}

// The daemon builds a dispatcher for each project run while its buffer analyzer lives on. A buffer request must not
// read the module facts that run cached through the process-wide facts pointer.
func TestSingleFileAnalyzerIgnoresALaterDispatchersGradleFacts(t *testing.T) {
	buildScript, src := gradleFactsProject(t)
	active := buildConfigDebugInLibraryRule(t)
	a := NewSingleFileAnalyzer(active, nil)
	projectRun := rules.NewDispatcher(active, nil)
	file, err := ParseSingle(context.Background(), src, []byte(gradleFactsSource))
	if err != nil {
		t.Fatal(err)
	}
	if cols := projectRun.Run(file); cols.Len() != 0 {
		t.Fatalf("project run on the application module: got %d findings, want 0", cols.Len())
	}
	writeBuildScript(t, buildScript, gradleFactsLibrary)
	if got := analyzeGradleFactsBuffer(t, a, src); got != 1 {
		t.Fatalf("buffer request after the module became a library: got %d findings, want 1", got)
	}
}
