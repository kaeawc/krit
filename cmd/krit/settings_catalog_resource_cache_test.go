package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RelativeClassResolution reports a relative layout_behavior class as an error in a library module and as a warning
// in an app, which it reads from the plugin the module's catalog alias names, through the catalogs a settings file
// declares too (#1872). Editing only the settings-declared catalog, or repointing and then reverting the settings
// block, must change a warm run's finding exactly as a cache-free run's (#1910 review P1).
func TestRelativeClassResolutionWarmRunFollowsSettingsDeclaredCatalog(t *testing.T) {
	const (
		settingsRel = "settings.gradle.kts"
		layout      = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<FrameLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    xmlns:app=\"http://schemas.android.com/apk/res-auto\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\">\n    <View\n        android:layout_width=\"match_parent\"\n        android:layout_height=\"match_parent\"\n        app:layout_behavior=\".Behavior\" />\n</FrameLayout>\n"
	)
	catalogOf := func(id string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
	}
	settingsOf := func(file string) string {
		return "include(\":lib\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            from(files(\"" + file + "\"))\n        }\n    }\n}\n"
	}
	app, library := catalogOf("com.android.application"), catalogOf("com.android.library")
	const libraryMessage, appMessage = "error: Relative class name (`.Behavior`) will resolve using applicationId", "warning: Relative class name (`.Behavior`) is fragile"
	type edit struct{ rel, body, want string }
	for _, test := range []struct {
		name  string
		edits []edit
	}{
		{"settings-declared catalog edited", []edit{
			{"deps/catalog.toml", app, appMessage},
			{"deps/catalog.toml", library + "# a library again\n", libraryMessage},
		}},
		{"settings repointed and reverted", []edit{
			{settingsRel, settingsOf("deps/app.toml"), appMessage},
			{settingsRel, settingsOf("deps/catalog.toml"), libraryMessage},
			{settingsRel, settingsOf("deps/app.toml"), appMessage},
		}},
		// Reverting the settings file after its catalog changed must not replay the entry the earlier settings
		// content wrote for the catalog's earlier content.
		{"catalog edited, then settings repointed and reverted", []edit{
			{"deps/catalog.toml", app, appMessage},
			{settingsRel, settingsOf("deps/other.toml"), libraryMessage},
			{settingsRel, settingsOf("deps/catalog.toml"), appMessage},
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
			write(settingsRel, settingsOf("deps/catalog.toml"))
			write("krit.yml", "android-lint:\n  RelativeClassResolution:\n    active: true\n")
			write("lib/build.gradle.kts", "plugins {\n    alias(libs.plugins.mod)\n}\n\nandroid {\n    namespace = \"com.example.lib\"\n}\n")
			write("deps/catalog.toml", library)
			write("deps/app.toml", app)
			write("deps/other.toml", library)
			write("lib/src/main/res/layout/view.xml", layout)
			findings := func(args ...string) string {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "-f", "json", "--config", filepath.Join(project, "krit.yml")}, args...), project)...)
				var report struct {
					Findings []struct {
						Rule     string `json:"rule"`
						Severity string `json:"severity"`
						Message  string `json:"message"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				var got []string
				for _, f := range report.Findings {
					if strings.EqualFold(f.Rule, "RelativeClassResolution") {
						got = append(got, f.Severity+": "+f.Message)
					}
				}
				return strings.Join(got, "\n")
			}
			truth := func() string { return findings("--no-cache", "--no-cross-file-cache") }
			if warm, cold := findings(), truth(); !strings.HasPrefix(warm, libraryMessage) || warm != cold {
				t.Fatalf("cold run: %q, cache-free %q, want %q", warm, cold, libraryMessage)
			}
			for _, e := range test.edits {
				write(e.rel, e.body)
				if warm, cold := findings(), truth(); warm != cold || !strings.HasPrefix(cold, e.want) {
					t.Fatalf("after writing %s: warm run %q, cache-free %q, want %q for both", e.rel, warm, cold, e.want)
				}
			}
		})
	}
}
