package oracle

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var errBreakerOpen = errors.New("daemon: circuit breaker open")

const (
	breakerThreshold = 3
	breakerCooldown  = 30 * time.Second
)

// startDaemonReady is the JSON message the daemon sends when --port is used.
type startDaemonReady struct {
	Ready bool `json:"ready"`
	Port  int  `json:"port"`
}

// daemonRequestTimeout returns the max wall-clock duration allowed for a
// single daemon request/response round-trip (env KRIT_TYPES_REQUEST_TIMEOUT,
// default 10 minutes). This is separate from the startup timeout: a single
// analyze call can legitimately take minutes on a large repo, but it should
// never take hours. A hang past this limit usually means the Analysis API hit
// an unhandled internal error.
func daemonRequestTimeout() time.Duration {
	if v := os.Getenv("KRIT_TYPES_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Minute
}

// send writes a request and reads the corresponding response, returning
// the full decoded envelope. Caller must hold d.mu.
//
// If the daemon doesn't produce a response within KRIT_TYPES_REQUEST_TIMEOUT
// (default 10m), the daemon process is force-killed, the Daemon is marked
// not-started so subsequent calls fail fast, and an error is returned. The
// caller is expected to fall back to tree-sitter-only analysis — the oracle
// was unreliable and any retry on the same Daemon will fail immediately.
//
// Most callers only need the Result field; they project via sendResult.
// The analyzeWithDeps path needs Errors + CacheDeps siblings of Result so
// it calls send directly.
func (d *Daemon) send(method string, params map[string]interface{}) (*daemonResponse, error) {
	if !d.started {
		return nil, fmt.Errorf("daemon not started")
	}

	if !d.breakerAdmit() {
		return nil, errBreakerOpen
	}
	resp, err := d.sendOnce(method, params)
	d.breakerRecord(err)
	return resp, err
}

func (d *Daemon) breakerAdmit() bool {
	d.breakerMu.Lock()
	defer d.breakerMu.Unlock()
	if d.breakerFailures < breakerThreshold {
		return true
	}
	return daemonNow().Sub(d.breakerOpenedAt) >= breakerCooldown
}

func (d *Daemon) breakerRecord(err error) {
	d.breakerMu.Lock()
	defer d.breakerMu.Unlock()
	if err == nil {
		d.breakerFailures = 0
		return
	}
	d.breakerFailures++
	if d.breakerFailures >= breakerThreshold {
		d.breakerOpenedAt = daemonNow()
	}
}

func (d *Daemon) sendOnce(method string, params map[string]interface{}) (*daemonResponse, error) {
	id := d.nextID
	d.nextID++

	req := daemonRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write newline-delimited JSON
	data = append(data, '\n')
	if _, err := d.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to daemon: %w", err)
	}

	// Read response. The Scan() call would block forever if the daemon
	// hangs mid-request (e.g. Analysis API FIR crash with no thrown
	// exception), so run it in a goroutine and select against a timer.
	type scanResult struct {
		line string
		ok   bool
		err  error
	}
	resultCh := make(chan scanResult, 1)
	go func() {
		if d.stdout.Scan() {
			resultCh <- scanResult{line: d.stdout.Text(), ok: true}
			return
		}
		resultCh <- scanResult{err: d.stdout.Err()}
	}()

	timeout := daemonRequestTimeout()
	var line string
	select {
	case res := <-resultCh:
		if !res.ok {
			if res.err != nil {
				return nil, fmt.Errorf("read from daemon: %w", res.err)
			}
			return nil, fmt.Errorf("daemon closed stdout unexpectedly")
		}
		line = res.line
	case <-time.After(timeout):
		// Kill the daemon so the hung Scan() goroutine can exit (closing
		// the stdout pipe makes Scan() return false). Mark started=false
		// so any subsequent call short-circuits without reading stale
		// bytes from a dead pipe.
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		d.started = false
		return nil, fmt.Errorf("daemon request timed out after %s (method=%s); daemon killed, falling back to tree-sitter", timeout, method)
	}

	var resp daemonResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (got: %s)", err, line)
	}

	if resp.ID != id {
		return nil, fmt.Errorf("response ID mismatch: expected %d, got %d", id, resp.ID)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	return &resp, nil
}

// sendResult is the legacy wrapper that projects a daemon response to just
// its Result field. Used by Analyze, AnalyzeAll, Rebuild, Ping, Checkpoint —
// methods that don't consume the Errors / CacheDeps sibling fields.
func (d *Daemon) sendResult(method string, params map[string]interface{}) (*json.RawMessage, error) {
	resp, err := d.send(method, params)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// waitPipeReady reads the first line from scanner with a timeout and validates
// it as a daemonResponse JSON. Returns the parsed response or an error.
// Kills cmd.Process on any failure.
func waitPipeReady(cmd *exec.Cmd, scanner *bufio.Scanner) (*daemonResponse, error) {
	type scanResult struct {
		line string
		err  error
	}
	readyCh := make(chan scanResult, 1)
	go func() {
		if scanner.Scan() {
			readyCh <- scanResult{line: scanner.Text()}
		} else {
			readyCh <- scanResult{err: scanner.Err()}
		}
	}()

	const startupTimeout = 30 * time.Second
	var line string
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon startup: %w", res.err)
		}
		if res.line == "" {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon closed stdout before sending ready message")
		}
		line = res.line
	case <-time.After(startupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup timed out after %s", startupTimeout)
	}
	var readyResp daemonResponse
	if err := json.Unmarshal([]byte(line), &readyResp); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon ready message: invalid JSON: %w (got: %s)", err, line)
	}
	if readyResp.Error != "" {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup error: %s", readyResp.Error)
	}
	return &readyResp, nil
}

// waitPortReady reads the first line from stdoutPipe with a timeout and validates
// it as a startDaemonReady JSON (ready+port). Kills cmd.Process on failure.
func waitPortReady(cmd *exec.Cmd, stdoutPipe io.Reader) (*startDaemonReady, error) {
	type scanResult struct {
		line string
		err  error
	}
	readyCh := make(chan scanResult, 1)
	go func() {
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)
		if sc.Scan() {
			readyCh <- scanResult{line: sc.Text()}
		} else {
			readyCh <- scanResult{err: sc.Err()}
		}
	}()

	const daemonStartupTimeout = 30 * time.Second
	var line string
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon startup: %w", res.err)
		}
		if res.line == "" {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon closed stdout before sending ready message")
		}
		line = res.line
	case <-time.After(daemonStartupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup timed out after %s (JVM may be slow to initialize)", daemonStartupTimeout)
	}

	var ready startDaemonReady
	if err := json.Unmarshal([]byte(line), &ready); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon ready message: invalid JSON: %w (got: %s)", err, line)
	}
	if !ready.Ready || ready.Port == 0 {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon did not report ready with port (got: %s)", line)
	}
	return &ready, nil
}
