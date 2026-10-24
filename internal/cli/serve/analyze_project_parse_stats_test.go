package serve

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// kotlinFixture returns a Kotlin source string ≥ 1KB so the parse
// cache (which skips small files via parseCacheMinFileSize) actually
// participates.
func kotlinFixture(name string) string {
	body := strings.Repeat("    val padding = \"x\"\n", 60)
	return "package demo\n\nclass " + name + " {\n" + body + "}\n"
}

// TestAnalyzeProject_ParseCacheStatsReportPerCallDelta asserts the
// parse-cache hit/miss counts on AnalyzeProjectStats reflect the
// per-call work, not cumulative-since-daemon-startup. First call over
// a 3-file fixture should populate misses; the second call should be
// served by the findings bundle before parse-cache decode.
func TestAnalyzeProject_ParseCacheStatsReportPerCallDelta(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "A.kt", kotlinFixture("A"))
	writeKotlinFile(t, state.root, "B.kt", kotlinFixture("B"))
	writeKotlinFile(t, state.root, "C.kt", kotlinFixture("C"))

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.ParseMisses == 0 {
		t.Errorf("first call should record parse misses; got Stats=%+v", first.Stats)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.Stats.FindingsBundleHit {
		t.Fatalf("second call should hit the findings bundle; got Stats=%+v", second.Stats)
	}
	if second.Stats.ParseHits != 0 || second.Stats.ParseMisses != 0 {
		t.Errorf("bundle hit should bypass parse cache; got Stats=%+v", second.Stats)
	}
}

// TestAnalyzeProject_PreParseBundleHitBeforeDiskSave pins the case that made
// TestAnalyzeProject_ParseCacheStatsReportPerCallDelta flaky. The first call
// writes its findings bundle to disk on the background-save worker but stashes
// it in memory synchronously. A second call that arrives before the disk write
// lands must still be served by the pre-parse bundle shortcut from the
// in-memory copy, not fall through to a full parse that only hits the bundle
// afterwards. The worker is held so the disk write cannot land in between.
func TestAnalyzeProject_PreParseBundleHitBeforeDiskSave(t *testing.T) {
	socket, state := startServerForTest(t)
	release := make(chan struct{})
	// Registered after startServerForTest so it runs before the worker flush.
	t.Cleanup(func() { close(release) })
	state.workspace.EnqueueBackgroundSave(func() { <-release })

	writeKotlinFile(t, state.root, "A.kt", kotlinFixture("A"))
	writeKotlinFile(t, state.root, "B.kt", kotlinFixture("B"))
	writeKotlinFile(t, state.root, "C.kt", kotlinFixture("C"))

	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{NoFir: true}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.Stats.FindingsBundleHit {
		t.Fatalf("second call should hit the findings bundle; got Stats=%+v", second.Stats)
	}
	if second.Stats.ParseHits != 0 || second.Stats.ParseMisses != 0 {
		t.Errorf("bundle still only in memory: the pre-parse shortcut should serve it without parsing; got Stats=%+v", second.Stats)
	}
	if second.Stats.FindingsCount != first.Stats.FindingsCount {
		t.Errorf("bundle hit must replay the same finding count: first=%d second=%d",
			first.Stats.FindingsCount, second.Stats.FindingsCount)
	}
}
