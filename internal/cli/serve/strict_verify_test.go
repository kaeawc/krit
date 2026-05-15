package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAnalyzeProject_StrictVerifyHappyPath turns on the strict-verify
// gate and confirms that the analyze-project response still returns
// successfully when the daemon's resident-state path matches a fresh
// in-process baseline. The path is the one alpha clients will exercise
// during divergence hunts; this test pins it as the no-divergence
// regression baseline.
//
// Doesn't measure: timing (strict-verify ~2x by design), nor the
// envelope's stats fields. Both are covered by the non-strict-verify
// TestAnalyzeProject_RoundTrip suite.
func TestAnalyzeProject_StrictVerifyHappyPath(t *testing.T) {
	socket, state := startServerForTest(t)
	state.strictVerify = true
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &got); err != nil {
		t.Fatalf("strict-verify analyze call: %v", err)
	}
	if len(got.Findings) == 0 {
		t.Fatal("expected non-empty Findings JSON under strict-verify")
	}
	var probe map[string]any
	if err := json.Unmarshal(got.Findings, &probe); err != nil {
		t.Fatalf("findings JSON does not parse: %v\n%s", err, got.Findings)
	}
}

// TestRunStrictVerify_DetectsAddedRow constructs a daemonCols set with
// one extra synthetic finding the in-process baseline cannot produce
// and asserts runStrictVerify returns a divergence error plus a log
// path under .krit/.
//
// Drives the helper directly (no daemon socket / wire involved) so the
// failure mode is easy to read: any future regression that masks
// divergence in the comparison or the log-path allocation surfaces
// here without an end-to-end fixture.
func TestRunStrictVerify_DetectsAddedRow(t *testing.T) {
	root := t.TempDir()
	writeKotlinFile(t, root, "X.kt", "package demo\n\nclass X\n")

	state := newDaemonState(root)
	state.strictVerify = true

	// Synthesize a daemon-side row that the cold baseline cannot
	// emit: a fictional rule + file pair. daemon.Compare flags it as
	// AddedByDaemon, which is the divergence shape we want to catch.
	daemonCols := scanner.CollectFindings([]scanner.Finding{
		{
			File:     filepath.Join(root, "X.kt"),
			Line:     1,
			Col:      1,
			Severity: "error",
			RuleSet:  "synthetic",
			Rule:     "DaemonOnlyRule",
			Message:  "row the baseline does not emit",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logPath, err := state.runStrictVerify(ctx, daemon.AnalyzeProjectArgs{}, &daemonCols)
	if err == nil {
		t.Fatalf("expected divergence error; got logPath=%q err=nil", logPath)
	}
	if logPath == "" {
		t.Fatalf("expected divergence log path; got empty (err=%v)", err)
	}
	if !strings.Contains(err.Error(), "divergence") {
		t.Errorf("expected error message to mention divergence; got %q", err.Error())
	}
	if _, statErr := os.Stat(logPath); statErr != nil {
		t.Fatalf("divergence log not written at %s: %v", logPath, statErr)
	}
}
