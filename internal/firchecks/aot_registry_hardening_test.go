package firchecks

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/fslock"
)

// The lock is held for a stop's SIGTERM and SIGKILL waits, so the claim wait
// must exceed them or everything queued behind a stop would give up.
func TestClaimWaitExceedsStopWorstCase(t *testing.T) {
	if firClaimWait <= firStopTermWait+firStopKillWait {
		t.Fatalf("firClaimWait %v must exceed the stop's %v + %v", firClaimWait, firStopTermWait, firStopKillWait)
	}
}

func spawnReaped(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
	return cmd
}

// A publisher queued behind a slow stop waits for it and then publishes.
func TestPublishQueuedBehindStopSucceeds(t *testing.T) {
	isolateRegistry(t)
	// The stop is sped up; the claim wait stays above its (shrunk) worst case,
	// as TestClaimWaitExceedsStopWorstCase asserts for production values.
	firClaimWait = 3 * time.Second
	oldTerm, oldKill := firStopTermWait, firStopKillWait
	firStopTermWait, firStopKillWait = 600*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() { firStopTermWait, firStopKillWait = oldTerm, oldKill })

	// A daemon that ignores SIGTERM makes the stop run its full wait.
	stubborn := spawnReaped(t, "sh", "-c", `trap "" TERM; while :; do sleep 1; done`)
	time.Sleep(100 * time.Millisecond) // let the trap install
	_, port := pingListener(t)
	must(t, writeFirRegistry(stubborn.Process.Pid, port, "queued", firAOTMeta{PID: stubborn.Process.Pid, Verified: true}))

	stopped := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		stopFirDaemon("queued", false)
		stopped <- time.Since(start)
	}()
	// Wait until the stop holds the lock.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if release, ok := fslock.Acquire(firLockPath("queued"), time.Millisecond); ok {
			release()
			time.Sleep(2 * time.Millisecond)
			continue
		}
		break
	}

	newcomer := livePIDs(t, 1)[0]
	if err := writeFirRegistry(newcomer, 6000, "queued", firAOTMeta{PID: newcomer, Verified: true}); err != nil {
		t.Fatalf("publish queued behind a stop failed: %v", err)
	}
	if took := <-stopped; took < 500*time.Millisecond {
		t.Fatalf("stop finished in %v; the publisher was not actually queued behind it", took)
	}
	if pid, ok := registeredFirPID("queued"); !ok || pid != newcomer {
		t.Fatalf("registry names %d, want the queued publisher %d", pid, newcomer)
	}
}

// A killed-and-reaped child, or an unreaped zombie, is not a live owner.
func TestReapedChildIsNotAnOwner(t *testing.T) {
	isolateRegistry(t)
	_, port := pingListener(t) // a port that would accept
	child := exec.Command("sleep", "60")
	must(t, child.Start())
	pid := child.Process.Pid
	must(t, writeFirRegistry(pid, port, "reaped", firAOTMeta{PID: pid, Verified: true}))

	must(t, child.Process.Kill())
	// Unreaped: still answers signal 0, but must not count as alive.
	if _, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		deadline := time.Now().Add(2 * time.Second)
		for firPIDAlive(pid) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if firPIDAlive(pid) {
			t.Fatal("zombie counted as alive")
		}
	}
	_ = child.Wait()
	if firPIDAlive(pid) {
		t.Fatal("reaped child counted as alive")
	}
	newcomer := livePIDs(t, 1)[0]
	if err := writeFirRegistry(newcomer, 6001, "reaped", firAOTMeta{PID: newcomer, Verified: true}); err != nil {
		t.Fatalf("publish over a reaped owner failed: %v", err)
	}
}

// Every kill of a daemon this program launched is followed by a reap.
func TestKillAndReapLeavesNoZombie(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	must(t, cmd.Start())
	pid := cmd.Process.Pid
	killAndReap(cmd)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if p, err := os.FindProcess(pid); err == nil && p.Signal(syscall.Signal(0)) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("killed process was never reaped")
}

// A request timeout kills the daemon; it must be reaped too, so a long-lived
// parent does not keep a zombie that looks like a live registered owner.
func TestRequestTimeoutKillReapsTheDaemon(t *testing.T) {
	isolateRegistry(t)
	t.Setenv("KRIT_FIR_REQUEST_TIMEOUT", "150ms")
	cmd := exec.Command("sleep", "60")
	must(t, cmd.Start())
	pid := cmd.Process.Pid
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	go func() { // reads requests, never answers
		sc := bufio.NewScanner(server)
		for sc.Scan() {
		}
	}()
	d := &FirDaemon{cmd: cmd, conn: client, reader: bufio.NewScanner(client), nextID: 1, started: true}
	if _, err := d.Check([]fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}); err == nil {
		t.Fatal("expected a timeout")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !firPIDAlive(pid) {
			if p, _ := os.FindProcess(pid); p.Signal(syscall.Signal(0)) != nil {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon killed on timeout was not reaped")
}

// A live PID is not proof of a daemon: if the registered port does not accept
// (or is missing), the record is replaceable.
func TestLiveNonDaemonPIDWithDeadPortIsReplaceable(t *testing.T) {
	isolateRegistry(t)
	stranger := livePIDs(t, 1)[0] // e.g. a PID reused by an unrelated process
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}
	deadPort := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	must(t, writeFirRegistryClaimed(stranger, deadPort, "reused", firAOTMeta{PID: stranger, Verified: true}))

	newcomer := livePIDs(t, 1)[0]
	if err := writeFirRegistry(newcomer, 6002, "reused", firAOTMeta{PID: newcomer, Verified: true}); err != nil {
		t.Fatalf("a live PID with a dead port blocked publishing: %v", err)
	}
	if pid, _ := registeredFirPID("reused"); pid != newcomer {
		t.Fatalf("registry names %d, want %d", pid, newcomer)
	}

	// Port file missing entirely: also replaceable.
	must(t, writeFirRegistryClaimed(stranger, deadPort, "noport", firAOTMeta{PID: stranger, Verified: true}))
	must(t, os.Remove(firPortPath("noport")))
	if err := writeFirRegistry(newcomer, 6003, "noport", firAOTMeta{PID: newcomer, Verified: true}); err != nil {
		t.Fatalf("a live PID without a port file blocked publishing: %v", err)
	}
}
