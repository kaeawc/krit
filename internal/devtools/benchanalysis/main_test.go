//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFlattenTimingsAndAttr(t *testing.T) {
	entries := []timing{{Name: "indexPhaseRun", DurationMs: 10, Children: []timing{
		{Name: "typeOracle", DurationMs: 7, Children: []timing{
			{Name: "jvmAnalyze", DurationMs: 6, Children: []timing{
				{Name: "oracleBackend", Attributes: map[string]string{"backend": "fir", "jar": "krit-fir.jar"}},
			}},
		}},
	}}, {Name: "indexPhaseRun", DurationMs: 5}}
	phases, attrs := map[string]int64{}, map[string]string{}
	flattenTimings(entries, "", phases, attrs)
	if phases["indexPhaseRun"] != 15 || phases["indexPhaseRun/typeOracle/jvmAnalyze"] != 6 {
		t.Fatalf("phases = %v", phases)
	}
	if got := attr(attrs, "oracleBackend", "backend"); got != "fir" {
		t.Fatalf("backend attr = %q", got)
	}
}

func TestCheckModeVerifiesBackend(t *testing.T) {
	firOracle := Run{Attributes: map[string]string{"indexPhaseRun/typeOracle/jvmAnalyze/oracleBackend.backend": "fir"}}
	otherBackend := Run{Attributes: map[string]string{"indexPhaseRun/typeOracle/jvmAnalyze/oracleBackend.backend": "other"}}
	none := Run{Attributes: map[string]string{}}
	firOK := Run{Attributes: map[string]string{
		"a/oracleBackend.backend": "fir", "firCheckAndCollect/firCheck/firCheckOutcome.status": "ok",
	}}
	cases := []struct {
		mode   string
		run    Run
		reject bool
	}{
		{"fir-oracle", firOracle, false},
		{"fir-oracle", otherBackend, true}, // asked for FIR, another backend ran
		{"fir-oracle", none, true},         // oracle silently skipped
		{"structural", none, false},
		{"structural", firOracle, true},
		{"fir-checkers", firOK, false},
		{"fir-checkers", Run{Attributes: map[string]string{"a/oracleBackend.backend": "fir"}}, true},
	}
	for _, c := range cases {
		if got := checkMode(modes[c.mode], c.run) != ""; got != c.reject {
			t.Errorf("%s: rejected = %v, want %v (%s)", c.mode, got, c.reject, checkMode(modes[c.mode], c.run))
		}
	}
}

func TestFindingsChecksumIgnoresOrder(t *testing.T) {
	a := json.RawMessage(`{"file":"/w/one/A.kt","line":1,"column":2,"rule":"R","message":"m"}`)
	b := json.RawMessage(`{"file":"/w/one/B.kt","line":3,"column":4,"rule":"R","message":"see /w/one/A.kt"}`)
	if findingsChecksum([]json.RawMessage{a, b}, "/w/one") != findingsChecksum([]json.RawMessage{b, a}, "/w/one") {
		t.Fatal("checksum depends on finding order")
	}
	if findingsChecksum([]json.RawMessage{a}, "/w/one") == findingsChecksum([]json.RawMessage{a, b}, "/w/one") {
		t.Fatal("checksum ignores a finding")
	}
	// The same findings from a corpus copy at another path must match.
	a2 := json.RawMessage(`{"file":"/w/two/A.kt","line":1,"column":2,"rule":"R","message":"m"}`)
	b2 := json.RawMessage(`{"file":"/w/two/B.kt","line":3,"column":4,"rule":"R","message":"see /w/two/A.kt"}`)
	if findingsChecksum([]json.RawMessage{a, b}, "/w/one") != findingsChecksum([]json.RawMessage{a2, b2}, "/w/two") {
		t.Fatal("checksum depends on the corpus location")
	}
}

func TestStatsPercentiles(t *testing.T) {
	var runs []Run
	for _, ms := range []int64{50, 10, 40, 20, 30} {
		runs = append(runs, Run{WallMs: ms})
	}
	st := stats(runs)
	if st.N != 5 || st.Median != 30 || st.P90 != 50 || st.Min != 10 || st.Max != 50 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestEditText(t *testing.T) {
	if got := editText("body", 2); got != "\n// krit-bench body edit 2\n" {
		t.Errorf("body edit = %q", got)
	}
	if got := editText("abi", 3); got != "\nfun kritBenchAbiEdit3(): Int = 3\n" {
		t.Errorf("abi edit = %q", got)
	}
}

func TestCopyTreeSkipsCachesAndBuildOutputs(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "copy")
	for _, rel := range []string{"src/A.kt", ".krit/types.json", "app/build/out.class", "settings.gradle.kts"} {
		path := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]bool{"src/A.kt": true, "settings.gradle.kts": true, ".krit/types.json": false, "app/build/out.class": false} {
		_, err := os.Stat(filepath.Join(dst, rel))
		if (err == nil) != want {
			t.Errorf("%s copied = %v, want %v", rel, err == nil, want)
		}
	}
}

func TestKotlinVersionReadsBuildScript(t *testing.T) {
	script := filepath.Join(t.TempDir(), "build.gradle.kts")
	if err := os.WriteFile(script, []byte("plugins {}\nval kotlinVersion = \"2.4.20\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := kotlinVersion(script); got != "2.4.20" {
		t.Fatalf("kotlinVersion = %q", got)
	}
}

func warmBundleHit(state string, measured bool) Run {
	return Run{State: state, Measured: measured,
		Attributes: map[string]string{"dispatchBundleLoad.outcome": "hit", "dispatchPath.path": "cacheHitFullBundle"},
		Caches:     map[string]CacheIO{"findings-bundle-cache": {Hits: 1}}}
}

func TestCheckModeAcceptsWarmBundleHit(t *testing.T) {
	for _, mode := range []string{"fir-oracle"} {
		for _, state := range []string{"warm-disk", "warm-daemon"} {
			if got := checkMode(modes[mode], warmBundleHit(state, true)); got != "" {
				t.Errorf("%s/%s rejected: %s", mode, state, got)
			}
		}
	}
	r := warmBundleHit("warm-disk", true)
	if got := oracleSkipReason(r); got != "findings-bundle-hit" {
		t.Errorf("skip reason = %q", got)
	}
	// fir-checkers still needs the checker pass to succeed.
	r.Attributes["firCheck/firCheckOutcome.status"] = "ok"
	if got := checkMode(modes["fir-checkers"], r); got != "" {
		t.Errorf("fir-checkers bundle hit rejected: %s", got)
	}
}

func TestCheckModeRejectsOracleMissingWithoutCacheEvidence(t *testing.T) {
	cases := map[string]Run{
		"cold no oracle":              {State: "cold", Measured: true, Attributes: map[string]string{}},
		"cold claiming a bundle hit":  warmBundleHit("cold", true),
		"warm, missing jar, no cache": {State: "warm-disk", Measured: true, Attributes: map[string]string{}, Caches: map[string]CacheIO{"findings-bundle-cache": {Misses: 1}}},
		"fill run with a bundle hit":  warmBundleHit("warm-disk", false),
		"clean baseline":              warmBundleHit("", false),
	}
	for name, r := range cases {
		if got := checkMode(modes["fir-oracle"], r); got == "" {
			t.Errorf("%s: accepted, want rejection", name)
		}
	}
}

func TestExecRunRecordsOracleSkip(t *testing.T) {
	report := `{"durationMs":5,"findings":[],"rules":[],"caches":[{"name":"findings-bundle-cache","stats":{"hits":1,"misses":0}}],` +
		`"perfTiming":[{"name":"dispatchBundleLoad","attributes":{"outcome":"hit"}},{"name":"dispatchPath","attributes":{"path":"cacheHitFullBundle"}}]}`
	bin := filepath.Join(t.TempDir(), "fake-krit")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat <<'EOF'\n"+report+"\nEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	warm := execRun(bin, t.TempDir(), nil, os.Environ(), modes["fir-oracle"], Run{State: "warm-disk", Measured: true})
	if warm.Rejected != "" || warm.OracleSkip != "findings-bundle-hit" {
		t.Fatalf("warm: rejected=%q skip=%q", warm.Rejected, warm.OracleSkip)
	}
	cold := execRun(bin, t.TempDir(), nil, os.Environ(), modes["fir-oracle"], Run{State: "cold", Measured: true})
	if cold.Rejected == "" {
		t.Fatal("cold run without the oracle was accepted")
	}
}

func TestSummaryListsRunKindOracleAndRejections(t *testing.T) {
	d := &DaemonUsage{CPUMs: 4000, PeakRSSKB: 3 * 1024, PIDs: []int{9}}
	ok := true
	runs := []Run{
		{Mode: "fir-oracle", State: "warm-disk", Measured: true, Index: 1, WallMs: 10, OracleSkip: "findings-bundle-hit", Daemon: d, Verified: &ok},
		{Mode: "fir-oracle", State: "warm-disk", Measured: true, Index: 2, Rejected: "no oracleBackend in the perf report"},
	}
	out := summarize(runs, options{modes: []string{"fir-oracle"}, states: []string{"warm-disk"}})
	for _, want := range []string{"run kind", "| warm-disk | 1/2 |", "skipped: findings-bundle-hit (1)", "| 4000 | 3 |", "Rejected runs", "fir-oracle/warm-disk run 2"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestCheckDaemonUsedRejectsInProcessFallback(t *testing.T) {
	fell := Run{State: "warm-daemon", Warnings: []string{"warning: krit daemon unavailable (timeout); falling back to in-process"}}
	if checkDaemonUsed(fell) == "" {
		t.Error("in-process fallback accepted as a warm-daemon run")
	}
	fell.State = "warm-disk"
	if checkDaemonUsed(fell) != "" {
		t.Error("warm-disk run rejected for daemon fallback")
	}
}

func TestIsDaemonFallbackMatchesEveryVariant(t *testing.T) {
	for _, w := range []string{
		"warning: krit daemon unavailable (timeout); falling back to in-process",
		"warning: krit daemon call failed (EOF); falling back to in-process",
		"warning: krit daemon rejected request (bad); falling back to in-process. Restart it with: krit daemon restart",
		"warning: krit daemon clear-cache failed (x); falling back to in-process",
	} {
		r := Run{State: "warm-daemon", Warnings: []string{w}}
		if checkDaemonUsed(r) == "" {
			t.Errorf("not rejected: %s", w)
		}
	}
	for _, w := range []string{"warning: jar predates the flag", "warning: watch backend unavailable; falling back to fsnotify"} {
		if checkDaemonUsed(Run{State: "warm-daemon", DaemonServed: true, Warnings: []string{w}}) != "" {
			t.Errorf("unrelated warning rejected: %s", w)
		}
	}
}

func TestCumulativeCacheHitsAreNeverEvidence(t *testing.T) {
	// A daemon-state run whose only evidence is Hits > 0 (a lifetime counter)
	// is rejected, whether or not the count is stale.
	r := Run{State: "warm-daemon", Measured: true, Attributes: map[string]string{},
		Caches: map[string]CacheIO{"findings-bundle-cache": {Hits: 3}, "oracle-cache": {Hits: 2}}}
	if checkMode(modes["fir-oracle"], r) == "" || oracleSkipReason(r) != "" {
		t.Fatal("cumulative counters accepted as evidence")
	}
}

func TestCacheDeltaIsPerRun(t *testing.T) {
	if d := cacheDelta(CacheIO{Hits: 3, Misses: 1}, CacheIO{Hits: 3, Misses: 1}); d != (CacheIO{}) {
		t.Errorf("stale delta = %+v", d)
	}
	if d := cacheDelta(CacheIO{Hits: 4, Misses: 1}, CacheIO{Hits: 3, Misses: 1}); d.Hits != 1 {
		t.Errorf("delta = %+v", d)
	}
	if d := cacheDelta(CacheIO{Hits: 1}, CacheIO{Hits: 5}); d.Hits != 1 {
		t.Errorf("restart delta = %+v", d)
	}
}

func TestEditStatesAndPartialBundlesNeverSkipOracle(t *testing.T) {
	for _, state := range []string{"body-edit", "abi-edit"} {
		for _, path := range []string{"cacheHitDeltaBundle", "cacheHitAffectedSetBundle", "cacheHitFullBundle"} {
			r := Run{State: state, Measured: true, Attributes: map[string]string{
				"dispatchBundleLoad.outcome": "hit", "dispatchPath.path": path}}
			if checkMode(modes["fir-oracle"], r) == "" {
				t.Errorf("%s/%s accepted without the oracle", state, path)
			}
		}
	}
	for _, state := range []string{"warm-disk", "warm-daemon"} {
		r := Run{State: state, Measured: true, Attributes: map[string]string{
			"dispatchBundleLoad.outcome": "hit", "dispatchPath.path": "cacheHitDeltaBundle"}}
		if checkMode(modes["fir-oracle"], r) == "" {
			t.Errorf("%s accepted a delta bundle hit", state)
		}
	}
}

func TestDaemonStateRequiresResidentOracleSource(t *testing.T) {
	inproc := Run{State: "warm-daemon", Attributes: map[string]string{"a/oracleBackend.backend": "fir"}}
	if checkDaemonUsed(inproc) == "" {
		t.Error("in-process oracle accepted in a daemon state")
	}
	resident := Run{State: "warm-daemon", DaemonServed: true, Attributes: map[string]string{"a/oracleBackend.backend": "fir", "a/oracleBackend.source": "resident-daemon"}}
	if checkDaemonUsed(resident) != "" {
		t.Error("resident oracle rejected")
	}
}

func TestDaemonResponseCacheBundleHitNeedsHitOutcome(t *testing.T) {
	mk := func(state, outcome string) Run {
		attrs := map[string]string{}
		if outcome != "" {
			attrs["bundleHit/bundleOutcome.outcome"] = outcome
		}
		return Run{State: state, Measured: true, Phases: map[string]int64{"bundleHit": 0, "bundleHit/bundleOutcome": 0}, Attributes: attrs}
	}
	if got := checkMode(modes["fir-oracle"], mk("warm-daemon", "hit")); got != "" {
		t.Fatalf("hit rejected: %s", got)
	}
	// The bundleHit scope is also left behind by a lookup that missed.
	for _, outcome := range []string{"miss", ""} {
		if checkMode(modes["fir-oracle"], mk("warm-daemon", outcome)) == "" {
			t.Errorf("outcome %q with no oracle accepted", outcome)
		}
	}
	if checkMode(modes["fir-oracle"], mk("cold", "hit")) == "" {
		t.Fatal("cold run accepted")
	}
}

func TestRejectUnverifiedSkip(t *testing.T) {
	r := Run{OracleSkip: "findings-bundle-hit"}
	rejectUnverifiedSkip(&r, true)
	if r.Rejected != "" {
		t.Fatal("verified skip rejected")
	}
	rejectUnverifiedSkip(&r, false)
	if r.Rejected == "" {
		t.Fatal("unverified skip accepted")
	}
	none := Run{}
	rejectUnverifiedSkip(&none, false)
	if none.Rejected != "" {
		t.Fatal("run without a skip rejected")
	}
}

func TestSummaryShowsNoSampledDaemonAsNA(t *testing.T) {
	ok := true
	runs := []Run{{Mode: "structural", State: "cold", Measured: true, Index: 1, Verified: &ok, Daemon: &DaemonUsage{}}}
	out := summarize(runs, options{modes: []string{"structural"}, states: []string{"cold"}})
	if !strings.Contains(out, "| n/a | n/a |") {
		t.Errorf("empty daemon sample not n/a:\n%s", out)
	}
}

func TestDaemonStateRequiresServedMarker(t *testing.T) {
	if checkDaemonUsed(Run{State: "warm-daemon"}) == "" {
		t.Error("daemon state without the served marker accepted (structural or bundle run, or --no-daemon)")
	}
	if got := checkDaemonUsed(Run{State: "warm-daemon", DaemonServed: true}); got != "" {
		t.Errorf("marked run rejected: %s", got)
	}
	if checkDaemonUsed(Run{State: "warm-disk"}) != "" {
		t.Error("non-daemon state needs no marker")
	}
}

func TestExecRunDetectsServedMarker(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "fake-krit")
	script := "#!/bin/sh\necho '" + daemonServedMarker + "' >&2\necho '{\"durationMs\":1,\"findings\":[],\"rules\":[]}'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	r := execRun(bin, t.TempDir(), nil, os.Environ(), modes["structural"], Run{State: "warm-daemon", Measured: true})
	if !r.DaemonServed || r.Rejected != "" {
		t.Fatalf("served=%v rejected=%q", r.DaemonServed, r.Rejected)
	}
}

func TestCheckExtraArgsRejectsNoDaemonInDaemonStates(t *testing.T) {
	for _, a := range []string{"--no-daemon", "-no-daemon", "--no-daemon=true"} {
		if checkExtraArgs(options{states: []string{"cold", "warm-daemon"}, extraArgs: "--foo " + a}) == nil {
			t.Errorf("%s accepted", a)
		}
	}
	if err := checkExtraArgs(options{states: []string{"cold", "warm-disk"}, extraArgs: "--no-daemon"}); err != nil {
		t.Errorf("non-daemon states rejected: %v", err)
	}
}

func TestDaemonBaselineSurvivesInProcessFallback(t *testing.T) {
	daemon1 := map[string]CacheIO{"findings-bundle-cache": {Hits: 4, Misses: 1}}
	base := nextDaemonBaseline(nil, Run{DaemonServed: true, rawCaches: daemon1})
	// An in-process fallback reports its own tiny counters: ignored.
	base = nextDaemonBaseline(base, Run{rawCaches: map[string]CacheIO{"findings-bundle-cache": {Hits: 0, Misses: 1}}})
	if base["findings-bundle-cache"].Hits != 4 {
		t.Fatalf("baseline overwritten by fallback: %+v", base)
	}
	// The next daemon run (cumulative 5 hits) is exactly one hit.
	next := Run{DaemonServed: true, rawCaches: map[string]CacheIO{"findings-bundle-cache": {Hits: 5, Misses: 1}}}
	if d := cacheDelta(next.rawCaches["findings-bundle-cache"], base["findings-bundle-cache"]); d.Hits != 1 || d.Misses != 0 {
		t.Fatalf("delta = %+v", d)
	}
	// A restarted daemon (counters below the baseline) is taken as-is.
	if d := cacheDelta(CacheIO{Hits: 1}, base["findings-bundle-cache"]); d.Hits != 1 {
		t.Fatalf("restart delta = %+v", d)
	}
}

func TestDaemonIdentityDistinguishesRestartsAndRediffs(t *testing.T) {
	work, proc := t.TempDir(), t.TempDir()
	_ = os.MkdirAll(filepath.Join(work, ".krit"), 0o755)
	write := func(pid, start string) {
		_ = os.WriteFile(filepath.Join(work, ".krit", "daemon.pid"), []byte(pid+"\n"), 0o644)
		_ = os.MkdirAll(filepath.Join(proc, pid), 0o755)
		_ = os.WriteFile(filepath.Join(proc, pid, "stat"), []byte(pid+" (krit) S 1 1 1 0 -1 0 0 0 0 0 1 1 0 0 20 0 5 0 "+start+" 1 1\n"), 0o644)
	}
	if daemonIdentity(proc, work) != "" {
		t.Fatal("identity without a pid file")
	}
	write("42", "100")
	first := daemonIdentity(proc, work)
	write("42", "200") // same PID reused by a new process
	second := daemonIdentity(proc, work)
	write("43", "300")
	third := daemonIdentity(proc, work)
	if first == second || second == third || first == "" {
		t.Fatalf("identities = %q %q %q", first, second, third)
	}
	// A restart whose counters are equal or larger than the old baseline is
	// still diffed against zero once the identity changed.
	r := Run{Caches: map[string]CacheIO{}, rawCaches: map[string]CacheIO{"findings-bundle-cache": {Hits: 9, Misses: 2}}}
	rediffCaches(&r, nil)
	if r.Caches["findings-bundle-cache"] != (CacheIO{Hits: 9, Misses: 2}) {
		t.Fatalf("caches = %+v", r.Caches)
	}
}

func TestStopDaemonUsesPortableStopFirst(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "fake-krit")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	h := &harness{krit: bin, work: work, registry: t.TempDir()}
	h.stopDaemon()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if lines[0] != "serve --stop --root "+work {
		t.Fatalf("first call = %q, want serve --stop --root %s", lines[0], work)
	}
}

func TestDaemonGoneWaitsForPIDAndSocket(t *testing.T) {
	work := t.TempDir()
	_ = os.MkdirAll(filepath.Join(work, ".krit"), 0o755)
	if !daemonGone(work) {
		t.Fatal("empty work reported as having a daemon")
	}
	_ = os.WriteFile(filepath.Join(work, ".krit", "daemon.sock"), nil, 0o644)
	if daemonGone(work) {
		t.Fatal("socket file ignored")
	}
	_ = os.Remove(filepath.Join(work, ".krit", "daemon.sock"))
	_ = os.WriteFile(filepath.Join(work, ".krit", "daemon.pid"), []byte(strconv.Itoa(os.Getpid())), 0o644)
	if daemonGone(work) {
		t.Fatal("live pid ignored")
	}
	// A pid recorded before the stop keeps the daemon "running" after its
	// socket and pid file are gone (serve still flushing on its way out).
	_ = os.Remove(filepath.Join(work, ".krit", "daemon.pid"))
	if !daemonGone(work) {
		t.Fatal("nothing left, still reported running")
	}
	if daemonGone(work, os.Getpid()) {
		t.Fatal("recorded live pid ignored once the pid file is gone")
	}
}

func TestBinaryVCSWithoutStampFallsBack(t *testing.T) {
	if _, _, ok := binaryVCS(filepath.Join(t.TempDir(), "missing")); ok {
		t.Fatal("a missing binary has no VCS stamp")
	}
	notGo := filepath.Join(t.TempDir(), "krit")
	if err := os.WriteFile(notGo, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := binaryVCS(notGo); ok {
		t.Fatal("a non-Go binary has no VCS stamp")
	}
}
