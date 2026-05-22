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

// TestOracleJarForBackend_PicksMatchingJar pins the backend → jar
// routing. BackendKAA must consult oracle.FindJar (krit-types);
// BackendFIR must consult firchecks.FindFirJar. Empty/unknown
// backends fall through to KAA so callers can pass the result of
// oracle.ParseBackend without re-checking. We exercise the routing
// against a tmpdir with neither jar installed so both branches
// return "" and the call is observable purely through whether it
// took the FIR or KAA discovery path; that's captured by the t
// helper sub-tests below which seed a jar in the matching layout
// for each backend.
func TestOracleJarForBackend_PicksMatchingJar(t *testing.T) {
	t.Run("kaa-finds-krit-types", func(t *testing.T) {
		tmp := t.TempDir()
		seedJar(t, tmp, ".krit/krit-types.jar")
		if got := oracleJarForBackend([]string{tmp}, oracle.BackendKAA); got == "" {
			t.Error("BackendKAA must return non-empty when krit-types.jar is in place")
		}
	})
	t.Run("fir-finds-krit-fir", func(t *testing.T) {
		tmp := t.TempDir()
		seedJar(t, tmp, ".krit/krit-fir.jar")
		if got := oracleJarForBackend([]string{tmp}, oracle.BackendFIR); got == "" {
			t.Error("BackendFIR must return non-empty when krit-fir.jar is in place")
		}
	})
	t.Run("fir-ignores-krit-types", func(t *testing.T) {
		// Only krit-types.jar present; FIR lookup must not return it.
		tmp := t.TempDir()
		seedJar(t, tmp, ".krit/krit-types.jar")
		if got := oracleJarForBackend([]string{tmp}, oracle.BackendFIR); got != "" {
			t.Errorf("BackendFIR returned %q but only krit-types.jar exists; routing leaked the wrong jar", got)
		}
	})
	t.Run("kaa-ignores-krit-fir", func(t *testing.T) {
		tmp := t.TempDir()
		seedJar(t, tmp, ".krit/krit-fir.jar")
		if got := oracleJarForBackend([]string{tmp}, oracle.BackendKAA); got != "" {
			t.Errorf("BackendKAA returned %q but only krit-fir.jar exists; routing leaked the wrong jar", got)
		}
	})
}

// TestEnsureOracleDaemon_BackendKeyIsolation verifies that the two
// backends populate independent cache slots; a KAA call must not
// reuse the FIR entry and vice versa. Uses the fake starter so we
// can prove the routing without spinning real JVMs. Each starter
// call gets a distinct *oracle.Daemon pointer so the test can
// detect any cache-key collision.
func TestEnsureOracleDaemon_BackendKeyIsolation(t *testing.T) {
	tmp := t.TempDir()
	seedJar(t, tmp, ".krit/krit-types.jar")
	seedJar(t, tmp, ".krit/krit-fir.jar")

	state := newDaemonState(tmp)
	t.Cleanup(state.closeOracleDaemons)
	kaa := &oracle.Daemon{}
	fir := &oracle.Daemon{}
	state.oracleDaemonStarter = &fakeOracleDaemonStarter{returns: []*oracle.Daemon{kaa, fir}}

	dk, err := state.ensureOracleDaemon([]string{tmp}, oracle.BackendKAA)
	if err != nil {
		t.Fatalf("KAA ensure: %v", err)
	}
	df, err := state.ensureOracleDaemon([]string{tmp}, oracle.BackendFIR)
	if err != nil {
		t.Fatalf("FIR ensure: %v", err)
	}
	if dk == df {
		t.Errorf("KAA and FIR daemons share a cache slot: dk=%p df=%p (key isolation broken)", dk, df)
	}
	// Second KAA call must reuse the cached pointer.
	dk2, err := state.ensureOracleDaemon([]string{tmp}, oracle.BackendKAA)
	if err != nil {
		t.Fatalf("KAA second ensure: %v", err)
	}
	if dk2 != dk {
		t.Errorf("KAA cache miss on second call: dk=%p dk2=%p", dk, dk2)
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
