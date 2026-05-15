package daemoncmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestPIDFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := pidFilePath(dir)

	if _, err := readPIDFile(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist for missing pid file, got %v", err)
	}

	if err := writePIDFile(path, 4242); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}
	got, err := readPIDFile(path)
	if err != nil {
		t.Fatalf("readPIDFile: %v", err)
	}
	if got != 4242 {
		t.Fatalf("readPIDFile = %d, want 4242", got)
	}

	if err := removePIDFile(path); err != nil {
		t.Fatalf("removePIDFile: %v", err)
	}
	// Idempotent.
	if err := removePIDFile(path); err != nil {
		t.Fatalf("removePIDFile (missing): %v", err)
	}
}

func TestReadPIDFileRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	path := pidFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-a-number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPIDFile(path); err == nil {
		t.Fatalf("expected error for garbage pid file")
	}
}

func TestProcessAlive(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatalf("processAlive(self) = false, want true")
	}
	// PID 1 is init on Linux/macOS and is essentially always running,
	// but signal-0 may return EPERM for an unprivileged caller. Our
	// processAlive treats EPERM as alive, so this is a stable assertion.
	if !processAlive(1) {
		t.Fatalf("processAlive(1) = false, want true (EPERM is alive)")
	}
	// A throwaway PID we can be confident isn't ours.
	if processAlive(99999999) {
		t.Fatalf("processAlive(99999999) = true, want false")
	}
}

func TestCollectStatusNoDaemon(t *testing.T) {
	dir := t.TempDir()
	st := collectStatus(dir)
	if st.Running {
		t.Fatalf("expected Running=false, got %+v", st)
	}
	if st.StaleEntries != 0 {
		t.Fatalf("expected no stale entries on clean dir, got %d", st.StaleEntries)
	}
}

func TestCollectStatusStalePID(t *testing.T) {
	dir := t.TempDir()
	// Spawn a short-lived process, capture its pid, wait for it to exit,
	// then write the dead pid into the file.
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("seed process: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := writePIDFile(pidFilePath(dir), deadPID); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}
	st := collectStatus(dir)
	if st.Running {
		t.Fatalf("expected Running=false for dead pid, got %+v", st)
	}
	if st.StaleEntries == 0 {
		t.Fatalf("expected at least one stale entry, got %+v", st)
	}
}

func TestRunStartRejectsBadRepo(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	code := runStart([]string{"--repo", missing})
	if code != 2 {
		t.Fatalf("expected exit 2 for missing repo, got %d", code)
	}
}

func TestRunStopNoDaemon(t *testing.T) {
	dir := t.TempDir()
	code := runStop([]string{"--repo", dir})
	if code != 0 {
		t.Fatalf("stop on empty repo: want 0, got %d", code)
	}
}

func TestRunStopRemovesStalePID(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dead := cmd.Process.Pid
	if err := writePIDFile(pidFilePath(dir), dead); err != nil {
		t.Fatal(err)
	}
	code := runStop([]string{"--repo", dir})
	if code != 0 {
		t.Fatalf("stop on stale pid: want 0, got %d", code)
	}
	if _, err := os.Stat(pidFilePath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected pid file to be removed, got %v", err)
	}
}

func TestRunStopForcesAfterGrace(t *testing.T) {
	dir := t.TempDir()
	// Spawn a process that ignores SIGTERM so we have to fall through
	// to SIGKILL. `sh -c 'trap "" TERM; sleep 30'` traps and discards
	// SIGTERM.
	cmd := exec.Command("/bin/sh", "-c", "trap '' TERM; sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start victim: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	pid := cmd.Process.Pid
	if err := writePIDFile(pidFilePath(dir), pid); err != nil {
		t.Fatal(err)
	}
	orig := stopGrace
	stopGrace = 250 * time.Millisecond
	t.Cleanup(func() { stopGrace = orig })

	code := runStop([]string{"--repo", dir, "--timeout", "250ms"})
	if code != ExitForceKill {
		t.Fatalf("expected exit %d (force kill), got %d", ExitForceKill, code)
	}
}

func TestResolveDaemonBinaryNotFound(t *testing.T) {
	// Point --binary at a path that definitely doesn't exist.
	missing := filepath.Join(t.TempDir(), "krit-daemon-nope")
	if _, err := resolveDaemonBinary(missing); err == nil {
		t.Fatalf("expected error for missing --binary, got nil")
	}
}

func writePIDForTest(t *testing.T, dir string, pid int) {
	t.Helper()
	if err := writePIDFile(pidFilePath(dir), pid); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}
}

func TestCollectStatusLivePIDNoSocket(t *testing.T) {
	dir := t.TempDir()
	writePIDForTest(t, dir, os.Getpid())
	st := collectStatus(dir)
	if !st.Running {
		t.Fatalf("expected Running=true for live pid w/o socket, got %+v", st)
	}
	if st.PID != os.Getpid() {
		t.Fatalf("PID = %d, want %d", st.PID, os.Getpid())
	}
}

func TestShortHash(t *testing.T) {
	if got, want := shortHash("abcdef"), "abcdef"; got != want {
		t.Fatalf("short hash, want %q, got %q", want, got)
	}
	if got := shortHash("0123456789abcdef0123"); got != "0123456789ab" {
		t.Fatalf("12-char prefix, got %q", got)
	}
}

func TestRunRestartParsesFlags(t *testing.T) {
	// Restart with --repo pointing at an empty dir should succeed
	// when the binary is unreachable — we only need stop to be
	// idempotent and start to fail loudly. The flag parser failing
	// here would produce a usage error (2); we want to confirm the
	// composite flagset accepts every flag start/stop care about.
	dir := t.TempDir()
	code := runRestart([]string{
		"--repo", dir,
		"--socket", filepath.Join(dir, "x.sock"),
		"--binary", filepath.Join(dir, "no-binary"),
	})
	// Stop succeeds (no daemon). Start should fail because the
	// binary doesn't exist. Exit 1 is the documented "user-visible
	// failure" exit code.
	if code != 1 {
		t.Fatalf("restart with missing binary: want exit 1, got %d", code)
	}
}

func TestPIDFileLineEnding(t *testing.T) {
	// Some operators hand-edit pid files; tolerate trailing newlines
	// and surrounding whitespace.
	dir := t.TempDir()
	path := pidFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("  1234  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readPIDFile(path)
	if err != nil {
		t.Fatalf("readPIDFile: %v", err)
	}
	if got != 1234 {
		t.Fatalf("readPIDFile = %d, want 1234", got)
	}
	// Sanity-check Atoi parity since readPIDFile relies on it.
	if _, err := strconv.Atoi("1234"); err != nil {
		t.Fatalf("strconv parity: %v", err)
	}
}
