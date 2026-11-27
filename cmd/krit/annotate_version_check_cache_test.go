package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

// A scan below a module root, such as lib/src/main/java, holds no build script, yet AnnotateVersionCheck reads the
// module's plugins and ObsoleteSdkInt its minSdk from it. A warm run must follow an edit to that script exactly as a
// cache-free run does, at every scan root; so must commenting out the plugin declaration, which no longer applies it.
// Each step edits forward only.
func TestModuleFactsWarmRunFollowsBuildScriptBelowModuleRoot(t *testing.T) {
	scriptOf := func(plugin string, minSdk int) string {
		return "plugins {\n    " + plugin + "\n}\n\nandroid {\n    namespace = \"com.example\"\n    defaultConfig {\n        minSdk = " +
			strconv.Itoa(minSdk) + "\n    }\n}\n"
	}
	const (
		library     = `id("com.android.library")`
		application = `id("com.android.application")`
		source      = "package com.example\n\nimport android.os.Build\n\nobject Versions {\n    fun isAtLeastP() = Build.VERSION.SDK_INT >= 28\n\n" +
			"    fun legacy(): Int {\n        if (Build.VERSION.SDK_INT >= 23) {\n            return 1\n        }\n        return 0\n    }\n}\n"
	)
	type counts struct{ annotate, obsolete int }
	for _, scan := range []string{"", "lib/src/main", "lib/src/main/java"} {
		t.Run("scan /"+scan, func(t *testing.T) {
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
			write("lib/build.gradle.kts", scriptOf(library, 21))
			write("lib/src/main/AndroidManifest.xml", "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n")
			write("lib/src/main/java/com/example/Versions.kt", source)
			count := func(args ...string) counts {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--enable-rules", "AnnotateVersionCheck,ObsoleteSdkInt", "-f", "json"}, args...), filepath.Join(project, scan))...)
				var report struct {
					Findings []struct {
						Rule string `json:"rule"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				var c counts
				for _, f := range report.Findings {
					switch f.Rule {
					case "AnnotateVersionCheck":
						c.annotate++
					case "ObsoleteSdkInt":
						c.obsolete++
					}
				}
				return c
			}
			truth := func() counts { return count("--no-cache", "--no-cross-file-cache") }
			if warm, cold := count(), truth(); warm != (counts{1, 0}) || cold != warm {
				t.Fatalf("cold run: %+v, cache-free %+v, want {1 0}", warm, cold)
			}
			for _, step := range []struct {
				name, script string
				want         counts
			}{
				{"commented-out library plugin", scriptOf("// "+library, 21), counts{0, 0}},
				{"library plugin restored", scriptOf(library, 21) + "// restored\n", counts{1, 0}},
				{"application plugin", scriptOf(application, 21), counts{0, 0}},
				{"minSdk raised", scriptOf(application, 30), counts{0, 2}},
			} {
				write("lib/build.gradle.kts", step.script)
				if warm, cold := count(), truth(); warm != cold || cold != step.want {
					t.Fatalf("after %s: warm run %+v, cache-free %+v, want %+v for both", step.name, warm, cold, step.want)
				}
			}
		})
	}
}
