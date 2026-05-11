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
		daemon.AnalyzeProjectArgs{}, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.Stats.ParseMisses == 0 {
		t.Errorf("first call should record parse misses; got Stats=%+v", first.Stats)
	}

	var second daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.Stats.FindingsBundleHit {
		t.Fatalf("second call should hit the findings bundle; got Stats=%+v", second.Stats)
	}
	if second.Stats.ParseHits != 0 || second.Stats.ParseMisses != 0 {
		t.Errorf("bundle hit should bypass parse cache; got Stats=%+v", second.Stats)
	}
}
