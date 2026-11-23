package librarymodel

import (
	"os"
	"path/filepath"
	"testing"
)

// Maven artifacts declared for rules_jvm_external give a Bazel project the
// same presence facts a Gradle dependency block does, without claiming the
// dependency list is complete.
func TestProfileFromPathsReadsBazelArtifacts(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "MODULE.bazel")
	if err := os.WriteFile(module, []byte(`
maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven")
maven.install(artifacts = ["androidx.room:room-runtime:2.6.1", "androidx.room:room-runtime:2.6.1"])
`), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, "maven_install.json")
	if err := os.WriteFile(lock, []byte(`{"artifacts": {"app.cash.sqldelight:runtime": {"version": "2.0.2"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	profile := ProfileFromPaths([]string{module, lock})
	if !profile.HasDependency("androidx.room", "room-runtime") || !profile.HasDependency("app.cash.sqldelight", "runtime") {
		t.Fatalf("dependencies = %+v", profile.Dependencies)
	}
	if len(profile.Dependencies) != 2 {
		t.Errorf("duplicate artifacts not merged: %+v", profile.Dependencies)
	}
	if profile.DependencyExtractionComplete || profile.HasGradle {
		t.Error("Bazel artifacts must not make dependency extraction complete")
	}
	if profile.ProvesDependencyAbsent(Coordinate{Group: "com.example", Name: "missing"}) {
		t.Error("a library missing from Bazel artifacts must not count as absent")
	}
	facts := FactsForProfile(profile)
	if !facts.Database.Room.Enabled || !facts.Database.SQLDelight.Enabled {
		t.Error("Room and SQLDelight facts not enabled from Bazel artifacts")
	}
}
