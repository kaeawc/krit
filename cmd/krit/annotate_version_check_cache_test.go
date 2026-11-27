package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// AnnotateVersionCheck reports only in Android library modules, which a build script can declare through a
// version-catalog plugin alias. The per-file cache key folds in the module plugins, so a catalog-only edit that turns
// the library into an app, or back, must change a warm run's findings exactly as a cache-free run's. A catalog edit
// that leaves every plugin id alone must keep them. Each step edits forward only, so a warm run can't pass by
// replaying an entry an earlier state wrote.
func TestAnnotateVersionCheckWarmRunFollowsCatalogPluginID(t *testing.T) {
	const (
		catalogRel = "gradle/libs.versions.toml"
		build      = "plugins {\n    alias(libs.plugins.mod)\n}\n\nandroid {\n    namespace = \"com.example\"\n    defaultConfig {\n        minSdk = 21\n    }\n}\n"
		source     = "package com.example\n\nimport android.os.Build\n\nobject Versions {\n    fun isAtLeastP() = Build.VERSION.SDK_INT >= 28\n}\n"
	)
	catalogOf := func(id, version string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"" + version + "\" }\n"
	}
	project := filepath.Join(t.TempDir(), "project")
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.gradle.kts", "include(\":lib\")\n")
	write("lib/build.gradle.kts", build)
	write("lib/src/main/AndroidManifest.xml", "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n")
	write("lib/src/main/java/com/example/Versions.kt", source)
	write(catalogRel, catalogOf("com.android.library", "8.5.0"))
	count := func(args ...string) int {
		t.Helper()
		stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--enable-rules", "AnnotateVersionCheck", "-f", "json"}, args...), project)...)
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "AnnotateVersionCheck" {
				n++
			}
		}
		return n
	}
	truth := func() int { return count("--no-cache", "--no-cross-file-cache") }
	if warm, cold := count(), truth(); warm != 1 || cold != 1 {
		t.Fatalf("cold run: %d findings, cache-free %d, want 1", warm, cold)
	}
	for _, step := range []struct {
		catalog string
		want    int
	}{
		{catalogOf("com.android.application", "8.5.0"), 0},
		{catalogOf("com.android.library", "8.5.0") + "# a library again\n", 1},
		// A version bump leaves the plugin id, so the findings stay.
		{catalogOf("com.android.library", "8.6.0"), 1},
	} {
		write(catalogRel, step.catalog)
		if warm, cold := count(), truth(); warm != cold || cold != step.want {
			t.Fatalf("after writing %q: warm run %d findings, cache-free %d, want %d for both", step.catalog, warm, cold, step.want)
		}
	}
}
