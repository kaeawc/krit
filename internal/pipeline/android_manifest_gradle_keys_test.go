package pipeline

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registeredRule(t *testing.T, id string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%s is not registered", id)
	return nil
}

// Every manifest rule reads whether its module is an app or a library through readManifestGradleInfo, which resolves
// the build script's plugin aliases through the version catalogs its search reaches. DataExtractionRules skips
// library modules, so a catalog-only edit that flips the module must re-run it, through every key that replays
// manifest findings: the per-manifest, manifest-bundle and Android project-bundle keys, and the findings bundle's XML
// inputs. Catalogs the search doesn't reach must leave those keys alone (#1853). Edits run forward only.
func TestManifestRules_KeyOnVersionCatalogSearch(t *testing.T) {
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
	// unkeyed marks a file the rules don't read, whose edit must leave the keys alone.
	type edit struct {
		rel, body string
		want      int
		unkeyed   bool
	}
	rule := registeredRule(t, "DataExtractionRules")
	for _, test := range []struct {
		name       string
		noSettings bool
		catalog    string
		cold       int
		edits      []edit
	}{
		{"catalog plugin id", false, app, 1, []edit{
			{catalogRel, library, 0, false},
			{catalogRel, app + "# back to an app\n", 1, false},
		}},
		{"no settings file, catalog plugin id", true, app, 1, []edit{
			{catalogRel, library, 0, false},
			{catalogRel, app + "# back to an app\n", 1, false},
		}},
		{"catalog created", false, "", 1, []edit{{catalogRel, library, 0, false}}},
		{"no settings file, catalog created", true, "", 1, []edit{{catalogRel, library, 0, false}}},
		{"nearer catalog created", false, app, 1, []edit{{"app/gradle/libs.versions.toml", library, 0, false}}},
		{"second catalog created", false, app, 1, []edit{{"gradle/more.versions.toml", "[plugins]\nother = { id = \"x\" }\n", 1, false}}},
		{"unrelated catalogs", false, app, 1, []edit{
			{"tools/gradle/libs.versions.toml", library, 1, true},
			{"gradle/notes.toml", library, 1, true},
			{catalogRel, library, 0, false},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			if !test.noSettings {
				p.write(settingsRel, "include(\":app\")\n")
			}
			p.write(buildRel, build)
			p.write(manifestRel, manifest)
			if test.catalog != "" {
				p.write(catalogRel, test.catalog)
			}
			project := &android.Project{
				ManifestPaths: []string{p.path(manifestRel)},
				GradlePaths:   []string{p.path(buildRel)},
			}
			if !test.noSettings {
				project.GradlePaths = append(project.GradlePaths, p.path(settingsRel))
			}
			reads := buildFileReadersOf([]*api.Rule{rule})
			cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
			run := func(cacheDir string) int {
				t.Helper()
				hashutil.ResetDefault()
				writer := scanner.NewAndroidCacheWriter(2)
				res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
					Project:        project,
					ActiveRules:    []*api.Rule{rule},
					Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
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
				return res.Findings.Len()
			}
			if got := run(""); got != test.cold {
				t.Fatalf("cold cache-free findings = %d, want %d", got, test.cold)
			}
			if got := run(cacheDir); got != test.cold {
				t.Fatalf("cold findings = %d, want %d", got, test.cold)
			}
			for _, e := range test.edits {
				hashutil.ResetDefault()
				xmlBefore := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads)
				p.write(e.rel, e.body)
				hashutil.ResetDefault()
				if moved := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads) != xmlBefore; moved == e.unkeyed {
					t.Errorf("edit of %s: findings-bundle XML inputs fingerprint moved = %v, want %v", e.rel, moved, !e.unkeyed)
				}
				if got := run(""); got != e.want {
					t.Fatalf("edit of %s: cache-free findings = %d, want %d", e.rel, got, e.want)
				}
				if got := run(cacheDir); got != e.want {
					t.Fatalf("edit of %s: warm findings = %d, cache-free %d", e.rel, got, e.want)
				}
			}
		})
	}
}

// The catalog section is only written when an active manifest or manifest-source rule reads the module's build
// script: a run with no such rule keeps catalog edits out of the key.
func TestBuildFileReadersOf(t *testing.T) {
	for _, test := range []struct {
		ids  []string
		want buildFileReaders
	}{
		{nil, buildFileReaders{}},
		{[]string{"MagicNumber"}, buildFileReaders{}},
		{[]string{"DataExtractionRules"}, buildFileReaders{gradleInfo: true}},
		{[]string{"ProviderReadPermissionOnly"}, buildFileReaders{moduleGraph: true, gradleInfo: true}},
	} {
		var active []*api.Rule
		for _, id := range test.ids {
			active = append(active, registeredRule(t, id))
		}
		if got := buildFileReadersOf(active); got != test.want {
			t.Errorf("buildFileReadersOf(%v) = %+v, want %+v", test.ids, got, test.want)
		}
	}
}

// The analysis cache records 16-hex content hashes (cache.ComputeFileHash) while a parsed file is hashed through the
// memo in full. The manifest-source fingerprint must give the same key for the same content either way, or the first
// warm run after a cold one misses (#1853).
func TestManifestSourceSourceFingerprint_OneHashFormat(t *testing.T) {
	p := newManifestSourceProject(t, msManifestWithService, msSourceMarker)
	path := p.path(msSourceRel)
	hashutil.ResetDefault()
	parsed := AndroidInput{
		SourceFiles: []*scanner.File{parseKotlinForTest(t, path, readFile(t, path))},
		SourcePaths: []string{path},
	}
	full, err := hashutil.Default().HashFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, ok := parsed.manifestSourceSourceFingerprint()
	if !ok {
		t.Fatal("fingerprint failed")
	}
	for name, in := range map[string]AndroidInput{
		"cached 16-hex hash":         {SourcePaths: []string{path}, SourceHashes: map[string]string{path: cache.ComputeFileHash(path)}},
		"cached full hash":           {SourcePaths: []string{path}, SourceHashes: map[string]string{path: full}},
		"parsed with a 16-hex hash":  {SourceFiles: parsed.SourceFiles, SourcePaths: []string{path}, SourceHashes: map[string]string{path: cache.ComputeFileHash(path)}},
		"unparsed, hashed from disk": {SourcePaths: []string{path}},
		"parsed, path list left out": {SourceFiles: parsed.SourceFiles},
	} {
		if got, ok := in.manifestSourceSourceFingerprint(); !ok || got != want {
			t.Errorf("%s: fingerprint = %q (ok %v), want the parsed run's %q", name, got, ok, want)
		}
	}
}

// A cold run hashes every source file it parses in full; a warm run takes the analysis cache's 16-hex hashes for the
// files it skipped. The manifest-source result the cold run wrote must serve the first warm run, and after a source
// edit, the run that re-parsed the file must serve the next res-only run, where the file is a cache hit again (#1853).
func TestManifestSourcePass_HitsAcrossHashFormats(t *testing.T) {
	const resRel = "app/src/main/res/values/strings.xml"
	p := newManifestSourceProject(t, msManifestWithService, msSourceMarker)
	p.write(resRel, "<resources><string name=\"a\">A</string></resources>\n")
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	var calls int64
	rule := manifestSourceJoinRule(&calls)
	source := p.path(msSourceRel)
	// step runs the phase the way a project run does: parsed is whether the parse phase parsed the source file (an
	// analysis-cache miss); otherwise it is a hit and its cached 16-hex hash is passed on.
	step := func(label string, parsed bool, wantCalls int64) {
		t.Helper()
		hashutil.ResetDefault()
		in := AndroidInput{
			Project:        &android.Project{ManifestPaths: []string{p.path(msManifestRel)}, ResDirs: []string{p.path("app/src/main/res")}},
			ActiveRules:    []*api.Rule{rule},
			Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
			SourcePaths:    []string{source},
			RuleHash:       "test-rule-hash",
			LibraryFactsFP: "test-lf",
			CacheDir:       cacheDir,
			CacheWriter:    scanner.NewAndroidCacheWriter(2),
		}
		if parsed {
			in.SourceFiles = []*scanner.File{parseKotlinForTest(t, source, readFile(t, source))}
		} else {
			in.SourceHashes = map[string]string{source: cache.ComputeFileHash(source)}
		}
		before := atomic.LoadInt64(&calls)
		res, err := (AndroidPhase{}).Run(context.Background(), in)
		if err != nil {
			t.Fatalf("%s: AndroidPhase.Run: %v", label, err)
		}
		if err := in.CacheWriter.Close(); err != nil {
			t.Fatal(err)
		}
		if got := res.Findings.Len(); got != 1 {
			t.Fatalf("%s: findings = %d, want 1", label, got)
		}
		if got := atomic.LoadInt64(&calls) - before; got != wantCalls {
			t.Fatalf("%s: rule calls = %d, want %d", label, got, wantCalls)
		}
	}
	step("cold", true, 1)
	step("first warm run", false, 0)
	p.write(resRel, "<resources><string name=\"a\">B</string></resources>\n")
	step("res-only edit", false, 0)
	p.write(msSourceRel, msSourceMarker+"// edited\n")
	step("source edit", true, 1)
	p.write(resRel, "<resources><string name=\"a\">C</string></resources>\n")
	step("res-only edit after the source edit", false, 0)
}

// readManifestGradleInfo also resolves the build script's libs. dependency aliases through librarymodel's catalog
// discovery, which reads every ancestor's gradle/*.versions.toml and the catalogs a settings file declares. That
// decides whether the module uses the SplashScreen library, which WearSplashScreen defers to, so editing only one of
// those catalogs must re-run it, and a catalog the discovery doesn't find must leave the keys alone (#1853).
func TestManifestRules_KeyOnDependencyCatalogSources(t *testing.T) {
	const (
		settingsRel      = "settings.gradle.kts"
		buildRel         = "app/build.gradle.kts"
		manifestRel      = "app/src/main/AndroidManifest.xml"
		build            = "android {\n    defaultConfig {\n        minSdk = 26\n    }\n}\n\ndependencies {\n    implementation(libs.splash)\n}\n"
		manifest         = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <uses-feature android:name=\"android.hardware.type.watch\" />\n    <application>\n        <activity android:name=\".SplashActivity\" android:exported=\"true\">\n            <intent-filter>\n                <action android:name=\"android.intent.action.MAIN\" />\n                <category android:name=\"android.intent.category.LAUNCHER\" />\n            </intent-filter>\n        </activity>\n    </application>\n</manifest>\n"
		settingsCatalogs = "include(\":app\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n"
	)
	catalogOf := func(module string) string {
		return "[libraries]\nsplash = { module = \"" + module + "\", version = \"1.0.1\" }\n"
	}
	splash, other := catalogOf("androidx.core:core-splashscreen"), catalogOf("androidx.core:core-ktx")
	type edit struct {
		rel, body string
		want      int
		unkeyed   bool
	}
	rule := registeredRule(t, "WearSplashScreen")
	for _, test := range []struct {
		name, settings, catalogRel string
		edits                      []edit
	}{
		{"standard catalog", "include(\":app\")\n", "gradle/libs.versions.toml", []edit{
			{"tools/gradle/libs.versions.toml", other, 0, true},
			{"gradle/libs.versions.toml", other, 1, false},
		}},
		{"settings-declared catalog", settingsCatalogs, "deps/catalog.toml", []edit{
			{"deps/unlisted.toml", other, 0, true},
			{"deps/catalog.toml", other, 1, false},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			p.write(settingsRel, test.settings)
			p.write(buildRel, build)
			p.write(manifestRel, manifest)
			p.write(test.catalogRel, splash)
			project := &android.Project{
				ManifestPaths: []string{p.path(manifestRel)},
				GradlePaths:   []string{p.path(buildRel), p.path(settingsRel)},
			}
			reads := buildFileReadersOf([]*api.Rule{rule})
			cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
			run := func(cacheDir string) int {
				t.Helper()
				hashutil.ResetDefault()
				writer := scanner.NewAndroidCacheWriter(2)
				res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
					Project:        project,
					ActiveRules:    []*api.Rule{rule},
					Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
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
				return res.Findings.Len()
			}
			if got := run(""); got != 0 {
				t.Fatalf("cold cache-free findings = %d, want 0", got)
			}
			if got := run(cacheDir); got != 0 {
				t.Fatalf("cold findings = %d, want 0", got)
			}
			for _, e := range test.edits {
				hashutil.ResetDefault()
				xmlBefore := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads)
				p.write(e.rel, e.body)
				hashutil.ResetDefault()
				if moved := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads) != xmlBefore; moved == e.unkeyed {
					t.Errorf("edit of %s: findings-bundle XML inputs fingerprint moved = %v, want %v", e.rel, moved, !e.unkeyed)
				}
				if got := run(""); got != e.want {
					t.Fatalf("edit of %s: cache-free findings = %d, want %d", e.rel, got, e.want)
				}
				if got := run(cacheDir); got != e.want {
					t.Fatalf("edit of %s: warm findings = %d, cache-free %d", e.rel, got, e.want)
				}
			}
		})
	}
}

// Plugin aliases resolve through librarymodel's catalog discovery, which reads the catalogs a settings file declares
// and the names it declares them under (#1872). A scan of the module alone doesn't hold the settings file or the
// catalogs, so the gradle-info fingerprint must key what discovery derives from them: editing a settings-declared
// catalog, renaming its create("<name>") or the default catalog, pointing from(files(...)) elsewhere, or editing a
// programmatic declaration must re-run DataExtractionRules, which skips library modules, through every key that
// replays manifest findings. A toml no discovery reaches, and a settings edit that leaves every catalog source alone,
// must leave the keys alone. Edits run forward only.
func TestManifestRules_KeyOnSettingsDeclaredCatalogs(t *testing.T) {
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
	settingsOf := func(name, file, extra string) string {
		return "include(\":app\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"" + name + "\") {\n            from(files(\"" + file + "\"))\n        }\n    }\n}\n" + extra
	}
	programmatic := func(id string) string {
		return "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"libs\") {\n            plugin(\"mod\", \"" + id + "\").version(\"8.5.0\")\n        }\n    }\n}\n"
	}
	app, library := catalogOf("com.android.application"), catalogOf("com.android.library")
	type edit struct {
		rel, body string
		want      int
		unkeyed   bool
	}
	rule := registeredRule(t, "DataExtractionRules")
	for _, test := range []struct {
		name     string
		settings string
		catalogs map[string]string
		cold     int
		edits    []edit
		// build replaces the build script, which otherwise reaches the catalog as libs.
		build string
	}{
		{"settings-declared catalog edited", settingsOf("libs", "deps/catalog.toml", ""), map[string]string{"deps/catalog.toml": app}, 1, []edit{
			{"deps/catalog.toml", library, 0, false},
			{"deps/catalog.toml", app + "# back to an app\n", 1, false},
		}, ""},
		{"create renamed", settingsOf("libs", "deps/catalog.toml", ""), map[string]string{"deps/catalog.toml": library}, 0, []edit{
			{settingsRel, settingsOf("tools", "deps/catalog.toml", ""), 1, false},
			{settingsRel, settingsOf("libs", "deps/catalog.toml", "// libs again\n"), 0, false},
		}, ""},
		{"from(files) pointed elsewhere", settingsOf("libs", "deps/catalog.toml", ""), map[string]string{"deps/catalog.toml": library, "deps/app.toml": app}, 0, []edit{
			{settingsRel, settingsOf("libs", "deps/app.toml", ""), 1, false},
		}, ""},
		{"default catalog renamed", "include(\":app\")\n", map[string]string{"gradle/libs.versions.toml": library}, 0, []edit{
			{settingsRel, "include(\":app\")\n\ndependencyResolutionManagement {\n    defaultLibrariesExtensionName = \"projectLibs\"\n}\n", 1, false},
		}, ""},
		{"programmatic declaration edited", programmatic("com.android.library"), nil, 0, []edit{
			{settingsRel, programmatic("com.android.application"), 1, false},
		}, ""},
		// A block that declares a published deps catalog takes the name from the unimported gradle/deps.versions.toml,
		// so the alias no longer resolves and the module is no longer a known library (#1872 review).
		{"published catalog declared under an unimported file's name", "include(\":app\")\n", map[string]string{"gradle/deps.versions.toml": library}, 0, []edit{
			{settingsRel, "include(\":app\")\n\ndependencyResolutionManagement {\n    versionCatalogs {\n        create(\"deps\") {\n            from(\"com.example:catalog:1.0\")\n        }\n    }\n}\n", 1, false},
		}, strings.Replace(build, "libs.plugins", "deps.plugins", 1)},
		{"unrelated files", settingsOf("libs", "deps/catalog.toml", ""), map[string]string{"deps/catalog.toml": library}, 0, []edit{
			{"deps/unused.toml", app, 0, true},
			{"gradle/notes.toml", app, 0, true},
			{"tools/gradle/libs.versions.toml", app, 0, true},
			{settingsRel, settingsOf("libs", "deps/catalog.toml", "include(\":other\")\n"), 0, true},
			{"deps/catalog.toml", app, 1, false},
		}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &manifestSourceProject{t: t, dir: t.TempDir()}
			p.write(settingsRel, test.settings)
			p.write(buildRel, cmp.Or(test.build, build))
			p.write(manifestRel, manifest)
			for rel, body := range test.catalogs {
				p.write(rel, body)
			}
			// A scan of the module alone: neither the settings file nor any catalog is a scanned Gradle path.
			project := &android.Project{
				ManifestPaths: []string{p.path(manifestRel)},
				GradlePaths:   []string{p.path(buildRel)},
			}
			reads := buildFileReadersOf([]*api.Rule{rule})
			cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
			run := func(cacheDir string) int {
				t.Helper()
				hashutil.ResetDefault()
				writer := scanner.NewAndroidCacheWriter(2)
				res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
					Project:        project,
					ActiveRules:    []*api.Rule{rule},
					Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
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
				return res.Findings.Len()
			}
			if got := run(""); got != test.cold {
				t.Fatalf("cold cache-free findings = %d, want %d", got, test.cold)
			}
			if got := run(cacheDir); got != test.cold {
				t.Fatalf("cold findings = %d, want %d", got, test.cold)
			}
			for _, e := range test.edits {
				hashutil.ResetDefault()
				xmlBefore := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads)
				p.write(e.rel, e.body)
				hashutil.ResetDefault()
				if moved := xmlInputsFingerprint(ProjectHostState{}, project, nil, reads) != xmlBefore; moved == e.unkeyed {
					t.Errorf("edit of %s: findings-bundle XML inputs fingerprint moved = %v, want %v", e.rel, moved, !e.unkeyed)
				}
				if got := run(""); got != e.want {
					t.Fatalf("edit of %s: cache-free findings = %d, want %d", e.rel, got, e.want)
				}
				if got := run(cacheDir); got != e.want {
					t.Fatalf("edit of %s: warm findings = %d, cache-free %d", e.rel, got, e.want)
				}
			}
		})
	}
}

// The librarymodel caches of parsed catalog sources live for the process, as a daemon keeps them across analyses.
// Rewriting a settings-declared catalog with content of the same length and restoring its mtime must still change the
// next analysis's findings, cache-free and warm (#1910 review round 3).
func TestManifestRules_SettingsDeclaredCatalogSameStatEdit(t *testing.T) {
	const (
		catalogRel  = "deps/catalog.toml"
		buildRel    = "app/build.gradle.kts"
		manifestRel = "app/src/main/AndroidManifest.xml"
		library     = "[plugins]\nmod = { id = \"com.android.library\", version = \"8.5.0\" }\n###\n"
		app         = "[plugins]\nmod = { id = \"com.android.application\", version = \"8.5.0\" }\n"
	)
	if len(library) != len(app) {
		t.Fatalf("test catalogs differ in length: %d vs %d", len(library), len(app))
	}
	rule := registeredRule(t, "DataExtractionRules")
	p := &manifestSourceProject{t: t, dir: t.TempDir()}
	p.write("settings.gradle.kts", "dependencyResolutionManagement {\n    versionCatalogs {\n        create(\"tools\") {\n            from(files(\"deps/catalog.toml\"))\n        }\n    }\n}\n")
	p.write(buildRel, "plugins {\n    alias(tools.plugins.mod)\n}\n\nandroid {\n    defaultConfig {\n        minSdk = 21\n        targetSdk = 33\n    }\n}\n")
	p.write(manifestRel, "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" package=\"com.example\">\n    <application android:allowBackup=\"false\" />\n</manifest>\n")
	p.write(catalogRel, library)
	info, err := os.Stat(p.path(catalogRel))
	if err != nil {
		t.Fatal(err)
	}
	project := &android.Project{
		ManifestPaths: []string{p.path(manifestRel)},
		GradlePaths:   []string{p.path(buildRel), p.path("settings.gradle.kts")},
	}
	cacheDir := filepath.Join(p.dir, ".krit", "android-findings-cache")
	run := func(cacheDir string) int {
		t.Helper()
		hashutil.ResetDefault()
		writer := scanner.NewAndroidCacheWriter(2)
		res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
			Project:        project,
			ActiveRules:    []*api.Rule{rule},
			Dispatcher:     rules.NewDispatcher([]*api.Rule{rule}, nil),
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
		return res.Findings.Len()
	}
	if got := run(""); got != 0 {
		t.Fatalf("library catalog: cache-free findings = %d, want 0", got)
	}
	if got := run(cacheDir); got != 0 {
		t.Fatalf("library catalog: warm findings = %d, want 0", got)
	}
	p.write(catalogRel, app)
	if err := os.Chtimes(p.path(catalogRel), info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := run(""); got != 1 {
		t.Fatalf("same-size, same-mtime edit to an application catalog: cache-free findings = %d, want 1", got)
	}
	if got := run(cacheDir); got != 1 {
		t.Fatalf("same-size, same-mtime edit to an application catalog: warm findings = %d, want 1", got)
	}
}
