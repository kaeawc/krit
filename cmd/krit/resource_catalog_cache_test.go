package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LibraryCustomView reports a hardcoded namespace only in a library module, which it reads from the module's build
// script, resolving a plugin alias through the version catalogs the script's search reaches. A catalog-only edit that
// turns the module into an app must change a warm run's findings exactly as a cache-free run's (#1883). Each case
// edits forward only, so a warm run can't pass by replaying an entry an earlier state wrote.
func TestResourceRulesWarmRunFollowsVersionCatalog(t *testing.T) {
	const (
		settingsRel = "settings.gradle.kts"
		buildRel    = "lib/build.gradle.kts"
		layoutRel   = "lib/src/main/res/layout/view.xml"
		layout      = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<LinearLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    xmlns:app=\"http://schemas.android.com/apk/res/com.example.lib\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\" />\n"
		config      = "android-lint:\n  LibraryCustomView:\n    active: true\n"
	)
	catalogOf := func(id string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
	}
	buildOf := func(accessor string) string {
		return "plugins {\n    alias(" + accessor + ".plugins.mod)\n}\n\nandroid {\n    namespace = \"com.example.lib\"\n}\n"
	}
	app, library := catalogOf("com.android.application"), catalogOf("com.android.library")
	type edit struct {
		rel, body string
		want      int
	}
	for _, test := range []struct {
		name string
		// noSettings leaves out the settings file.
		noSettings bool
		// catalogRel is the catalog before the cold run, which the build script reaches as accessor.
		catalogRel, accessor string
		// scan is the scanned path under the project; "" scans the project root.
		scan  string
		edits []edit
	}{
		{"catalog plugin id", false, "gradle/libs.versions.toml", "libs", "", []edit{
			{"gradle/libs.versions.toml", app, 0},
			{"gradle/libs.versions.toml", library + "# a library again\n", 1},
		}},
		{"no settings file, catalog plugin id", true, "gradle/libs.versions.toml", "libs", "", []edit{
			{"gradle/libs.versions.toml", app, 0},
		}},
		{"differently named catalog", false, "gradle/deps.versions.toml", "deps", "", []edit{
			{"gradle/deps.versions.toml", app, 0},
			{"gradle/deps.versions.toml", library + "# a library again\n", 1},
		}},
		{"no settings file, differently named catalog", true, "gradle/deps.versions.toml", "deps", "", []edit{
			{"gradle/deps.versions.toml", app, 0},
		}},
		// A scan of the res directory only holds no build file.
		{"res scan, catalog plugin id", false, "gradle/deps.versions.toml", "deps", "lib/src/main/res", []edit{
			{"gradle/deps.versions.toml", app, 0},
		}},
		// The search from the module directory stops at the nearest directory with a catalog.
		{"nearer catalog created", false, "gradle/libs.versions.toml", "libs", "", []edit{
			{"lib/gradle/libs.versions.toml", app, 0},
		}},
		{"build script edit", false, "gradle/libs.versions.toml", "libs", "", []edit{
			{buildRel, "plugins {\n    id(\"com.android.application\")\n}\n", 0},
		}},
		// Catalogs the search never reads leave the findings alone.
		{"unrelated catalogs", false, "gradle/libs.versions.toml", "libs", "", []edit{
			{"tools/gradle/libs.versions.toml", app, 1},
			{"gradle/notes.toml", app, 1},
			{"gradle/libs.versions.toml", app, 0},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			if !test.noSettings {
				write(settingsRel, "include(\":lib\")\n")
			}
			write("krit.yml", config)
			write(buildRel, buildOf(test.accessor))
			write(test.catalogRel, library)
			write(layoutRel, layout)
			scan := project
			if test.scan != "" {
				scan = filepath.Join(project, test.scan)
			}
			count := func(args ...string) int {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "-f", "json", "--config", filepath.Join(project, "krit.yml")}, args...), scan)...)
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
					if strings.EqualFold(f.Rule, "LibraryCustomView") {
						n++
					}
				}
				return n
			}
			truth := func() int { return count("--no-cache", "--no-cross-file-cache") }
			if warm, cold := count(), truth(); warm != 1 || cold != 1 {
				t.Fatalf("cold run: %d findings, cache-free %d, want 1", warm, cold)
			}
			if warm := count(); warm != 1 {
				t.Fatalf("unchanged warm run: %d findings, want 1", warm)
			}
			for _, e := range test.edits {
				write(e.rel, e.body)
				if warm, cold := count(), truth(); warm != cold || cold != e.want {
					t.Fatalf("after writing %s: warm run %d findings, cache-free %d, want %d for both", e.rel, warm, cold, e.want)
				}
			}
		})
	}
}
