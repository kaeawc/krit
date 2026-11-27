package librarymodel

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/hashutil"
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

// Accessor names and shadowing leave the source list other consumers read unchanged: the same paths and kinds in the same order,
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
		// The settings declare a tools catalog, so the file isn't one by its name; the block imports it as one.
		{Path: filepath.Join(root, "gradle", "tools.versions.toml"), Kind: CatalogSourceMergedTOML, Shadowed: true},
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
		// Text in a string literal or a comment declares no catalog and imports no file (#1872 review).
		{"string literal holding a block", `val s = "versionCatalogs { create('tools') { from(files('deps/catalog.toml')) } }"
val t = """
    versionCatalogs { create("tools") { from(files("deps/catalog.toml")) } }
"""
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs"}},
		{"groovy string literal holding a block", `def s = 'versionCatalogs { create("tools") { from(files("deps/catalog.toml")) } }'
def t = '''
versionCatalogs { create('tools') { from(files('deps/catalog.toml')) } }
'''
def u = """versionCatalogs { create('tools') { from(files('deps/catalog.toml')) } }"""
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs"}},
		{"comment holding a block", `// versionCatalogs { create('tools') { from(files('deps/catalog.toml')) } }
/* versionCatalogs {
    create('tools') { from(files('deps/catalog.toml')) }
} */
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs"}},
		// A real block beside those still imports its file under its name.
		{"real block beside a string and a comment", `// versionCatalogs { create('tools') { from(files('deps/tools.toml')) } }
val s = "versionCatalogs { create('tools') { from(files('deps/tools.toml')) } }"
dependencyResolutionManagement {
    versionCatalogs {
        create("deps") {
            from(files("deps/catalog.toml"))
        }
    }
}
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		{"groovy paren-less from files", `dependencyResolutionManagement {
    versionCatalogs {
        deps {
            from files('deps/catalog.toml')
        }
    }
}
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		// A string template whose code holds quotes and braces doesn't unbalance the blocks around it.
		{"string template with nested quotes", `dependencyResolutionManagement {
    versionCatalogs {
        val v = "${if (true) "{" else ""}"
        create("deps") {
            val w = "${"}"}"
            from(files("deps/catalog.toml"))
        }
    }
}
`, map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		{"string braces and nested blocks", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"deps\") {\n            val s = \"}{\"\n            if (true) { println(s) }\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "deps"}},
		// #1910 review round 3: create("name") with a chained .from(...) instead of a block, in a versionCatalogs block
		// and in the dotted chain, names the catalog it imports.
		{"brace-less create", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\").from(files(\"deps/catalog.toml\"))\n        create(\"more\")\n            .from(files(\"deps/more.toml\"))\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools", "deps/more.toml settings_toml": "more"}},
		{"dotted brace-less create", "dependencyResolutionManagement.versionCatalogs.create(\"tools\").from(files(\"deps/catalog.toml\"))\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		{"dotted brace-less maybeCreate and register", "dependencyResolutionManagement.versionCatalogs.maybeCreate(\"tools\").from(files(\"deps/catalog.toml\"))\ndependencyResolutionManagement.versionCatalogs.register(\"more\") { from(files(\"deps/more.toml\")) }\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools", "deps/more.toml settings_toml": "more"}},
		// The from(...) call ends the catalog: a later from(files(...)) outside any block names none.
		{"brace-less create ends at its from call", "dependencyResolutionManagement.versionCatalogs.create(\"tools\").from(files(\"deps/catalog.toml\"))\nval f = from(files(\"deps/more.toml\"))\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools", "deps/more.toml settings_toml": ""}},
		{"groovy brace-less create", "dependencyResolutionManagement {\n    versionCatalogs {\n        create('tools').from(files('deps/catalog.toml'))\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		{"groovy dotted brace-less create with paren-less from", "dependencyResolutionManagement.versionCatalogs.create('tools').from files('deps/catalog.toml')\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		// #1910 review round 3: a triple-quoted string holding a quote and a comment marker hides no later block.
		{"raw string holding a quote and a comment marker", "val note = \"\"\"a \" // b\"\"\"\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		{"groovy triple-quoted strings holding a quote and a comment marker", "def note = '''a ' // b'''\ndef other = \"\"\"a \" /* b\"\"\"\ndependencyResolutionManagement {\n    versionCatalogs {\n        tools {\n            from(files('deps/catalog.toml'))\n        }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		// Kotlin block comments nest; Groovy's end at the first */ (#1910 review round 3).
		{"nested block comment", "/** a /* b */ versionCatalogs { create(\"tools\") { from(files(\"deps/catalog.toml\")) } } */\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"deps\") { from(files(\"deps/real.toml\")) }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/real.toml settings_toml": "deps"}},
		{"groovy block comments don't nest", "/* a /* b */ dependencyResolutionManagement {\n    versionCatalogs {\n        tools { from(files('deps/catalog.toml')) }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "tools"}},
		// An explicit name is a catalog even when a bare block of that name configures something else (#1910 review
		// round 3); Groovy's bare all { } configures every catalog and names none.
		{"explicit create of a reserved name", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"all\") { from(files(\"deps/catalog.toml\")) }\n        maybeCreate(\"apply\").from(files(\"deps/more.toml\"))\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": "all", "deps/more.toml settings_toml": "apply"}},
		{"groovy bare all block", "dependencyResolutionManagement {\n    versionCatalogs {\n        all { from(files('deps/catalog.toml')) }\n    }\n}\n",
			map[string]string{"gradle/libs.versions.toml standard_toml": "libs", "deps/catalog.toml settings_toml": ""}},
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

// A settings block that declares a catalog krit can't read, such as a published one, takes the accessor: the
// unimported gradle/<name>.versions.toml of the same name is not that catalog, so its aliases resolve to nothing
// (#1872 review). Without such a block the file still serves its own name.
func TestCatalogPluginsByAccessor_DeclaredCatalogShadowsUnimportedFile(t *testing.T) {
	for _, test := range []struct {
		name, settingsName, settings, want string
	}{
		{"published catalog", "settings.gradle.kts", `dependencyResolutionManagement {
    versionCatalogs {
        create("deps") {
            from("com.example:catalog:1.0")
        }
    }
}
`, ""},
		{"groovy published catalog", "settings.gradle", `dependencyResolutionManagement {
    versionCatalogs {
        deps {
            from('com.example:catalog:1.0')
        }
    }
}
`, ""},
		{"chained create", "settings.gradle.kts", `dependencyResolutionManagement.versionCatalogs.create("deps") {
    from("com.example:catalog:1.0")
}
`, ""},
		{"other catalog declared", "settings.gradle.kts", `dependencyResolutionManagement {
    versionCatalogs {
        create("tools") {
            from("com.example:catalog:1.0")
        }
    }
}
`, "com.android.application"},
		{"block only in a string", "settings.gradle.kts", `val s = "versionCatalogs { create('deps') { from('com.example:catalog:1.0') } }"
`, "com.android.application"},
		// #1910 review round 3: the brace-less create("deps").from(...) declares deps too.
		{"brace-less published catalog", "settings.gradle.kts", `dependencyResolutionManagement {
    versionCatalogs {
        create("deps").from("com.example:catalog:1.0")
    }
}
`, ""},
		{"dotted brace-less published catalog", "settings.gradle.kts", `dependencyResolutionManagement.versionCatalogs.create("deps").from("com.example:catalog:1.0")
`, ""},
		{"groovy dotted brace-less published catalog", "settings.gradle", `dependencyResolutionManagement.versionCatalogs.create('deps').from 'com.example:catalog:1.0'
`, ""},
		{"no settings file", "", "", "com.android.application"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.settingsName != "" {
				writeTestFileMkdir(t, filepath.Join(root, test.settingsName), test.settings)
			}
			writeTestFileMkdir(t, filepath.Join(root, "gradle", "deps.versions.toml"), "[plugins]\nmod = { id = \"com.android.application\", version = \"8.5.0\" }\n")
			sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
			if got := LoadCatalogPluginsByAccessor(sources).Plugins("deps")["mod"].ID; got != test.want {
				t.Fatalf("Plugins(deps)[mod] = %q, want %q (sources %#v)", got, test.want, sources)
			}
		})
	}
}

// Gradle rejects an unquoted dotted key in [plugins], such as android.application = { id = "..." }: TOML reads it as
// a nested table, and Gradle's TomlCatalogFileParser fails the whole catalog ("Dots (.) in TOML keys create nested
// entries and cannot be used in alias names"). Such a build never configures, so krit reads no plugin from the key,
// under neither name. A quoted dotted key is one alias, which Gradle accepts (#1872 review).
func TestCatalogPlugins_DottedKeys(t *testing.T) {
	root := t.TempDir()
	writeTestFileMkdir(t, filepath.Join(root, "gradle", "libs.versions.toml"), `[plugins]
android.application = { id = "com.android.application", version = "8.5.0" }
kotlin.android = "org.jetbrains.kotlin.android:2.0.0"
"android.library" = { id = "com.android.library", version = "8.5.0" }
`)
	sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
	plugins := LoadCatalogPluginsByAccessor(sources).Plugins("libs")
	want := map[string]CatalogPlugin{"android.library": {ID: "com.android.library", Version: "8.5.0"}}
	if !reflect.DeepEqual(plugins, want) {
		t.Fatalf("Plugins(libs) = %#v, want %#v", plugins, want)
	}
}

// Only a bare block head of a reserved name, such as Groovy's all { }, which configures every catalog, declares no
// catalog: an explicit create("all") or maybeCreate("all") declares one, which takes the accessor from an unimported
// gradle/all.versions.toml (#1910 review round 3).
func TestCatalogPluginsByAccessor_ExplicitReservedNameShadows(t *testing.T) {
	for _, test := range []struct {
		name, settingsName, settings, want string
	}{
		{"explicit create", "settings.gradle.kts", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"all\") {\n            from(\"com.example:catalog:1.0\")\n        }\n    }\n}\n", ""},
		{"explicit maybeCreate", "settings.gradle.kts", "dependencyResolutionManagement.versionCatalogs.maybeCreate(\"all\").from(\"com.example:catalog:1.0\")\n", ""},
		{"groovy bare all block", "settings.gradle", "dependencyResolutionManagement {\n    versionCatalogs {\n        all {\n            from('com.example:catalog:1.0')\n        }\n    }\n}\n", "com.android.application"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFileMkdir(t, filepath.Join(root, test.settingsName), test.settings)
			writeTestFileMkdir(t, filepath.Join(root, "gradle", "all.versions.toml"), "[plugins]\nmod = { id = \"com.android.application\", version = \"8.5.0\" }\n")
			sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
			if got := LoadCatalogPluginsByAccessor(sources).Plugins("all")["mod"].ID; got != test.want {
				t.Fatalf("Plugins(all)[mod] = %q, want %q (sources %#v)", got, test.want, sources)
			}
		})
	}
}

// A programmatic catalog's plugin(...) declarations after a triple-quoted string holding a quote and a comment marker
// still count (#1910 review round 3).
func TestCatalogPluginsByAccessor_ProgrammaticAfterTripleQuotedString(t *testing.T) {
	for _, test := range []struct{ name, settingsName, prelude string }{
		{"kotlin raw string", "settings.gradle.kts", "val note = \"\"\"a \" // b\"\"\"\n"},
		{"groovy triple-single-quoted string", "settings.gradle", "def note = '''a ' // b'''\n"},
		{"groovy triple-double-quoted string", "settings.gradle", "def note = \"\"\"a \" /* b\"\"\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFileMkdir(t, filepath.Join(root, test.settingsName), test.prelude+"dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"progs\") {\n            plugin(\"mod\", \"com.android.library\").version(\"8.5.0\")\n        }\n    }\n}\n")
			sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
			if got := LoadCatalogPluginsByAccessor(sources).Plugins("progs")["mod"].ID; got != "com.android.library" {
				t.Fatalf("Plugins(progs)[mod] = %q, want com.android.library (sources %#v)", got, sources)
			}
		})
	}
}

// The parsed-source caches outlive one analysis (a daemon keeps them), so they must follow a catalog's content, not
// its stat: an edit that keeps the file's size and mtime still reaches the next analysis. Each analysis starts by
// clearing the shared hash memo, as the CLI and the daemon do (#1910 review round 3).
func TestCatalogCachesFollowContentAcrossAnalyses(t *testing.T) {
	const (
		library = "[plugins]\nmod = { id = \"com.android.library\", version = \"8.5.0\" }\n###\n"
		app     = "[plugins]\nmod = { id = \"com.android.application\", version = \"8.5.0\" }\n"
	)
	if len(library) != len(app) {
		t.Fatalf("test catalogs differ in length: %d vs %d", len(library), len(app))
	}
	progOf := func(id string) string {
		return "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"progs\") {\n            plugin(\"mod\", \"" + id + "\")\n        }\n    }\n}\n"
	}
	for _, test := range []struct {
		name, rel, accessor string
		before, after       string
		settings            string
	}{
		{"settings-declared toml", "deps/catalog.toml", "tools", library, app,
			"dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") { from(files(\"deps/catalog.toml\")) }\n    }\n}\n"},
		{"default toml", "gradle/libs.versions.toml", "libs", library, app, ""},
		{"programmatic settings", "settings.gradle.kts", "progs", progOf("com.example.aaa"), progOf("com.example.bbb"), ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.settings != "" {
				writeTestFileMkdir(t, filepath.Join(root, "settings.gradle.kts"), test.settings)
			}
			path := filepath.Join(root, test.rel)
			build := filepath.Join(root, "app", "build.gradle.kts")
			old := time.Now().Add(-time.Hour)
			write := func(content string) {
				t.Helper()
				writeTestFileMkdir(t, path, content)
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
			}
			analyze := func() (byAccessor, merged string) {
				t.Helper()
				hashutil.ResetDefault()
				sources, _ := FindVersionCatalogSourcesForGradlePath(build)
				catalog, _ := LoadVersionCatalogForGradlePath(build)
				return LoadCatalogPluginsByAccessor(sources).Plugins(test.accessor)["mod"].ID, catalog.Plugins["mod"].ID
			}
			idOf := func(content string) string {
				if test.rel == "settings.gradle.kts" {
					return ParseSettingsVersionCatalogContent(content).Plugins["mod"].ID
				}
				return ParseVersionCatalogContent(content).Plugins["mod"].ID
			}
			write(test.before)
			if got, merged := analyze(); got != idOf(test.before) || merged != idOf(test.before) {
				t.Fatalf("first analysis: Plugins(%s)[mod] = %q, merged %q, want %q", test.accessor, got, merged, idOf(test.before))
			}
			write(test.after)
			if got, merged := analyze(); got != idOf(test.after) || merged != idOf(test.after) {
				t.Fatalf("after a same-size, same-mtime edit: Plugins(%s)[mod] = %q, merged %q, want %q", test.accessor, got, merged, idOf(test.after))
			}
		})
	}
}

// library(...) and plugin(...) text in a string literal declares no programmatic catalog entry, and a settings file
// whose only versionCatalogs or declarations are in strings is no programmatic source (#1937 item 5, #1910 review
// round 3).
func TestProgrammaticCatalogIgnoresStringLiterals(t *testing.T) {
	const real = "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"progs\") {\n            library(\"core\", \"androidx.core\", \"core-ktx\").version(\"1.13.0\")\n        }\n    }\n}\n"
	for _, test := range []struct {
		name, settingsName, settings string
		wantSource                   bool
		wantLibraries                []string
	}{
		{"declarations only in strings", "settings.gradle.kts", "val s = \"versionCatalogs { create('x') { library('a', 'g', 'n') } }\"\nval t = \"\"\"plugin(\"p\", \"com.android.library\")\"\"\"\n", false, nil},
		{"versionCatalogs only in a string", "settings.gradle.kts", "val s = \"versionCatalogs\"\nfun library(a: String, b: String, c: String) = Unit\nlibrary(\"a\", \"g\", \"n\")\n", false, []string{"a"}},
		{"string beside a real block", "settings.gradle.kts", "val s = \"library('fake', 'g', 'n')\"\n" + real, true, []string{"core"}},
		{"groovy string beside a real block", "settings.gradle", "def s = '''library(\"fake\", \"g\", \"n\")'''\n" + real, true, []string{"core"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settings := filepath.Join(root, test.settingsName)
			writeTestFileMkdir(t, settings, test.settings)
			sources, _ := FindVersionCatalogSourcesForGradlePath(filepath.Join(root, "app", "build.gradle.kts"))
			gotSource := false
			for _, source := range sources {
				gotSource = gotSource || (source.Kind == CatalogSourceSettingsProgrammatic && source.Path == settings)
			}
			if gotSource != test.wantSource {
				t.Fatalf("programmatic source = %v, want %v (sources %#v)", gotSource, test.wantSource, sources)
			}
			catalog, _ := loadVersionCatalogFromSources([]CatalogSource{{Path: settings, Kind: CatalogSourceSettingsProgrammatic}})
			var libraries []string
			for alias := range catalog.Libraries {
				libraries = append(libraries, alias)
			}
			slices.Sort(libraries)
			if !reflect.DeepEqual(libraries, test.wantLibraries) {
				t.Fatalf("libraries = %v, want %v", libraries, test.wantLibraries)
			}
		})
	}
}
