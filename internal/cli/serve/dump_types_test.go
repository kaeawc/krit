package serve

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cli/scan"
	"github.com/kaeawc/krit/internal/daemon"
)

// TestDumpTypesVerb_NoJarReturnsExitCode2 pins the no-os.Exit contract
// for the dump-types daemon verb: when no krit-types.jar can be located
// (which is the case under t.TempDir() roots), the verb must surface
// exit code 2 with an explanatory stderr line — not crash the daemon
// process. Mirrors TestListRulesVerb_BadMaturityReturnsExitCode2's
// graceful-degrade pattern for the rest of the meta verbs.
//
// This is also the equivalence check the in-process flow exercises:
// scan.RunOutputTypesTo returns the same exit 2 + the same stderr
// banner when invoked with a missing jar, so daemon-routed and
// in-process --output-types invocations against a bare TempDir produce
// byte-identical stderr/exit-code triples.
func TestDumpTypesVerb_NoJarReturnsExitCode2(t *testing.T) {
	socket, state := startServerForTest(t)
	outputPath := filepath.Join(state.root, "types.json")

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbDumpTypes, daemon.DumpTypesArgs{
		Paths:      []string{state.root},
		OutputPath: outputPath,
	}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 2 {
		t.Fatalf("expected exit 2 for missing krit-types.jar, got %d (stderr=%q)",
			got.ExitCode, string(got.Stderr))
	}
	if !strings.Contains(string(got.Stderr), "krit-types.jar not found") {
		t.Errorf("expected jar-not-found stderr; got %q", string(got.Stderr))
	}
}

// TestDumpTypesVerb_MissingOutputPathReturnsExitCode2 asserts the
// daemon rejects an empty OutputPath with a typed error instead of
// silently dropping the request or attempting to write to "". The CLI
// gate prevents this in practice but the daemon must not trust the
// caller.
func TestDumpTypesVerb_MissingOutputPathReturnsExitCode2(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbDumpTypes, daemon.DumpTypesArgs{
		Paths: []string{"."},
	}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 2 {
		t.Fatalf("expected exit 2 for missing output_path, got %d", got.ExitCode)
	}
	if !strings.Contains(string(got.Stderr), "output_path is required") {
		t.Errorf("expected output_path-required stderr; got %q", string(got.Stderr))
	}
}

// TestDumpTypesVerb_PathsDefaultToRoot pins that an empty Paths slice
// falls back to the daemon's --root so callers that just want a dump
// of the resident project don't have to repeat the root.
func TestDumpTypesVerb_PathsDefaultToRoot(t *testing.T) {
	socket, state := startServerForTest(t)
	outputPath := filepath.Join(state.root, "types.json")

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbDumpTypes, daemon.DumpTypesArgs{
		// Paths intentionally omitted.
		OutputPath: outputPath,
	}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	// Without krit-types.jar the call still fails with exit 2; the
	// assertion is that the daemon reached the jar-lookup phase at
	// all (i.e. didn't reject the empty Paths up front).
	if got.ExitCode != 2 {
		t.Fatalf("expected exit 2 for missing jar against root, got %d (stderr=%q)",
			got.ExitCode, string(got.Stderr))
	}
	if !strings.Contains(string(got.Stderr), "krit-types.jar not found") {
		t.Errorf("expected jar-not-found stderr against root; got %q", string(got.Stderr))
	}
}

// TestDumpTypesVerb_EquivalentToInProcess pins byte-equivalence
// between the daemon-served dump-types path and the in-process
// scan.RunOutputTypesTo helper for the same inputs. Both flows must
// produce the same stderr banner and exit code so users see no
// behavioural drift when a daemon happens to be reachable.
//
// The test runs in a bare TempDir where no krit-types.jar can be
// located — the dump fails identically on both paths with exit 2 plus
// the documented "jar not found" line. This is the only equivalence
// check we can run without a JVM in the test environment; the JVM
// path itself is the same `oracle.Invoke` / `oracle.InvokeCached`
// helper both sides call into.
func TestDumpTypesVerb_EquivalentToInProcess(t *testing.T) {
	socket, state := startServerForTest(t)
	outputPath := filepath.Join(state.root, "types.json")

	var daemonRes daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbDumpTypes, daemon.DumpTypesArgs{
		Paths:      []string{state.root},
		OutputPath: outputPath,
	}, &daemonRes); err != nil {
		t.Fatalf("daemon call: %v", err)
	}

	var inProcStderr bytes.Buffer
	inProcCode := scan.RunOutputTypesTo(&inProcStderr, scan.RunOutputTypesOpts{
		OutputPath: outputPath,
		Paths:      []string{state.root},
	})

	if daemonRes.ExitCode != inProcCode {
		t.Errorf("exit code mismatch: daemon=%d inproc=%d", daemonRes.ExitCode, inProcCode)
	}
	if got, want := string(daemonRes.Stderr), inProcStderr.String(); got != want {
		t.Errorf("stderr mismatch:\ndaemon=%q\ninproc=%q", got, want)
	}
}
