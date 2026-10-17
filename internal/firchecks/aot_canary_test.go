package firchecks

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/jvmaot"
)

const healthyCanaryResponse = `{"id":1,"rules":["DefaultLocale"],"findings":[{"path":"/tmp/krit-fir-canary-x/Canary.kt","line":3,"col":23,"rule":"DefaultLocale","message":"canary"}]}`

func isCanaryRequest(line string) bool {
	return strings.Contains(line, `"command":"canary"`)
}

func seedAOTCache(t *testing.T) (jar, cache string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	jar = filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, cache, _, err := jvmaot.Paths(jar, "fir")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if err := os.WriteFile(p, []byte("seed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return jar, cache
}

func stubRetryDaemon(t *testing.T, retries *int32, response string) {
	t.Helper()
	original := startFirWithoutAOT
	t.Cleanup(func() { startFirWithoutAOT = original })
	startFirWithoutAOT = func(_ string, _ bool, _ ...CompileTarget) (*FirDaemon, error) {
		atomic.AddInt32(retries, 1)
		return fakeAOTDaemon(t, response), nil
	}
}

func TestCanaryHealthy(t *testing.T) {
	cases := map[string]struct {
		resp *CheckResponse
		want bool
	}{
		"nil":         {nil, false},
		"no findings": {&CheckResponse{rulesPresent: true, Rules: []string{aotCanaryRule}}, false},
		"other rule":  {&CheckResponse{Findings: []FirFinding{{Rule: "InjectDispatcher", Path: "/t/Canary.kt"}}}, false},
		"other file":  {&CheckResponse{Findings: []FirFinding{{Rule: aotCanaryRule, Path: "/t/A.kt"}}}, false},
		"found":       {&CheckResponse{Findings: []FirFinding{{Rule: aotCanaryRule, Path: "/t/Canary.kt"}}}, true},
	}
	for name, tc := range cases {
		if got := canaryHealthy(tc.resp); got != tc.want {
			t.Errorf("%s: canaryHealthy = %v, want %v", name, got, tc.want)
		}
	}
}

// The canary must not depend on the project: it names no files, source dirs,
// classpath, or JVM target, so the daemon cannot compile the user's sources.
func TestCanaryRequestIsProjectIndependent(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var canaryLine string
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			canaryLine = line
			return healthyCanaryResponse
		}
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	_, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, []string{"/src"}, []string{"/lib.jar"}, []string{"InjectDispatcher"}, nil, FileFacts{}, false, CompileTarget{JvmTarget: "17"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"files", "sourceDirs", "classpath", "jvmTarget", "kotlinArgs", "rules"} {
		if strings.Contains(canaryLine, `"`+field+`"`) {
			t.Fatalf("canary request carries %s: %s", field, canaryLine)
		}
	}
	if !strings.Contains(canaryLine, `"command":"canary"`) {
		t.Fatalf("canary line = %s", canaryLine)
	}
}

// A JVM that registers its rules but returns no findings is the failure #768
// describes. The zero-registered-rules check cannot see it; the canary must.
func TestCanaryCatchesRegisteredRulesWithZeroFindings(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var realRequests int32
	first := fakeAOTDaemonFunc(t, func(line string) string {
		if !isCanaryRequest(line) {
			atomic.AddInt32(&realRequests, 1)
		}
		return `{"id":1,"rules":["InjectDispatcher","DefaultLocale"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	var retries int32
	stubRetryDaemon(t, &retries, `{"id":1,"rules":["InjectDispatcher"],"findings":[{"path":"/src/A.kt","line":1,"col":1,"rule":"InjectDispatcher","message":"found"}]}`)

	resp, err := checkFirWithRecovery(first, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if retries != 1 || len(resp.Findings) != 1 || resp.Findings[0].Message != "found" {
		t.Fatalf("retries=%d response=%+v", retries, resp)
	}
	if realRequests != 0 {
		t.Fatalf("poisoned daemon answered %d real requests", realRequests)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("poisoned artifact remains %s: %v", p, err)
		}
	}
}

// A jar without the canary request answers "Unknown command": it must never
// be trusted.
func TestOldJarWithoutCanaryIsNotTrusted(t *testing.T) {
	jar, cache := seedAOTCache(t)
	first := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			return `{"id":1,"error":"Unknown command: canary"}`
		}
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	var retries int32
	stubRetryDaemon(t, &retries, `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`)
	if _, err := checkFirWithRecovery(first, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false); err != nil {
		t.Fatal(err)
	}
	if retries != 1 || first.aotVerified {
		t.Fatalf("retries=%d verified=%v", retries, first.aotVerified)
	}
}

func TestCanaryFailsClosedWhenRetryCannotReportToo(t *testing.T) {
	jar, cache := seedAOTCache(t)
	first := fakeAOTDaemon(t, `{"id":1,"rules":[],"findings":[]}`, "-XX:AOTCache="+cache)
	var retries int32
	stubRetryDaemon(t, &retries, `{"id":1,"rules":[],"findings":[]}`)
	_, err := checkFirWithRecovery(first, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
	if err == nil || !strings.Contains(err.Error(), errFIRDegraded.Error()) {
		t.Fatalf("err = %v, want errFIRDegraded", err)
	}
}

// A canary transport error must not let a later call skip verification.
func TestCanaryTransportErrorKeepsDaemonUnverified(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var canaries, real int32
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			if atomic.AddInt32(&canaries, 1) == 1 {
				return `not json`
			}
			return healthyCanaryResponse
		}
		atomic.AddInt32(&real, 1)
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	var retries int32
	stubRetryDaemon(t, &retries, `{}`)
	call := func() error {
		_, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
		return err
	}
	if err := call(); err == nil {
		t.Fatal("canary transport error was swallowed")
	}
	if d.aotVerified || real != 0 {
		t.Fatalf("verified=%v real=%d after canary error", d.aotVerified, real)
	}
	if err := call(); err != nil {
		t.Fatal(err)
	}
	if canaries != 2 || real != 1 || !d.aotVerified || retries != 0 {
		t.Fatalf("canaries=%d real=%d verified=%v retries=%d", canaries, real, d.aotVerified, retries)
	}
}

func TestHealthyCanaryKeepsAOTDaemonAndRunsOnce(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var canaries, real int32
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			atomic.AddInt32(&canaries, 1)
			return healthyCanaryResponse
		}
		atomic.AddInt32(&real, 1)
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	var retries int32
	stubRetryDaemon(t, &retries, `{}`)
	for i := 0; i < 3; i++ {
		resp, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Findings) != 0 {
			t.Fatalf("response = %+v", resp)
		}
	}
	if canaries != 1 || real != 3 || retries != 0 {
		t.Fatalf("canaries=%d real=%d retries=%d", canaries, real, retries)
	}
	if !d.aotVerified {
		t.Fatal("daemon not marked verified")
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("healthy cache %s removed: %v", p, err)
		}
	}
}

func TestNoAOTDaemonSkipsCanary(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	var canaries int32
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			atomic.AddInt32(&canaries, 1)
		}
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	})
	if _, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false); err != nil {
		t.Fatal(err)
	}
	if canaries != 0 {
		t.Fatalf("canary ran %d times on a daemon without AOT", canaries)
	}
}

// pingListener is a registered-daemon stand-in: a live PID plus a TCP port
// that answers ping.
func pingListener(t *testing.T) (pid, port int) {
	t.Helper()
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
					_, _ = fmt.Fprintln(conn, `{}`)
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
	return proc.Process.Pid, listener.Addr().(*net.TCPAddr).Port
}

// connectExistingFirDaemon decides reuse: only daemons known to be non-AOT or
// AOT-and-verified are reusable; anything else, including unreadable
// metadata, is untrusted.
func TestReconnectReuseDecision(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	pid, port := pingListener(t)
	cases := []struct {
		name      string
		setup     func(key string)
		reusable  bool
		wantCache string
	}{
		{"non-AOT", func(key string) {
			must(t, writeFirPIDFile(pid, port, key))
		}, true, ""},
		{"AOT verified", func(key string) {
			must(t, writeFirRegistry(pid, port, key, firAOTMeta{PID: pid, Cache: "/c.aot", Verified: true}))
		}, true, "/c.aot"},
		{"AOT unverified", func(key string) {
			must(t, writeFirPIDFile(pid, port, key, "/c.aot"))
		}, false, "/c.aot"},
		{"metadata missing", func(key string) {
			must(t, writeFirPIDFile(pid, port, key))
			must(t, os.Remove(firAOTPath(key)))
		}, false, unknownAOTState},
		{"metadata corrupt", func(key string) {
			must(t, writeFirPIDFile(pid, port, key))
			must(t, os.WriteFile(firAOTPath(key), []byte("{not json"), 0o644))
		}, false, unknownAOTState},
		{"metadata for another PID", func(key string) {
			must(t, writeFirRegistry(pid, port, key, firAOTMeta{PID: pid + 1, Cache: "", Verified: true}))
		}, false, unknownAOTState},
	}
	for i, tc := range cases {
		key := fmt.Sprintf("reuse-%d", i)
		tc.setup(key)
		d, err := connectExistingFirDaemon(key, false)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if d.aotVerified != tc.reusable || d.aotCachePath != tc.wantCache {
			t.Errorf("%s: verified=%v cache=%q, want %v %q", tc.name, d.aotVerified, d.aotCachePath, tc.reusable, tc.wantCache)
		}
		_ = d.conn.Close()
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAOTMetadataWriteIsAtomicAndLeavesNoTemps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", dir)
	for i := 0; i < 5; i++ {
		must(t, writeFirAOTMeta("atomic", firAOTMeta{PID: i + 1, Cache: "/c.aot", Verified: i%2 == 0}))
	}
	data, err := os.ReadFile(firAOTPath("atomic"))
	if err != nil || !strings.Contains(string(data), `"pid":5`) {
		t.Fatalf("metadata = %q, %v", data, err)
	}
	var leftovers []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, _ error) error {
		if info != nil && strings.Contains(info.Name(), ".tmp.") {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

// An AOT daemon stays out of the registry until verified, so a concurrent
// krit process cannot reuse or retire it mid-canary; verification publishes.
func TestVerificationPublishesUnpublishedDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	pid, port := pingListener(t)
	d := &FirDaemon{sourcesHash: "publish", aotCachePath: "/c.aot", aotPID: pid, port: port, unpublished: true}
	if _, ok := registeredFirPID("publish"); ok {
		t.Fatal("registered before verification")
	}
	d.markAOTVerified()
	if got, ok := registeredFirPID("publish"); !ok || got != pid || d.unpublished || !d.aotVerified {
		t.Fatalf("registered=%d,%v unpublished=%v verified=%v", got, ok, d.unpublished, d.aotVerified)
	}
	re, err := connectExistingFirDaemon("publish", false)
	if err != nil || !re.aotVerified {
		t.Fatalf("reconnect = %+v, %v", re, err)
	}
	_ = re.conn.Close()
}

// Another live daemon already holding the key must not be overwritten.
func TestVerificationDoesNotClobberLiveRegisteredDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	otherPID, otherPort := pingListener(t)
	must(t, writeFirPIDFile(otherPID, otherPort, "clobber"))
	d := &FirDaemon{sourcesHash: "clobber", aotCachePath: "/c.aot", aotPID: otherPID + 1000000, port: 1, unpublished: true}
	d.markAOTVerified()
	if got, _ := registeredFirPID("clobber"); got != otherPID {
		t.Fatalf("registry now names %d, want %d", got, otherPID)
	}
	if !d.unpublished {
		t.Fatal("daemon published over a live one")
	}
	data, _ := os.ReadFile(firAOTPath("clobber"))
	if strings.Contains(string(data), "/c.aot") {
		t.Fatalf("metadata overwritten: %s", data)
	}
}

// A registered daemon's metadata is only updated while the PID file names it.
func TestVerifiedMetadataOnlyWrittenWhilePIDFileStillNamesDaemon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	must(t, writeFirPIDFile(4242, 1, "ownership", "/c.aot"))
	stale := &FirDaemon{sourcesHash: "ownership", aotCachePath: "/c.aot", aotPID: 7}
	stale.markAOTVerified()
	if data, _ := os.ReadFile(firAOTPath("ownership")); strings.Contains(string(data), `"verified":true`) {
		t.Fatalf("daemon that no longer owns the key marked it verified: %s", data)
	}
	owner := &FirDaemon{sourcesHash: "ownership", aotCachePath: "/c.aot", aotPID: 4242}
	owner.markAOTVerified()
	if data, _ := os.ReadFile(firAOTPath("ownership")); !strings.Contains(string(data), `"verified":true`) {
		t.Fatalf("owner not recorded: %s", data)
	}
}

// Releasing or closing a daemon that never published must neither keep it
// running nor delete the registry entry of another daemon.
func TestUnpublishedDaemonIsClosedOnReleaseAndLeavesRegistryAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	otherPID, otherPort := pingListener(t)
	must(t, writeFirPIDFile(otherPID, otherPort, "unpub"))
	d := fakeAOTDaemon(t, `{}`, "-XX:AOTCache=/c.aot")
	d.sourcesHash, d.port, d.unpublished = "unpub", 9, true
	if err := d.Release(); err != nil {
		t.Fatal(err)
	}
	if d.started {
		t.Fatal("unpublished daemon left running after Release")
	}
	if got, ok := registeredFirPID("unpub"); !ok || got != otherPID {
		t.Fatalf("registry entry changed: %d %v", got, ok)
	}
}
