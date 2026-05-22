package serve

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestHandleAnalyzeProject_RejectsUnknownOracleBackend pins the new
// validation gate. An AnalyzeProjectArgs.OracleBackend value that
// oracle.ParseBackend can't classify must come back with the typed
// ErrUnsupportedOracleBackendPrefix so the CLI's runDaemonAnalyze
// can route to the silent in-process fallback instead of failing
// the scan.
func TestHandleAnalyzeProject_RejectsUnknownOracleBackend(t *testing.T) {
	state := newDaemonState(t.TempDir())
	args := daemon.AnalyzeProjectArgs{OracleBackend: "totally-not-a-backend"}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = handleAnalyzeProject(context.Background(), state, raw)
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
	if !strings.Contains(err.Error(), daemon.ErrUnsupportedOracleBackendPrefix) {
		t.Errorf("error missing typed prefix %q: %v", daemon.ErrUnsupportedOracleBackendPrefix, err)
	}
}

// TestHandleAnalyzeProject_RejectsFIRBackendForNow holds the
// transitional contract: the daemon's resident OracleDaemon is still
// spawned via oracle.FindJar (krit-types). Until a later PR plumbs
// krit-fir into the same lifecycle, the FIR backend must return the
// typed ErrUnsupportedOracleBackendPrefix so the CLI falls back to
// in-process — preserving the historical `--oracle-backend fir`
// behavior end users had before the wire carried this field.
func TestHandleAnalyzeProject_RejectsFIRBackendForNow(t *testing.T) {
	state := newDaemonState(t.TempDir())
	args := daemon.AnalyzeProjectArgs{OracleBackend: "fir"}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_, err = handleAnalyzeProject(context.Background(), state, raw)
	if err == nil {
		t.Fatal("expected typed error for FIR backend (not yet supported daemon-side)")
	}
	if !strings.Contains(err.Error(), daemon.ErrUnsupportedOracleBackendPrefix) {
		t.Errorf("error missing typed prefix %q: %v", daemon.ErrUnsupportedOracleBackendPrefix, err)
	}
	if !strings.Contains(err.Error(), "fir") {
		t.Errorf("error message should mention the offending backend; got %v", err)
	}
}
