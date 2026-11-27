package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

// writeCatalogProject writes a project's files under a new root and returns the path of app/src/main/AndroidManifest.xml.
func writeCatalogProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "app", "src", "main", "AndroidManifest.xml")
}

const settingsDeclaredCatalog = "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n"

// GoogleAppIndexingWarningManifest only reports application modules. An app whose plugin alias resolves through a
// catalog the settings file declares outside gradle/ is an application module, as Gradle and Lint see it (#1872).
func TestGoogleAppIndexingWarningManifest_SettingsDeclaredCatalogApp(t *testing.T) {
	r := findManifestRule(t, "GoogleAppIndexingWarningManifest")
	launcher := rules.ManifestComponent{
		Tag: "activity", Name: ".MainActivity", Line: 10,
		HasIntentFilter:     true,
		IntentFilterActions: []string{"android.intent.action.MAIN"},
	}
	for _, test := range []struct {
		name  string
		files map[string]string
		want  int
	}{
		{"settings-declared libs catalog", map[string]string{
			"settings.gradle.kts":  settingsDeclaredCatalog,
			"deps/catalog.toml":    "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts": "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}, 1},
		{"default libs catalog", map[string]string{
			"gradle/libs.versions.toml": "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts":      "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}, 1},
		// No catalog is imported as tools, so the alias applies nothing krit can see.
		{"accessor names no catalog", map[string]string{
			"gradle/libs.versions.toml": "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts":      "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, 0},
		// #1872 review: a settings block declaring a published deps catalog takes that name from the unimported
		// gradle/deps.versions.toml; a block in a string literal declares nothing; Groovy's paren-less from files(...)
		// imports its file.
		{"published catalog shadows an unimported file", map[string]string{
			"settings.gradle.kts":       "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"deps\") {\n            from(\"com.example:catalog:1.0\")\n        }\n    }\n}\n",
			"gradle/deps.versions.toml": "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts":      "plugins {\n    alias(deps.plugins.android.application)\n}\n",
		}, 0},
		{"block in a settings string literal", map[string]string{
			"settings.gradle.kts":  "val s = \"versionCatalogs { create('tools') { from(files('deps/catalog.toml')) } }\"\n",
			"deps/catalog.toml":    "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts": "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, 0},
		{"groovy paren-less from files", map[string]string{
			"settings.gradle":      "dependencyResolutionManagement {\n    versionCatalogs {\n        tools {\n            from files('deps/catalog.toml')\n        }\n    }\n}\n",
			"deps/catalog.toml":    "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n",
			"app/build.gradle.kts": "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeCatalogProject(t, test.files)
			m := &rules.Manifest{Path: path, Application: &rules.ManifestApplication{Activities: []rules.ManifestComponent{launcher}}}
			if got := len(runManifestRule(r, m)); got != test.want {
				t.Fatalf("findings = %d, want %d", got, test.want)
			}
		})
	}
}

// DataExtractionRules skips library modules. When two catalogs use the same plugin key, the alias's accessor decides
// which one applies: alias(tools.plugins.android) is the application plugin of the tools catalog, not the library
// plugin libs maps the same key to (#1872).
func TestDataExtractionRules_AliasResolvesThroughItsAccessor(t *testing.T) {
	r := findManifestRule(t, "DataExtractionRules")
	manifest := applicationManifest(21, `android:allowBackup="false"`)
	for _, test := range []struct {
		name, alias string
		want        int
	}{
		{"tools accessor: application", "tools.plugins.android", 1},
		{"libs accessor: library", "libs.plugins.android", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeCatalogProject(t, map[string]string{
				"settings.gradle.kts":              "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"gradle/tools.versions.toml\"))\n        }\n    }\n}\n",
				"gradle/libs.versions.toml":        "[plugins]\nandroid = { id = \"com.android.library\", version = \"8.5.0\" }\n",
				"gradle/tools.versions.toml":       "[plugins]\nandroid = { id = \"com.android.application\", version = \"8.5.0\" }\n",
				"app/build.gradle.kts":             "plugins {\n    alias(" + test.alias + ")\n}\n",
				"app/src/main/AndroidManifest.xml": manifest,
			})
			m := parseManifestForTest(t, manifest)
			m.Path = path
			if got := len(runManifestRule(r, m)); got != test.want {
				t.Fatalf("findings = %d, want %d", got, test.want)
			}
		})
	}
}

// DataExtractionRules skips library modules. alias(tools.plugins.mod) applies the library plugin of the catalog the
// settings import as tools from gradle/shared.versions.toml, however they declare it: by a brace-less
// create("tools").from(...), in a versionCatalogs block or the dotted chain, or by a block after a triple-quoted string
// holding a quote and a comment marker. Main resolved these by plugin key alone; the accessor must keep them resolved
// (#1910 review round 3).
func TestDataExtractionRules_SettingsCatalogDeclarationForms(t *testing.T) {
	r := findManifestRule(t, "DataExtractionRules")
	manifest := applicationManifest(21, `android:allowBackup="false"`)
	for _, test := range []struct {
		name, settingsName, settings string
	}{
		{"brace-less create", "settings.gradle.kts", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\").from(files(\"gradle/shared.versions.toml\"))\n    }\n}\n"},
		{"dotted brace-less create", "settings.gradle.kts", "dependencyResolutionManagement.versionCatalogs.create(\"tools\").from(files(\"gradle/shared.versions.toml\"))\n"},
		{"dotted brace-less maybeCreate", "settings.gradle.kts", "dependencyResolutionManagement.versionCatalogs.maybeCreate(\"tools\").from(files(\"gradle/shared.versions.toml\"))\n"},
		{"groovy brace-less create", "settings.gradle", "dependencyResolutionManagement {\n    versionCatalogs {\n        create('tools').from(files('gradle/shared.versions.toml'))\n    }\n}\n"},
		{"groovy dotted brace-less create, paren-less from", "settings.gradle", "dependencyResolutionManagement.versionCatalogs.create('tools').from files('gradle/shared.versions.toml')\n"},
		{"raw string before the block", "settings.gradle.kts", "val note = \"\"\"a \" // b\"\"\"\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"gradle/shared.versions.toml\"))\n        }\n    }\n}\n"},
		{"groovy triple-quoted strings before the block", "settings.gradle", "def note = '''a ' // b'''\ndef other = \"\"\"a \" // b\"\"\"\ndependencyResolutionManagement {\n    versionCatalogs {\n        tools {\n            from(files('gradle/shared.versions.toml'))\n        }\n    }\n}\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeCatalogProject(t, map[string]string{
				test.settingsName:                  test.settings,
				"gradle/shared.versions.toml":      "[plugins]\nmod = { id = \"com.android.library\", version = \"8.5.0\" }\n",
				"app/build.gradle.kts":             "plugins {\n    alias(tools.plugins.mod)\n}\n",
				"app/src/main/AndroidManifest.xml": manifest,
			})
			m := parseManifestForTest(t, manifest)
			m.Path = path
			if got := len(runManifestRule(r, m)); got != 0 {
				t.Fatalf("findings = %d, want 0: tools.plugins.mod is the library plugin", got)
			}
		})
	}
}
