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
	"github.com/kaeawc/krit/internal/oracle"
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

// TestRunStrictVerify_RejectsBogusBackend pins the validation gate.
// runStrictVerify must surface oracle.ParseBackend's error rather
// than silently substituting the default, otherwise a typo'd
// AnalyzeProjectArgs.OracleBackend slips through as "matched
// KAA on both sides" and masks any FIR-only daemon bug.
func TestRunStrictVerify_RejectsBogusBackend(t *testing.T) {
	root := t.TempDir()
	writeKotlinFile(t, root, "X.kt", "package demo\n\nclass X\n")

	state := newDaemonState(root)
	state.strictVerify = true

	daemonCols := scanner.FindingColumns{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := daemon.AnalyzeProjectArgs{OracleBackend: "not-a-backend"}
	_, err := state.runStrictVerify(ctx, args, &daemonCols)
	if err == nil {
		t.Fatal("expected error for bogus baseline oracle backend")
	}
	if !strings.Contains(err.Error(), "baseline oracle backend") {
		t.Errorf("expected error to mention baseline oracle backend; got %q", err.Error())
	}
}

// TestRunStrictVerify_KAABaselineSharesOracleDaemon pins the
// apples-to-apples contract for KAA: the baseline must reuse the
// daemon's resident oracle handle (same JVM, same session) so a
// no-source-change comparison only exercises non-oracle resident
// state. Two ensureOracleDaemon calls with the same backend return
// the cached entry, so we can assert pointer equality after a
// strict-verify run.
//
// Drives the helper directly: a fake starter records pointer
// identities, and we cross-check that the slot used by the daemon
// call is the same one runStrictVerify wires into ProjectHostState.
func TestRunStrictVerify_KAABaselineSharesOracleDaemon(t *testing.T) {
	root := t.TempDir()
	seedJar(t, root, ".krit/krit-types.jar")
	writeKotlinFile(t, root, "X.kt", "package demo\n\nclass X\n")

	state := newDaemonState(root)
	state.strictVerify = true
	resident := &oracle.Daemon{}
	state.oracleDaemonStarter = &fakeOracleDaemonStarter{returns: []*oracle.Daemon{resident}}

	// First call seeds the cache as the daemon analyze path would.
	if _, err := state.ensureOracleDaemon([]string{root}, oracle.BackendKAA); err != nil {
		t.Fatalf("ensure KAA: %v", err)
	}

	// The strict-verify baseline must reuse the same slot. We can't
	// easily peek at ProjectHostState the helper builds, but the
	// public observable is: a second ensureOracleDaemon call from
	// runStrictVerify must return resident without incrementing the
	// starter call count.
	prevCalls := state.oracleDaemonStarter.(*fakeOracleDaemonStarter).calls.Load()
	daemonCols := scanner.FindingColumns{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Use OracleBackend="" which parses to DefaultBackend (KAA).
	_, _ = state.runStrictVerify(ctx, daemon.AnalyzeProjectArgs{}, &daemonCols)
	newCalls := state.oracleDaemonStarter.(*fakeOracleDaemonStarter).calls.Load()
	if newCalls != prevCalls {
		t.Errorf("baseline spawned a fresh starter call (%d → %d); expected to reuse the daemon's resident KAA slot", prevCalls, newCalls)
	}
}

// TestRunStrictVerify_FIRBaselineSharesOracleDaemon mirrors the KAA
// case for BackendFIR — the post-#586 daemon spawns krit-fir under
// the same ensureOracleDaemon contract. Without this guard a
// strict-verify run with --oracle-backend=fir would silently spawn
// a second JVM for the baseline, defeating the apples-to-apples
// intent.
func TestRunStrictVerify_FIRBaselineSharesOracleDaemon(t *testing.T) {
	root := t.TempDir()
	seedJar(t, root, ".krit/krit-fir.jar")
	writeKotlinFile(t, root, "X.kt", "package demo\n\nclass X\n")

	state := newDaemonState(root)
	state.strictVerify = true
	resident := &oracle.Daemon{}
	state.oracleDaemonStarter = &fakeOracleDaemonStarter{returns: []*oracle.Daemon{resident}}

	if _, err := state.ensureOracleDaemon([]string{root}, oracle.BackendFIR); err != nil {
		t.Fatalf("ensure FIR: %v", err)
	}

	prevCalls := state.oracleDaemonStarter.(*fakeOracleDaemonStarter).calls.Load()
	daemonCols := scanner.FindingColumns{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = state.runStrictVerify(ctx, daemon.AnalyzeProjectArgs{OracleBackend: "fir"}, &daemonCols)
	newCalls := state.oracleDaemonStarter.(*fakeOracleDaemonStarter).calls.Load()
	if newCalls != prevCalls {
		t.Errorf("FIR baseline spawned a fresh starter call (%d → %d); expected to reuse the daemon's resident FIR slot", prevCalls, newCalls)
	}
}
