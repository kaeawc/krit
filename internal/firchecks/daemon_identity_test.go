package firchecks

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// failStart replaces the daemon launcher with one that records the call and
// fails, so reconnect tests never start a JVM.
func failStart(t *testing.T) *int {
	t.Helper()
	calls := new(int)
	old, oldOracle := startFirDaemon, startFirOracleDaemon
	startFirDaemon = func(string, bool, ...CompileTarget) (*FirDaemon, error) {
		*calls++
		return nil, errors.New("test: no JVM")
	}
	startFirOracleDaemon = startFirDaemon
	t.Cleanup(func() { startFirDaemon, startFirOracleDaemon = old, oldOracle })
	return calls
}

// deadPort returns a loopback port nothing listens on.
func deadPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// writeRawFirRecord writes a registry record verbatim, bypassing the
// publisher that stamps the process start time.
func writeRawFirRecord(t *testing.T, key string, pid, port int, meta firAOTMeta) {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(firAOTPath(key), data, 0o644))
	must(t, os.WriteFile(firPortPath(key), []byte(strconv.Itoa(port)+"\n"), 0o644))
	must(t, os.WriteFile(firPIDPath(key), []byte(strconv.Itoa(pid)+"\n"), 0o644))
}

func testFirKey(t *testing.T) (jar string, sources []string, key string) {
	t.Helper()
	dir := t.TempDir()
	jar = filepath.Join(dir, "krit-fir.jar")
	must(t, os.WriteFile(jar, []byte("jar"), 0o644))
	sources = []string{filepath.Join(dir, "src")}
	return jar, sources, firRegistryKeyFor("", jar, sources, nil)
}

// A crashed daemon leaves its record behind and the PID is reused by an
// unrelated live process. The dial is refused, but the process must not be
// signalled: the record carries no (or a different) start time, so the PID is
// unproven. The stale record is dropped and a fresh daemon is started.
func TestReconnectNeverSignalsUnprovenReusedPID(t *testing.T) {
	cases := map[string]string{
		"legacy record without start time": "",
		"start time of another process":    "1",
	}
	for name, startTime := range cases {
		t.Run(name, func(t *testing.T) {
			isolateRegistry(t)
			calls := failStart(t)
			jar, sources, key := testFirKey(t)
			pid := livePIDs(t, 1)[0]
			writeRawFirRecord(t, key, pid, deadPort(t), firAOTMeta{PID: pid, Verified: true, StartTime: startTime})
			oldTerm, oldKill := firStopTermWait, firStopKillWait
			firStopTermWait, firStopKillWait = 100*time.Millisecond, 50*time.Millisecond
			t.Cleanup(func() { firStopTermWait, firStopKillWait = oldTerm, oldKill })

			_, err := connectOrStartFirDaemon("", jar, sources, nil, false)
			if err == nil || *calls != 1 {
				t.Fatalf("expected a fresh daemon start (calls=%d, err=%v)", *calls, err)
			}
			time.Sleep(200 * time.Millisecond)
			if !processAlive(pid) {
				t.Fatal("an unrelated process owning the reused PID was signalled")
			}
			if _, ok := registeredFirPID(key); ok {
				t.Fatal("stale registry entry was kept")
			}
		})
	}
}

// A daemon whose identity is proven and whose port refuses connections is
// dead weight and is still stopped.
func TestReconnectStopsProvenDaemonWithRefusedDial(t *testing.T) {
	isolateRegistry(t)
	calls := failStart(t)
	jar, sources, key := testFirKey(t)
	pid := livePIDs(t, 1)[0]
	must(t, writeFirRegistry(pid, deadPort(t), key, firAOTMeta{PID: pid, Verified: true}))
	oldTerm, oldKill := firStopTermWait, firStopKillWait
	firStopTermWait, firStopKillWait = time.Second, 50*time.Millisecond
	t.Cleanup(func() { firStopTermWait, firStopKillWait = oldTerm, oldKill })

	if _, err := connectOrStartFirDaemon("", jar, sources, nil, false); err == nil || *calls != 1 {
		t.Fatalf("expected a fresh daemon start (calls=%d, err=%v)", *calls, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatal("proven daemon with a refused port was not stopped")
	}
	if _, ok := registeredFirPID(key); ok {
		t.Fatal("record of the stopped daemon was kept")
	}
}

// A daemon serves one client at a time, so a second run's ping queues behind
// the first client and times out. The port accepted the connection, so the
// daemon is busy, not dead: it stays running and registered, and this run
// starts its own daemon.
func TestReconnectDoesNotStopBusyDaemon(t *testing.T) {
	isolateRegistry(t)
	calls := failStart(t)
	old := firPingTimeout
	firPingTimeout = 150 * time.Millisecond
	t.Cleanup(func() { firPingTimeout = old })
	jar, sources, key := testFirKey(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Accept but never answer, like a daemon busy with another client.
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	pid := livePIDs(t, 1)[0]
	port := listener.Addr().(*net.TCPAddr).Port
	must(t, writeFirRegistry(pid, port, key, firAOTMeta{PID: pid, Verified: true}))

	if _, err := connectExistingFirDaemon(key, false); !errors.Is(err, errFirDaemonBusy) {
		t.Fatalf("connectExistingFirDaemon error = %v, want errFirDaemonBusy", err)
	}
	oldTerm, oldKill := firStopTermWait, firStopKillWait
	firStopTermWait, firStopKillWait = 100*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { firStopTermWait, firStopKillWait = oldTerm, oldKill })

	if _, err := connectOrStartFirDaemon("", jar, sources, nil, false); err == nil || *calls != 1 {
		t.Fatalf("expected a private daemon start (calls=%d, err=%v)", *calls, err)
	}
	time.Sleep(200 * time.Millisecond)
	if !processAlive(pid) {
		t.Fatal("a busy daemon was killed")
	}
	if got, ok := registeredFirPID(key); !ok || got != pid {
		t.Fatalf("busy daemon's registry entry changed: pid=%d ok=%v", got, ok)
	}
}

// silentListener accepts connections and never answers, like a busy daemon or
// an unrelated program that does not speak the protocol.
func silentListener(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// A stale record whose PID was reused and whose old port is now held by an
// unrelated program that accepts but never answers is not a busy daemon: the
// PID is unproven, so the record is dropped (without signalling) instead of
// being kept and making every later run wait out the ping timeout.
func TestReconnectDropsUnprovenRecordWhosePortIsSilent(t *testing.T) {
	isolateRegistry(t)
	calls := failStart(t)
	old := firPingTimeout
	firPingTimeout = 150 * time.Millisecond
	t.Cleanup(func() { firPingTimeout = old })
	jar, sources, key := testFirKey(t)
	pid := livePIDs(t, 1)[0]
	writeRawFirRecord(t, key, pid, silentListener(t), firAOTMeta{PID: pid, Verified: true, StartTime: "1"})

	if _, err := connectExistingFirDaemon(key, false); err == nil || errors.Is(err, errFirDaemonBusy) {
		t.Fatalf("connectExistingFirDaemon error = %v, want a non-busy rejection", err)
	}
	oldTerm, oldKill := firStopTermWait, firStopKillWait
	firStopTermWait, firStopKillWait = 100*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { firStopTermWait, firStopKillWait = oldTerm, oldKill })

	if _, err := connectOrStartFirDaemon("", jar, sources, nil, false); err == nil || *calls != 1 {
		t.Fatalf("expected a fresh daemon start (calls=%d, err=%v)", *calls, err)
	}
	time.Sleep(200 * time.Millisecond)
	if !processAlive(pid) {
		t.Fatal("an unrelated process owning the reused PID was signalled")
	}
	if _, ok := registeredFirPID(key); ok {
		t.Fatal("stale record with an unproven PID was kept")
	}
}
