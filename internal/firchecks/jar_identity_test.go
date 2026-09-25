package firchecks

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectOrStartFirDaemonReplacedJarIsNotReused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	sources := []string{t.TempDir()}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	var connections atomic.Int32
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			go func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					_, _ = conn.Write([]byte("{}\n"))
				}
			}()
		}
	}()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
	oldKey := firRegistryKey(jar, sources)
	if err := writeFirPIDFile(cmd.Process.Pid, listener.Addr().(*net.TCPAddr).Port, oldKey); err != nil {
		t.Fatal(err)
	}
	d, err := connectExistingFirDaemon(firRegistryKey(jar, sources), false)
	if err != nil {
		t.Fatalf("sanity connect: %v", err)
	}
	if !d.MatchesRepo(jar, sources) {
		t.Fatal("original jar does not match")
	}
	_ = d.Release()
	before := connections.Load()
	if err := os.WriteFile(jar, []byte("replaced jar"), 0644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(jar, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if d.MatchesRepo(jar, sources) {
		t.Fatal("replaced jar still matches")
	}
	t.Setenv("PATH", t.TempDir())
	d, err = ConnectOrStartFirDaemon(jar, sources, nil, false)
	if d != nil || err == nil || !strings.Contains(err.Error(), "java") {
		if d != nil {
			_ = d.Release()
		}
		t.Fatalf("expected fresh start error, daemon=%v err=%v", d, err)
	}
	if connections.Load() != before {
		t.Fatal("old listener received a new connection")
	}
	if err := cmd.Process.Signal(os.Interrupt); err == nil {
		t.Fatal("superseded process remained alive")
	}
	if _, err := os.Stat(firPIDPath(oldKey)); !os.IsNotExist(err) {
		t.Fatalf("old PID file remains: %v", err)
	}
	if _, err := os.Stat(firPortPath(oldKey)); !os.IsNotExist(err) {
		t.Fatalf("old port file remains: %v", err)
	}
}

func TestFirDaemonRetiresLegacyAndPreservesOtherEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	sources := []string{t.TempDir()}
	otherJar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(otherJar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := hashFirSources(sources)
	otherPath := firRegistryKey(otherJar, sources)
	otherSources := firRegistryKey(jar, []string{t.TempDir()})
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
			go func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					_, _ = conn.Write([]byte("{}\n"))
				}
			}()
		}
	}()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })
	if err := writeFirPIDFile(cmd.Process.Pid, listener.Addr().(*net.TCPAddr).Port, legacy); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{otherPath, otherSources} {
		if err := writeFirPIDFile(99999999, 1, key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", t.TempDir())
	_, _ = ConnectOrStartFirDaemon(jar, sources, nil, false)
	if err := cmd.Process.Signal(os.Interrupt); err == nil {
		t.Fatal("legacy process remains alive")
	}
	for key, want := range map[string]bool{legacy: false, otherPath: true, otherSources: true} {
		_, err := os.Stat(firPIDPath(key))
		if (err == nil) != want {
			t.Fatalf("key %s exists=%t, want %t: %v", key, err == nil, want, err)
		}
	}
}

func TestConnectOrStartFirDaemonRegistersJarIdentityObservedBeforeStartup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("A"), 0644); err != nil {
		t.Fatal(err)
	}
	sources := []string{t.TempDir()}
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
			go func() {
				defer conn.Close()
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					_, _ = conn.Write([]byte("{}\n"))
				}
			}()
		}
	}()
	// A fake java that replaces the jar it was given before reporting ready,
	// as if the jar were upgraded while the JVM was starting.
	javaDir := t.TempDir()
	script := `#!/bin/sh
jar=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-jar" ]; then jar="$a"; fi
  prev="$a"
done
printf 'replaced during daemon startup' > "$jar"
echo '{"ready":true,"port":` + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port) + `}'
exec sleep 60
`
	if err := os.WriteFile(filepath.Join(javaDir, "java"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", javaDir)
	preStartKey := firRegistryKey(jar, sources)

	d, err := ConnectOrStartFirDaemon(jar, sources, nil, false)
	if err != nil {
		t.Fatalf("start fir daemon: %v", err)
	}
	t.Cleanup(func() { _ = d.cmd.Process.Kill(); _ = d.Close() })

	if firRegistryKey(jar, sources) == preStartKey {
		t.Fatal("fake java did not replace the jar during startup")
	}
	if d.MatchesRepo(jar, sources) {
		t.Fatal("daemon started while the jar was replaced claims to match the replacement jar")
	}
	if _, err := os.Stat(firPIDPath(preStartKey)); err != nil {
		t.Fatalf("daemon not registered under the pre-start jar identity: %v", err)
	}
	if reused, err := connectExistingFirDaemon(firRegistryKey(jar, sources), false); err == nil {
		_ = reused.Release()
		t.Fatal("daemon started while the jar was replaced is reused for the replacement jar")
	}
}
