package firchecks

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/fslock"
)

func isolateRegistry(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	old := firClaimWait
	firClaimWait = 100 * time.Millisecond
	t.Cleanup(func() { firClaimWait = old })
}

func processAlive(pid int) bool { return firPIDAlive(pid) }

// The PID file is the discoverability marker and goes last, so a live PID
// always implies a complete record, and a failed publication is never
// discoverable.
func TestPIDFileIsWrittenLastAndPartialPublishIsNotDiscoverable(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 1)
	// Make the port write fail: a directory cannot be replaced by a file.
	must(t, os.MkdirAll(firPortPath("order"), 0o755))
	must(t, os.WriteFile(firPortPath("order")+"/x", nil, 0o644))
	if err := writeFirRegistryClaimed(pids[0], 1234, "order", firAOTMeta{PID: pids[0], Verified: true}); err == nil {
		t.Fatal("expected the port write to fail")
	}
	if pid, ok := registeredFirPID("order"); ok {
		t.Fatalf("PID %d discoverable although the record is incomplete", pid)
	}

	// A complete publication leaves metadata, port and PID all present.
	must(t, os.RemoveAll(firPortPath("order")))
	must(t, writeFirRegistryClaimed(pids[0], 1234, "order", firAOTMeta{PID: pids[0], Verified: true}))
	for _, p := range []string{firAOTPath("order"), firPortPath("order"), firPIDPath("order")} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("complete record missing %s: %v", p, err)
		}
	}
}

// A record with a live PID but no port, observed while a publisher holds the
// lock, is "being published": the reader must not kill the publisher.
func TestReaderSeeingPIDWithoutPortDoesNotKillPublisher(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 1)
	must(t, writeFileAtomic(firPIDPath("midpublish"), []byte(strconv.Itoa(pids[0])+"\n")))

	if _, err := connectExistingFirDaemon("midpublish", false); err == nil {
		t.Fatal("connected to a record with no port")
	}
	if !processAlive(pids[0]) {
		t.Fatal("reading an incomplete record killed the daemon")
	}

	holder, ok := fslock.Acquire(firLockPath("midpublish"), time.Second)
	if !ok {
		t.Fatal("could not simulate the publisher's lock")
	}
	defer holder()
	if stopFirDaemonIf("midpublish", pids[0], true) {
		t.Fatal("stop acted while the publisher held the lock")
	}
	if stopFirDaemon("midpublish", true) {
		t.Fatal("unconditional stop acted while the publisher held the lock")
	}
	if !processAlive(pids[0]) {
		t.Fatal("stop under contention killed the publisher's daemon")
	}
	if _, err := os.Stat(firPIDPath("midpublish")); err != nil {
		t.Fatalf("stop under contention deleted the record: %v", err)
	}
}

// Without contention the stop path still works, and it re-validates the PID it
// was told to stop.
func TestStopRevalidatesExpectedPIDUnderLock(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 2)
	pid, port := pids[0], 4000
	must(t, writeFirRegistry(pid, port, "reval", firAOTMeta{PID: pid, Verified: true}))

	// The record now names pids[0]; a stop aimed at pids[1] must leave it.
	if stopFirDaemonIf("reval", pids[1], false) {
		t.Fatal("stopped a record that no longer names the rejected PID")
	}
	if !processAlive(pid) {
		t.Fatal("daemon killed although the record changed")
	}
	if !stopFirDaemonIf("reval", pid, false) {
		t.Fatal("stop of the matching record did nothing")
	}
	deadline := time.Now().Add(3 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := registeredFirPID("reval"); ok {
		t.Fatal("record not removed after stop")
	}
}

// Close/Release cleanup deletes only the record it owns, and never while
// another process holds the lock.
func TestCleanupUnderContentionDoesNotDeleteAnotherPublishersRecord(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 2)
	owner, other := pids[0], pids[1]
	must(t, writeFirRegistry(other, 4001, "cleanup", firAOTMeta{PID: other, Verified: true}))

	// Record belongs to someone else: not ours to delete.
	removeFirRegistryIfOwned("cleanup", owner)
	if pid, ok := registeredFirPID("cleanup"); !ok || pid != other {
		t.Fatalf("cleanup deleted a record it does not own: %d %v", pid, ok)
	}

	// Record is ours but another process holds the lock: leave it.
	removeFirRegistryFiles("cleanup")
	must(t, writeFirRegistry(owner, 4002, "cleanup", firAOTMeta{PID: owner, Verified: true}))
	holder, ok := fslock.Acquire(firLockPath("cleanup"), time.Second)
	if !ok {
		t.Fatal("could not simulate the publisher's lock")
	}
	removeFirRegistryIfOwned("cleanup", owner)
	if _, ok := registeredFirPID("cleanup"); !ok {
		t.Fatal("cleanup deleted the record while the lock was held elsewhere")
	}
	holder()

	// Uncontended and owned: removed.
	removeFirRegistryIfOwned("cleanup", owner)
	if _, ok := registeredFirPID("cleanup"); ok {
		t.Fatal("owned record not removed")
	}
}

// Close on a daemon whose registry key has been taken over by another daemon
// must not delete the new record.
func TestCloseDoesNotDeleteTakenOverRecord(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 1)
	must(t, writeFirRegistry(pids[0], 4003, "takeover", firAOTMeta{PID: pids[0], Verified: true}))
	cmd := exec.Command("true")
	must(t, cmd.Start())
	d := &FirDaemon{cmd: cmd, sourcesHash: "takeover", port: 4004}
	_ = d.Close()
	if pid, ok := registeredFirPID("takeover"); !ok || pid != pids[0] {
		t.Fatalf("Close deleted another daemon's record: %d %v", pid, ok)
	}
}

// A live daemon already registered for the key is never replaced: the new
// daemon's publish is refused with errFirRegistryOwned (checked under the
// lock) and publishFirDaemon keeps it private.
func TestPublishDoesNotReplaceLiveRegisteredDaemon(t *testing.T) {
	isolateRegistry(t)
	pids := livePIDs(t, 2)
	incumbent, newcomer := pids[0], pids[1]
	must(t, writeFirRegistry(incumbent, 5000, "incumbent", firAOTMeta{PID: incumbent, Verified: true}))
	before, _ := os.ReadFile(firPortPath("incumbent"))

	if err := writeFirRegistry(newcomer, 5001, "incumbent", firAOTMeta{PID: newcomer, Verified: true}); !errors.Is(err, errFirRegistryOwned) {
		t.Fatalf("writeFirRegistry over a live daemon = %v, want errFirRegistryOwned", err)
	}
	d := &FirDaemon{port: 5001}
	if err := publishFirDaemon(d, newcomer, "incumbent", true); err != nil || !d.unpublished {
		t.Fatalf("publish over a live daemon: err=%v unpublished=%v, want private", err, d.unpublished)
	}
	if pid, ok := registeredFirPID("incumbent"); !ok || pid != incumbent {
		t.Fatalf("registry now names %d, want incumbent %d", pid, incumbent)
	}
	if after, _ := os.ReadFile(firPortPath("incumbent")); string(after) != string(before) {
		t.Fatalf("incumbent port record changed: %q -> %q", before, after)
	}

	// A dead incumbent is replaceable, and the same PID may re-publish.
	must(t, writeFirRegistry(incumbent, 5000, "incumbent", firAOTMeta{PID: incumbent, Verified: true}))
	dead := exec.Command("true")
	must(t, dead.Run())
	must(t, writeFirRegistryClaimed(dead.Process.Pid, 5002, "gone", firAOTMeta{PID: dead.Process.Pid, Verified: true}))
	must(t, writeFirRegistry(newcomer, 5003, "gone", firAOTMeta{PID: newcomer, Verified: true}))
	if pid, _ := registeredFirPID("gone"); pid != newcomer {
		t.Fatalf("dead incumbent not replaced: %d", pid)
	}
}
