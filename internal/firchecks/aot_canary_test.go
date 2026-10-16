package firchecks

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/jvmaot"
)

const healthyCanaryResponse = `{"id":1,"rules":["DefaultLocale"],"findings":[{"path":"/tmp/krit-fir-canary-x/Canary.kt","line":3,"col":23,"rule":"DefaultLocale","message":"canary"}]}`

func isCanaryRequest(line string) bool {
	return strings.Contains(line, `"rules":["DefaultLocale"]`) && strings.Contains(line, "Canary.kt")
}

func seedAOTCache(t *testing.T) (jar, cache string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
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
	startFirWithoutAOT = func(_ string, _ bool, _ ...string) (*FirDaemon, error) {
		atomic.AddInt32(retries, 1)
		return fakeAOTDaemon(t, response), nil
	}
}

func TestCanaryRequestIsRecognisedAndSourceHasOneFinding(t *testing.T) {
	if !strings.Contains(aotCanarySource, "String.format(") || strings.Contains(aotCanarySource, "Locale") {
		t.Fatalf("canary source must call String.format without a Locale: %q", aotCanarySource)
	}
	if !knownFIRRules[aotCanaryRule] {
		t.Fatalf("canary rule %s is not a registered FIR rule", aotCanaryRule)
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

func TestVerifiedFlagPersistsAcrossReconnect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	key := "verified-aot"
	if err := writeFirPIDFile(4242, 1, key, "/cache.aot"); err != nil {
		t.Fatal(err)
	}
	d := &FirDaemon{sourcesHash: key, aotCachePath: "/cache.aot", aotPID: 4242}
	d.markAOTVerified()
	data, err := os.ReadFile(firAOTPath(key))
	if err != nil || !strings.Contains(string(data), `"verified":true`) || !strings.Contains(string(data), `"pid":4242`) {
		t.Fatalf("metadata = %q, %v", data, err)
	}
}
