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
// per-call work, not cumulative-since-daemon-startup. First call
// over a 3-file fixture should populate misses; the second call
// over the same content should be all hits.
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
	// The second call's delta should not include the first call's
	// misses (proves ParseHits/ParseMisses are per-call deltas, not
	// cumulative). On a warm cache we expect hits to dominate.
	if second.Stats.ParseMisses >= first.Stats.ParseMisses {
		t.Errorf("second call's ParseMisses (%d) should be lower than the first's (%d) once the cache is warm — looks like cumulative, not delta",
			second.Stats.ParseMisses, first.Stats.ParseMisses)
	}
	if second.Stats.ParseHits == 0 {
		t.Errorf("second call should record parse hits against the warm cache; got Stats=%+v", second.Stats)
	}
}
