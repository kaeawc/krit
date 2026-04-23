package oracle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

func TestConfiguredKritTypesShards(t *testing.T) {
	t.Setenv("KRIT_TYPES_SHARDS", "")
	if got := configuredKritTypesShards(10); got != 1 {
		t.Fatalf("unset shards = %d, want 1", got)
	}

	t.Setenv("KRIT_TYPES_SHARDS", "4")
	if got := configuredKritTypesShards(10); got != 4 {
		t.Fatalf("configured shards = %d, want 4", got)
	}

	t.Setenv("KRIT_TYPES_SHARDS", "8")
	if got := configuredKritTypesShards(3); got != 3 {
		t.Fatalf("capped shards = %d, want 3", got)
	}

	t.Setenv("KRIT_TYPES_SHARDS", "auto")
	if got := configuredKritTypesShards(10); got != 1 {
		t.Fatalf("unsupported auto shards = %d, want 1", got)
	}
}

func TestShouldUseOneShotMissAnalysisDefaultsToParallelOneShot(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_DAEMON_POOL", "")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	got, reason := shouldUseOneShotMissAnalysis(1)
	if !got {
		t.Fatalf("default miss analysis should use one-shot")
	}
	if !strings.Contains(reason, "default parallel one-shot") {
		t.Fatalf("reason = %q, want default parallel one-shot", reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsDaemonOptIn(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "on")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	got, reason := shouldUseOneShotMissAnalysis(1)
	if got {
		t.Fatalf("KRIT_DAEMON_CACHE=on should use daemon path, reason=%q", reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsDaemonPoolOptIn(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	got, reason := shouldUseOneShotMissAnalysis(2)
	if got {
		t.Fatalf("daemon pool should use daemon path, reason=%q", reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsExplicitOff(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "off")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "0")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	got, reason := shouldUseOneShotMissAnalysis(1)
	if !got {
		t.Fatal("KRIT_DAEMON_CACHE=off should force one-shot")
	}
	if reason != "KRIT_DAEMON_CACHE=off" {
		t.Fatalf("reason = %q, want KRIT_DAEMON_CACHE=off", reason)
	}
}

func TestJVMArgsForKritTypesShardCapsActiveProcessors(t *testing.T) {
	old := runtime.GOMAXPROCS(10)
	defer runtime.GOMAXPROCS(old)

	if got := activeProcessorCountForKritTypesShard(1); got != 0 {
		t.Fatalf("single shard active processor count = %d, want 0", got)
	}
	if got := activeProcessorCountForKritTypesShard(4); got != 2 {
		t.Fatalf("active processor count = %d, want ceil(10/4)-1=2", got)
	}

	wantArgs := []string{"-XX:ActiveProcessorCount=2"}
	if got := jvmArgsForKritTypesShard(4); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("jvm args = %#v, want %#v", got, wantArgs)
	}
}

func TestSplitMissesForKAA_EqualCostDeterministic(t *testing.T) {
	paths := []string{"A.kt", "B.kt", "C.kt", "D.kt", "E.kt"}
	got := splitMissesForKAA(paths, 2)
	want := [][]string{{"A.kt", "C.kt", "E.kt"}, {"B.kt", "D.kt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split = %#v, want %#v", got, want)
	}

	got = splitMissesForKAA(paths[:2], 4)
	want = [][]string{{"A.kt"}, {"B.kt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capped split = %#v, want %#v", got, want)
	}
}

func TestSplitMissesForKAA_CostBalancedDeterministic(t *testing.T) {
	dir := t.TempDir()
	heavy := writeShardCostFile(t, dir, "heavy.kt", strings.Repeat("x", 1000))
	medium := writeShardCostFile(t, dir, "medium.kt", strings.Repeat("x", 700))
	smallA := writeShardCostFile(t, dir, "small-a.kt", strings.Repeat("x", 200))
	smallB := writeShardCostFile(t, dir, "small-b.kt", strings.Repeat("x", 100))

	got := splitMissesForKAA([]string{smallA, heavy, smallB, medium}, 2)
	want := [][]string{{heavy}, {medium, smallA, smallB}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cost-balanced split = %#v, want %#v", got, want)
	}
}

func TestEstimateKAAMissCostWeightsCallTokens(t *testing.T) {
	dir := t.TempDir()
	plain := writeShardCostFile(t, dir, "plain.kt", strings.Repeat("x", 600))
	callHeavy := writeShardCostFile(t, dir, "calls.kt", strings.Repeat("f()\n", 4))

	plainCost := estimateKAAMissCost(plain)
	callCost := estimateKAAMissCost(callHeavy)
	if callCost.CallTokens != 4 {
		t.Fatalf("call tokens = %d, want 4", callCost.CallTokens)
	}
	if callCost.Cost <= plainCost.Cost {
		t.Fatalf("expected call-heavy file to outrank plain file, call=%+v plain=%+v", callCost, plainCost)
	}
}

func TestMergeOracleData(t *testing.T) {
	a := &OracleData{
		Version:       1,
		KotlinVersion: "2.3.20",
		Files: map[string]*OracleFile{
			"/tmp/A.kt": {Package: "a"},
		},
		Dependencies: map[string]*OracleClass{
			"kotlin.Any":  {FQN: "kotlin.Any", Kind: "class"},
			"kotlin.Unit": {FQN: "kotlin.Unit", Kind: "first"},
		},
	}
	b := &OracleData{
		Version: 1,
		Files: map[string]*OracleFile{
			"/tmp/B.kt": {Package: "b"},
		},
		Dependencies: map[string]*OracleClass{
			"kotlin.Unit":   {FQN: "kotlin.Unit", Kind: "second"},
			"kotlin.String": {FQN: "kotlin.String", Kind: "class"},
		},
	}

	got := mergeOracleData(a, b)
	if got.KotlinVersion != "2.3.20" {
		t.Fatalf("kotlinVersion = %q", got.KotlinVersion)
	}
	if got.Files["/tmp/A.kt"] == nil || got.Files["/tmp/B.kt"] == nil {
		t.Fatalf("merged files missing: %#v", got.Files)
	}
	if got.Dependencies["kotlin.Unit"].Kind != "second" {
		t.Fatalf("later dependency did not win: %#v", got.Dependencies["kotlin.Unit"])
	}
	if got.Dependencies["kotlin.Any"] == nil || got.Dependencies["kotlin.String"] == nil {
		t.Fatalf("merged dependencies missing: %#v", got.Dependencies)
	}
}

func TestMergeCacheDeps(t *testing.T) {
	a := &CacheDepsFile{
		Version:       1,
		Approximation: "symbol-resolved-sources",
		Files: map[string]*CacheDepsEntry{
			"/tmp/A.kt": {DepPaths: []string{"/tmp/DepA.kt"}},
		},
		Crashed: map[string]string{"/tmp/CrashedA.kt": "boom A"},
	}
	b := &CacheDepsFile{
		Version:       1,
		Approximation: "symbol-resolved-sources",
		Files: map[string]*CacheDepsEntry{
			"/tmp/B.kt": {DepPaths: []string{"/tmp/DepB.kt"}},
		},
		Crashed: map[string]string{"/tmp/CrashedB.kt": "boom B"},
	}

	got := mergeCacheDeps(a, b)
	if got == nil {
		t.Fatal("mergeCacheDeps returned nil")
	}
	if got.Approximation != "symbol-resolved-sources" {
		t.Fatalf("approximation = %q", got.Approximation)
	}
	if got.Files["/tmp/A.kt"] == nil || got.Files["/tmp/B.kt"] == nil {
		t.Fatalf("merged deps files missing: %#v", got.Files)
	}
	if got.Crashed["/tmp/CrashedA.kt"] == "" || got.Crashed["/tmp/CrashedB.kt"] == "" {
		t.Fatalf("merged crashed markers missing: %#v", got.Crashed)
	}
}

func TestRunKritTypesCachedShardedWithRunner_MergesOutputs(t *testing.T) {
	misses := []string{"/tmp/A.kt", "/tmp/B.kt", "/tmp/C.kt", "/tmp/D.kt"}
	var mu sync.Mutex
	groupsByFirst := map[string][]string{}

	runner := func(_ string, _ []string, missListPath, freshOutPath, depsOutPath string, _ bool, _ perf.Tracker) error {
		data, err := os.ReadFile(missListPath)
		if err != nil {
			return err
		}
		lines := nonEmptyLines(string(data))
		if len(lines) == 0 {
			return errors.New("empty shard")
		}
		mu.Lock()
		groupsByFirst[lines[0]] = append([]string(nil), lines...)
		mu.Unlock()

		fresh := &OracleData{
			Version:       1,
			KotlinVersion: "2.3.20",
			Files:         map[string]*OracleFile{},
			Dependencies:  map[string]*OracleClass{},
		}
		deps := &CacheDepsFile{
			Version:       1,
			Approximation: "symbol-resolved-sources",
			Files:         map[string]*CacheDepsEntry{},
			Crashed:       map[string]string{},
		}
		for _, path := range lines {
			fresh.Files[path] = &OracleFile{Package: "p"}
			fresh.Dependencies["dep."+path] = &OracleClass{FQN: "dep." + path, Kind: "class"}
			deps.Files[path] = &CacheDepsEntry{
				DepPaths:    []string{path + ".dep"},
				PerFileDeps: map[string]*OracleClass{"dep." + path: &OracleClass{FQN: "dep." + path, Kind: "class"}},
			}
		}
		if err := writeOracleJSON(freshOutPath, fresh); err != nil {
			return err
		}
		body, err := json.Marshal(deps)
		if err != nil {
			return err
		}
		return os.WriteFile(depsOutPath, body, 0o644)
	}

	fresh, deps, err := runKritTypesCachedShardedWithRunner("krit-types.jar", nil, misses, 2, false, perf.New(true), runner)
	if err != nil {
		t.Fatalf("sharded run failed: %v", err)
	}
	if len(fresh.Files) != len(misses) {
		t.Fatalf("fresh files = %d, want %d", len(fresh.Files), len(misses))
	}
	if deps == nil || len(deps.Files) != len(misses) {
		t.Fatalf("deps files = %#v, want %d entries", deps, len(misses))
	}

	wantA := []string{"/tmp/A.kt", "/tmp/C.kt"}
	wantB := []string{"/tmp/B.kt", "/tmp/D.kt"}
	if !reflect.DeepEqual(groupsByFirst["/tmp/A.kt"], wantA) || !reflect.DeepEqual(groupsByFirst["/tmp/B.kt"], wantB) {
		t.Fatalf("unexpected shard groups: %#v", groupsByFirst)
	}
}

func TestRunKritTypesCachedShardedWithRunner_ReturnsShardError(t *testing.T) {
	runner := func(_ string, _ []string, missListPath, freshOutPath, depsOutPath string, _ bool, _ perf.Tracker) error {
		data, err := os.ReadFile(missListPath)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "B.kt") {
			return errors.New("simulated shard failure")
		}
		if err := writeOracleJSON(freshOutPath, &OracleData{
			Version:      1,
			Files:        map[string]*OracleFile{},
			Dependencies: map[string]*OracleClass{},
		}); err != nil {
			return err
		}
		body, err := json.Marshal(&CacheDepsFile{Version: 1, Files: map[string]*CacheDepsEntry{}})
		if err != nil {
			return err
		}
		return os.WriteFile(depsOutPath, body, 0o644)
	}

	_, _, err := runKritTypesCachedShardedWithRunner("krit-types.jar", nil, []string{"A.kt", "B.kt"}, 2, false, perf.New(false), runner)
	if err == nil {
		t.Fatal("expected shard error")
	}
	if !strings.Contains(err.Error(), "krit-types shard") || !strings.Contains(err.Error(), "simulated shard failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeShardCostFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
