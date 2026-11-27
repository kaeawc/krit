package librarymodel

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const accessorSettings = `
dependencyResolutionManagement {
    // create("commented") { from(files("deps/commented.toml")) }
    versionCatalogs {
        create("tools") {
            from(files("gradle/tools.versions.toml"))
        }
        register("extra") { from(files("deps/extra.toml")) }
        create("progs") {
            plugin("android-application", "com.android.application").version("8.5.0")
            library("core", "androidx.core", "core-ktx").version("1.13.0")
        }
    }
}
`

func writeAccessorProject(t *testing.T, settings string) (root, build string) {
	t.Helper()
	root = t.TempDir()
	writeTestFileMkdir(t, filepath.Join(root, "settings.gradle.kts"), settings)
	writeTestFileMkdir(t, filepath.Join(root, "gradle", "libs.versions.toml"), "[plugins]\nandroid-library = { id = \"com.android.library\", version = \"8.5.0\" }\n[libraries]\njunit = { module = \"junit:junit\", version = \"4.13.2\" }\n")
	writeTestFileMkdir(t, filepath.Join(root, "gradle", "tools.versions.toml"), "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n")
	writeTestFileMkdir(t, filepath.Join(root, "gradle", "other.versions.toml"), "[plugins]\nandroid-test = { id = \"com.android.test\", version = \"8.5.0\" }\n")
	writeTestFileMkdir(t, filepath.Join(root, "deps", "extra.toml"), "[plugins]\nkotlin-android = { id = \"org.jetbrains.kotlin.android\", version = \"2.0.0\" }\n")
	build = filepath.Join(root, "app", "build.gradle.kts")
	writeTestFileMkdir(t, build, "plugins { alias(libs.plugins.android.library) }\n")
	return root, build
}

// Accessor names leave the source list other consumers read unchanged: the same paths and kinds in the same order,
// and the same merged catalog (#1872).
func TestCatalogSourcesKeepTheirListWithAccessors(t *testing.T) {
	root, build := writeAccessorProject(t, accessorSettings)
	sources, ok := FindVersionCatalogSourcesForGradlePath(build)
	if !ok {
		t.Fatal("no catalog sources")
	}
	want := []CatalogSource{
		{Path: filepath.Join(root, "gradle", "libs.versions.toml"), Kind: CatalogSourceMergedTOML, Accessor: "libs"},
		{Path: filepath.Join(root, "gradle", "other.versions.toml"), Kind: CatalogSourceMergedTOML},
		{Path: filepath.Join(root, "gradle", "tools.versions.toml"), Kind: CatalogSourceMergedTOML},
		{Path: filepath.Join(root, "gradle", "tools.versions.toml"), Kind: CatalogSourceSettingsTOML, Accessor: "tools"},
		{Path: filepath.Join(root, "deps", "extra.toml"), Kind: CatalogSourceSettingsTOML, Accessor: "extra"},
		{Path: filepath.Join(root, "settings.gradle.kts"), Kind: CatalogSourceSettingsProgrammatic},
	}
	if !reflect.DeepEqual(sources, want) {
		t.Fatalf("sources = %#v\nwant %#v", sources, want)
	}
	catalog, ok := LoadVersionCatalogForGradlePath(build)
	if !ok {
		t.Fatal("no catalog")
	}
	for _, alias := range []string{"android-library", "android-application", "android-test", "kotlin-android"} {
		if _, ok := catalog.Plugins[alias]; !ok {
			t.Errorf("merged catalog lost plugin %q: %v", alias, catalog.Plugins)
		}
	}
	if _, ok := catalog.Libraries["core"]; !ok {
		t.Errorf("merged catalog lost the programmatic library: %v", catalog.Libraries)
	}
	if catalog.Completeness != CatalogCompletenessSettingsProgrammatic {
		t.Errorf("completeness = %v, want settings programmatic", catalog.Completeness)
	}
}

func TestCatalogPluginsByAccessor(t *testing.T) {
	_, build := writeAccessorProject(t, accessorSettings)
	sources, _ := FindVersionCatalogSourcesForGradlePath(build)
	plugins := LoadCatalogPluginsByAccessor(sources)
	for _, test := range []struct {
		accessor, alias, want string
	}{
		{"libs", "android-library", "com.android.library"},
		{"libs", "android-application", ""},
		{"tools", "android-application", "com.android.application"},
		{"tools", "android-library", ""},
		{"extra", "kotlin-android", "org.jetbrains.kotlin.android"},
		{"progs", "android-application", "com.android.application"},
		// gradle/other.versions.toml isn't imported; only its own name reaches it.
		{"other", "android-test", "com.android.test"},
		{"libs", "android-test", ""},
		{"commented", "android-application", ""},
	} {
		if got := plugins.Plugins(test.accessor)[test.alias].ID; got != test.want {
			t.Errorf("Plugins(%q)[%q] = %q, want %q", test.accessor, test.alias, got, test.want)
		}
	}
}

func TestCatalogAccessorNamesFromSettings(t *testing.T) {
	for _, test := range []struct {
		name, settings string
		// want maps each source path, relative to the root, with its kind, to its accessor.
		want map[string]string
	}{
		{"kotlin renamed default", "dependencyResolutionManagement {\n    defaultLibrariesExtensionName = \"projectLibs\"\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "projectLibs"}},
		{"groovy renamed default via set", "dependencyResolutionManagement {\n    defaultLibrariesExtensionName.set('projectLibs')\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "projectLibs"}},
		{"groovy named block", "dependencyResolutionManagement {\n    versionCatalogs {\n        deps {\n            from(files('deps/catalog.toml'))\n        }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		{"chained create", "dependencyResolutionManagement.versionCatalogs.create(\"deps\") {\n    from(files(\"deps/catalog.toml\"))\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		{"from outside any catalog block", "val f = from(files(\"deps/catalog.toml\"))\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": ""}},
		{"string braces and nested blocks", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"deps\") {\n            val s = \"}{\"\n            if (true) { println(s) }\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsName := "settings.gradle.kts"
			if strings.HasPrefix(test.name, "groovy") {
				settingsName = "settings.gradle"
			}
			writeTestFileMkdir(t, filepath.Join(root, settingsName), test.settings)
			writeTestFileMkdir(t, filepath.Join(root, "gradle", "libs.versions.toml"), "")
			sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
			got := map[string]string{}
			for _, source := range sources {
				rel, _ := filepath.Rel(root, source.Path)
				got[filepath.ToSlash(rel)+" "+string(source.Kind)] = source.Accessor
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("accessors = %v, want %v", got, test.want)
			}
		})
	}
}
