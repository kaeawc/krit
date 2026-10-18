//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlattenTimingsAndAttr(t *testing.T) {
	entries := []timing{{Name: "indexPhaseRun", DurationMs: 10, Children: []timing{
		{Name: "typeOracle", DurationMs: 7, Children: []timing{
			{Name: "jvmAnalyze", DurationMs: 6, Children: []timing{
				{Name: "oracleBackend", Attributes: map[string]string{"backend": "kaa", "jar": "krit-types.jar"}},
			}},
		}},
	}}, {Name: "indexPhaseRun", DurationMs: 5}}
	phases, attrs := map[string]int64{}, map[string]string{}
	flattenTimings(entries, "", phases, attrs)
	if phases["indexPhaseRun"] != 15 || phases["indexPhaseRun/typeOracle/jvmAnalyze"] != 6 {
		t.Fatalf("phases = %v", phases)
	}
	if got := attr(attrs, "oracleBackend", "backend"); got != "kaa" {
		t.Fatalf("backend attr = %q", got)
	}
}

func TestCheckModeVerifiesBackend(t *testing.T) {
	kaa := Run{Attributes: map[string]string{"indexPhaseRun/typeOracle/jvmAnalyze/oracleBackend.backend": "kaa"}}
	none := Run{Attributes: map[string]string{}}
	firOK := Run{Attributes: map[string]string{
		"a/oracleBackend.backend": "fir", "firCheckAndCollect/firCheck/firCheckOutcome.status": "ok",
	}}
	cases := []struct {
		mode   string
		run    Run
		reject bool
	}{
		{"kaa-oracle", kaa, false},
		{"fir-oracle", kaa, true},  // asked for FIR, KAA ran
		{"kaa-oracle", none, true}, // oracle silently skipped
		{"structural", none, false},
		{"structural", kaa, true},
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
	for _, mode := range []string{"fir-oracle", "kaa-oracle"} {
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
		`"perfTiming":[{"name":"dispatchBundleLoad","attributes":{"outcome":"hit"}}]}`
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
	d := &DaemonUsage{CPUMs: 4000, PeakRSSKB: 3 * 1024}
	runs := []Run{
		{Mode: "fir-oracle", State: "warm-disk", Measured: true, Index: 1, WallMs: 10, OracleSkip: "findings-bundle-hit", Daemon: d},
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
