package scan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// parseCacheMockDaemon is an analyze-project daemon that records the raw
// request and, when confirm is set, confirms a requested parse-cache bypass
// the way a daemon that knows no_parse_cache does. A daemon that predates the
// field ignores it and confirms nothing. The tests speak wire JSON because the
// field names are the contract between client and daemon versions (#1919).
type parseCacheMockDaemon struct {
	confirm bool
	mu      sync.Mutex
	raw     []string
}

func (m *parseCacheMockDaemon) requests() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.raw...)
}

func (m *parseCacheMockDaemon) start(t *testing.T) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-deleg-pc-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Root: currentMockRoot()}, nil
	})
	srv.Register(daemon.VerbAnalyzeProject, func(_ context.Context, raw json.RawMessage) (any, error) {
		m.mu.Lock()
		m.raw = append(m.raw, string(raw))
		m.mu.Unlock()
		var req map[string]any
		_ = json.Unmarshal(raw, &req)
		stats := map[string]any{"findings_count": 0}
		if m.confirm && req["no_parse_cache"] == true {
			stats["parse_cache_bypassed"] = true
		}
		return map[string]any{
			"findings": json.RawMessage(`{"rules":[],"line":[]}`),
			"stats":    stats,
		}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !daemon.Available(socket) {
		time.Sleep(5 * time.Millisecond)
	}
	return socket
}

func delegateWithMock(t *testing.T, m *parseCacheMockDaemon, noParseCache bool) bool {
	t.Helper()
	socket := m.start(t)
	root := newRoot(t)
	linkSock(t, root, socket)
	redirectStdout(t)
	f := freshScanFlags(t)
	*f.NoParseCache = noParseCache
	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	return handled
}

// A delegated --no-parse-cache scan asks the daemon to skip its parse cache.
func TestTryDaemonDelegate_ForwardsNoParseCache(t *testing.T) {
	m := &parseCacheMockDaemon{confirm: true}
	if !delegateWithMock(t, m, true) {
		t.Fatal("a daemon that confirms the bypass should serve the scan")
	}
	reqs := m.requests()
	if len(reqs) != 1 || !strings.Contains(reqs[0], `"no_parse_cache":true`) {
		t.Errorf("request did not carry no_parse_cache: %q", reqs)
	}
}

// A daemon that predates no_parse_cache serves the request from its parse
// cache and says nothing about it. The client must not take those findings
// as a cold-parse result: it falls back to an in-process scan.
func TestTryDaemonDelegate_NoParseCacheFallsBackWhenDaemonDoesNotConfirm(t *testing.T) {
	m := &parseCacheMockDaemon{confirm: false}
	if delegateWithMock(t, m, true) {
		t.Error("an unconfirmed --no-parse-cache delegation was accepted; want an in-process fallback")
	}
}

// Without --no-parse-cache the client sends nothing new and needs no
// confirmation, so an older daemon keeps serving ordinary scans.
func TestTryDaemonDelegate_WithoutNoParseCacheNeedsNoConfirmation(t *testing.T) {
	m := &parseCacheMockDaemon{confirm: false}
	if !delegateWithMock(t, m, false) {
		t.Fatal("an ordinary scan should be served by a daemon that predates no_parse_cache")
	}
	if reqs := m.requests(); len(reqs) != 1 || strings.Contains(reqs[0], "no_parse_cache") {
		t.Errorf("ordinary request should not mention no_parse_cache: %q", reqs)
	}
}
