package di

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestBuildGraph_CapturesScopesAcrossModules(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "settings.gradle.kts"), `include(":app", ":feature", ":shared")`)
	writeFile(t, filepath.Join(root, "app", "src", "main", "kotlin", "com", "app", "UserRepository.kt"), `package com.app

import com.feature.Router

@Singleton
class UserRepository @Inject constructor(
    private val router: Router,
)`)
	writeFile(t, filepath.Join(root, "feature", "src", "main", "kotlin", "com", "feature", "Router.kt"), `package com.feature

import com.shared.Api

@ActivityScoped
class Router @Inject constructor(
    private val api: Api,
)`)
	writeFile(t, filepath.Join(root, "shared", "src", "main", "kotlin", "com", "shared", "Api.kt"), `package com.shared

@Singleton
class Api @Inject constructor()`)

	moduleGraph, err := module.DiscoverModules(root)
	if err != nil {
		t.Fatalf("DiscoverModules: %v", err)
	}
	if moduleGraph == nil {
		t.Fatal("expected module graph")
	}

	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		t.Fatalf("CollectKotlinFiles: %v", err)
	}
	files, errs := scanner.ScanFiles(paths, 2)
	if len(errs) > 0 {
		t.Fatalf("ScanFiles returned errors: %v", errs)
	}

	graph := BuildGraph(files, moduleGraph)
	if got := len(graph.Bindings); got != 3 {
		t.Fatalf("expected 3 bindings, got %d", got)
	}

	repo := graph.Binding("com.app.UserRepository")
	if repo == nil {
		t.Fatal("expected repository binding")
	}
	if repo.ModulePath != ":app" {
		t.Fatalf("expected repository module :app, got %q", repo.ModulePath)
	}
	if repo.Scope.Name != "Singleton" || !repo.Scope.Known {
		t.Fatalf("expected known Singleton scope, got %+v", repo.Scope)
	}
	if len(repo.Dependencies) != 1 {
		t.Fatalf("expected 1 repository dependency, got %d", len(repo.Dependencies))
	}
	if repo.Dependencies[0].Target != "com.feature.Router" {
		t.Fatalf("expected dependency target com.feature.Router, got %q", repo.Dependencies[0].Target)
	}

	router := graph.Binding("com.feature.Router")
	if router == nil {
		t.Fatal("expected router binding")
	}
	if router.ModulePath != ":feature" {
		t.Fatalf("expected router module :feature, got %q", router.ModulePath)
	}
	if router.Scope.Name != "ActivityScoped" || !router.Scope.Known {
		t.Fatalf("expected known ActivityScoped scope, got %+v", router.Scope)
	}
	if len(router.Dependencies) != 1 || router.Dependencies[0].Target != "com.shared.Api" {
		t.Fatalf("expected router -> com.shared.Api edge, got %+v", router.Dependencies)
	}

	violations := graph.ScopeViolations()
	if len(violations) != 1 {
		t.Fatalf("expected 1 scope violation, got %d", len(violations))
	}
	if violations[0].Root.FQN != "com.app.UserRepository" {
		t.Fatalf("expected root com.app.UserRepository, got %q", violations[0].Root.FQN)
	}
	if violations[0].Offender.FQN != "com.feature.Router" {
		t.Fatalf("expected offender com.feature.Router, got %q", violations[0].Offender.FQN)
	}
	wantPath := []string{"com.app.UserRepository", "com.feature.Router"}
	if !reflect.DeepEqual(violations[0].Path, wantPath) {
		t.Fatalf("expected path %v, got %v", wantPath, violations[0].Path)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
