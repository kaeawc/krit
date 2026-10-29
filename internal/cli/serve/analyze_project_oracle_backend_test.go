package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestHandleAnalyzeProject_RejectsUnknownOracleBackend pins the
// validation gate. An AnalyzeProjectArgs.OracleBackend value that
// oracle.ParseBackend can't classify must come back with the typed
// ErrUnsupportedOracleBackendPrefix so the CLI's runDaemonAnalyze
// can route to the silent in-process fallback instead of failing
// the scan.
func TestHandleAnalyzeProject_RejectsUnknownOracleBackend(t *testing.T) {
	state := newDaemonState(t.TempDir())
	args := daemon.AnalyzeProjectArgs{NoFir: true, OracleBackend: "totally-not-a-backend"}
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

// TestOracleJarForBackend_FindsKritFir pins that the daemon resolves the
// krit-fir jar from the project's .krit directory.
func TestOracleJarForBackend_FindsKritFir(t *testing.T) {
	tmp := t.TempDir()
	seedJar(t, tmp, ".krit/krit-fir.jar")
	if got := oracleJarForBackend([]string{tmp}, oracle.BackendFIR); got == "" {
		t.Error("BackendFIR must return non-empty when krit-fir.jar is in place")
	}
}

// seedJar writes a zero-byte file at root/rel so the jar-discovery
// path returns a non-empty match. Useful for asserting the routing
// without actually spawning a JVM. Cleaned up automatically by
// t.TempDir's lifecycle.
func seedJar(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, nil, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
