package oracle

import (
	"strings"
	"testing"
)

func TestShouldUseOneShotMissAnalysisSmallMissDefaultsToDaemon(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_DAEMON_POOL", "")

	// 1-file warm-rerun miss (the dominant warm-cache workload):
	// must route to the persistent daemon. A 1-file analysis is fast, so
	// a one-shot JVM's cold-start would dominate.
	got, reason := shouldUseOneShotMissAnalysis(1)
	if got {
		t.Fatalf("1 miss should use the persistent daemon, got one-shot (reason=%q)", reason)
	}
}

func TestShouldUseOneShotMissAnalysisLargeMissDefaultsToOneShot(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "")
	t.Setenv("KRIT_DAEMON_POOL", "")

	// Above the threshold a fresh one-shot JVM takes the miss wave;
	// preserve the historical default for this regime.
	for _, misses := range []int{0, daemonPreferredMissThreshold + 1} {
		got, reason := shouldUseOneShotMissAnalysis(misses)
		if !got {
			t.Fatalf("%d misses should use one-shot, got daemon (reason=%q)", misses, reason)
		}
		if !strings.Contains(reason, "default one-shot") {
			t.Fatalf("reason = %q, want default one-shot", reason)
		}
	}
	if got, reason := shouldUseOneShotMissAnalysis(daemonPreferredMissThreshold); got {
		t.Fatalf("%d misses should still use the daemon, got one-shot (reason=%q)", daemonPreferredMissThreshold, reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsDaemonOptIn(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "on")

	got, reason := shouldUseOneShotMissAnalysis(100)
	if got {
		t.Fatalf("KRIT_DAEMON_CACHE=on should use daemon path, reason=%q", reason)
	}
}

func TestShouldUseOneShotMissAnalysisHonorsExplicitOff(t *testing.T) {
	t.Setenv("KRIT_DAEMON_CACHE", "off")

	got, reason := shouldUseOneShotMissAnalysis(1)
	if !got {
		t.Fatal("KRIT_DAEMON_CACHE=off should force one-shot")
	}
	if reason != "KRIT_DAEMON_CACHE=off" {
		t.Fatalf("reason = %q, want KRIT_DAEMON_CACHE=off", reason)
	}
}
