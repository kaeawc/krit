package oracle

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// isolateDaemonRegistry points the registry at a private home.
func isolateDaemonRegistry(t *testing.T) {
	t.Helper()
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", "")
	t.Setenv("HOME", t.TempDir())
}

// noJava makes a fresh daemon start fail fast ("java not found") so
// ConnectOrStartDaemon never launches a JVM. Start any helper process first.
func noJava(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

// A reader that finds a live PID before the port file exists is looking at a
// publication in flight (or a torn record). It must not conclude the daemon
// is stuck and kill it.
func TestConnectOrStartDaemon_PIDWithoutPortDoesNotKill(t *testing.T) {
	isolateDaemonRegistry(t)
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	noJava(t)
	key := daemonRegistryKey(jar, sources)
	pidPath := daemonPIDPathForSlot(key, 0)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil {
		_ = d.Release()
	}
	if err == nil {
		t.Fatal("expected the fresh start to fail (no java on PATH)")
	}
	if !isProcessAlive(pid) {
		t.Fatalf("PID %d was killed on the strength of a PID file without a port", pid)
	}
}

// Two publishers for one key must not interleave their PID and port writes:
// whichever finishes last owns both files.
func TestWritePIDFileSlot_ConcurrentPublishersStayConsistent(t *testing.T) {
	isolateDaemonRegistry(t)
	key := hashSources([]string{"/race/repo"})

	for round := 0; round < 25; round++ {
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 8; i++ {
			pid := 4_000_000 + i
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				// Errors are fine: a loser may be refused. What must hold is
				// that the record left behind is one publisher's own.
				_ = writePIDFileSlot(pid, pid+1, key, 0)
			}()
		}
		close(start)
		wg.Wait()
		info, err := readPIDFileSlot(key, 0)
		if err != nil {
			t.Fatalf("round %d: record unreadable after publishing: %v", round, err)
		}
		if info.Port != info.PID+1 {
			t.Fatalf("round %d: interleaved record: pid %d with port %d", round, info.PID, info.Port)
		}
		removePIDFileSlot(key, 0)
	}
}

// stopDaemonSlotIf re-validates the PID it decided against once it holds the
// lock: a record that now names a different daemon is left alone.
func TestStopDaemonSlotIf_ExpectedPIDNoLongerMatchesDoesNothing(t *testing.T) {
	isolateDaemonRegistry(t)
	fake := NewFakeDaemon(t)
	t.Cleanup(fake.Close)
	pid := testLivePID(t)
	key := hashSources([]string{"/revalidate/repo"})
	if err := writePIDFileSlot(pid, fake.Port, key, 0); err != nil {
		t.Fatal(err)
	}

	if stopDaemonSlotIf(key, 0, pid+1, false) {
		t.Fatal("stop acted although the record no longer names the expected PID")
	}
	if !isProcessAlive(pid) {
		t.Fatal("the daemon named by the record was killed for a stale expectation")
	}
	if info, err := readPIDFileSlot(key, 0); err != nil || info.PID != pid || info.Port != fake.Port {
		t.Fatalf("record was altered: info=%+v err=%v", info, err)
	}
}

// removeDaemonRegistryIfOwned only deletes a record that still names the
// caller, never the record of the daemon that replaced it.
func TestRemoveDaemonRegistryIfOwned_LeavesSuccessorRecord(t *testing.T) {
	isolateDaemonRegistry(t)
	key := hashSources([]string{"/successor/repo"})
	if err := writePIDFileSlot(4_100_001, 41, key, 0); err != nil {
		t.Fatal(err)
	}

	removeDaemonRegistryIfOwned(key, 0, 4_100_002)
	if _, err := readPIDFileSlot(key, 0); err != nil {
		t.Fatalf("successor record deleted by a different PID: %v", err)
	}
	removeDaemonRegistryIfOwned(key, 0, 4_100_001)
	if _, err := readPIDFileSlot(key, 0); err == nil {
		t.Fatal("owner could not remove its own record")
	}
}

// While a publisher holds the per-key lock, a concurrent cold start must
// neither kill nor delete the record it sees: the publication is in flight.
func TestStopDaemonSlotIf_LockedRegistryIsLeftAlone(t *testing.T) {
	isolateDaemonRegistry(t)
	fake := NewFakeDaemon(t)
	t.Cleanup(fake.Close)
	pid := testLivePID(t)
	key := hashSources([]string{"/locked/repo"})
	if err := writePIDFileSlot(pid, fake.Port, key, 0); err != nil {
		t.Fatal(err)
	}
	prev := daemonClaimWait
	daemonClaimWait = 50 * time.Millisecond
	t.Cleanup(func() { daemonClaimWait = prev })

	release, ok := claimDaemonRegistry(key, 0)
	if !ok {
		t.Fatal("could not take the registry lock")
	}
	acted := stopDaemonSlotIf(key, 0, pid, false)
	removePIDFileSlot(key, 0)
	removeDaemonRegistryIfOwned(key, 0, pid)
	release()

	if acted {
		t.Fatal("stop acted while another process held the lock")
	}
	if !isProcessAlive(pid) {
		t.Fatal("daemon killed while its record was locked")
	}
	if _, err := readPIDFileSlot(key, 0); err != nil {
		t.Fatalf("record deleted while locked: %v", err)
	}
}

// A second publisher for a key a live daemon already serves must not displace
// it.
func TestWritePIDFileSlot_LiveOwnerIsNotDisplaced(t *testing.T) {
	isolateDaemonRegistry(t)
	fake := NewFakeDaemon(t)
	t.Cleanup(fake.Close)
	key := hashSources([]string{"/owned/repo"})
	if err := writePIDFileSlot(os.Getpid(), fake.Port, key, 0); err != nil {
		t.Fatal(err)
	}

	err := writePIDFileSlot(4_100_003, 43, key, 0)
	if !errors.Is(err, errDaemonRegistryOwned) {
		t.Fatalf("second publisher err = %v, want errDaemonRegistryOwned", err)
	}
	if info, err := readPIDFileSlot(key, 0); err != nil || info.PID != os.Getpid() || info.Port != fake.Port {
		t.Fatalf("owner's record changed: info=%+v err=%v", info, err)
	}
}

// Publishing leaves no temp files behind in the registry directory, which
// retirement globs by suffix.
func TestWritePIDFileSlot_LeavesNoTempFiles(t *testing.T) {
	isolateDaemonRegistry(t)
	key := hashSources([]string{"/tmpfiles/repo"})
	if err := writePIDFileSlot(4_100_004, 44, key, 0); err != nil {
		t.Fatal(err)
	}
	dir, err := daemonsDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}
