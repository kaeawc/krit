package sessdaemon

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// fakeOracleStarter records every Start call and returns a pre-canned
// daemon (or error) so tests exercise the lifecycle without spinning
// up a real JVM. A nil daemon + nil err entry encodes the "JVM not
// configured" signal.
type fakeOracleStarter struct {
	calls   atomic.Int32
	returns []*oracle.Daemon
	errs    []error
}

func (f *fakeOracleStarter) Start(scanPaths []string) (*oracle.Daemon, error) {
	idx := int(f.calls.Add(1)) - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.returns) {
		return f.returns[idx], nil
	}
	return nil, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer(t.Context(), Options{
		RepoDir:    t.TempDir(),
		SocketPath: filepath.Join(t.TempDir(), "d.sock"),
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.session.Close() })
	return srv
}

// TestEnsureOracle_NotConfiguredDoesNotLatch verifies that the
// starter's nil-daemon-nil-err "not configured" signal returns nil
// without latching disabled — a deferred jar install must be able to
// recover on a later request without restarting the daemon.
func TestEnsureOracle_NotConfiguredDoesNotLatch(t *testing.T) {
	srv := newTestServer(t)
	fake := &fakeOracleStarter{}
	srv.oracle.starter = fake

	if d := srv.ensureOracle([]string{srv.repoDir}); d != nil {
		t.Errorf("ensureOracle returned %v; want nil when starter reports not configured", d)
	}
	if got := fake.calls.Load(); got != 1 {
		t.Errorf("starter called %d times; want 1", got)
	}
	if srv.oracle.disabled {
		t.Errorf("'not configured' should not latch disabled")
	}
}

// TestEnsureOracle_DisabledShortCircuits verifies that once disabled,
// the path stays disabled — no Start call, no Ping call.
func TestEnsureOracle_DisabledShortCircuits(t *testing.T) {
	srv := newTestServer(t)
	fake := &fakeOracleStarter{}
	srv.oracle.starter = fake
	srv.oracle.disabled = true

	if d := srv.ensureOracle([]string{srv.repoDir}); d != nil {
		t.Errorf("ensureOracle returned %v; want nil when disabled", d)
	}
	if got := fake.calls.Load(); got != 0 {
		t.Errorf("starter called %d times; want 0 when disabled", got)
	}
}

// TestEnsureOracle_StartErrorRetriesOnceThenDisables verifies the
// retry-once-then-fall-back contract from issue #207: two consecutive
// failures latch disabled, and subsequent requests skip the JVM
// without re-calling the starter.
func TestEnsureOracle_StartErrorRetriesOnceThenDisables(t *testing.T) {
	srv := newTestServer(t)
	fake := &fakeOracleStarter{errs: []error{errors.New("jvm boom"), errors.New("still boom")}}
	srv.oracle.starter = fake

	if d := srv.ensureOracle([]string{srv.repoDir}); d != nil {
		t.Errorf("ensureOracle returned %v; want nil after retry exhaustion", d)
	}
	if got := fake.calls.Load(); got != 2 {
		t.Errorf("starter called %d times; want 2 (initial + 1 retry)", got)
	}
	if !srv.oracle.disabled {
		t.Errorf("oracle.disabled = false; want true after both attempts failed")
	}

	prev := fake.calls.Load()
	if d := srv.ensureOracle([]string{srv.repoDir}); d != nil {
		t.Errorf("ensureOracle returned %v on subsequent call; want nil", d)
	}
	if got := fake.calls.Load(); got != prev {
		t.Errorf("starter called %d times after disabled latch; want %d", got, prev)
	}
}

// TestEnsureOracle_FirstAttemptSucceedsAfterRetry exercises the
// transient-failure path: first Start errors, second succeeds. No
// disable latch; the handle is stored on session.OracleDaemon.
func TestEnsureOracle_FirstAttemptSucceedsAfterRetry(t *testing.T) {
	srv := newTestServer(t)
	stub := &oracle.Daemon{}
	fake := &fakeOracleStarter{
		returns: []*oracle.Daemon{nil, stub},
		errs:    []error{errors.New("transient"), nil},
	}
	srv.oracle.starter = fake

	got := srv.ensureOracle([]string{srv.repoDir})
	if got != stub {
		t.Errorf("ensureOracle returned %v; want %v after transient failure recovery", got, stub)
	}
	if srv.oracle.disabled {
		t.Errorf("oracle.disabled latched after a recoverable failure")
	}
	if fake.calls.Load() != 2 {
		t.Errorf("starter called %d times; want 2", fake.calls.Load())
	}
}

// TestEnsureOracle_StoresOnSession verifies that once started the
// Daemon is stored on session.OracleDaemon so Session.Close stops the
// JVM during daemon shutdown.
func TestEnsureOracle_StoresOnSession(t *testing.T) {
	srv := newTestServer(t)
	stub := &oracle.Daemon{}
	fake := &fakeOracleStarter{returns: []*oracle.Daemon{stub}}
	srv.oracle.starter = fake

	got := srv.ensureOracle([]string{srv.repoDir})
	if got != stub {
		t.Errorf("ensureOracle returned %v; want %v", got, stub)
	}
	if srv.session.OracleDaemon != stub {
		t.Errorf("session.OracleDaemon = %v; want %v (so Session.Close shuts it down)", srv.session.OracleDaemon, stub)
	}
}
