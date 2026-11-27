package pipeline

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// RelativeClassResolution reports a relative layout_behavior class as an error in a library module and as a warning
// in an app, and reads which the module is from the plugin its catalog alias names, through the catalogs a settings
// file declares too (#1872). Its res-dir key must follow what that resolution reads: editing only a settings-declared
// catalog, repointing the settings block at another file, and reverting the settings file must each give a warm run
// the findings of a cache-free one (#1910 review P1).
func TestRelativeClassResolution_KeysOnSettingsDeclaredCatalogs(t *testing.T) {
	const (
		settingsRel = "settings.gradle.kts"
		buildRel    = "lib/build.gradle.kts"
		resDirRel   = "lib/src/main/res"
		layoutRel   = "lib/src/main/res/layout/view.xml"
		build       = "plugins {\n    alias(libs.plugins.mod)\n}\n\nandroid {\n    namespace = \"com.example.lib\"\n}\n"
		layout      = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<FrameLayout xmlns:android=\"http://schemas.android.com/apk/res/android\"\n    xmlns:app=\"http://schemas.android.com/apk/res-auto\"\n    android:layout_width=\"match_parent\"\n    android:layout_height=\"match_parent\">\n    <View\n        android:layout_width=\"match_parent\"\n        android:layout_height=\"match_parent\"\n        app:layout_behavior=\".Behavior\" />\n</FrameLayout>\n"
	)
	catalogOf := func(id string) string {
		return "[plugins]\nmod = { id = \"" + id + "\", version = \"8.5.0\" }\n"
	}
	settingsOf := func(file string) string {
		return "include(\":lib\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            from(files(\"" + file + "\"))\n        }\n    }\n}\n"
	}
	app, library := catalogOf("com.android.application"), catalogOf("com.android.library")
	type edit struct{ rel, body, want string }
	for _, test := range []struct {
		name     string
		catalogs map[string]string
		edits    []edit
	}{
		{"settings-declared catalog edited", map[string]string{"deps/catalog.toml": library}, []edit{
			{"deps/catalog.toml", app, "warning"},
			{"deps/catalog.toml", library + "# a library again\n", "error"},
		}},
		// Reverting the settings file returns to inputs a warm run has seen: the entry it replays must be the one
		// for those inputs.
		{"settings repointed and reverted", map[string]string{"deps/catalog.toml": library, "deps/app.toml": app}, []edit{
			{settingsRel, settingsOf("deps/app.toml"), "warning"},
			{settingsRel, settingsOf("deps/catalog.toml"), "error"},
			{settingsRel, settingsOf("deps/app.toml"), "warning"},
		}},
		// Reverting the settings file after its catalog changed must not replay the entry the earlier settings
		// content wrote for the catalog's earlier content.
		{"catalog edited, then settings repointed and reverted", map[string]string{"deps/catalog.toml": library, "deps/other.toml": library}, []edit{
			{"deps/catalog.toml", app, "warning"},
			{settingsRel, settingsOf("deps/other.toml"), "error"},
			{settingsRel, settingsOf("deps/catalog.toml"), "warning"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rule := registeredRule(t, "RelativeClassResolution")
			active := []*api.Rule{rule}
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			p.write(settingsRel, settingsOf("deps/catalog.toml"))
			p.write(buildRel, build)
			p.write(layoutRel, layout)
			for rel, body := range test.catalogs {
				p.write(rel, body)
			}
			project := &android.Project{ResDirs: []string{p.path(resDirRel)}, GradlePaths: []string{p.path(buildRel), p.path(settingsRel)}}
			cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
			severities := func(cacheDir string) string {
				t.Helper()
				hashutil.ResetDefault()
				writer := scanner.NewAndroidCacheWriter(2)
				res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
					Project:        project,
					ActiveRules:    active,
					Dispatcher:     rules.NewDispatcher(active, nil),
					RuleHash:       "test-rule-hash",
					LibraryFactsFP: "test-lf",
					CacheDir:       cacheDir,
					CacheWriter:    writer,
				})
				if err != nil {
					t.Fatalf("AndroidPhase.Run: %v", err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				var got []string
				for i := 0; i < res.Findings.Len(); i++ {
					got = append(got, res.Findings.SeverityAt(i))
				}
				return strings.Join(got, ",")
			}
			if cold, truth := severities(cacheDir), severities(""); cold != "error" || truth != cold {
				t.Fatalf("cold run severities %q, cache-free %q, want error", cold, truth)
			}
			for _, e := range test.edits {
				p.write(e.rel, e.body)
				truth := severities("")
				if warm := severities(cacheDir); warm != truth || truth != e.want {
					t.Fatalf("edit of %s: warm severities %q, cache-free %q, want %q", e.rel, warm, truth, e.want)
				}
			}
		})
	}
}
