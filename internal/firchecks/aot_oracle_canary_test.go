package firchecks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/jvmaot"
)

const healthyOracleCanaryResponse = `{"id":1,"rules":["DefaultLocale"],"findings":[{"path":"/tmp/krit-fir-canary-x/Canary.kt","line":3,"col":23,"rule":"DefaultLocale","message":"canary"}],"oracleCanary":{"packageName":"krit.canary","expressions":1}}`

// stubStartDaemon makes connectOrStartFirDaemon launch d instead of a JVM,
// and records which role's launcher it used.
func stubStartDaemon(t *testing.T, d *FirDaemon) *[]string {
	t.Helper()
	var launched []string
	old, oldOracle := startFirDaemon, startFirOracleDaemon
	startFirDaemon = func(string, bool, ...CompileTarget) (*FirDaemon, error) {
		launched = append(launched, firCheckRole)
		return d, nil
	}
	startFirOracleDaemon = func(string, bool, ...CompileTarget) (*FirDaemon, error) {
		launched = append(launched, "oracle")
		return d, nil
	}
	t.Cleanup(func() { startFirDaemon, startFirOracleDaemon = old, oldOracle })
	return &launched
}

// oracleRoleAOTDaemon is an AOT-launched fake whose canary answers canary
// and every other request answers `{}`, counting the requests that reach it.
func oracleRoleAOTDaemon(t *testing.T, cache, canary string, real *int32) *FirDaemon {
	t.Helper()
	return fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			return canary
		}
		atomic.AddInt32(real, 1)
		return `{"id":1,"result":{}}`
	}, "-XX:AOTCache="+cache)
}

func readMeta(t *testing.T, key string) firAOTMeta {
	t.Helper()
	meta, found := readFirAOTMeta(key)
	if !found {
		t.Fatalf("no registry metadata for %s", key)
	}
	return meta
}

func TestOracleCanaryHealthy(t *testing.T) {
	cases := map[string]struct {
		facts *oracleCanaryFacts
		want  bool
	}{
		"missing (old jar)": {nil, false},
		"error":             {&oracleCanaryFacts{PackageName: aotCanaryPackage, Expressions: 1, Error: "boom"}, false},
		"no expressions":    {&oracleCanaryFacts{PackageName: aotCanaryPackage}, false},
		"wrong package":     {&oracleCanaryFacts{PackageName: "other", Expressions: 1}, false},
		"healthy":           {&oracleCanaryFacts{PackageName: aotCanaryPackage, Expressions: 2}, true},
	}
	for name, tc := range cases {
		if got := oracleCanaryHealthy(tc.facts); got != tc.want {
			t.Errorf("%s: oracleCanaryHealthy = %v, want %v", name, got, tc.want)
		}
	}
}

// A passing canary publishes the oracle-role AOT daemon as verified, before
// it serves any request, and keeps the cache.
func TestOracleRoleAOTDaemonPublishedOnceCanaryPasses(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var real, retries int32
	var canaryLine string
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			canaryLine = line
			return healthyOracleCanaryResponse
		}
		atomic.AddInt32(&real, 1)
		return `{}`
	}, "-XX:AOTCache="+cache)
	launched := stubStartDaemon(t, d)
	stubRetryDaemon(t, &retries, `{}`)

	got, err := ConnectOrStartFirDaemon(jar, []string{t.TempDir()}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(*launched) != 1 || (*launched)[0] != "oracle" {
		t.Fatalf("launchers = %v, want the oracle role's", *launched)
	}
	if got != d || !d.aotVerified || d.unpublished || retries != 0 || real != 0 {
		t.Fatalf("daemon=%p want %p verified=%v unpublished=%v retries=%d real=%d", got, d, d.aotVerified, d.unpublished, retries, real)
	}
	if !strings.Contains(canaryLine, `"oracle":true`) {
		t.Fatalf("oracle-role canary did not ask for oracle facts: %s", canaryLine)
	}
	if pid, ok := registeredFirPID(d.sourcesHash); !ok || pid != d.cmd.Process.Pid {
		t.Fatalf("registered=%d,%v want %d", pid, ok, d.cmd.Process.Pid)
	}
	if meta := readMeta(t, d.sourcesHash); meta.Cache != cache || !meta.Verified {
		t.Fatalf("metadata = %+v", meta)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("healthy cache %s removed: %v", p, err)
		}
	}
}

// The #768 failure for the oracle role: the checker canary passes but the
// oracle facts do not come back (or the jar predates them). The AOT daemon
// is closed unregistered, the cache is discarded, and the caller gets a
// daemon launched without AOT, registered as non-AOT.
func TestOracleRoleAOTDaemonFailingCanaryFallsBackWithoutAOT(t *testing.T) {
	for name, canary := range map[string]string{
		"no checker finding": `{"id":1,"rules":["DefaultLocale"],"findings":[],"oracleCanary":{"packageName":"krit.canary","expressions":1}}`,
		"no oracle facts":    `{"id":1,"rules":["DefaultLocale"],"findings":[{"path":"/t/Canary.kt","line":3,"col":23,"rule":"DefaultLocale","message":"canary"}],"oracleCanary":{"packageName":"krit.canary","expressions":0}}`,
		"old jar":            healthyCanaryResponse,
		"unknown command":    `{"id":1,"error":"Unknown command: canary"}`,
	} {
		t.Run(name, func(t *testing.T) {
			jar, cache := seedAOTCache(t)
			var real, retries int32
			d := oracleRoleAOTDaemon(t, cache, canary, &real)
			stubStartDaemon(t, d)
			stubRetryDaemon(t, &retries, `{}`)

			got, err := ConnectOrStartFirDaemon(jar, []string{t.TempDir()}, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if got == d || retries != 1 || real != 0 {
				t.Fatalf("got AOT daemon=%v retries=%d real=%d", got == d, retries, real)
			}
			if d.aotVerified || d.started {
				t.Fatalf("failed AOT daemon verified=%v started=%v", d.aotVerified, d.started)
			}
			if got.aotCachePath != "" || got.unpublished || got.role != "" || got.sourcesHash == "" {
				t.Fatalf("retry daemon = cache %q unpublished=%v role=%q key=%q", got.aotCachePath, got.unpublished, got.role, got.sourcesHash)
			}
			if pid, ok := registeredFirPID(got.sourcesHash); !ok || pid != got.cmd.Process.Pid {
				t.Fatalf("registered=%d,%v want retry %d", pid, ok, got.cmd.Process.Pid)
			}
			if meta := readMeta(t, got.sourcesHash); meta.Cache != "" || !meta.Verified || meta.PID != got.cmd.Process.Pid {
				t.Fatalf("metadata = %+v", meta)
			}
			for _, p := range []string{cache, cache + ".meta.json"} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("poisoned artifact remains %s: %v", p, err)
				}
			}
			for _, arg := range buildFirJVMArgs(jar, "missing-java", 27) {
				if strings.HasPrefix(arg, "-XX:AOT") || strings.HasPrefix(arg, "-Xlog:aot") {
					t.Fatalf("later launch uses AOT: %v", arg)
				}
			}
		})
	}
}

// A daemon that does not answer its canary is not proven poisoned: the
// cache is kept on disk but not used again by this process.
func TestOracleRoleAOTCanaryTransportErrorFallsBackAndKeepsCache(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var real, retries int32
	d := oracleRoleAOTDaemon(t, cache, `not json`, &real)
	stubStartDaemon(t, d)
	stubRetryDaemon(t, &retries, `{}`)

	got, err := ConnectOrStartFirDaemon(jar, []string{t.TempDir()}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got == d || retries != 1 || real != 0 || d.aotVerified {
		t.Fatalf("got AOT daemon=%v retries=%d real=%d verified=%v", got == d, retries, real, d.aotVerified)
	}
	for _, p := range []string{cache, cache + ".meta.json"} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("cache %s removed after a transport error: %v", p, err)
		}
	}
	for _, arg := range buildFirJVMArgs(jar, "missing-java", 27) {
		if strings.HasPrefix(arg, "-XX:AOT") || strings.HasPrefix(arg, "-Xlog:aot") {
			t.Fatalf("later launch uses AOT: %v", arg)
		}
	}
}

// The checker role is unchanged: its AOT daemon is not canaried on connect
// and stays unregistered until its first check runs the canary.
func TestCheckerRoleAOTDaemonIsVerifiedOnFirstCheckNotOnConnect(t *testing.T) {
	jar, cache := seedAOTCache(t)
	var canaries, real, retries int32
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			atomic.AddInt32(&canaries, 1)
			if strings.Contains(line, `"oracle"`) {
				t.Errorf("checker canary asked for oracle facts: %s", line)
			}
			return healthyCanaryResponse
		}
		atomic.AddInt32(&real, 1)
		return `{"id":1,"rules":["InjectDispatcher"],"findings":[]}`
	}, "-XX:AOTCache="+cache)
	launched := stubStartDaemon(t, d)
	stubRetryDaemon(t, &retries, `{}`)

	got, err := connectOrStartFirCheckDaemon(jar, []string{t.TempDir()}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(*launched) != 1 || (*launched)[0] != firCheckRole {
		t.Fatalf("launchers = %v, want the checker role's", *launched)
	}
	if got != d || canaries != 0 || !d.unpublished || d.aotVerified {
		t.Fatalf("on connect: canaries=%d unpublished=%v verified=%v", canaries, d.unpublished, d.aotVerified)
	}
	if _, ok := registeredFirPID(d.sourcesHash); ok {
		t.Fatal("checker AOT daemon registered before its canary")
	}
	if _, err := checkFirWithRecovery(d, jar, []fileRef{{Path: "/src/A.kt"}}, nil, nil, []string{"InjectDispatcher"}, nil, FileFacts{}, false); err != nil {
		t.Fatal(err)
	}
	if canaries != 1 || real != 1 || retries != 0 || !d.aotVerified || d.unpublished {
		t.Fatalf("after check: canaries=%d real=%d retries=%d verified=%v unpublished=%v", canaries, real, retries, d.aotVerified, d.unpublished)
	}
}

// A daemon launched without AOT (no cache yet, or an older JDK) needs no
// canary in either role.
func TestOracleRoleWithoutAOTSkipsCanary(t *testing.T) {
	jar, _ := seedAOTCache(t)
	var canaries int32
	d := fakeAOTDaemonFunc(t, func(line string) string {
		if isCanaryRequest(line) {
			atomic.AddInt32(&canaries, 1)
		}
		return `{}`
	})
	stubStartDaemon(t, d)
	got, err := ConnectOrStartFirDaemon(jar, []string{t.TempDir()}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != d || canaries != 0 || d.unpublished {
		t.Fatalf("canaries=%d unpublished=%v", canaries, d.unpublished)
	}
	if meta := readMeta(t, d.sourcesHash); meta.Cache != "" || !meta.Verified {
		t.Fatalf("metadata = %+v", meta)
	}
}

// An oracle-role daemon is never closed by the krit run that started it, so
// it must not record a Leyden profile (the recording would never be
// finalized and would hold the build lock); it still uses a valid cache.
func TestOracleRoleLaunchNeverRecordsLeydenProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := buildFirJVMArgsWithAOTMode(jar, "missing-java", 27, false, firAOTNoRecord)
	if jvmaot.IsRecording(args) || slices.Contains(args, "-XX:AOTMode=record") {
		t.Fatalf("oracle-role launch records: %v", args)
	}
	checker := buildFirJVMArgsWithAOTMode(jar, "missing-java", 27, false, firAOTFull)
	if !jvmaot.IsRecording(checker) {
		t.Fatalf("checker-role launch does not record: %v", checker)
	}
	jvmaot.FinalizeRecording(checker, false)
}
