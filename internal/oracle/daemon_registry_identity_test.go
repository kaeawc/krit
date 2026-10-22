package oracle

import (
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
)

// silentDaemonPort listens on a loopback port that accepts connections and
// never answers: what a daemon busy with another krit run looks like to a
// client that pings it, and what an unrelated program holding a stale record's
// port looks like too.
func silentDaemonPort(t *testing.T) int {
	t.Helper()
	ln := listenLocalTCP(t)
	var (
		mu    sync.Mutex
		conns []net.Conn
	)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return ln.Addr().(*net.TCPAddr).Port
}

// closedPort returns a loopback port nothing listens on.
func closedPort(t *testing.T) int {
	t.Helper()
	ln := listenLocalTCP(t)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// writeLegacyDaemonRecord writes a registry record the way a krit that
// predates identity metadata did: PID and port files, nothing else.
func writeLegacyDaemonRecord(t *testing.T, key string, slot int, pid, port int) {
	t.Helper()
	if err := os.WriteFile(daemonPortPathForSlot(key, slot), []byte(strconv.Itoa(port)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(daemonPIDPathForSlot(key, slot), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordExists(key string, slot int) bool {
	_, err := os.Stat(daemonPIDPathForSlot(key, slot))
	return err == nil
}

// A live PID is not proof of the daemon: after a crash or reboot it may be an
// unrelated process of the same user. A record without identity metadata names
// a PID nobody can vouch for, so it is dropped without signalling it.
func TestConnectOrStartDaemon_UnprovenReusedPIDIsNotSignalled(t *testing.T) {
	isolateDaemonRegistry(t)
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	noJava(t)
	key := daemonRegistryKey(jar, sources)
	writeLegacyDaemonRecord(t, key, 0, pid, closedPort(t))

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil {
		_ = d.Release()
	}
	if err == nil {
		t.Fatal("expected the fresh start to fail (no java on PATH)")
	}
	if !isProcessAlive(pid) {
		t.Fatalf("unrelated PID %d was signalled on the strength of an unproven record", pid)
	}
	if recordExists(key, 0) {
		t.Fatal("the unproven record should have been dropped")
	}
}

// A listener that accepts but never answers, behind a record whose PID cannot
// be proven, is a stale record (a reused PID whose old port someone else now
// holds). It is dropped; nothing is signalled.
func TestConnectOrStartDaemon_UnprovenSilentListenerRecordIsDropped(t *testing.T) {
	isolateDaemonRegistry(t)
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", "300ms")
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	noJava(t)
	key := daemonRegistryKey(jar, sources)
	writeLegacyDaemonRecord(t, key, 0, pid, silentDaemonPort(t))

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil {
		_ = d.Release()
	}
	if err == nil {
		t.Fatal("expected the fresh start to fail (no java on PATH)")
	}
	if !isProcessAlive(pid) {
		t.Fatalf("PID %d was signalled although it could not be proven", pid)
	}
	if recordExists(key, 0) {
		t.Fatal("the stale record behind a silent listener should have been dropped")
	}
}

// A daemon whose PID is proven and whose port accepts the connection but is
// slow to answer is serving another run. It is neither stopped nor
// unregistered.
func TestConnectOrStartDaemon_BusyProvenDaemonIsNotStopped(t *testing.T) {
	isolateDaemonRegistry(t)
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", "300ms")
	jar := testJar(t)
	sources := []string{t.TempDir()}
	pid := testLivePID(t)
	noJava(t)
	key := daemonRegistryKey(jar, sources)
	port := silentDaemonPort(t)
	// Publishing stamps the live process's start time, which proves the PID.
	if err := writePIDFileSlot(pid, port, key, 0); err != nil {
		t.Fatal(err)
	}

	d, err := ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil {
		_ = d.Release()
	}
	if err == nil {
		t.Fatal("expected the private start to fail (no java on PATH)")
	}
	if !isProcessAlive(pid) {
		t.Fatalf("busy daemon PID %d was killed", pid)
	}
	info, rerr := readPIDFileSlot(key, 0)
	if rerr != nil || info.PID != pid || info.Port != port {
		t.Fatalf("busy daemon was unregistered: info=%+v err=%v", info, rerr)
	}
}
