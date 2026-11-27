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

// VersionCatalogSearch, which versionCatalogsFor and the cache keys share (#1800), globs from the module directory
// upward and stops at the first directory with a catalog.
func TestVersionCatalogSearchStopsAtTheNearestCatalog(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "gradle/libs.versions.toml", "")
	writeModuleFile(t, root, "gradle/more.versions.toml", "")
	writeModuleFile(t, root, "gradle/notes.toml", "")
	module := filepath.Join(root, "group", "app")
	searched, matches := VersionCatalogSearch(module, VersionCatalogGlob)
	if want := []string{module, filepath.Dir(module), root}; strings.Join(searched, "|") != strings.Join(want, "|") {
		t.Fatalf("searched = %v, want %v", searched, want)
	}
	want := []string{filepath.Join(root, "gradle", "libs.versions.toml"), filepath.Join(root, "gradle", "more.versions.toml")}
	if strings.Join(matches, "|") != strings.Join(want, "|") {
		t.Fatalf("matches = %v, want %v", matches, want)
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
