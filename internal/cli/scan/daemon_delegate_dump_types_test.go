package scan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestTryDaemonDelegate_OutputTypesRoutesViaMetaVerb pins that the CLI
// dispatches --output-types through the daemon's dump-types verb
// (instead of falling back to in-process) when a daemon is reachable.
// The mock daemon captures the args it received so the test can assert
// the absolute-path absolutization and verbose / no-cache-oracle wire
// fields land correctly.
func TestTryDaemonDelegate_OutputTypesRoutesViaMetaVerb(t *testing.T) {
	var capturedArgs atomic.Value
	socketDir := startMockDumpTypesDaemon(t, &capturedArgs, daemon.MetaResult{
		Stderr:   []byte("daemon dump-types: ok\n"),
		ExitCode: 0,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	// Relative path on purpose: the CLI must absolutize before
	// forwarding so the daemon (which runs from its own CWD) writes to
	// the user's intended location.
	*f.OutputTypes = "rel/out.json"

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true; got fall-through")
	}
	if code != 0 {
		t.Errorf("expected exit 0; got %d", code)
	}
	got, ok := capturedArgs.Load().(daemon.DumpTypesArgs)
	if !ok {
		t.Fatal("daemon did not receive dump-types call")
	}
	if !filepath.IsAbs(got.OutputPath) {
		t.Errorf("expected absolute OutputPath; got %q", got.OutputPath)
	}
	if !strings.HasSuffix(got.OutputPath, "rel/out.json") {
		t.Errorf("unexpected OutputPath suffix: %q", got.OutputPath)
	}
}

// TestTryDaemonDelegate_OutputTypesPropagatesFlags asserts the
// --no-cache-oracle and --verbose flags ride the wire so daemon-served
// dump-types matches the in-process flow byte-for-byte on those knobs.
func TestTryDaemonDelegate_OutputTypesPropagatesFlags(t *testing.T) {
	var capturedArgs atomic.Value
	socketDir := startMockDumpTypesDaemon(t, &capturedArgs, daemon.MetaResult{ExitCode: 0})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.OutputTypes = "/tmp/out.json"
	*f.NoCacheOracle = true
	*f.Verbose = true

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true; got fall-through")
	}
	got := capturedArgs.Load().(daemon.DumpTypesArgs)
	if !got.NoCacheOracle {
		t.Errorf("NoCacheOracle not propagated")
	}
	if !got.Verbose {
		t.Errorf("Verbose not propagated")
	}
}

// startMockDumpTypesDaemon spins up a mock daemon that registers the
// dump-types verb only. The captured args is stored in capturedArgs
// so the calling test can assert on absolutization and propagation
// without exposing daemon internals.
func startMockDumpTypesDaemon(t *testing.T, capturedArgs *atomic.Value, reply daemon.MetaResult) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-deleg-dump-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(daemon.VerbDumpTypes, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.DumpTypesArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
		}
		capturedArgs.Store(args)
		return reply, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir
}
