package modshard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

// writeSymlinkedModel writes a two-project model under a resolved directory,
// spelling its paths through a symlink to it when viaLink is set, and returns
// the resolved root, the symlink and the model directory.
func writeSymlinkedModel(t *testing.T, viaLink bool) (resolvedRoot, link, modelDir string) {
	t.Helper()
	resolvedRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link = filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(resolvedRoot, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	spelled := resolvedRoot
	if viaLink {
		spelled = link
	}
	var projects []string
	for _, name := range []string{"app", "lib"} {
		src := filepath.Join(resolvedRoot, name, "src", "main", "kotlin")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, strings.ToUpper(name)+".kt"), []byte("package "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		projects = append(projects, fmt.Sprintf(`{"path":":%s","dir":%q,"sourceSets":[{"name":"main","kind":"main","platform":"jvm","sourceDirs":[%q],"generatedSourceDirs":[%q]}]}`,
			name, filepath.Join(spelled, name), filepath.Join(spelled, name, "src", "main", "kotlin"), filepath.Join(spelled, name, "build", "generated")))
	}
	modelDir = filepath.Join(resolvedRoot, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "m.json"), []byte(`{"schema":2,"projects":[`+strings.Join(projects, ",")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return resolvedRoot, link, modelDir
}

func loadForTest(t *testing.T, modelDir string) *gradlemodel.Model {
	t.Helper()
	model, _, err := gradlemodel.Load(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

// A scan through a symlink of the model's resolved root (macOS /tmp and
// /private/tmp) plans the scanned source sets, with files spelled like the
// scan, so the check request's files match them (#1674).
func TestPlanMatchesAScanPathThatIsASymlinkOfTheModelRoot(t *testing.T) {
	resolvedRoot, link, modelDir := writeSymlinkedModel(t, false)
	model := loadForTest(t, modelDir)
	if units, _ := Plan(model, resolvedRoot, []string{link}); len(units) != 0 {
		t.Fatalf("an unaligned model matched the symlinked scan path (%d units); the test no longer covers #1674", len(units))
	}
	alignModelPaths(model, []string{link})
	units, _ := Plan(model, link, []string{link})
	if len(units) != 2 {
		t.Fatalf("got %d units, want :app:main and :lib:main", len(units))
	}
	want := filepath.Join(link, "app", "src", "main", "kotlin", "APP.kt")
	check := &checkContext{files: map[string]bool{want: true}}
	if got := check.requestedAmong(units[0].Sources); len(got) != 1 || got[0] != want {
		t.Fatalf("requested files of %s = %v, want [%s]; sources %v", units[0].Spec.ID, got, want, units[0].Sources)
	}
	if got := units[0].Spec.GeneratedSourceRoots; len(got) != 1 || got[0] != filepath.Join(link, "app", "build", "generated") {
		t.Fatalf("generated roots %v not respelled", got)
	}
}

// The reverse: the model spells its paths through the symlink and the scan
// path is the resolved directory.
func TestAlignModelPathsRespellsASymlinkedModelForAResolvedScan(t *testing.T) {
	resolvedRoot, _, modelDir := writeSymlinkedModel(t, true)
	model := loadForTest(t, modelDir)
	alignModelPaths(model, []string{resolvedRoot})
	if units, _ := Plan(model, resolvedRoot, []string{resolvedRoot}); len(units) != 2 {
		t.Fatalf("got %d units, want 2", len(units))
	}
	if got := model.Projects[0].Dir; got != filepath.Join(resolvedRoot, "app") {
		t.Fatalf("project dir %s, want %s", got, filepath.Join(resolvedRoot, "app"))
	}
}

// A scan of one project through the symlink plans only that project; a
// sibling project outside the scan path keeps the model's spelling.
func TestAlignModelPathsForAScanBelowTheRoot(t *testing.T) {
	resolvedRoot, link, modelDir := writeSymlinkedModel(t, false)
	model := loadForTest(t, modelDir)
	scan := filepath.Join(link, "lib")
	alignModelPaths(model, []string{scan})
	units, _ := Plan(model, link, []string{scan})
	if len(units) != 1 || units[0].Spec.ID != ":lib:main" || !strings.HasPrefix(units[0].Sources[0], scan) {
		t.Fatalf("units %+v, want only :lib:main spelled under %s", units, scan)
	}
	if got := model.Projects[0].Dir; got != filepath.Join(resolvedRoot, "app") {
		t.Fatalf("sibling project dir %s, want it unchanged", got)
	}
}

// A project above the scan path is respelled with it: a scan of
// <link>/app/src finds :app's source set under the link.
func TestAlignModelPathsRespellsAProjectAboveTheScanPath(t *testing.T) {
	_, link, modelDir := writeSymlinkedModel(t, false)
	model := loadForTest(t, modelDir)
	scan := filepath.Join(link, "app", "src")
	alignModelPaths(model, []string{scan})
	if got := model.Projects[0].Dir; got != filepath.Join(link, "app") {
		t.Fatalf("project dir %s, want %s", got, filepath.Join(link, "app"))
	}
	if units, _ := Plan(model, link, []string{scan}); len(units) != 1 || units[0].Spec.ID != ":app:main" {
		t.Fatalf("units %+v, want only :app:main", units)
	}
}

// A model already spelled like the scan is left untouched.
func TestAlignModelPathsKeepsAMatchingSpelling(t *testing.T) {
	resolvedRoot, _, modelDir := writeSymlinkedModel(t, false)
	model := loadForTest(t, modelDir)
	before := model.Projects[1].SourceSets[0].SourceDirs[0]
	alignModelPaths(model, []string{resolvedRoot})
	if got := model.Projects[1].SourceSets[0].SourceDirs[0]; got != before {
		t.Fatalf("source dir %s changed to %s", before, got)
	}
}

// The settings split logs why it does not run instead of returning silently.
func TestCheckSettingsGroupsLogsWhyItDoesNotSplit(t *testing.T) {
	var log strings.Builder
	logf := func(format string, args ...any) { fmt.Fprintf(&log, format, args...) }
	request := []byte(`{"id":1,"rules":["R"],"files":[{"path":"/x/A.kt"}]}`)
	_, _, _, err := CheckSettingsGroups(t.Context(), Input{ScanPaths: []string{t.TempDir()}, JarPath: "j", JavaPath: "java", CacheDir: t.TempDir(), CheckRequest: request, Logf: logf})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "verbose: FIR settings: not split: no Gradle model") {
		t.Fatalf("log %q has no reason", log.String())
	}
}
