package oracle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testJar(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "krit-types.jar")
	if err := os.WriteFile(p, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func replaceTestJar(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("replaced jar"), 0644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func testLivePID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
	return cmd.Process.Pid
}

func TestJarIdentityChangesWithStat(t *testing.T) {
	p := testJar(t)
	first := JarIdentity(p)
	if len(first) != 8 || first == "missing" {
		t.Fatalf("initial identity = %q", first)
	}
	if err := os.WriteFile(p, []byte("longer"), 0644); err != nil {
		t.Fatal(err)
	}
	second := JarIdentity(p)
	if second == first {
		t.Fatal("size change did not change identity")
	}
	stamp := time.Now().Add(10 * time.Second)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if JarIdentity(p) == second {
		t.Fatal("mtime change did not change identity")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if JarIdentity(p) != "missing" {
		t.Fatal("missing jar did not get missing identity")
	}
}

func TestConnectOrStartDaemon_ReplacedJarIsNotReused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := testJar(t)
	sources := []string{t.TempDir()}
	fake := NewFakeDaemon(t)
	t.Cleanup(fake.Close)
	pid := testLivePID(t)
	oldKey := daemonRegistryKey(jar, sources)
	if err := writePIDFile(pid, fake.Port, oldKey); err != nil {
		t.Fatal(err)
	}
	d, err := connectExistingDaemon(jar, sources, false)
	if err != nil {
		t.Fatalf("sanity connect: %v", err)
	}
	_ = d.Release()
	fake.mu.Lock()
	before := len(fake.conns)
	fake.mu.Unlock()
	replaceTestJar(t, jar)
	t.Setenv("PATH", t.TempDir())
	d, err = ConnectOrStartDaemon(jar, sources, nil, false)
	if d != nil || err == nil || !strings.Contains(err.Error(), "java not found") {
		if d != nil {
			_ = d.Release()
		}
		t.Fatalf("expected fresh start error, daemon=%v err=%v", d, err)
	}
	fake.mu.Lock()
	after := len(fake.conns)
	fake.mu.Unlock()
	if after != before {
		t.Fatalf("old listener received %d new connections", after-before)
	}
	if isProcessAlive(pid) {
		t.Fatal("superseded daemon process is alive")
	}
	if _, err := os.Stat(daemonPIDPathForSlot(oldKey, 0)); !os.IsNotExist(err) {
		t.Fatalf("old PID file remains: %v", err)
	}
	if _, err := os.Stat(daemonPortPathForSlot(oldKey, 0)); !os.IsNotExist(err) {
		t.Fatalf("old port file remains: %v", err)
	}
}

func TestDaemonRetiresLegacyAndPreservesOtherInputs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := testJar(t)
	sources := []string{t.TempDir()}
	otherJar := filepath.Join(t.TempDir(), filepath.Base(jar))
	if err := os.WriteFile(otherJar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := jarTag(jar) + "-" + hashSources(sources)
	otherPath := daemonRegistryKey(otherJar, sources)
	otherSources := daemonRegistryKey(jar, []string{t.TempDir()})
	otherClasspath := daemonRegistryKey(jar, sources, "/different/classpath.jar")
	legacyFake := NewFakeDaemon(t)
	t.Cleanup(legacyFake.Close)
	legacyPID := testLivePID(t)
	if err := writePIDFile(legacyPID, legacyFake.Port, legacy); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{otherPath, otherSources, otherClasspath} {
		if err := writePIDFile(99999999, 1, key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	_, _ = ConnectOrStartDaemon(jar, sources, nil, false)
	if isProcessAlive(legacyPID) {
		t.Fatal("legacy process remains alive")
	}
	for key, want := range map[string]bool{legacy: false, otherPath: true, otherSources: true, otherClasspath: true} {
		_, err := os.Stat(daemonPIDPathForSlot(key, 0))
		if (err == nil) != want {
			t.Fatalf("key %s exists=%t, want %t: %v", key, err == nil, want, err)
		}
	}
}

func TestConnectOrStartDaemonPoolRetiresSupersededSlot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := testJar(t)
	sources := []string{t.TempDir()}
	oldKey := daemonRegistryKey(jar, sources)
	oldFake := NewFakeDaemon(t)
	t.Cleanup(oldFake.Close)
	pid := testLivePID(t)
	if err := writePIDFileSlot(pid, oldFake.Port, oldKey, 1); err != nil {
		t.Fatal(err)
	}
	replaceTestJar(t, jar)
	currentFake := NewFakeDaemon(t)
	t.Cleanup(currentFake.Close)
	if err := writePIDFileSlot(os.Getpid(), currentFake.Port, daemonRegistryKey(jar, sources), 0); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	pool, err := ConnectOrStartDaemonPool(jar, sources, nil, 2, false)
	if pool != nil {
		_ = pool.Release()
	}
	if err == nil || !strings.Contains(err.Error(), "java not found") {
		t.Fatalf("expected slot-1 fresh start error: %v", err)
	}
	if isProcessAlive(pid) {
		t.Fatal("superseded slot process remains alive")
	}
	if _, err := os.Stat(daemonPIDPathForSlot(oldKey, 1)); !os.IsNotExist(err) {
		t.Fatalf("superseded slot file remains: %v", err)
	}
}

func TestDaemonMatchesRepoAfterJarReplacement(t *testing.T) {
	jar := testJar(t)
	sources := []string{t.TempDir()}
	d := &Daemon{sourcesHash: daemonRegistryKey(jar, sources)}
	if !d.MatchesRepo(jar, sources) {
		t.Fatal("initial jar does not match")
	}
	replaceTestJar(t, jar)
	if d.MatchesRepo(jar, sources) {
		t.Fatal("replaced jar still matches")
	}
}
