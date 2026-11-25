package gradlemodel

import (
	"os"
	"path/filepath"
	"testing"
)

// The krit Bazel module's export_model writes .krit/bazel-model/ at the
// workspace root, which has a MODULE.bazel and no settings.gradle.
func TestDiscoverBazelModel(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "MODULE.bazel"), "")
	write(t, filepath.Join(root, ".git", "HEAD"), "")
	child := filepath.Join(root, "java", "com", "app")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	bazelModel := filepath.Join(root, ".krit", "bazel-model")
	if err := os.MkdirAll(bazelModel, 0755); err != nil {
		t.Fatal(err)
	}
	if got := Discover(child); got != bazelModel {
		t.Fatalf("discover = %q, want %q", got, bazelModel)
	}

	// A directory with both prefers the Gradle model.
	gradleModel := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(gradleModel, 0755); err != nil {
		t.Fatal(err)
	}
	if got := Discover(child); got != gradleModel {
		t.Fatalf("discover with both = %q, want %q", got, gradleModel)
	}
}

// A Bazel model in the schema export_model writes loads like a Gradle one:
// absolute paths, one project per target, the label as the project path.
func TestLoadBazelModel(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "bazel-out", "k8-fastbuild", "bin", "lib", "lib.abi.jar")
	write(t, jar, "jar")
	pkg := filepath.Join(root, "app")
	write(t, filepath.Join(pkg, "App.kt"), "package app\n")
	write(t, filepath.Join(root, ".krit", "bazel-model", "app_app.krit-model.json"), `{
  "schema": 2,
  "projects": [{
    "path": "//app:app",
    "dir": "`+filepath.ToSlash(pkg)+`",
    "sourceSets": [{
      "name": "//app:app",
      "kind": "main",
      "platform": "jvm",
      "sourceDirs": ["`+filepath.ToSlash(pkg)+`"],
      "classpath": ["`+filepath.ToSlash(jar)+`"],
      "projectDeps": ["//lib:lib"],
      "kotlin": {"languageVersion": "2.4", "apiVersion": "2.4", "freeCompilerArgs": ["-Xlambdas=class"]}
    }]
  }]
}
`)
	model, warnings, err := Load(Discover(pkg))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings: %v", warnings)
	}
	if len(model.Projects) != 1 || model.Projects[0].Path != "//app:app" {
		t.Fatalf("projects: %+v", model.Projects)
	}
	if cp := model.CompileClasspath(); len(cp) != 1 || cp[0] != filepath.ToSlash(jar) {
		t.Fatalf("classpath: %v", cp)
	}
	if got := model.EffectiveKotlinSettings().LanguageVersion; got != "2.4" {
		t.Fatalf("language version: %q", got)
	}
}
