package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModuleFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestGradleInfoResolvesVersionCatalogAliases(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "gradle/libs.versions.toml", `
[versions]
agp = "9.0.0"

[plugins]
android-library = { id = "com.android.library", version.ref = "agp" }
android-application = { id = "com.android.application", version.ref = "agp" }
kotlin-android = { id = "org.jetbrains.kotlin.android", version = "2.0.0" }

[libraries]
junit = { module = "junit:junit", version = "4.13" }
`)
	writeModuleFile(t, root, "lib/build.gradle.kts", "plugins {\n  alias(libs.plugins.android.library)\n  alias(libs.plugins.kotlin.android)\n}\n")
	writeModuleFile(t, root, "lib/src/main/AndroidManifest.xml", "<manifest/>")
	writeModuleFile(t, root, "app/build.gradle.kts", "plugins {\n  alias(libs.plugins.android.application)\n}\n")
	writeModuleFile(t, root, "app/src/main/AndroidManifest.xml", "<manifest/>")

	if !isLibraryOrTestModuleManifest(filepath.Join(root, "lib/src/main/AndroidManifest.xml")) {
		t.Error("a module applying alias(libs.plugins.android.library) should be a library module")
	}
	if isLibraryOrTestModuleManifest(filepath.Join(root, "app/src/main/AndroidManifest.xml")) {
		t.Error("a module applying alias(libs.plugins.android.application) should not be a library module")
	}
}

// Cache keys hash ManifestBuildScriptCandidates for the build script lookupManifestGradleInfo reads (#1800), so every
// script it can read must be a candidate: each candidate alone is read, and a script past the last one is not.
func TestManifestBuildScriptCandidatesCoverTheLookup(t *testing.T) {
	const rel = "outer/m/app/src/main/AndroidManifest.xml"
	candidates := ManifestBuildScriptCandidates(filepath.Join("/r", rel))
	want := []string{"/r/outer/m/app/src", "/r/outer/m/app", "/r/outer/m", "/r/outer"}
	if len(candidates) != 2*len(want) {
		t.Fatalf("candidates = %v, want both script names in each of %v", candidates, want)
	}
	for i, dir := range want {
		if candidates[2*i] != filepath.Join(dir, "build.gradle.kts") || candidates[2*i+1] != filepath.Join(dir, "build.gradle") {
			t.Fatalf("candidates = %v, want both script names in each of %v", candidates, want)
		}
	}
	for i := range candidates {
		root := t.TempDir()
		manifest := filepath.Join(root, rel)
		got := ManifestBuildScriptCandidates(manifest)[i]
		writeModuleFile(t, root, rel, "<manifest/>")
		writeModuleFile(t, root, strings.TrimPrefix(got, root+string(filepath.Separator)), "plugins { id(\"com.android.application\") }\n")
		if !lookupManifestGradleInfo(manifest).isApplication {
			t.Errorf("a build script only at %s is not read", got)
		}
	}
	root := t.TempDir()
	writeModuleFile(t, root, rel, "<manifest/>")
	writeModuleFile(t, root, "build.gradle.kts", "plugins { id(\"com.android.application\") }\n")
	if lookupManifestGradleInfo(filepath.Join(root, rel)).found {
		t.Error("a build script past the last candidate is read")
	}
}

// ManifestBuildScript is the script lookupManifestGradleInfo reads: the nearest candidate that exists, .kts first.
func TestManifestBuildScriptIsTheOneRead(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "app/src/main/AndroidManifest.xml")
	writeModuleFile(t, root, "app/src/main/AndroidManifest.xml", "<manifest/>")
	if got := ManifestBuildScript(manifest); got != "" {
		t.Fatalf("no script: ManifestBuildScript = %q, want \"\"", got)
	}
	writeModuleFile(t, root, "build.gradle.kts", "")
	writeModuleFile(t, root, "app/build.gradle", "")
	writeModuleFile(t, root, "app/build.gradle.kts", "plugins { id(\"com.android.library\") }\n")
	if got, want := ManifestBuildScript(manifest), filepath.Join(root, "app/build.gradle.kts"); got != want {
		t.Fatalf("ManifestBuildScript = %q, want %q", got, want)
	}
	if !lookupManifestGradleInfo(manifest).isLibrary {
		t.Fatal("lookupManifestGradleInfo did not read app/build.gradle.kts")
	}
}

// BuildScriptCatalogReads gates the cache keys' catalog searches on the same aliases readManifestGradleInfo resolves
// (#1853).
func TestBuildScriptCatalogReads(t *testing.T) {
	for _, test := range []struct {
		content                          string
		pluginAliases, dependencyAliases bool
	}{
		{"plugins { id(\"com.android.application\") }\n", false, false},
		{"plugins { alias(libs.plugins.android.application) }\n", true, true},
		{"plugins { alias(tools.plugins.android.application) }\n", true, false},
		{"plugins { alias(libs.android.application) }\n", false, true},
		{"dependencies { implementation(libs.core.splash) }\n", false, true},
	} {
		plugins, deps := BuildScriptCatalogReads(test.content)
		if plugins != test.pluginAliases || deps != test.dependencyAliases {
			t.Errorf("BuildScriptCatalogReads(%q) = %v, %v, want %v, %v", test.content, plugins, deps, test.pluginAliases, test.dependencyAliases)
		}
	}
}

// manifestGradleInfoOf writes the files of a project under a new root and returns the module facts of
// app/src/main/AndroidManifest.xml.
func manifestGradleInfoOf(t *testing.T, files map[string]string) manifestGradleInfo {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		writeModuleFile(t, root, rel, content)
	}
	writeModuleFile(t, root, "app/src/main/AndroidManifest.xml", "<manifest/>")
	return lookupManifestGradleInfo(filepath.Join(root, "app/src/main/AndroidManifest.xml"))
}

const (
	testCatalogApplication = "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n"
	testCatalogLibrary     = "[plugins]\nandroid-library = { id = \"com.android.library\", version = \"8.5.0\" }\n"
)

// Plugin aliases resolve against the catalogs Gradle builds: the default gradle/libs.versions.toml as `libs`, and the
// catalogs the settings file declares under their create("<name>") names, wherever their files live (#1872).
func TestManifestGradleInfoResolvesSettingsDeclaredCatalogs(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
		app   bool
		lib   bool
	}{
		{"kotlin settings create(libs) from a file outside gradle/", map[string]string{
			"settings.gradle.kts":  "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n",
			"deps/catalog.toml":    testCatalogApplication,
			"app/build.gradle.kts": "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}, true, false},
		{"groovy settings named block, groovy build script", map[string]string{
			"settings.gradle":   "dependencyResolutionManagement {\n    versionCatalogs {\n        libs {\n            from(files('deps/catalog.toml'))\n        }\n    }\n}\n",
			"deps/catalog.toml": testCatalogLibrary,
			"app/build.gradle":  "plugins {\n    alias(libs.plugins.android.library)\n}\n",
		}, false, true},
		{"settings create(tools) from gradle/tools.versions.toml", map[string]string{
			"settings.gradle.kts":        "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"gradle/tools.versions.toml\"))\n        }\n    }\n}\n",
			"gradle/tools.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":       "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, true, false},
		{"settings versionCatalogs.create chain", map[string]string{
			"settings.gradle.kts":  "dependencyResolutionManagement.versionCatalogs.create(\"tools\") {\n    from(files(\"deps/tools.toml\"))\n}\n",
			"deps/tools.toml":      testCatalogApplication,
			"app/build.gradle.kts": "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, true, false},
		{"programmatic settings catalog", map[string]string{
			"settings.gradle.kts":  "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            plugin(\"android-application\", \"com.android.application\").version(\"8.5.0\")\n        }\n    }\n}\n",
			"app/build.gradle.kts": "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}, true, false},
		{"renamed default catalog", map[string]string{
			"settings.gradle.kts":       "dependencyResolutionManagement {\n    defaultLibrariesExtensionName = \"projectLibs\"\n}\n",
			"gradle/libs.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":      "plugins {\n    alias(projectLibs.plugins.android.application)\n}\n",
		}, true, false},
		// The plain default catalog, no settings file needed: pins the pre-#1872 behavior.
		{"default libs catalog", map[string]string{
			"gradle/libs.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":      "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}, true, false},
		// A gradle/<name>.versions.toml no settings file imports is matched by its file name when no catalog has the
		// accessor's name, the convention every documented import follows.
		{"unimported gradle/deps.versions.toml by file name", map[string]string{
			"settings.gradle.kts":       "include(\":app\")\n",
			"gradle/deps.versions.toml": testCatalogLibrary,
			"app/build.gradle.kts":      "plugins {\n    alias(deps.plugins.android.library)\n}\n",
		}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := manifestGradleInfoOf(t, test.files)
			if !info.found || info.isApplication != test.app || info.isLibrary != test.lib {
				t.Fatalf("found=%v isApplication=%v isLibrary=%v, want app=%v lib=%v", info.found, info.isApplication, info.isLibrary, test.app, test.lib)
			}
		})
	}
}

// An alias resolves only against the catalog its accessor names: Gradle generates one extension per catalog, so
// alias(tools.plugins.x) never reads libs, and alias(libs.plugins.x) never reads a catalog imported as tools (#1872).
func TestManifestGradleInfoMatchesTheAliasAccessor(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
	}{
		{"tools alias, only the default libs catalog", map[string]string{
			"settings.gradle.kts":       "include(\":app\")\n",
			"gradle/libs.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":      "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}},
		{"libs alias, catalog imported as tools", map[string]string{
			"settings.gradle.kts":        "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"gradle/tools.versions.toml\"))\n        }\n    }\n}\n",
			"gradle/tools.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":       "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}},
		{"libs alias, unimported gradle/other.versions.toml", map[string]string{
			"settings.gradle.kts":        "include(\":app\")\n",
			"gradle/other.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":       "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}},
		{"libs alias, default catalog renamed", map[string]string{
			"settings.gradle.kts":       "dependencyResolutionManagement {\n    defaultLibrariesExtensionName.set(\"projectLibs\")\n}\n",
			"gradle/libs.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":      "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}},
		{"libs alias, programmatic catalog named tools", map[string]string{
			"settings.gradle.kts":  "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            plugin(\"android-application\", \"com.android.application\").version(\"8.5.0\")\n        }\n    }\n}\n",
			"app/build.gradle.kts": "plugins {\n    alias(libs.plugins.android.application)\n}\n",
		}},
		// tools names a catalog, so the gradle/tools.versions.toml file-name fallback doesn't apply.
		{"tools alias, tools imported from another file", map[string]string{
			"settings.gradle.kts":        "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"deps/tools.toml\"))\n        }\n    }\n}\n",
			"deps/tools.toml":            testCatalogLibrary,
			"gradle/tools.versions.toml": testCatalogApplication,
			"app/build.gradle.kts":       "plugins {\n    alias(tools.plugins.android.application)\n}\n",
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if info := manifestGradleInfoOf(t, test.files); !info.found || info.isApplication {
				t.Fatalf("found=%v isApplication=%v, want a module that is not recognised as an app", info.found, info.isApplication)
			}
		})
	}
}
