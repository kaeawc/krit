package serve

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/pipeline"
)

// An added file must be analyzed by the next warm call. The daemon
// prepopulates the source-path lists from the prior manifest, which cannot name
// a file created after it was written, and the bundle fingerprint is computed
// over those lists, so nothing downstream notices the file is missing. Found by
// the warm-versus-clean mutation matrix (tests/parity, #786).
func TestAnalyzeProject_AddedFileIsSeenByWarmCall(t *testing.T) {
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
	known := map[string]string{"/r/src/A.kt": "h"}
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
