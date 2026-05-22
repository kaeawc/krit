// Package daemonclient lets short-lived CLI verbs call a long-lived
// krit daemon when one is reachable, and fall back to in-process
// execution otherwise. Auto-spawn (start `krit serve` if no daemon is
// running) is opt-in via EnsureRunning.
package daemonclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// Client is a lightweight wrapper that remembers a socket path and
// lets verbs send daemon.Call without re-resolving the path. A
// zero-value Client (or nil) is unusable; construct via Discover or
// EnsureRunning.
type Client struct {
	socketPath string
}

// SocketPath returns the configured socket path.
func (c *Client) SocketPath() string {
	if c == nil {
		return ""
	}
	return c.socketPath
}

// Discover resolves the daemon socket path for a project rooted at
// repoRoot and returns a Client only when a daemon is reachable.
// Returns (nil, false) when no daemon is running — callers should
// fall back to in-process execution or call EnsureRunning to spawn
// one.
func Discover(repoRoot string) (*Client, bool) {
	socket := daemon.DefaultSocketPath(repoRoot)
	if !daemon.Available(socket) {
		return nil, false
	}
	return &Client{socketPath: socket}, true
}

// TryConnect is Discover with an explicit socket-path override. When
// socketOverride is empty the behaviour matches Discover; otherwise
// the override drives the dial directly so `--daemon-socket PATH`
// can point at non-default locations (test fixtures, multi-root
// setups). Never returns an error — the CLI's auto-detect contract
// is that socket noise does not bubble up to the user.
func TryConnect(repoRoot, socketOverride string) (*Client, bool) {
	if socketOverride == "" {
		return Discover(repoRoot)
	}
	if !daemon.Available(socketOverride) {
		return nil, false
	}
	return &Client{socketPath: socketOverride}, true
}

// CurrentBinaryHash returns the SHA-256 hex digest of the running CLI's
// binary. Empty when the executable can't be located or read; callers
// treat empty as "skip the handshake" so out-of-tree builds and tests
// stay usable.
func CurrentBinaryHash() string { return currentBinaryHash() }

// IsBinaryHashMismatch reports whether err is the daemon's
// "binary hash mismatch" rejection. Used by the CLI to print a
// one-line warning and fall back to in-process execution after a
// `go install` left a stale daemon resident.
func IsBinaryHashMismatch(err error) bool {
	return err != nil && strings.Contains(err.Error(), daemon.ErrBinaryHashMismatchPrefix)
}

// IsUnsupportedOracleBackend reports whether err is the daemon's
// "unsupported oracle backend" rejection. Used by the CLI to fall
// back to in-process execution when the user picked a backend the
// daemon doesn't spawn yet (currently anything other than
// AnalyzeProjectArgs.OracleBackend == "" / "kaa"). Mirrors
// IsBinaryHashMismatch's contract so daemon_delegate's
// runDaemonAnalyze can route both rejections through the same
// "warn + fall through" branch.
func IsUnsupportedOracleBackend(err error) bool {
	return err != nil && strings.Contains(err.Error(), daemon.ErrUnsupportedOracleBackendPrefix)
}

// EnsureCompatible discovers a running daemon, compares its
// reported binary hash against the running CLI's, and shuts the
// daemon down + (when opts.AutoRestart is true) spawns a fresh one
// when they differ. Use this from CLI entry points so a `krit`
// upgrade doesn't leave callers talking to a stale daemon. Returns
// nil with ok=false when no daemon is running and AutoRestart is
// false — callers fall back to in-process.
func EnsureCompatible(repoRoot string, opts SpawnOptions) (*Client, bool, error) {
	c, ok := Discover(repoRoot)
	if !ok {
		if !opts.AutoRestart {
			return nil, false, nil
		}
		c, err := EnsureRunning(repoRoot, opts)
		return c, c != nil, err
	}
	st, err := c.Status()
	if err != nil {
		// Daemon is up but its status verb is broken. Surface the
		// error so callers can decide between using the (possibly
		// degraded) client and falling back to in-process; we can't
		// tell from here.
		return c, true, fmt.Errorf("daemonclient: status: %w", err)
	}
	cliHash := currentBinaryHash()
	if st.BinaryHash == "" || cliHash == "" || st.BinaryHash == cliHash {
		return c, true, nil
	}
	// Versions diverged. Shut the running daemon down and (optionally)
	// spawn a fresh one.
	_ = c.Shutdown()
	waitForSocketGone(daemon.DefaultSocketPath(repoRoot), 2*time.Second)
	if !opts.AutoRestart {
		return nil, false, nil
	}
	fresh, err := EnsureRunning(repoRoot, opts)
	return fresh, fresh != nil, err
}

// waitForSocketGone polls until the socket path no longer exists or
// timeout elapses. Best-effort; the caller's downstream behavior is
// unchanged either way.
func waitForSocketGone(socket string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socket); os.IsNotExist(err) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// currentBinaryHash returns the SHA-256 hex digest of the running
// CLI's binary. Empty string disables the hash comparison so old
// daemons (or unreadable executables) don't trigger spurious
// restarts. Cached after the first call — the executable path
// doesn't change within a process lifetime, and the daemon-handshake
// path would otherwise re-read + re-hash a 50MB binary on every
// AnalyzeProject invocation.
func currentBinaryHash() string {
	if cached := binaryHashCache.Load(); cached != nil {
		return *cached
	}
	hash := computeCurrentBinaryHash()
	binaryHashCache.Store(&hash)
	return hash
}

func computeCurrentBinaryHash() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

var binaryHashCache atomic.Pointer[string]

// attachSpawnLog opens path append-only and assigns it to cmd.Stdout
// and cmd.Stderr. The caller is responsible for closing the returned
// *os.File after cmd.Start (or on Start failure) — exec.Cmd dup's the
// fd into the child at Start, so the parent's handle would otherwise
// leak one fd per spawn.
//
// Returns (nil, nil) when path is empty: the daemon's stdout/stderr
// are nil'd out and there is no parent fd to manage.
func attachSpawnLog(cmd *exec.Cmd, path string) (*os.File, error) {
	if path == "" {
		cmd.Stdout = nil
		cmd.Stderr = nil
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	cmd.Stdout = f
	cmd.Stderr = f
	return f, nil
}

// SpawnOptions controls EnsureRunning's behaviour.
type SpawnOptions struct {
	// Binary is the krit binary to exec. Empty defaults to os.Executable
	// (the current process's binary), which is the right choice when
	// the CLI itself spawns the daemon.
	Binary string
	// WaitTimeout is how long to wait for the daemon's socket to come
	// up after fork. Zero defaults to 2 seconds — short enough that a
	// pre-commit hook with this on the critical path stays responsive,
	// long enough that a normal cold start still wins. First-run cold
	// projects with massive warm cost should bump this explicitly.
	WaitTimeout time.Duration
	// PollInterval controls how often EnsureRunning re-checks for the
	// socket. Zero defaults to 25ms.
	PollInterval time.Duration
	// Env, when non-nil, overrides the spawned daemon's environment.
	// Nil inherits os.Environ.
	Env []string
	// LogPath, when non-empty, is the file the spawned daemon's
	// stdout+stderr land in. Empty discards them so daemon banner
	// output doesn't leak into the parent's terminal (relevant for
	// pre-commit hooks).
	LogPath string
	// AutoRestart controls EnsureCompatible's behaviour on a
	// version mismatch and on a missing daemon. False = honor only
	// the running daemon; true = spawn a fresh one when needed.
	AutoRestart bool
}

// EnsureRunning returns a Client connected to a daemon at the
// conventional socket path under repoRoot. If no daemon is reachable,
// EnsureRunning forks `<binary> serve --root <repoRoot>` in the
// background and waits until its socket is reachable.
//
// The spawned process is detached: closing the parent process does
// not stop the daemon. Callers that want a self-contained lifecycle
// (e.g. tests) should send `daemon.VerbShutdown` explicitly before
// exiting.
func EnsureRunning(repoRoot string, opts SpawnOptions) (*Client, error) {
	if c, ok := Discover(repoRoot); ok {
		return c, nil
	}
	binary := opts.Binary
	if binary == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("daemonclient: locate krit binary: %w", err)
		}
		binary = exe
	}
	socket := daemon.DefaultSocketPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		return nil, fmt.Errorf("daemonclient: prepare socket dir: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), binary, "serve", "--root", repoRoot)
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	cmd.Stdin = nil
	logFile, err := attachSpawnLog(cmd, opts.LogPath)
	if err != nil {
		return nil, fmt.Errorf("daemonclient: open log %s: %w", opts.LogPath, err)
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return nil, fmt.Errorf("daemonclient: spawn %s serve: %w", binary, err)
	}
	// exec.Cmd dup'd the fd into the child at Start; the parent's
	// handle is no longer needed and would otherwise leak one fd per
	// daemon respawn in a long-lived LSP/MCP session.
	if logFile != nil {
		_ = logFile.Close()
	}
	// Detach so the daemon outlives the parent. Wait must be reaped
	// somewhere — do it in a goroutine to avoid a zombie.
	go func() { _ = cmd.Wait() }()

	timeout := opts.WaitTimeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	poll := opts.PollInterval
	if poll == 0 {
		poll = 25 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			return &Client{socketPath: socket}, nil
		}
		time.Sleep(poll)
	}
	return nil, errors.New("daemonclient: spawned daemon did not become reachable before timeout")
}

// AnalyzeBuffer dispatches the analyze-buffer verb against the
// daemon. Returns the daemon's response on success.
func (c *Client) AnalyzeBuffer(args daemon.AnalyzeBufferArgs) (daemon.AnalyzeBufferResult, error) {
	if c == nil {
		return daemon.AnalyzeBufferResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.AnalyzeBufferResult
	if err := daemon.Call(c.socketPath, daemon.VerbAnalyzeBuffer, args, &result); err != nil {
		return daemon.AnalyzeBufferResult{}, err
	}
	return result, nil
}

// AnalyzeBuffers dispatches the batched analyze-buffers verb. The
// daemon processes the entire batch in one round trip, so callers
// with N staged files trade N dial+RTT cycles for one.
func (c *Client) AnalyzeBuffers(args daemon.AnalyzeBuffersArgs) (daemon.AnalyzeBuffersResult, error) {
	if c == nil {
		return daemon.AnalyzeBuffersResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.AnalyzeBuffersResult
	if err := daemon.Call(c.socketPath, daemon.VerbAnalyzeBuffers, args, &result); err != nil {
		return daemon.AnalyzeBuffersResult{}, err
	}
	return result, nil
}

// AnalyzeProject dispatches the analyze-project verb. The caller's
// binary hash is injected automatically when args.ClientBinaryHash is
// empty, so default callers always participate in the handshake. A
// daemon-side hash mismatch surfaces as an error that
// IsBinaryHashMismatch matches.
func (c *Client) AnalyzeProject(args daemon.AnalyzeProjectArgs) (daemon.AnalyzeProjectResult, error) {
	if c == nil {
		return daemon.AnalyzeProjectResult{}, errors.New("daemonclient: nil client")
	}
	if args.ClientBinaryHash == "" {
		args.ClientBinaryHash = currentBinaryHash()
	}
	var result daemon.AnalyzeProjectResult
	if err := daemon.Call(c.socketPath, daemon.VerbAnalyzeProject, args, &result); err != nil {
		return daemon.AnalyzeProjectResult{}, err
	}
	return result, nil
}

// Status returns the daemon's status payload. Mostly useful for the
// CLI's --daemon-status flag and for tests.
func (c *Client) Status() (daemon.StatusResult, error) {
	if c == nil {
		return daemon.StatusResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.StatusResult
	if err := daemon.Call(c.socketPath, daemon.VerbStatus, nil, &result); err != nil {
		return daemon.StatusResult{}, err
	}
	return result, nil
}

// Shutdown asks the daemon to exit. Safe to call after the daemon is
// already gone.
func (c *Client) Shutdown() error {
	if c == nil {
		return nil
	}
	return daemon.Call(c.socketPath, daemon.VerbShutdown, nil, nil)
}

// ListRules dispatches the list-rules verb against the daemon and
// returns the captured stdout/stderr/exit-code triple. The CLI replays
// these against its own streams so daemon-routed and in-process
// invocations of `--list-rules` are byte-equivalent.
func (c *Client) ListRules(args daemon.ListRulesArgs) (daemon.MetaResult, error) {
	if c == nil {
		return daemon.MetaResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.MetaResult
	if err := daemon.Call(c.socketPath, daemon.VerbListRules, args, &result); err != nil {
		return daemon.MetaResult{}, err
	}
	return result, nil
}

// ListExperiments dispatches the list-experiments verb against the
// daemon. Mirrors ListRules' captured-stream contract.
func (c *Client) ListExperiments(args daemon.ListExperimentsArgs) (daemon.MetaResult, error) {
	if c == nil {
		return daemon.MetaResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.MetaResult
	if err := daemon.Call(c.socketPath, daemon.VerbListExperiments, args, &result); err != nil {
		return daemon.MetaResult{}, err
	}
	return result, nil
}

// ValidateConfig dispatches the validate-config verb against the
// daemon. ConfigPath="" lets the daemon use its resident config; pass
// an explicit path to validate a specific file (mirrors --config FILE
// semantics).
func (c *Client) ValidateConfig(args daemon.ValidateConfigArgs) (daemon.MetaResult, error) {
	if c == nil {
		return daemon.MetaResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.MetaResult
	if err := daemon.Call(c.socketPath, daemon.VerbValidateConfig, args, &result); err != nil {
		return daemon.MetaResult{}, err
	}
	return result, nil
}

// OracleFilterFingerprint dispatches the oracle-filter-fingerprint
// verb. The daemon walks the requested paths, builds the active rule
// set, and emits the JSON fingerprint report the CI drift gate
// consumes — without invoking the krit-types JVM.
func (c *Client) OracleFilterFingerprint(args daemon.OracleFilterFingerprintArgs) (daemon.MetaResult, error) {
	if c == nil {
		return daemon.MetaResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.MetaResult
	if err := daemon.Call(c.socketPath, daemon.VerbOracleFilterFingerprint, args, &result); err != nil {
		return daemon.MetaResult{}, err
	}
	return result, nil
}

// DumpTypes dispatches the dump-types verb (CLI's --output-types).
// The daemon runs krit-types against the requested scan paths and
// writes the oracle JSON dump to args.OutputPath, which must be
// absolute. Captured stderr / exit code is replayed by the CLI so
// daemon-routed and in-process --output-types invocations stay
// byte-equivalent.
func (c *Client) DumpTypes(args daemon.DumpTypesArgs) (daemon.MetaResult, error) {
	if c == nil {
		return daemon.MetaResult{}, errors.New("daemonclient: nil client")
	}
	var result daemon.MetaResult
	if err := daemon.Call(c.socketPath, daemon.VerbDumpTypes, args, &result); err != nil {
		return daemon.MetaResult{}, err
	}
	return result, nil
}

// ClearCache asks the daemon to delete its on-disk caches and drop
// resident WorkspaceState slots. The caller's binary hash is injected
// automatically when args.ClientBinaryHash is empty.
func (c *Client) ClearCache(args daemon.ClearCacheArgs) (daemon.ClearCacheResult, error) {
	if c == nil {
		return daemon.ClearCacheResult{}, errors.New("daemonclient: nil client")
	}
	if args.ClientBinaryHash == "" {
		args.ClientBinaryHash = currentBinaryHash()
	}
	var result daemon.ClearCacheResult
	if err := daemon.Call(c.socketPath, daemon.VerbClearCache, args, &result); err != nil {
		return daemon.ClearCacheResult{}, err
	}
	return result, nil
}

// ClearMatrixCache asks the daemon to delete the experiment-matrix
// baseline cache. The matrix cache is not held resident in the daemon,
// so this is purely a wrapped on-disk delete; the verb exists so a
// running daemon can serve the request without spawning a fresh
// in-process krit when the user passes --clear-matrix-cache.
func (c *Client) ClearMatrixCache(args daemon.ClearMatrixCacheArgs) (daemon.ClearMatrixCacheResult, error) {
	if c == nil {
		return daemon.ClearMatrixCacheResult{}, errors.New("daemonclient: nil client")
	}
	if args.ClientBinaryHash == "" {
		args.ClientBinaryHash = currentBinaryHash()
	}
	var result daemon.ClearMatrixCacheResult
	if err := daemon.Call(c.socketPath, daemon.VerbClearMatrixCache, args, &result); err != nil {
		return daemon.ClearMatrixCacheResult{}, err
	}
	return result, nil
}
