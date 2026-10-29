package serve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/devjar"
	"github.com/kaeawc/krit/internal/pipeline"
)

// An added file must be analyzed by the next warm call. The daemon
// prepopulates the source-path lists from the prior manifest, which cannot name
// a file created after it was written, and the bundle fingerprint is computed
// over those lists, so nothing downstream notices the file is missing. Found by
// the warm-versus-clean mutation matrix (tests/parity, #786).
func TestAnalyzeProject_AddedFileIsSeenByWarmCall(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A {\n    fun a() {}\n}\n")
	call := func() (r daemon.AnalyzeProjectResult) {
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, daemon.AnalyzeProjectArgs{NoFir: true}, &r); err != nil {
			t.Fatal(err)
		}
		return
	}
	call()
	if !call().Stats.FindingsBundleHit {
		t.Fatal("control must hit the bundle")
	}
	p := writeKotlinFile(t, state.root, "B.kt", "package demo\n\nclass B\n")
	state.workspace.Touch(p)
	state.workspace.BumpSourceMTimeVersion()
	warm := call()
	if !strings.Contains(string(warm.Findings), "B.kt") {
		t.Fatalf("added B.kt missing from warm findings (bundleHit=%v)", warm.Stats.FindingsBundleHit)
	}
}

// A daemon started after a file was added (a restart, or a first call after
// the project changed while no daemon ran) must see it too: its first call
// used to take the path lists from the on-disk manifest of the previous daemon.
func TestAnalyzeProject_FileAddedWhileDaemonWasDownIsSeen(t *testing.T) {
	isolateFromHelperJars(t)
	root := t.TempDir()
	writeKotlinFile(t, root, "A.kt", "package demo\n\nclass A {\n    fun a() {}\n}\n")

	analyze := func(socket string) daemon.AnalyzeProjectResult {
		var r daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, daemon.AnalyzeProjectArgs{NoFir: true}, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	socket, _ := startServerWith(t, root, 5*time.Second)
	analyze(socket)
	// The manifest is written in the background; a daemon that stays up long
	// enough for the project to change has persisted it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if m, _ := filepath.Glob(filepath.Join(root, ".krit", "findings-bundle-cache", "manifests", "*", "*")); len(m) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first daemon never persisted its bundle manifest; the test would prove nothing")
		}
		time.Sleep(20 * time.Millisecond)
	}

	writeKotlinFile(t, root, "B.kt", "package demo\n\nclass B\n")
	socket2, _ := startServerWith(t, root, 5*time.Second)
	if got := analyze(socket2); !strings.Contains(string(got.Findings), "B.kt") {
		t.Fatalf("B.kt, added while no daemon ran, is missing from the new daemon's first call (bundleHit=%v)", got.Stats.FindingsBundleHit)
	}
}

func TestDropStalePrepopulatedPaths(t *testing.T) {
	known := map[string]string{"/r/src/A.kt": "h", "/r/gone/S.kt": "h"}
	newInput := func() *pipeline.ProjectInput {
		in := &pipeline.ProjectInput{}
		in.Args.Paths = []string{"/r"}
		in.Args.KotlinPaths = []string{"/r/src/A.kt"}
		in.Args.JavaPaths = []string{"/r/src/B.java"}
		in.Host.PriorContentHashes = known
		return in
	}
	cases := []struct {
		name  string
		cold  bool
		dirty []string
		drop  bool
	}{
		{"warm and clean", false, nil, false},
		{"warm edit of a known file", false, []string{"/r/src/A.kt"}, false},
		{"warm created file", false, []string{"/r/src/New.kt"}, true},
		{"warm created java file", false, []string{"/r/src/New.java"}, true},
		{"warm known edit plus created file", false, []string{"/r/src/A.kt", "/r/src/New.kt"}, true},
		{"cold start", true, nil, true},
		{"warm directory gone with known sources", false, []string{"/r/gone"}, true},
		{"warm path gone with no known sources", false, []string{"/r/other"}, false},
		{"warm layout xml edit", false, []string{"/r/app/src/main/res/layout/a.xml"}, false},
		{"warm krit.yml edit", false, []string{"/r/krit.yml"}, false},
		{"warm version catalog edit", false, []string{"/r/gradle/libs.versions.toml"}, false},
		{"warm groovy gradle edit", false, []string{"/r/app/build.gradle"}, false},
		{"warm xml edit plus known source edit", false, []string{"/r/res/layout/a.xml", "/r/src/A.kt"}, false},
		{"warm xml edit plus created source", false, []string{"/r/res/layout/a.xml", "/r/src/New.kt"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := newInput()
			dropStalePrepopulatedPaths(in, tc.cold, tc.dirty)
			if dropped := in.Args.KotlinPaths == nil && in.Args.JavaPaths == nil; dropped != tc.drop {
				t.Fatalf("dropped = %v, want %v", dropped, tc.drop)
			}
		})
	}
}

// A directory moved out of the tree must stop contributing findings. The
// watcher reports one event for the directory (and none for its files), and
// the next analyze has to notice that the source set shrank.
func TestAnalyzeProject_DirectoryMovedOutIsForgotten(t *testing.T) {
	isolateFromHelperJars(t)
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A {\n    fun a() {}\n}\n")
	sub := filepath.Join(state.root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeKotlinFile(t, sub, "S.kt", "package demo\n\nclass S {\n    fun s() {}\n}\n")

	call := func() (r daemon.AnalyzeProjectResult) {
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject, daemon.AnalyzeProjectArgs{NoFir: true}, &r); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !strings.Contains(string(call().Findings), "S.kt") {
		t.Fatal("setup: S.kt must produce a finding before the move (the test would prove nothing)")
	}
	if !call().Stats.FindingsBundleHit {
		t.Fatal("control must hit the bundle")
	}

	elsewhere := filepath.Join(t.TempDir(), "sub")
	if err := os.Rename(sub, elsewhere); err != nil {
		t.Skipf("cannot move a directory out of the project: %v", err)
	}
	// What the filesystem watcher does for the directory's Rename event.
	state.workspace.Touch(sub)
	state.workspace.BumpSourceMTimeVersion()
	warm := call()
	if strings.Contains(string(warm.Findings), "S.kt") {
		t.Fatalf("findings for S.kt, moved out of the tree, are still served (bundleHit=%v)", warm.Stats.FindingsBundleHit)
	}
}

// With a scan root such as ../lib the manifest spells its keys relative to the
// working directory; a directory moved out from under that root must still be
// recognised as gone.
func TestSourceSetMayHaveChanged_CwdRelativeManifestKeys(t *testing.T) {
	parent := t.TempDir()
	work := filepath.Join(parent, "app")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	gone := filepath.Join(parent, "lib", "gone") // never created: moved out
	manifest := map[string]string{"../lib/gone/B.kt": "h"}
	if !sourceSetMayHaveChanged([]string{gone}, manifest, []string{"../lib"}) {
		t.Fatal("a vanished directory with cwd-relative manifest entries beneath it went undetected")
	}
	if sourceSetMayHaveChanged([]string{filepath.Join(parent, "lib", "other")}, manifest, []string{"../lib"}) {
		t.Fatal("a vanished path with no known sources beneath it must not count")
	}
}

// isolateFromHelperJars keeps a daemon test off any krit-fir jar
// installed on the machine: with one present the daemon would connect to (or
// start) a real oracle JVM instead of running the structural pipeline alone.
func isolateFromHelperJars(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_FIR_JAR", "")
	t.Setenv(devjar.DirEnv, t.TempDir())
}
