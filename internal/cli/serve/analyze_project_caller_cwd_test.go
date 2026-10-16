package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/output"
)

// callerCwdFixture lays out one daemon root holding two projects whose
// files share relative spellings ("src/....kt") but not contents, so a
// scan that resolves a path against the wrong directory, or replays
// state built from another directory, shows up in the reported files.
//
//	<root>/proj/src/{ProjA,ProjB}.kt
//	<root>/other/src/Other.kt
func callerCwdFixture(t *testing.T) (socket, root, proj, other string) {
	t.Helper()
	socket, state := startServerForTest(t)
	root = state.root
	proj = filepath.Join(root, "proj")
	other = filepath.Join(root, "other")
	for dir, classes := range map[string][]string{
		filepath.Join(proj, "src"):  {"ProjA", "ProjB"},
		filepath.Join(other, "src"): {"Other"},
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for _, class := range classes {
			writeKotlinFile(t, dir, class+".kt", smellyKotlinClass(class))
		}
	}
	return socket, root, proj, other
}

// smellyKotlinClass returns a class the default rule set reports on, so
// each scanned file contributes at least one finding path.
func smellyKotlinClass(name string) string {
	return "package demo\n\nclass " + name + " {\n" +
		"    fun run() {\n" +
		"        try {\n" +
		"            println(\"" + name + "\")\n" +
		"        } catch (e: Exception) {\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
}

// analyzeFrom calls analyze-project the way a CLI invocation in cwd
// would: its working directory on the wire, paths spelled relative to it.
func analyzeFrom(t *testing.T, socket, cwd string, paths ...string) daemon.AnalyzeProjectResult {
	t.Helper()
	var res daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true, Cwd: cwd, Paths: paths}, &res); err != nil {
		t.Fatalf("analyze-project cwd=%s paths=%v: %v", cwd, paths, err)
	}
	return res
}

// findingFiles returns the sorted, deduplicated file paths the response
// reports findings for, in the spelling the daemon emitted.
func findingFiles(t *testing.T, res daemon.AnalyzeProjectResult) []string {
	t.Helper()
	var report output.JSONReport
	if err := json.Unmarshal(res.Findings, &report); err != nil {
		t.Fatalf("decode findings: %v\n%s", err, res.Findings)
	}
	seen := map[string]bool{}
	for _, f := range report.Findings {
		seen[f.File] = true
	}
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

func assertScan(t *testing.T, label string, res daemon.AnalyzeProjectResult, want ...string) {
	t.Helper()
	if res.Stats.FilesScanned != len(want) {
		t.Errorf("%s: FilesScanned = %d, want %d", label, res.Stats.FilesScanned, len(want))
	}
	got := findingFiles(t, res)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s: finding files = %v, want %v", label, got, want)
	}
}

// TestAnalyzeProject_SameProjectFromTwoDirectories scans one project
// through one daemon under two relative spellings, each from the
// directory that spelling is relative to. Both name the same absolute
// path, so a manifest keyed on location alone hands the second scan the
// first one's file list: paths spelled "proj/src/..." that do not exist
// from inside proj.
func TestAnalyzeProject_SameProjectFromTwoDirectories(t *testing.T) {
	socket, root, proj, _ := callerCwdFixture(t)

	fromRoot := []string{filepath.Join("proj", "src", "ProjA.kt"), filepath.Join("proj", "src", "ProjB.kt")}
	fromProj := []string{filepath.Join("src", "ProjA.kt"), filepath.Join("src", "ProjB.kt")}

	t.Chdir(root)
	assertScan(t, "krit proj (from root)", analyzeFrom(t, socket, root, "proj"), fromRoot...)

	t.Chdir(proj)
	assertScan(t, "krit . (from proj)", analyzeFrom(t, socket, proj, "."), fromProj...)

	// Back again: each spelling keeps its own manifest and replays it.
	t.Chdir(root)
	again := analyzeFrom(t, socket, root, "proj")
	assertScan(t, "krit proj (from root, again)", again, fromRoot...)
	if !again.Stats.FindingsBundleHit {
		t.Errorf("repeat scan under the first spelling should replay its bundle; got %+v", again.Stats)
	}
}

// TestAnalyzeProject_ResolvesPathsAgainstCallerCwd is the daemon that
// was spawned somewhere else: the caller's working directory is not the
// daemon's. The scan must cover the caller's tree and report it in the
// caller's spelling, and the same spelling from a different directory
// must not be served the previous directory's resident state.
func TestAnalyzeProject_ResolvesPathsAgainstCallerCwd(t *testing.T) {
	socket, root, proj, other := callerCwdFixture(t)
	// The daemon runs in this process; park it at the root the way a
	// CLI invocation from there would have spawned it. t.Chdir also
	// restores the directory the daemon leaves the process in.
	t.Chdir(root)

	assertScan(t, "krit . (from proj)", analyzeFrom(t, socket, proj, "."),
		filepath.Join("src", "ProjA.kt"), filepath.Join("src", "ProjB.kt"))

	assertScan(t, "krit . (from other)", analyzeFrom(t, socket, other, "."),
		filepath.Join("src", "Other.kt"))

	assertScan(t, "krit ../proj (from other)", analyzeFrom(t, socket, other, filepath.Join("..", "proj")),
		filepath.Join("..", "proj", "src", "ProjA.kt"), filepath.Join("..", "proj", "src", "ProjB.kt"))

	assertScan(t, "krit proj other (from root)", analyzeFrom(t, socket, root, "proj", "other"),
		filepath.Join("other", "src", "Other.kt"),
		filepath.Join("proj", "src", "ProjA.kt"), filepath.Join("proj", "src", "ProjB.kt"))
}

// TestAnalyzeProject_CallerCwdChangeIsCold pins that resident state is
// rebuilt, not trusted, once the caller's directory changes: the
// watcher's "nothing changed" only vouches for the file set it was
// asked about.
func TestAnalyzeProject_CallerCwdChangeIsCold(t *testing.T) {
	socket, root, proj, other := callerCwdFixture(t)
	t.Chdir(root)

	if first := analyzeFrom(t, socket, proj, "."); !first.Stats.Cold {
		t.Fatalf("first scan should be cold; got %+v", first.Stats)
	}
	if same := analyzeFrom(t, socket, proj, "."); same.Stats.Cold {
		t.Errorf("repeat scan from the same directory should be warm; got %+v", same.Stats)
	}
	if moved := analyzeFrom(t, socket, other, "."); !moved.Stats.Cold {
		t.Errorf("scan from a new directory should be cold; got %+v", moved.Stats)
	}
}

func TestAnalyzeProject_RejectsUnusableCallerCwd(t *testing.T) {
	socket, root, _, _ := callerCwdFixture(t)
	t.Chdir(root)

	for name, cwd := range map[string]string{
		"missing":  filepath.Join(root, "no-such-dir"),
		"relative": "proj",
	} {
		t.Run(name, func(t *testing.T) {
			var res daemon.AnalyzeProjectResult
			err := daemon.Call(socket, daemon.VerbAnalyzeProject,
				daemon.AnalyzeProjectArgs{NoFir: true, Cwd: cwd, Paths: []string{"."}}, &res)
			if err == nil || !strings.Contains(err.Error(), "caller working directory") {
				t.Fatalf("expected a caller-working-directory error, got err=%v stats=%+v", err, res.Stats)
			}
		})
	}
	// The failed requests must not have moved the daemon.
	assertScan(t, "krit proj (from root)", analyzeFrom(t, socket, root, "proj"),
		filepath.Join("proj", "src", "ProjA.kt"), filepath.Join("proj", "src", "ProjB.kt"))
}
