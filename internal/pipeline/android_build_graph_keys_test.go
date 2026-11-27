package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
)

// buildGraphKeyProject lays out settings + app + lib + an unscanned feature module and returns the project a scan of
// only app/ and lib/ detects, with a writer that gives every write a distinct mtime.
func buildGraphKeyProject(t *testing.T, libManifest string) (*android.Project, func(rel, body string)) {
	t.Helper()
	dir := t.TempDir()
	tick := 0
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		tick++
		mtime := time.Unix(1_700_000_000+int64(tick), 0)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.gradle.kts", "include(\":app\", \":lib\", \":feature\")\n")
	write("app/build.gradle.kts", "dependencies { implementation(project(\":feature\")) }\n")
	write("lib/build.gradle.kts", "")
	write("feature/build.gradle.kts", "dependencies { api(project(\":lib\")) }\n")
	write("app/src/main/AndroidManifest.xml", "<manifest/>")
	write("lib/src/main/AndroidManifest.xml", libManifest)
	return &android.Project{
		ManifestPaths: []string{filepath.Join(dir, "app/src/main/AndroidManifest.xml"), filepath.Join(dir, "lib/src/main/AndroidManifest.xml")},
		GradlePaths:   []string{filepath.Join(dir, "app/build.gradle.kts"), filepath.Join(dir, "lib/build.gradle.kts")},
	}, write
}

const buildGraphKeyProvider = `<manifest><application><provider android:name=".P" android:readPermission="r" /></application></manifest>`

// providerReads is what ProviderReadPermissionOnly reads (buildFileReadersOf): the module graph, and every manifest's
// build script and catalogs.
var providerReads = buildFileReaders{moduleGraph: true, gradleInfo: true}

// The module-graph section is skipped when no active rule reads the graph, and when no manifest declares what the
// rules discover a graph for; both keep an unscanned build-file edit out of the key (#1800 third review). An
// intermediate module's script is no manifest's build script, so the manifest rules' section doesn't key it either.
func TestBuildFilesFingerprint_GraphSectionOnlyWhenRead(t *testing.T) {
	for _, test := range []struct {
		name        string
		libManifest string
		reads       buildFileReaders
		wantMove    bool
	}{
		{"rule reads the graph", buildGraphKeyProvider, providerReads, true},
		{"no active rule reads the graph", buildGraphKeyProvider, buildFileReaders{}, false},
		{"only manifest rules read build scripts", buildGraphKeyProvider, buildFileReaders{gradleInfo: true}, false},
		{"no manifest needs the graph", "<manifest><application/></manifest>", providerReads, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, write := buildGraphKeyProject(t, test.libManifest)
			hashutil.ResetDefault()
			before, ok := buildFilesFingerprint(project, test.reads)
			if !ok {
				t.Fatal("fingerprint failed")
			}
			write("feature/build.gradle.kts", "dependencies { testImplementation(project(\":lib\")) }\n")
			hashutil.ResetDefault()
			after, _ := buildFilesFingerprint(project, test.reads)
			if moved := after != before; moved != test.wantMove {
				t.Fatalf("fingerprint moved = %v, want %v", moved, test.wantMove)
			}
		})
	}
}

// One run discovers each build once and shares the layout between keys; the next run (a cleared hashutil memo)
// discovers again, so a module added between runs is keyed.
func TestProjectBuildGraphs_SharedWithinARun(t *testing.T) {
	project, write := buildGraphKeyProject(t, buildGraphKeyProvider)
	hashutil.ResetDefault()
	first := projectBuildGraphs(project)
	if len(first) != 1 || len(first[0].modules) != 3 {
		t.Fatalf("layouts = %+v, want one build with 3 modules", first)
	}
	write("extra/build.gradle.kts", "")
	if again := projectBuildGraphs(project); len(again[0].modules) != 3 {
		t.Fatalf("same run: %d modules, want the shared layout's 3", len(again[0].modules))
	}
	hashutil.ResetDefault()
	if next := projectBuildGraphs(project); len(next[0].modules) != 4 {
		t.Fatalf("next run: %d modules, want 4 with the new module", len(next[0].modules))
	}
}

// Without a settings file there is no module graph, but ProviderReadPermissionOnly, like every manifest rule, still
// reads each manifest's own build script, found by searching up from the manifest. A scan of only the source
// directories hashes none of them, so the build-script section keys every candidate of that search, present or
// missing, and nothing else (#1800 fourth review). It is written whenever a manifest rule is active, whether or not a
// manifest needs the graph (#1853), and skipped when none is.
func TestBuildFilesFingerprint_ManifestBuildScriptsWithoutSettings(t *testing.T) {
	const plain = "<manifest><application/></manifest>"
	for _, test := range []struct {
		name, libManifest string
		reads             buildFileReaders
		rel               string
		wantMove          bool
	}{
		{"app script edited", buildGraphKeyProvider, providerReads, "app/build.gradle.kts", true},
		{"nearer script created", buildGraphKeyProvider, providerReads, "app/src/build.gradle", true},
		{"library script edited", buildGraphKeyProvider, providerReads, "lib/build.gradle.kts", true},
		{"script outside every search", buildGraphKeyProvider, providerReads, "tools/build.gradle.kts", false},
		{"no manifest needs the graph", plain, providerReads, "app/build.gradle.kts", true},
		{"manifest rules only, nearer script created", plain, buildFileReaders{gradleInfo: true}, "app/src/build.gradle", true},
		{"manifest rules only, script outside every search", plain, buildFileReaders{gradleInfo: true}, "tools/build.gradle.kts", false},
		{"no active rule reads build scripts", buildGraphKeyProvider, buildFileReaders{}, "app/build.gradle.kts", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, write := buildGraphKeyProject(t, test.libManifest)
			root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(project.ManifestPaths[0]))))
			if err := os.Remove(filepath.Join(root, "settings.gradle.kts")); err != nil {
				t.Fatal(err)
			}
			project.GradlePaths = nil
			hashutil.ResetDefault()
			before, ok := buildFilesFingerprint(project, test.reads)
			if !ok {
				t.Fatal("fingerprint failed")
			}
			write(test.rel, "plugins { id(\"com.android.library\") }\n")
			hashutil.ResetDefault()
			after, _ := buildFilesFingerprint(project, test.reads)
			if moved := after != before; moved != test.wantMove {
				t.Fatalf("fingerprint moved = %v, want %v", moved, test.wantMove)
			}
		})
	}
}

// Every manifest rule, ProviderReadPermissionOnly included, reads whether a module is an app from version-catalog
// plugin aliases too, so the build-script section keys the catalog sources librarymodel's discovery finds for a
// script with aliases, and their content. Catalogs off every discovery don't move it (#1800 fifth review), and
// neither do they when no script has an alias or no manifest rule is active; a manifest that needs no module graph is
// still searched (#1853). Discovery merges the catalogs of every ancestor, for plugin aliases as for libs. dependency
// aliases (#1872), so a catalog past the nearest match moves the key for either.
func TestBuildFilesFingerprint_CatalogDiscovery(t *testing.T) {
	const catalog = "[plugins]\nandroid-application = { id = \"com.android.application\", version = \"8.5.0\" }\n"
	const plain = "<manifest><application/></manifest>"
	for _, test := range []struct {
		name, libManifest string
		reads             buildFileReaders
		// accessor is the catalog accessor of the build scripts' plugin aliases; "" writes no alias.
		accessor string
		// existing catalogs are written before the first fingerprint.
		existing []string
		rel      string
		wantMove bool
	}{
		{"root catalog created", buildGraphKeyProvider, providerReads, "plugs", nil, "gradle/libs.versions.toml", true},
		{"root catalog edited", buildGraphKeyProvider, providerReads, "plugs", []string{"gradle/libs.versions.toml"}, "gradle/libs.versions.toml", true},
		{"second root catalog created", buildGraphKeyProvider, providerReads, "plugs", []string{"gradle/libs.versions.toml"}, "gradle/more.versions.toml", true},
		{"nearer module catalog created", buildGraphKeyProvider, providerReads, "plugs", []string{"gradle/libs.versions.toml"}, "app/gradle/libs.versions.toml", true},
		{"catalog past the nearest match", buildGraphKeyProvider, providerReads, "plugs", []string{"app/gradle/libs.versions.toml", "lib/gradle/libs.versions.toml"}, "gradle/libs.versions.toml", true},
		{"catalog past the nearest match, libs. accessor", buildGraphKeyProvider, providerReads, "libs", []string{"app/gradle/libs.versions.toml", "lib/gradle/libs.versions.toml"}, "gradle/libs.versions.toml", true},
		{"root catalog edited, libs. accessor", buildGraphKeyProvider, providerReads, "libs", []string{"gradle/libs.versions.toml"}, "gradle/libs.versions.toml", true},
		{"catalog off every search", buildGraphKeyProvider, providerReads, "plugs", nil, "tools/gradle/libs.versions.toml", false},
		{"catalog off every search, libs. accessor", buildGraphKeyProvider, providerReads, "libs", nil, "tools/gradle/libs.versions.toml", false},
		{"not a versions.toml", buildGraphKeyProvider, providerReads, "plugs", nil, "gradle/notes.toml", false},
		{"no script has a plugin alias", buildGraphKeyProvider, providerReads, "", nil, "gradle/libs.versions.toml", false},
		{"no active rule reads build scripts", buildGraphKeyProvider, buildFileReaders{}, "plugs", nil, "gradle/libs.versions.toml", false},
		{"no manifest needs the graph", plain, providerReads, "plugs", nil, "gradle/libs.versions.toml", true},
		{"manifest rules only", plain, buildFileReaders{gradleInfo: true}, "plugs", nil, "gradle/libs.versions.toml", true},
		{"manifest rules only, catalog off every search", plain, buildFileReaders{gradleInfo: true}, "plugs", nil, "tools/gradle/libs.versions.toml", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, write := buildGraphKeyProject(t, test.libManifest)
			if test.accessor != "" {
				write("app/build.gradle.kts", "plugins { alias("+test.accessor+".plugins.android.application) }\ndependencies { implementation(project(\":feature\")) }\n")
				write("lib/build.gradle.kts", "plugins { alias("+test.accessor+".plugins.android.library) }\n")
			}
			for _, rel := range test.existing {
				write(rel, catalog)
			}
			hashutil.ResetDefault()
			before, ok := buildFilesFingerprint(project, test.reads)
			if !ok {
				t.Fatal("fingerprint failed")
			}
			write(test.rel, strings.Replace(catalog, "application\"", "library\"", 1))
			hashutil.ResetDefault()
			after, _ := buildFilesFingerprint(project, test.reads)
			if moved := after != before; moved != test.wantMove {
				t.Fatalf("fingerprint moved = %v, want %v", moved, test.wantMove)
			}
		})
	}
}
