package module

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseStandardProjectDeps(t *testing.T) {
	content := `
dependencies {
  implementation(project(":core-util"))
  api(project(":lib"))
  testImplementation(project(":test-utils"))
  compileOnly(project(":annotations"))
}
`
	graph := NewModuleGraph("/tmp")
	deps := parseDeps(content, graph)
	if len(deps) != 4 {
		t.Fatalf("expected 4 deps, got %d: %v", len(deps), deps)
	}

	expected := map[string]string{
		":core-util":   "implementation",
		":lib":         "api",
		":test-utils":  "testImplementation",
		":annotations": "compileOnly",
	}
	for _, d := range deps {
		if want, ok := expected[d.ModulePath]; ok {
			if d.Configuration != want {
				t.Errorf("dep %s: config = %q, want %q", d.ModulePath, d.Configuration, want)
			}
		} else {
			t.Errorf("unexpected dep: %s (%s)", d.ModulePath, d.Configuration)
		}
	}
}

func TestParseFlavorQualifiedDeps(t *testing.T) {
	content := `
dependencies {
  "playImplementation"(project(":billing"))
  "nightlyImplementation"(project(":billing"))
  "spinnerImplementation"(project(":spinner"))
}
`
	graph := NewModuleGraph("/tmp")
	deps := parseDeps(content, graph)
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d: %v", len(deps), deps)
	}

	foundPlay := false
	foundNightly := false
	for _, d := range deps {
		if d.Configuration == "playImplementation" && d.ModulePath == ":billing" {
			foundPlay = true
		}
		if d.Configuration == "nightlyImplementation" && d.ModulePath == ":billing" {
			foundNightly = true
		}
	}
	if !foundPlay {
		t.Error("missing playImplementation(:billing)")
	}
	if !foundNightly {
		t.Error("missing nightlyImplementation(:billing)")
	}
}

func TestParseTypesafeAccessorDeps(t *testing.T) {
	content := `
dependencies {
  api(projects.circuitRuntime)
  api(projects.backstack)
  implementation(projects.internalTestUtils)
}
`
	graph := NewModuleGraph("/tmp")
	// Register known modules so lookup works.
	graph.Modules[":circuit-runtime"] = &Module{Path: ":circuit-runtime"}
	graph.Modules[":backstack"] = &Module{Path: ":backstack"}
	graph.Modules[":internal-test-utils"] = &Module{Path: ":internal-test-utils"}

	deps := parseDeps(content, graph)
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d: %v", len(deps), deps)
	}

	found := make(map[string]bool)
	for _, d := range deps {
		found[d.ModulePath] = true
	}
	for _, want := range []string{":circuit-runtime", ":backstack", ":internal-test-utils"} {
		if !found[want] {
			t.Errorf("missing dep %s", want)
		}
	}
}

func TestParseTypesafeNestedAccessorDeps(t *testing.T) {
	content := `
dependencies {
  implementation(projects.samples.star.benchmark)
}
`
	graph := NewModuleGraph("/tmp")
	graph.Modules[":samples:star:benchmark"] = &Module{Path: ":samples:star:benchmark"}

	deps := parseDeps(content, graph)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d: %v", len(deps), deps)
	}
	if deps[0].ModulePath != ":samples:star:benchmark" {
		t.Errorf("expected :samples:star:benchmark, got %q", deps[0].ModulePath)
	}
}

func TestParseTestFixturesDep(t *testing.T) {
	content := `
dependencies {
  testImplementation(testFixtures(project(":libsignal-service")))
}
`
	graph := NewModuleGraph("/tmp")
	deps := parseDeps(content, graph)
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d: %v", len(deps), deps)
	}
	if deps[0].ModulePath != ":libsignal-service" {
		t.Errorf("expected :libsignal-service, got %q", deps[0].ModulePath)
	}
	if deps[0].Configuration != "testImplementation" {
		t.Errorf("expected testImplementation, got %q", deps[0].Configuration)
	}
}

func TestDetectMavenPublish(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"kts id", `id("maven-publish")`, true},
		{"alias", `alias(libs.plugins.mavenPublish)`, true},
		{"groovy apply", `apply plugin: "maven-publish"`, true},
		{"no publish", `id("kotlin-android")`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMavenPublish(tt.content)
			if got != tt.want {
				t.Errorf("detectMavenPublish = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsumersReverseIndex(t *testing.T) {
	graph := NewModuleGraph("/tmp")
	graph.Modules[":app"] = &Module{Path: ":app", Dir: "/tmp/app"}
	graph.Modules[":lib"] = &Module{Path: ":lib", Dir: "/tmp/lib"}
	graph.Modules[":core"] = &Module{Path: ":core", Dir: "/tmp/core"}

	// Create build files.
	appDir := filepath.Join(t.TempDir(), "app")
	libDir := filepath.Join(t.TempDir(), "lib")
	os.MkdirAll(appDir, 0755)
	os.MkdirAll(libDir, 0755)

	graph.Modules[":app"].Dir = appDir
	graph.Modules[":lib"].Dir = libDir
	graph.Modules[":core"].Dir = t.TempDir() // no build file

	os.WriteFile(filepath.Join(appDir, "build.gradle.kts"), []byte(`
dependencies {
  implementation(project(":lib"))
  implementation(project(":core"))
}
`), 0644)

	os.WriteFile(filepath.Join(libDir, "build.gradle.kts"), []byte(`
dependencies {
  api(project(":core"))
}
`), 0644)

	err := ParseAllDependencies(graph)
	if err != nil {
		t.Fatalf("ParseAllDependencies: %v", err)
	}

	// :core is consumed by :app and :lib.
	consumers := graph.Consumers[":core"]
	if len(consumers) != 2 {
		t.Fatalf("expected 2 consumers of :core, got %d: %v", len(consumers), consumers)
	}

	// :lib is consumed by :app.
	consumers = graph.Consumers[":lib"]
	if len(consumers) != 1 {
		t.Fatalf("expected 1 consumer of :lib, got %d", len(consumers))
	}
	if consumers[0] != ":app" {
		t.Errorf("expected :app as consumer of :lib, got %q", consumers[0])
	}
}

