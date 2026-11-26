package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const providerAttributionSource = `package com.example.lib

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.net.Uri

class NotesProvider : ContentProvider() {
    override fun onCreate() = true
    override fun query(uri: Uri, p: Array<String>?, s: String?, a: Array<String>?, o: String?): Cursor? = null
    override fun getType(uri: Uri): String? = null
    override fun insert(uri: Uri, values: ContentValues?): Uri? = uri
    override fun update(uri: Uri, values: ContentValues?, s: String?, a: Array<String>?): Int = 0
    override fun delete(uri: Uri, s: String?, a: Array<String>?): Int = 0
}
`

const providerAttributionLibManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.lib">
    <application>
        <provider android:name=".NotesProvider" android:authorities="a" android:readPermission="com.example.READ" />
    </application>
</manifest>
`

// providerAttributionAppManifest is an app manifest with the given attributes on <manifest>/<application> and the
// given <application> children.
func providerAttributionAppManifest(manifestAttrs, applicationAttrs, children string) string {
	return `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="com.example.app"` + manifestAttrs + `>
    <application` + applicationAttrs + `>` + children + `</application>
</manifest>
`
}

// providerAttributionFindings writes the files under a temporary project root, runs ProviderReadPermissionOnly over
// every AndroidManifest.xml and .kt file among them, and returns the finding paths relative to the root, sorted.
func providerAttributionFindings(t *testing.T, files map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	var manifests []*rules.Manifest
	var sources []*scanner.File
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, rel := range names {
		path := filepath.Join(root, rel)
		if strings.HasSuffix(rel, "AndroidManifest.xml") {
			manifests = append(manifests, parseManifestAt(t, path, files[rel]))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[rel]), 0o644); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(rel, ".kt") {
			f, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			sources = append(sources, f)
		}
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "ProviderReadPermissionOnly" {
			rule = r
		}
	}
	cols, ok := rules.NewDispatcher([]*api.Rule{rule}, nil).RunManifestSource(manifests, sources)
	if !ok {
		t.Fatal("RunManifestSource recovered a panic")
	}
	var out []string
	for _, f := range cols.Findings() {
		rel, err := filepath.Rel(root, f.File)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

// #1625: a provider declared in a library module's manifest reaches Lint through the merged manifest of an app that
// depends on the library, directly or through other modules. It is not reported in a library-only scan, and app-side
// tools:ignore or tools:node="remove" apply.
func TestProviderReadPermissionOnlyLibraryManifestAttribution(t *testing.T) {
	const libReport = "lib/src/main/AndroidManifest.xml"
	const appPlugin = "plugins { id(\"com.android.application\") }\n"
	const libPlugin = "plugins { id(\"com.android.library\") }\n"
	appBuild := func(deps string) string {
		return appPlugin + "\ndependencies {\n" + deps + "}\n"
	}
	// project lays out a settings file including :app, :lib and the extra modules, the library module with its
	// provider, and the app module with the given build script and main manifest. extra adds or overrides files.
	project := func(appScript, appXML string, extra map[string]string, modules ...string) map[string]string {
		include := `include(":app", ":lib"`
		for _, m := range modules {
			include += `, "` + m + `"`
		}
		files := map[string]string{
			"settings.gradle.kts":                     include + ")\n",
			"lib/build.gradle.kts":                    libPlugin,
			"lib/src/main/AndroidManifest.xml":        providerAttributionLibManifest,
			"lib/src/main/java/com/example/lib/NP.kt": providerAttributionSource,
		}
		if appScript != "" {
			files["app/build.gradle.kts"] = appScript
			files["app/src/main/AndroidManifest.xml"] = appXML
		}
		for k, v := range extra {
			if v == "" {
				delete(files, k)
				continue
			}
			files[k] = v
		}
		return files
	}
	plainApp := providerAttributionAppManifest("", "", "")
	removeEntry := providerAttributionAppManifest("", "", `
        <provider android:name="com.example.lib.NotesProvider" tools:node="remove" />
    `)
	debugManifest := providerAttributionAppManifest("", "", "")
	dependsKts := appBuild("    implementation(project(\":lib\"))\n")

	for _, test := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"library-only scan", project("", "", nil), nil},
		{"library-only scan without a settings file", project("", "", map[string]string{"settings.gradle.kts": ""}), nil},
		{"app depends on the library", project(dependsKts, plainApp, nil), []string{libReport}},
		{"app depends through a type-safe accessor", project(appBuild("    implementation(projects.lib)\n"), plainApp, nil),
			[]string{libReport}},
		{"app does not depend on the library", project(appPlugin, plainApp, nil), nil},
		{"app-side tools:ignore on the entry", project(dependsKts, providerAttributionAppManifest("", "", `
        <provider android:name="com.example.lib.NotesProvider" tools:ignore="ProviderReadPermissionOnly" />
    `), nil), nil},
		{"app-side tools:node=remove", project(dependsKts, removeEntry, nil), nil},
		{"app-side tools:node=remove of another provider", project(dependsKts, providerAttributionAppManifest("", "", `
        <provider android:name="com.example.other.NotesProvider" tools:node="remove" />
    `), nil), []string{libReport}},

		// Groovy build scripts.
		{"Groovy project(':lib')", project("", "", map[string]string{
			"app/build.gradle":                 "plugins { id 'com.android.application' }\n\ndependencies {\n    implementation project(':lib')\n}\n",
			"app/src/main/AndroidManifest.xml": plainApp}), []string{libReport}},
		{"Groovy project(path: ':lib')", project("", "", map[string]string{
			"app/build.gradle":                 "plugins { id 'com.android.application' }\n\ndependencies {\n    implementation project(path: ':lib')\n}\n",
			"app/src/main/AndroidManifest.xml": plainApp}), []string{libReport}},

		// Dependencies that do not merge the library into the app manifest.
		{"commented-out dependency", project(appBuild("    // implementation(project(\":lib\"))\n"), plainApp, nil), nil},
		{"block-commented dependency", project(appBuild("    /* implementation(project(\":lib\")) */\n"), plainApp, nil), nil},
		{"testImplementation", project(appBuild("    testImplementation(project(\":lib\"))\n"), plainApp, nil), nil},
		{"androidTestImplementation", project(appBuild("    androidTestImplementation(project(\":lib\"))\n"), plainApp, nil), nil},

		// Transitive dependencies (#1625 review): app -> feature -> lib.
		{"transitive through a library", project(appBuild("    implementation(project(\":feature\"))\n"), plainApp,
			map[string]string{"feature/build.gradle.kts": libPlugin + "\ndependencies {\n    api(project(\":lib\"))\n}\n"},
			":feature"), []string{libReport}},
		{"transitive through a library without a manifest", project(appBuild("    implementation(projects.feature)\n"), plainApp,
			map[string]string{"feature/build.gradle.kts": libPlugin + "\ndependencies {\n    implementation(projects.lib)\n}\n"},
			":feature"), []string{libReport}},
		{"intermediate module does not depend on the library", project(appBuild("    implementation(project(\":feature\"))\n"),
			plainApp, map[string]string{"feature/build.gradle.kts": libPlugin}, ":feature"), nil},
		{"intermediate module depends on the library only for tests", project(appBuild("    implementation(project(\":feature\"))\n"),
			plainApp, map[string]string{"feature/build.gradle.kts": libPlugin + "\ndependencies {\n    testImplementation(project(\":lib\"))\n}\n"},
			":feature"), nil},

		// Source-set manifests of the app module are one consumer (#1625 review).
		{"remove in main with a debug manifest", project(dependsKts, removeEntry,
			map[string]string{"app/src/debug/AndroidManifest.xml": debugManifest}), nil},
		{"remove in the debug manifest", project(dependsKts, plainApp,
			map[string]string{"app/src/debug/AndroidManifest.xml": removeEntry}), nil},
		{"debug manifest without a removal", project(dependsKts, plainApp,
			map[string]string{"app/src/debug/AndroidManifest.xml": debugManifest}), []string{libReport}},

		// The Gradle namespace supplies the package of a manifest without one (#1625 review).
		{"library namespace from the build script", project(dependsKts, removeEntry, map[string]string{
			"lib/build.gradle.kts": libPlugin + "\nandroid {\n    namespace = \"com.example.lib\"\n}\n",
			"lib/src/main/AndroidManifest.xml": strings.Replace(providerAttributionLibManifest,
				` package="com.example.lib"`, "", 1)}), nil},
		{"Groovy library namespace", project(dependsKts, removeEntry, map[string]string{
			"lib/build.gradle.kts": "",
			"lib/build.gradle":     "plugins { id 'com.android.library' }\n\nandroid {\n    namespace 'com.example.lib'\n}\n",
			"lib/src/main/AndroidManifest.xml": strings.Replace(providerAttributionLibManifest,
				` package="com.example.lib"`, "", 1)}), nil},
		{"library namespace differs from the removed name", project(dependsKts, removeEntry, map[string]string{
			"lib/build.gradle.kts": libPlugin + "\nandroid {\n    namespace = \"com.example.other\"\n}\n",
			"lib/src/main/AndroidManifest.xml": strings.Replace(providerAttributionLibManifest,
				` package="com.example.lib"`, "", 1)}), []string{libReport}},

		// tools:ignore inherited from the app's <application> or <manifest> (#1625 review).
		{"app <application tools:ignore>", project(dependsKts,
			providerAttributionAppManifest("", ` tools:ignore="ProviderReadPermissionOnly"`, ""), nil), nil},
		{"app <manifest tools:ignore=all>", project(dependsKts,
			providerAttributionAppManifest(` tools:ignore="all"`, "", ""), nil), nil},
		{"app <application tools:ignore> of another rule", project(dependsKts,
			providerAttributionAppManifest("", ` tools:ignore="ExportedContentProvider"`, ""), nil), []string{libReport}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := providerAttributionFindings(t, test.files)
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("findings = %v, want %v", got, test.want)
			}
		})
	}
}

// #1625 review: a library is matched by its full project path, not its directory name, so an app depending on
// :core:lib does not pull in :feature:lib.
func TestProviderReadPermissionOnlyLibraryMatchedByProjectPath(t *testing.T) {
	libPlugin := "plugins { id(\"com.android.library\") }\n"
	files := map[string]string{
		"settings.gradle.kts":                              "include(\":app\", \":core:lib\", \":feature:lib\")\n",
		"app/build.gradle.kts":                             "plugins { id(\"com.android.application\") }\n\ndependencies {\n    implementation(project(\":core:lib\"))\n}\n",
		"app/src/main/AndroidManifest.xml":                 providerAttributionAppManifest("", "", ""),
		"core/lib/build.gradle.kts":                        libPlugin,
		"core/lib/src/main/AndroidManifest.xml":            providerAttributionLibManifest,
		"core/lib/src/main/java/com/example/lib/NP.kt":     providerAttributionSource,
		"feature/lib/build.gradle.kts":                     libPlugin,
		"feature/lib/src/main/AndroidManifest.xml":         strings.ReplaceAll(providerAttributionLibManifest, "com.example.lib", "com.example.feature"),
		"feature/lib/src/main/java/com/example/lib/NP2.kt": strings.Replace(providerAttributionSource, "package com.example.lib", "package com.example.feature", 1),
	}
	got := providerAttributionFindings(t, files)
	if want := "core/lib/src/main/AndroidManifest.xml"; strings.Join(got, ",") != want {
		t.Fatalf("findings = %v, want [%s]", got, want)
	}
}
