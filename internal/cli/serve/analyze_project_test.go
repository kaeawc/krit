package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_RoundTrip drives the verb end-to-end against a
// freshly-spun daemon: write one Kotlin file into the daemon root,
// call analyze-project, assert the response carries findings JSON
// for that file and Stats.Cold == true on the first call.
func TestAnalyzeProject_RoundTrip(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}

	if len(got.Findings) == 0 {
		t.Fatal("expected non-empty Findings JSON")
	}
	if !got.Stats.Cold {
		t.Errorf("first call should report Cold=true; got %+v", got.Stats)
	}
	if got.Stats.FilesScanned < 1 {
		t.Errorf("expected FilesScanned >= 1, got %d", got.Stats.FilesScanned)
	}
	// Findings JSON must parse — guards against a regression that
	// returns a partial / malformed payload.
	var probe map[string]any
	if err := json.Unmarshal(got.Findings, &probe); err != nil {
		t.Fatalf("findings JSON does not parse: %v\n%s", err, got.Findings)
	}
}

// TestAnalyzeProject_SecondCallNotCold confirms the coldDone flag
// flips after the first successful call. Subsequent verb invocations
// report Stats.Cold=false; this is the signal clients use to gate
// "warm-up indicator" UX.
func TestAnalyzeProject_SecondCallNotCold(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", "package demo\n\nclass A\n")

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if !first.Stats.Cold {
		t.Fatalf("first call: expected Cold=true, got %+v", first.Stats)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.Stats.Cold {
		t.Errorf("second call: expected Cold=false, got %+v", second.Stats)
	}
}

// TestAnalyzeProject_RequireWarmRejectsCold pins the documented
// fast-path: clients that opted into RequireWarm get a typed error
// on the first invocation rather than a long blocking wait.
func TestAnalyzeProject_RequireWarmRejectsCold(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "X.kt", "package demo\n\nclass X\n")

	var got daemon.AnalyzeProjectResult
	err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{RequireWarm: true}, &got)
	if err == nil {
		t.Fatalf("expected error on cold daemon, got result=%+v", got)
	}
	if !strings.Contains(err.Error(), "daemon not warm yet") {
		t.Errorf("expected 'daemon not warm yet' error; got %v", err)
	}
}

// TestAnalyzeProject_RequireWarmSucceedsAfterFirstCall confirms the
// gate flips: a second call with RequireWarm=true succeeds after the
// first warming call completes.
func TestAnalyzeProject_RequireWarmSucceedsAfterFirstCall(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Y.kt", "package demo\n\nclass Y\n")

	// First call (warming).
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &daemon.AnalyzeProjectResult{}); err != nil {
		t.Fatalf("warm call: %v", err)
	}
	// Second call (RequireWarm) should now pass.
	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{RequireWarm: true}, &got); err != nil {
		t.Fatalf("RequireWarm call after warming: %v", err)
	}
	if got.Stats.Cold {
		t.Errorf("post-warming call should report Cold=false, got %+v", got.Stats)
	}
}

// TestAnalyzeProject_DirtyFilesReportedFromWatcherTouch wires the
// full chain: filesystem write → watcher Touch → DrainDirty →
// Stats.DirtyFiles > 0. Without the watcher running this test
// directly Touches via the workspace API to keep the assertion
// hermetic; the end-to-end watcher path is covered separately in
// step 8's integration test.
func TestAnalyzeProject_DirtyFilesReportedFromWatcherTouch(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Z.kt", "package demo\n\nclass Z\n")

	// First call drains any dirty marks left from setup.
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &daemon.AnalyzeProjectResult{}); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Touch via the workspace API (simulates a watcher event).
	state.workspace.Touch(filepath.Join(state.root, "Z.kt"))

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &got); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got.Stats.DirtyFiles != 1 {
		t.Errorf("expected DirtyFiles=1, got %+v", got.Stats)
	}
}

// writeKotlinFile is a local helper for the verb tests — distinct
// from the pipeline package's writeKotlin which also parses. The
// daemon path runs ParsePhase itself, so we only need the file on
// disk.
func writeKotlinFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
