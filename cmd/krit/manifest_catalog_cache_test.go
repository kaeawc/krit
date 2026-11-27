package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Every manifest rule reads whether its module is an app or a library from the build script, which can apply its
// plugin through a version-catalog alias. DataExtractionRules skips library modules, so a catalog-only edit that turns
// the module into a library must change a warm run's findings exactly as a cache-free run's (#1853). Each case edits
// forward only, so a warm run can't pass by replaying an entry an earlier state wrote.
func TestManifestRulesWarmRunFollowsVersionCatalog(t *testing.T) {
	const (
		settingsRel = "settings.gradle.kts"
		catalogRel  = "gradle/libs.versions.toml"
		buildRel    = "app/build.gradle.kts"
		manifestRel = "app/src/main/AndroidManifest.xml"
		build       = "plugins {\n    alias(libs.plugins.mod)\n}\n\nandroid {\n    defaultConfig {\n        minSdk = 21\n        targetSdk = 33\n    }\n}\n"
		manifest    = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <application android:allowBackup=\"false\" />\n</manifest>\n"
	)
	catalogOf := func(id string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
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
		// catalog is the root catalog before the cold run; "" writes none.
		catalog string
		// scan is the scanned path under the project; "" scans the project root.
		scan  string
		cold  int
		edits []edit
	}{
		{"catalog plugin id", false, app, "", 1, []edit{
			{catalogRel, library, 0},
			{catalogRel, app + "# back to an app\n", 1},
		}},
		{"no settings file, catalog plugin id", true, app, "", 1, []edit{
			{catalogRel, library, 0},
			{catalogRel, app + "# back to an app\n", 1},
		}},
		// A scan of the module only holds no catalog and no settings file.
		{"module scan, catalog plugin id", false, app, "app", 1, []edit{
			{catalogRel, library, 0},
		}},
		// Without a catalog the alias resolves to nothing, so the module is neither an app nor a library.
		{"catalog created", false, "", "", 1, []edit{
			{catalogRel, library, 0},
		}},
		{"no settings file, catalog created", true, "", "", 1, []edit{
			{catalogRel, library, 0},
		}},
		// The search from the module directory stops at the nearest directory with a catalog.
		{"nearer catalog created", false, app, "", 1, []edit{
			{"app/gradle/libs.versions.toml", library, 0},
		}},
		// Catalogs the search never reads leave the findings alone.
		{"unrelated catalogs", false, app, "", 1, []edit{
			{"tools/gradle/libs.versions.toml", library, 1},
			{"gradle/notes.toml", library, 1},
			{catalogRel, library, 0},
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
				write(settingsRel, "include(\":app\")\n")
			}
			write(buildRel, build)
			write(manifestRel, manifest)
			if test.catalog != "" {
				write(catalogRel, test.catalog)
			}
			scan := project
			if test.scan != "" {
				scan = filepath.Join(project, test.scan)
			}
			count := func(args ...string) int {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "-f", "json"}, args...), scan)...)
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
					if f.Rule == "DataExtractionRules" {
						n++
					}
				}
				return n
			}
			truth := func() int { return count("--no-cache", "--no-cross-file-cache") }
			if warm, cold := count(), truth(); warm != test.cold || cold != test.cold {
				t.Fatalf("cold run: %d findings, cache-free %d, want %d", warm, cold, test.cold)
			}
			if warm := count(); warm != test.cold {
				t.Fatalf("unchanged warm run: %d findings, want %d", warm, test.cold)
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

// Plugin aliases resolve through the catalogs a settings file declares, under the names it declares them (#1872).
// DataExtractionRules skips library modules, so editing a settings-declared catalog, renaming its create("<name>"),
// or pointing its from(files(...)) at another file must change a warm run's findings exactly as a cache-free run's,
// and a toml the settings file doesn't import must leave them alone. Each case edits forward only.
func TestManifestRulesWarmRunFollowsSettingsDeclaredCatalog(t *testing.T) {
	const (
		settingsRel = "settings.gradle.kts"
		buildRel    = "app/build.gradle.kts"
		manifestRel = "app/src/main/AndroidManifest.xml"
		build       = "plugins {\n    alias(libs.plugins.mod)\n}\n\nandroid {\n    defaultConfig {\n        minSdk = 21\n        targetSdk = 33\n    }\n}\n"
		manifest    = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <application android:allowBackup=\"false\" />\n</manifest>\n"
	)
	catalogOf := func(id string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
	}
	settingsOf := func(name, file string) string {
		return "include(\":app\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"" + name + "\") {\n            from(files(\"" + file + "\"))\n        }\n    }\n}\n"
	}
	app, library := catalogOf("com.android.application"), catalogOf("com.android.library")
	type edit struct {
		rel, body string
		want      int
	}
	for _, test := range []struct {
		name string
		// scan is the scanned path under the project; "" scans the project root.
		scan  string
		files map[string]string
		cold  int
		edits []edit
	}{
		{"settings-declared catalog", "", map[string]string{"deps/catalog.toml": library}, 0, []edit{
			{"deps/unused.toml", app, 0},
			{"deps/catalog.toml", app, 1},
			{"deps/catalog.toml", library + "# a library again\n", 0},
		}},
		{"module scan, settings-declared catalog", "app", map[string]string{"deps/catalog.toml": library}, 0, []edit{
			{"deps/unused.toml", app, 0},
			{"deps/catalog.toml", app, 1},
		}},
		{"module scan, create renamed", "app", map[string]string{"deps/catalog.toml": library}, 0, []edit{
			{settingsRel, settingsOf("tools", "deps/catalog.toml"), 1},
		}},
		{"module scan, from(files) pointed elsewhere", "app", map[string]string{"deps/catalog.toml": library, "deps/app.toml": app}, 0, []edit{
			{settingsRel, settingsOf("libs", "deps/app.toml"), 1},
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
			write(settingsRel, settingsOf("libs", "deps/catalog.toml"))
			write(buildRel, build)
			write(manifestRel, manifest)
			for rel, body := range test.files {
				write(rel, body)
			}
			scan := project
			if test.scan != "" {
				scan = filepath.Join(project, test.scan)
			}
			count := func(args ...string) int {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "-f", "json"}, args...), scan)...)
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
					if f.Rule == "DataExtractionRules" {
						n++
					}
				}
				return n
			}
			truth := func() int { return count("--no-cache", "--no-cross-file-cache") }
			if warm, cold := count(), truth(); warm != test.cold || cold != test.cold {
				t.Fatalf("cold run: %d findings, cache-free %d, want %d", warm, cold, test.cold)
			}
			if warm := count(); warm != test.cold {
				t.Fatalf("unchanged warm run: %d findings, want %d", warm, test.cold)
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
