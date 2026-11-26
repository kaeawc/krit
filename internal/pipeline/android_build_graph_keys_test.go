package pipeline

import (
	"os"
	"path/filepath"
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

// The module-graph section is skipped when no active rule reads the graph, and when no manifest declares what the
// rules discover a graph for; both keep an unscanned build-file edit out of the key (#1800 third review).
func TestBuildFilesFingerprint_GraphSectionOnlyWhenRead(t *testing.T) {
	for _, test := range []struct {
		name        string
		libManifest string
		readsGraph  bool
		wantMove    bool
	}{
		{"rule reads the graph", buildGraphKeyProvider, true, true},
		{"no active rule reads the graph", buildGraphKeyProvider, false, false},
		{"no manifest needs the graph", "<manifest><application/></manifest>", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, write := buildGraphKeyProject(t, test.libManifest)
			hashutil.ResetDefault()
			before, ok := buildFilesFingerprint(project, test.readsGraph)
			if !ok {
				t.Fatal("fingerprint failed")
			}
			write("feature/build.gradle.kts", "dependencies { testImplementation(project(\":lib\")) }\n")
			hashutil.ResetDefault()
			after, _ := buildFilesFingerprint(project, test.readsGraph)
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
	first := projectBuildGraphs(project).layouts
	if len(first) != 1 || len(first[0].modules) != 3 {
		t.Fatalf("layouts = %+v, want one build with 3 modules", first)
	}
	write("extra/build.gradle.kts", "")
	if again := projectBuildGraphs(project).layouts; len(again[0].modules) != 3 {
		t.Fatalf("same run: %d modules, want the shared layout's 3", len(again[0].modules))
	}
	hashutil.ResetDefault()
	if next := projectBuildGraphs(project).layouts; len(next[0].modules) != 4 {
		t.Fatalf("next run: %d modules, want 4 with the new module", len(next[0].modules))
	}
}

// Without a settings file there is no module graph, but ProviderReadPermissionOnly still reads each manifest's own
// build script, found by searching up from the manifest. A scan of only the source directories hashes none of them,
// so the graph section keys every candidate of that search, present or missing, and nothing else (#1800 fourth
// review).
func TestBuildFilesFingerprint_ManifestBuildScriptsWithoutSettings(t *testing.T) {
	for _, test := range []struct {
		name, libManifest string
		readsGraph        bool
		rel               string
		wantMove          bool
	}{
		{"app script edited", buildGraphKeyProvider, true, "app/build.gradle.kts", true},
		{"nearer script created", buildGraphKeyProvider, true, "app/src/build.gradle", true},
		{"library script edited", buildGraphKeyProvider, true, "lib/build.gradle.kts", true},
		{"script outside every search", buildGraphKeyProvider, true, "tools/build.gradle.kts", false},
		{"no active rule reads the graph", buildGraphKeyProvider, false, "app/build.gradle.kts", false},
		{"no manifest needs the graph", "<manifest><application/></manifest>", true, "app/build.gradle.kts", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, write := buildGraphKeyProject(t, test.libManifest)
			root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(project.ManifestPaths[0]))))
			if err := os.Remove(filepath.Join(root, "settings.gradle.kts")); err != nil {
				t.Fatal(err)
			}
			project.GradlePaths = nil
			hashutil.ResetDefault()
			before, ok := buildFilesFingerprint(project, test.readsGraph)
			if !ok {
				t.Fatal("fingerprint failed")
			}
			write(test.rel, "plugins { id(\"com.android.library\") }\n")
			hashutil.ResetDefault()
			after, _ := buildFilesFingerprint(project, test.readsGraph)
			if moved := after != before; moved != test.wantMove {
				t.Fatalf("fingerprint moved = %v, want %v", moved, test.wantMove)
			}
		})
	}
}
