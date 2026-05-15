package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cli/daemoncmd"
)

var (
	daemonBinPath string
	daemonBinOnce sync.Once
	daemonBinErr  error
)

// buildDaemonBinary builds krit-daemon once per test run alongside the
// krit binary so resolveDaemonBinary's "next to krit" lookup succeeds.
func buildDaemonBinary(t *testing.T) string {
	t.Helper()
	daemonBinOnce.Do(func() {
		daemonBinPath = filepath.Join(filepath.Dir(binPath), "krit-daemon")
		cmd := exec.Command("go", "build", "-o", daemonBinPath, "../krit-daemon")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			daemonBinErr = err
		}
	})
	if daemonBinErr != nil {
		t.Fatalf("build krit-daemon: %v", daemonBinErr)
	}
	return daemonBinPath
}

func TestDaemonStartStatusStop(t *testing.T) {
	buildDaemonBinary(t)
	repo := shortTempDir(t)

	// start
	stdout, stderr, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s")
	if code != 0 {
		log, _ := os.ReadFile(filepath.Join(repo, ".krit", "daemon.log"))
		t.Fatalf("daemon start: exit=%d stdout=%q stderr=%q log=%q", code, stdout, stderr, log)
	}
	if !strings.Contains(stdout, "krit-daemon started") {
		t.Fatalf("expected start banner, got %q", stdout)
	}
	t.Cleanup(func() {
		_, _, _ = runKrit(t, "daemon", "stop", "--repo", repo)
	})

	// status (text)
	stdout, _, code = runKrit(t, "daemon", "status", "--repo", repo)
	if code != 0 {
		t.Fatalf("daemon status: exit=%d", code)
	}
	if !strings.Contains(stdout, "running") {
		t.Fatalf("expected running, got %q", stdout)
	}
	if !strings.Contains(stdout, "pid:") || !strings.Contains(stdout, "socket:") {
		t.Fatalf("status text missing fields: %q", stdout)
	}

	// status (json)
	stdout, _, code = runKrit(t, "daemon", "status", "--repo", repo, "--json")
	if code != 0 {
		t.Fatalf("daemon status --json: exit=%d", code)
	}
	var st daemoncmd.DaemonStatus
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		t.Fatalf("decode status json: %v\nbody=%s", err, stdout)
	}
	if !st.Running {
		t.Fatalf("status.Running = false, want true (%+v)", st)
	}
	if st.PID == 0 {
		t.Fatalf("status.PID = 0, want non-zero")
	}
	if !strings.HasSuffix(st.SocketPath, "daemon.sock") {
		t.Fatalf("status.SocketPath = %q", st.SocketPath)
	}

	// idempotent start
	stdout, _, code = runKrit(t, "daemon", "start", "--repo", repo)
	if code != 0 {
		t.Fatalf("idempotent start: exit=%d", code)
	}
	if !strings.Contains(stdout, "already running") {
		t.Fatalf("expected 'already running', got %q", stdout)
	}

	// stop
	stdout, _, code = runKrit(t, "daemon", "stop", "--repo", repo)
	if code != 0 {
		t.Fatalf("daemon stop: exit=%d stdout=%q", code, stdout)
	}

	// status after stop
	stdout, _, code = runKrit(t, "daemon", "status", "--repo", repo, "--json")
	if code != 0 {
		t.Fatalf("daemon status post-stop: exit=%d", code)
	}
	var afterStop daemoncmd.DaemonStatus
	if err := json.Unmarshal([]byte(stdout), &afterStop); err != nil {
		t.Fatalf("decode post-stop status: %v\nbody=%s", err, stdout)
	}
	if afterStop.Running {
		t.Fatalf("expected Running=false after stop, got %+v", afterStop)
	}
}

func TestDaemonRestart(t *testing.T) {
	buildDaemonBinary(t)
	repo := shortTempDir(t)
	t.Cleanup(func() {
		_, _, _ = runKrit(t, "daemon", "stop", "--repo", repo)
	})

	_, _, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s")
	if code != 0 {
		t.Fatalf("initial start: exit=%d", code)
	}
	firstStatus := jsonStatus(t, repo)
	if !firstStatus.Running {
		t.Fatalf("not running after start: %+v", firstStatus)
	}

	stdout, stderr, code := runKrit(t, "daemon", "restart", "--repo", repo)
	if code != 0 {
		t.Fatalf("restart: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	// Restart should produce a daemon with a different PID.
	deadline := time.Now().Add(10 * time.Second)
	var second daemoncmd.DaemonStatus
	for time.Now().Before(deadline) {
		second = jsonStatus(t, repo)
		if second.Running && second.PID != firstStatus.PID {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !second.Running {
		t.Fatalf("restart produced non-running daemon: %+v", second)
	}
	if second.PID == firstStatus.PID {
		t.Fatalf("restart did not replace the daemon (PID %d unchanged)", second.PID)
	}
}

func TestDaemonStatusStalePID(t *testing.T) {
	repo := shortTempDir(t)
	pidPath := filepath.Join(repo, ".krit", "daemon.pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("99999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runKrit(t, "daemon", "status", "--repo", repo, "--json")
	if code != 0 {
		t.Fatalf("status: exit=%d", code)
	}
	var st daemoncmd.DaemonStatus
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		t.Fatalf("decode status: %v\nbody=%s", err, stdout)
	}
	if st.Running {
		t.Fatalf("expected Running=false for stale pid, got %+v", st)
	}
	if st.StaleEntries == 0 {
		t.Fatalf("expected StaleEntries > 0, got %+v", st)
	}
}

func TestDaemonStopMissingPIDFile(t *testing.T) {
	repo := shortTempDir(t)
	stdout, _, code := runKrit(t, "daemon", "stop", "--repo", repo)
	if code != 0 {
		t.Fatalf("stop with no pid file: exit=%d", code)
	}
	if !strings.Contains(stdout, "not running") {
		t.Fatalf("expected 'not running', got %q", stdout)
	}
}

func TestDaemonStartCleansStalePIDFile(t *testing.T) {
	buildDaemonBinary(t)
	repo := shortTempDir(t)
	pidPath := filepath.Join(repo, ".krit", "daemon.pid")
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("99999999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _, _ = runKrit(t, "daemon", "stop", "--repo", repo) })

	_, stderr, code := runKrit(t, "daemon", "start", "--repo", repo, "--timeout", "10s")
	if code != 0 {
		t.Fatalf("start over stale pid: exit=%d stderr=%q", code, stderr)
	}
	st := jsonStatus(t, repo)
	if !st.Running {
		t.Fatalf("not running after start-over-stale: %+v", st)
	}
	if st.PID == 99999999 {
		t.Fatalf("daemon adopted stale pid?")
	}
}

func TestDaemonUsage(t *testing.T) {
	_, stderr, code := runKrit(t, "daemon")
	if code != 2 {
		t.Fatalf("bare `daemon` exit=%d, want 2", code)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Fatalf("expected usage, got %q", stderr)
	}
}

func jsonStatus(t *testing.T, repo string) daemoncmd.DaemonStatus {
	t.Helper()
	stdout, _, code := runKrit(t, "daemon", "status", "--repo", repo, "--json")
	if code != 0 {
		t.Fatalf("daemon status --json: exit=%d body=%s", code, stdout)
	}
	var st daemoncmd.DaemonStatus
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		t.Fatalf("decode status: %v\nbody=%s", err, stdout)
	}
	return st
}

// shortTempDir returns a temp directory under /tmp so the resulting
// "<dir>/.krit/daemon.sock" path fits within sun_path's ~104-byte
// limit on macOS. t.TempDir() roots under /var/folders/... which
// blows the limit and makes bind() fail with EINVAL.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "krit-d-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
