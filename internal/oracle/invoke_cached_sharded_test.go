package oracle

import (
	"strings"
	"testing"
)

func TestShouldUseOneShotMissAnalysisSmallMissDefaultsToDaemon(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_DAEMON_POOL", "")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	// 1-file warm-rerun miss (the dominant warm-cache workload):
	// must route to the persistent daemon. Parallel one-shot can't
	// parallelize a 1-file analysis and the JVM cold-start dominates.
	got, reason := shouldUseOneShotMissAnalysis(1)
	if got {
		t.Fatalf("1 miss should use the persistent daemon, got one-shot (reason=%q)", reason)
	}
}

func TestShouldUseOneShotMissAnalysisLargeMissDefaultsToParallelOneShot(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_DAEMON_POOL", "")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	// Above the threshold parallel one-shot's in-JVM file-worker
	// parallelism wins; preserve the historical default for this
	// regime.
	got, reason := shouldUseOneShotMissAnalysis(daemonPreferredMissThreshold + 1)
	if !got {
		t.Fatalf("large miss set should use parallel one-shot, got daemon (reason=%q)", reason)
	}
	if !strings.Contains(reason, "default parallel one-shot") {
		t.Fatalf("reason = %q, want default parallel one-shot", reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsDaemonOptIn(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "on")
	t.Setenv("KRIT_TYPES_PARALLEL_FILES", "")
	t.Setenv("KRIT_TYPES_SHARDS", "")

	got, reason := shouldUseOneShotMissAnalysis(100)
	if got {
		t.Fatalf("KRIT_DAEMON_CACHE=on should use daemon path, reason=%q", reason)
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
