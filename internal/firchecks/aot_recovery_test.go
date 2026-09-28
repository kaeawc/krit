package firchecks

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/jvmaot"
)

func fakeAOTDaemon(t *testing.T, response string, args ...string) *FirDaemon {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	go func() {
		reader := bufio.NewReader(server)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.Contains(line, `"command":"shutdown"`) {
				return
			}
			_, _ = fmt.Fprintln(server, response)
		}
	}()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	cmd.Args = append(cmd.Args, args...)
	return &FirDaemon{cmd: cmd, conn: client, reader: bufio.NewScanner(client), nextID: 1, started: true, aotCachePath: firAOTCacheArg(cmd.Args)}
}

func TestZeroRegisteredFIRRulesDiscardsAOTAndRetries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	_, cache, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if err := os.WriteFile(p, []byte("seed"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	first := fakeAOTDaemon(t, `{"id":1,"rules":[],"findings":[]}`, "-XX:AOTCache="+cache)
	original := startFirWithoutAOT
	t.Cleanup(func() { startFirWithoutAOT = original })
	retries := 0
	startFirWithoutAOT = func(_ string, _ bool, _ ...string) (*FirDaemon, error) {
		retries++
		args := buildFirJVMArgsWithAOT(jar, "missing-java", 27, false, false)
		for _, arg := range args {
			if strings.HasPrefix(arg, "-XX:AOT") || strings.HasPrefix(arg, "-Xlog:aot") {
				t.Fatalf("retry contains AOT: %v", args)
			}
		}
		return fakeAOTDaemon(t, `{"id":1,"rules":["InjectDispatcher"],"findings":[{"path":"/src/A.kt","line":1,"col":1,"rule":"InjectDispatcher","message":"found"}]}`), nil
	}
	resp, err := checkFirWithRecovery(first, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if retries != 1 || len(resp.Rules) != 1 || len(resp.Findings) != 1 {
		t.Fatalf("retries=%d response=%+v", retries, resp)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("poisoned artifact remains %s: %v", p, err)
		}
	}
	for _, arg := range buildFirJVMArgs(jar, "missing-java", 27) {
		if strings.HasPrefix(arg, "-XX:AOT") || strings.HasPrefix(arg, "-Xlog:aot") {
			t.Fatalf("later launch retrained AOT in same process: %s", arg)
		}
	}
}

func TestUnknownRuleDoesNotTriggerAOTRecovery(t *testing.T) {
	if missingKnownFIRRules([]string{"OnlyGoRule"}, &CheckResponse{rulesPresent: true}) {
		t.Fatal("unknown rule triggered recovery")
	}
}

func TestAbsentRulesFieldDoesNotTriggerAOTRecovery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, cache, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{cache, cache + ".meta.json"} {
		if err := os.WriteFile(path, []byte("seed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	d := fakeAOTDaemon(t, `{"id":1,"findings":[{"path":"/src/A.kt","line":1,"col":1,"rule":"InjectDispatcher","message":"original"}]}`, "-XX:AOTCache="+cache)
	original := startFirWithoutAOT
	t.Cleanup(func() { startFirWithoutAOT = original })
	startFirWithoutAOT = func(_ string, _ bool, _ ...string) (*FirDaemon, error) {
		t.Fatal("old jar response triggered retry")
		return nil, nil
	}
	resp, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Findings) != 1 || resp.Findings[0].Message != "original" {
		t.Fatalf("response = %+v", resp)
	}
	for _, path := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cache artifact %s removed: %v", path, err)
		}
	}
	_ = d.Close()
}

func TestReconnectedAOTDaemonIsRetiredAndRetried(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, cache, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{cache, cache + ".meta.json"} {
		if err := os.WriteFile(path, []byte("seed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
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
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					if strings.Contains(sc.Text(), `"command":"ping"`) {
						_, _ = fmt.Fprintln(conn, `{}`)
					} else {
						_, _ = fmt.Fprintln(conn, `{"id":2,"rules":[],"findings":[]}`)
					}
				}
			}()
		}
	}()
	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = proc.Wait(); close(done) }()
	t.Cleanup(func() { _ = proc.Process.Kill(); <-done })
	key := "reconnected-aot"
	if err := writeFirPIDFile(proc.Process.Pid, listener.Addr().(*net.TCPAddr).Port, key, cache); err != nil {
		t.Fatal(err)
	}
	d, err := connectExistingFirDaemon(key, false)
	if err != nil {
		t.Fatal(err)
	}
	if !d.shared || d.cmd != nil || d.aotCachePath != cache {
		t.Fatalf("reconnected daemon = %+v", d)
	}
	original := startFirWithoutAOT
	t.Cleanup(func() { startFirWithoutAOT = original })
	retries := 0
	startFirWithoutAOT = func(_ string, _ bool, _ ...string) (*FirDaemon, error) {
		retries++
		return fakeAOTDaemon(t, `{"id":1,"rules":["InjectDispatcher"],"findings":[{"path":"/src/A.kt","line":1,"col":1,"rule":"InjectDispatcher","message":"retried"}]}`), nil
	}
	resp, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if retries != 1 || len(resp.Findings) != 1 || resp.Findings[0].Message != "retried" {
		t.Fatalf("retries=%d response=%+v", retries, resp)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shared daemon was not killed")
	}
	for _, path := range []string{cache, cache + ".meta.json", firPIDPath(key), firPortPath(key), firAOTPath(key)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale artifact %s: %v", path, err)
		}
	}
	for _, arg := range buildFirJVMArgs(jar, "missing-java", 27) {
		if strings.HasPrefix(arg, "-XX:AOT") || strings.HasPrefix(arg, "-Xlog:aot") {
			t.Fatalf("later launch uses AOT: %v", arg)
		}
	}
}
