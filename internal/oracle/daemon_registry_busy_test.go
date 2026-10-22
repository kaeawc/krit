package oracle

import (
	"os"
	"testing"
	"time"
)

// recordingStarter replaces the daemon launcher with one that hands back a
// private stub daemon and counts the launches, so no JVM is started.
func recordingStarter(t *testing.T) *[]int {
	t.Helper()
	var slots []int
	prev := startDaemonWithPortSlot
	startDaemonWithPortSlot = func(_ string, _ []string, _ []string, _ bool, slot int, _ ...DaemonOption) (*Daemon, error) {
		slots = append(slots, slot)
		return &Daemon{started: true, slot: slot, unpublished: true}, nil
	}
	t.Cleanup(func() { startDaemonWithPortSlot = prev })
	return &slots
}

// A busy proven daemon stays registered and running; this run gets a private
// daemon instead.
func TestConnectOrStartDaemon_BusyDaemonGetsPrivateDaemon(t *testing.T) {
	isolateDaemonRegistry(t)
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", "300ms")
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	key := daemonRegistryKey(jar, sources)
	port := silentDaemonPort(t)
	if err := writePIDFileSlot(pid, port, key, 0); err != nil {
		t.Fatal(err)
	}
	starts := recordingStarter(t)

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if err != nil {
		t.Fatalf("ConnectOrStartDaemon: %v", err)
	}
	defer d.Release()

	if len(*starts) != 1 || !d.unpublished {
		t.Fatalf("expected one private daemon, starts=%v unpublished=%v", *starts, d.unpublished)
	}
	if !isProcessAlive(pid) {
		t.Fatal("busy daemon was killed")
	}
	if info, rerr := readPIDFileSlot(key, 0); rerr != nil || info.PID != pid || info.Port != port {
		t.Fatalf("busy daemon's record changed: info=%+v err=%v", info, rerr)
	}
}

// In a pool, only the busy slot is replaced by a private daemon; the busy
// daemon keeps its record and the other slots connect as usual.
func TestConnectOrStartDaemonPool_BusySlotGetsPrivateDaemon(t *testing.T) {
	isolateDaemonRegistry(t)
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", "300ms")
	const poolJar = "unused.jar"
	key := daemonRegistryKey(poolJar, testSourceDirs)
	fake := NewFakeDaemon(t)
	t.Cleanup(fake.Close)
	busyPID := testLivePID(t)
	busyPort := silentDaemonPort(t)
	if err := writePIDFileSlot(os.Getpid(), fake.Port, key, 0); err != nil {
		t.Fatal(err)
	}
	if err := writePIDFileSlot(busyPID, busyPort, key, 1); err != nil {
		t.Fatal(err)
	}
	starts := recordingStarter(t)

	pool, err := ConnectOrStartDaemonPool(poolJar, testSourceDirs, nil, 2, false)
	if err != nil {
		t.Fatalf("ConnectOrStartDaemonPool: %v", err)
	}
	defer pool.Release()

	if pool.Connected != 1 || pool.Started != 1 || len(*starts) != 1 || (*starts)[0] != 1 {
		t.Fatalf("pool = %+v starts=%v, want slot 0 reused and slot 1 started privately", pool, *starts)
	}
	if !isProcessAlive(busyPID) {
		t.Fatal("busy slot's daemon was killed")
	}
	if info, rerr := readPIDFileSlot(key, 1); rerr != nil || info.PID != busyPID || info.Port != busyPort {
		t.Fatalf("busy slot's record changed: info=%+v err=%v", info, rerr)
	}
}

// A record whose recorded start time no longer matches the live process names
// a reused PID: it is dropped, and the process is left alone.
func TestConnectOrStartDaemon_MismatchedStartTimeIsNotSignalled(t *testing.T) {
	isolateDaemonRegistry(t)
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	noJava(t)
	key := daemonRegistryKey(jar, sources)
	if err := writePIDFileSlot(pid, closedPort(t), key, 0); err != nil {
		t.Fatal(err)
	}
	meta, ok := readDaemonMeta(key, 0)
	if !ok || meta.StartTime == "" {
		t.Skipf("start time unreadable on this platform: %+v", meta)
	}
	meta.StartTime += "-earlier-incarnation"
	prev := daemonProcessStart
	daemonProcessStart = func(int) string { return meta.StartTime }
	if err := writeDaemonMeta(key, 0, pid); err != nil {
		t.Fatal(err)
	}
	daemonProcessStart = prev

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil {
		_ = d.Release()
	}
	if err == nil {
		t.Fatal("expected the fresh start to fail (no java on PATH)")
	}
	if !isProcessAlive(pid) {
		t.Fatalf("PID %d with a different start time was signalled", pid)
	}
	if recordExists(key, 0) {
		t.Fatal("the mismatched record should have been dropped")
	}
}

// A platform that cannot read start times can prove nothing, so it never
// signals.
func TestStopDaemonSlot_UnreadableStartTimeNeverSignals(t *testing.T) {
	isolateDaemonRegistry(t)
	pid := testLivePID(t)
	key := hashSources([]string{"/no-start-time/repo"})
	prev := daemonProcessStart
	daemonProcessStart = func(int) string { return "" }
	t.Cleanup(func() { daemonProcessStart = prev })
	if err := writePIDFileSlot(pid, closedPort(t), key, 0); err != nil {
		t.Fatal(err)
	}

	stopDaemonSlot(key, 0, false)

	if !isProcessAlive(pid) {
		t.Fatal("signalled a PID whose identity could not be proven")
	}
	if recordExists(key, 0) {
		t.Fatal("unproven record should have been dropped")
	}
}

// A PID proven to be the registered daemon, which no longer answers on its
// port, is stuck and is stopped.
func TestStopDaemonSlot_ProvenStuckDaemonIsStopped(t *testing.T) {
	isolateDaemonRegistry(t)
	pid := testLivePID(t)
	key := hashSources([]string{"/proven/repo"})
	if err := writePIDFileSlot(pid, closedPort(t), key, 0); err != nil {
		t.Fatal(err)
	}
	if meta, ok := readDaemonMeta(key, 0); !ok || meta.StartTime == "" {
		t.Skip("start time unreadable on this platform")
	}
	prevTerm := daemonStopTermWait
	daemonStopTermWait = 3 * time.Second
	t.Cleanup(func() { daemonStopTermWait = prevTerm })

	stopDaemonSlot(key, 0, false)

	if isProcessAlive(pid) {
		t.Fatal("proven stuck daemon was not stopped")
	}
	if recordExists(key, 0) {
		t.Fatal("record not removed after stopping the daemon")
	}
}

// The identity sidecar is written before the port and PID, and removed with
// them.
func TestWritePIDFileSlot_RecordsIdentityMetadata(t *testing.T) {
	isolateDaemonRegistry(t)
	key := hashSources([]string{"/meta/repo"})
	pid := os.Getpid()
	if err := writePIDFileSlot(pid, 4321, key, 1); err != nil {
		t.Fatal(err)
	}
	meta, ok := readDaemonMeta(key, 1)
	if !ok || meta.PID != pid {
		t.Fatalf("metadata = %+v ok=%v, want PID %d", meta, ok, pid)
	}
	if meta.StartTime != daemonProcessStart(pid) {
		t.Fatalf("recorded start time %q, live %q", meta.StartTime, daemonProcessStart(pid))
	}
	removePIDFileSlot(key, 1)
	if _, ok := readDaemonMeta(key, 1); ok {
		t.Fatal("metadata outlived its record")
	}
}
