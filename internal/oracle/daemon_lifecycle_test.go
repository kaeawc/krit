package oracle

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Bug A — Shutdown grace-period: the wait goroutine must always return.
// ---------------------------------------------------------------------------

// startSleepCmd starts a child process that will not exit on its own
// within the test's lifetime. Used to simulate a stuck JVM so we can
// exercise the Shutdown grace-period → Kill → Wait path. The "sleep"
// binary is available on every supported developer platform; if not
// found the test skips rather than fails.
func startSleepCmd(t *testing.T) *exec.Cmd {
	t.Helper()
	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	cmd := exec.Command(sleepBin, "300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	return cmd
}

// pipeStdio returns an in-memory pipe pair to use as stdin/stdout
// stand-ins on a Daemon. A background helper reads each request line
// off stdin and writes a canned `{"id": N, "result": {}}` response to
// stdout, so sendOnce sees a successful round-trip (does NOT time out
// and so does NOT pre-kill the test process). This lets Shutdown's
// grace-period path own the Kill decision.
func pipeStdio(t *testing.T) (io.WriteCloser, *bufio.Scanner) {
	t.Helper()
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	go func() {
		sc := bufio.NewScanner(stdinR)
		sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
		for sc.Scan() {
			var req daemonRequest
			if err := json.Unmarshal([]byte(sc.Text()), &req); err != nil {
				continue
			}
			resp := []byte(`{"id":` + strconv.Itoa(req.ID) + `,"result":{}}` + "\n")
			if _, err := stdoutW.Write(resp); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdoutW.Close()
	})
	sc := bufio.NewScanner(stdoutR)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	return stdinW, sc
}

// withShortShutdownGrace shortens the grace period so the test exercises
// the Kill→Wait join in well under a second.
func withShortShutdownGrace(t *testing.T, d time.Duration) {
	t.Helper()
	orig := shutdownGracePeriod
	shutdownGracePeriod = d
	t.Cleanup(func() { shutdownGracePeriod = orig })
}

// withShortRequestTimeout shortens daemonRequestTimeout so the in-Shutdown
// sendOnce call returns quickly when the fake "JVM" never responds.
func withShortRequestTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", d.String())
}

// TestShutdown_KillsStuckProcess_AndJoinsWaitGoroutine verifies fix A:
// when the JVM doesn't exit on its own, Shutdown issues Kill() and waits
// for the wait goroutine to return. We assert (a) Shutdown returns
// promptly and (b) no goroutine leak compared to the pre-Shutdown baseline.
func TestShutdown_KillsStuckProcess_AndJoinsWaitGoroutine(t *testing.T) {
	withShortShutdownGrace(t, 200*time.Millisecond)
	withShortRequestTimeout(t, 100*time.Millisecond)

	stdinW, stdoutSc := pipeStdio(t)
	cmd := startSleepCmd(t)
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	d := &Daemon{
		cmd:     cmd,
		stdin:   stdinW,
		stdout:  stdoutSc,
		nextID:  1,
		started: true,
	}

	// Let the runtime settle so the goroutine baseline is stable.
	runtime.Gosched()
	baseline := runtime.NumGoroutine()

	start := time.Now()
	err := d.Shutdown()
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "did not exit within timeout") {
		t.Fatalf("expected timeout-killed error, got %v", err)
	}
	// Grace 200ms + request timeout 100ms + slack. Anything past 2s
	// means we leaked the wait goroutine and timed out somewhere else.
	if elapsed > 2*time.Second {
		t.Errorf("Shutdown took %s; expected <2s", elapsed)
	}

	// Confirm the sleep process was actually reaped (Kill→Wait join).
	if cmd.ProcessState == nil {
		t.Errorf("cmd.ProcessState is nil; Wait() never observed exit")
	}

	// Wait a moment for any unrelated goroutines to settle, then check
	// the leak. We allow a small +/- slack because Go's runtime keeps a
	// few worker goroutines hot.
	time.Sleep(50 * time.Millisecond)
	runtime.Gosched()
	after := runtime.NumGoroutine()
	if after > baseline+1 {
		// Print stack on failure for debuggability.
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		t.Errorf("goroutine leak: before=%d after=%d delta=%d\n%s",
			baseline, after, after-baseline, buf[:n])
	}
}

// TestShutdown_NotStarted_NoOp confirms the early-return path still
// works after the started-flag claim refactor.
func TestShutdown_NotStarted_NoOp(t *testing.T) {
	d := &Daemon{started: false}
	if err := d.Shutdown(); err != nil {
		t.Errorf("Shutdown(not started) = %v; want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Bug B — Close + Shutdown concurrent calls must be race-free and idempotent.
// ---------------------------------------------------------------------------

// TestCloseShutdownConcurrent_NoRaceAndIdempotent calls Close and
// Shutdown from N goroutines concurrently and asserts no races
// (caught only under `go test -race`) and that the underlying process
// is killed exactly once with no double-Wait panic.
func TestCloseShutdownConcurrent_NoRaceAndIdempotent(t *testing.T) {
	withShortShutdownGrace(t, 200*time.Millisecond)
	withShortRequestTimeout(t, 100*time.Millisecond)

	stdinW, stdoutSc := pipeStdio(t)
	cmd := startSleepCmd(t)
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	d := &Daemon{
		cmd:     cmd,
		stdin:   stdinW,
		stdout:  stdoutSc,
		nextID:  1,
		started: true,
	}

	const N = 8
	var wg sync.WaitGroup
	wg.Add(N * 2)
	var closeOK, shutdownOK atomic.Int32
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if err := d.Close(); err == nil {
				closeOK.Add(1)
			}
		}()
		go func() {
			defer wg.Done()
			_ = d.Shutdown()
			shutdownOK.Add(1)
		}()
	}
	wg.Wait()

	// Confirm the started flag is false and the process is reaped.
	d.mu.Lock()
	if d.started {
		t.Errorf("expected started=false after concurrent shutdown/close")
	}
	d.mu.Unlock()
	if cmd.ProcessState == nil {
		t.Errorf("cmd.ProcessState nil; the winning caller did not Wait()")
	}
	// Every Shutdown must have returned (closeOK/shutdownOK counts are
	// only a sanity gate; the real assertion is "no -race report").
	if shutdownOK.Load() != N {
		t.Errorf("Shutdown called %d times; %d returned", N, shutdownOK.Load())
	}
}

// TestClose_ReadsAllFieldsUnderLock is a focused regression: Close used
// to read d.cmd / d.conn / d.started without holding d.mu. The fix
// snapshots those fields under the lock; this test uses -race to
// detect any remaining unsynchronised access.
func TestClose_ReadsAllFieldsUnderLock(t *testing.T) {
	withShortShutdownGrace(t, 50*time.Millisecond)
	withShortRequestTimeout(t, 30*time.Millisecond)

	stdinW, stdoutSc := pipeStdio(t)
	cmd := startSleepCmd(t)
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	d := &Daemon{
		cmd:     cmd,
		stdin:   stdinW,
		stdout:  stdoutSc,
		nextID:  1,
		started: true,
	}

	// Concurrently mutate d.started under the lock while Close runs;
	// without the locked snapshot in Close this would race.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				d.mu.Lock()
				_ = d.started
				d.mu.Unlock()
			}
		}
	}()

	_ = d.Close()
	close(stop)
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Bug C — writePIDFileSlot must clean up the .pid on .port write failure.
// ---------------------------------------------------------------------------

// TestWritePIDFileSlot_RollsBackPIDOnPortFailure proves that when the
// .port write fails (we force this by making the daemons dir
// read-only after the .pid write would have happened), the .pid file
// is removed so a stale entry is never left behind.
func TestWritePIDFileSlot_RollsBackPIDOnPortFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod 0500 won't block writes")
	}
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcHash := hashSources([]string{"/fake/repo/rollback"})
	daemonDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		t.Fatalf("mkdir daemons: %v", err)
	}

	// Strategy: pre-create the .port file as a directory so os.WriteFile
	// can't replace it. The .pid write still succeeds (a sibling new
	// file). We then assert .pid does not survive after the rollback.
	portPath := daemonPortPathForSlot(srcHash, 0)
	if err := os.Mkdir(portPath, 0755); err != nil {
		t.Fatalf("seed bogus .port directory: %v", err)
	}

	err := writePIDFileSlot(1234, 5678, srcHash, 0)
	if err == nil {
		t.Fatal("expected writePIDFileSlot to fail when .port path is a directory")
	}
	if !strings.Contains(err.Error(), "write port file") {
		t.Fatalf("expected 'write port file' error, got %v", err)
	}

	pidPath := daemonPIDPathForSlot(srcHash, 0)
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Errorf("expected .pid to be rolled back; stat err = %v", statErr)
	}
}

// TestWritePIDFileSlot_NoOrphanAfterRollback exercises the same fix
// via the readPIDFileSlot consumer: after a failed write, readPIDFile
// must return an error (no orphan) so the connect-existing-daemon
// flow doesn't see a stale PID pointing at a process we never
// successfully wired up.
func TestWritePIDFileSlot_NoOrphanAfterRollback(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod 0500 won't block writes")
	}
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcHash := hashSources([]string{"/fake/repo/orphan"})
	daemonDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	if err := os.MkdirAll(daemonDir, 0755); err != nil {
		t.Fatalf("mkdir daemons: %v", err)
	}
	// Make the .port path a directory so the .port write fails.
	if err := os.Mkdir(daemonPortPathForSlot(srcHash, 0), 0755); err != nil {
		t.Fatalf("seed bogus .port directory: %v", err)
	}

	_ = writePIDFileSlot(99, 100, srcHash, 0)

	if _, err := readPIDFileSlot(srcHash, 0); err == nil {
		t.Errorf("expected readPIDFileSlot to fail after rollback")
	}
}

// TestStartupCachePathHelperUnchanged ensures the daemon PID path
// constants didn't drift; the rollback fix touches the writer but
// must not change the on-disk shape that older daemons rely on.
func TestStartupCachePathHelperUnchanged(t *testing.T) {
	got := daemonPIDFileName("deadbeef00000000", 0)
	if got != "deadbeef00000000.pid" {
		t.Errorf("legacy slot 0 pid name = %q; want deadbeef00000000.pid", got)
	}
	got = daemonPIDFileName("deadbeef00000000", 3)
	if got != "deadbeef00000000.3.pid" {
		t.Errorf("slot 3 pid name = %q; want deadbeef00000000.3.pid", got)
	}
}

// TestWritePIDFileSlot_HappyPathStillWorks regression-tests that the
// rollback addition didn't accidentally break the success path.
func TestWritePIDFileSlot_HappyPathStillWorks(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcHash := hashSources([]string{"/fake/repo/happy"})
	if err := writePIDFileSlot(4242, 9999, srcHash, 0); err != nil {
		t.Fatalf("writePIDFileSlot: %v", err)
	}
	t.Cleanup(func() { removePIDFileSlot(srcHash, 0) })

	info, err := readPIDFileSlot(srcHash, 0)
	if err != nil {
		t.Fatalf("readPIDFileSlot: %v", err)
	}
	if info.PID != 4242 {
		t.Errorf("PID = %d; want 4242", info.PID)
	}
	if info.Port != 9999 {
		t.Errorf("Port = %d; want 9999", info.Port)
	}
}
