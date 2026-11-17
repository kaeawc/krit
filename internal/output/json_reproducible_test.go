package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// formatReproducible renders one report with every run-dependent input set.
func formatReproducible(t *testing.T, start time.Time, compact bool) []byte {
	t.Helper()
	cols := scanner.CollectFindings(testColumnFormatterFindings())
	var buf bytes.Buffer
	err := FormatJSONColumnsWithOptions(&buf, &cols, "1.0.0", 2, 3, start,
		[]perf.TimingEntry{{Name: "total", DurationMs: int64(start.Nanosecond() % 997)}},
		nil, nil,
		&cache.Stats{HitRate: 0.5, Cached: 1, Total: 2, LoadDurMs: 7},
		[]cacheutil.NamedCacheStats{{Name: "parse"}},
		&cacheutil.BudgetReport{CapBytes: 1, UsedBytes: 1},
		JSONOptions{Compact: compact, Reproducible: true},
		[]rules.RuleExecutionStat{{Rule: "MaxLen", DurationNs: 42}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFormatJSONReproducibleOmitsRunDependentFields(t *testing.T) {
	for _, compact := range []bool{false, true} {
		out := formatReproducible(t, time.Now().Add(-1500*time.Millisecond), compact)
		var report map[string]any
		if err := json.Unmarshal(out, &report); err != nil {
			t.Fatalf("compact=%v: invalid JSON: %v", compact, err)
		}
		if got := report["durationMs"]; got != float64(0) {
			t.Errorf("compact=%v: durationMs = %v, want 0", compact, got)
		}
		for _, key := range []string{"cache", "caches", "cacheBudget", "perfTiming", "perfRuleStats"} {
			if _, ok := report[key]; ok {
				t.Errorf("compact=%v: reproducible report has %q", compact, key)
			}
		}
		if findings, _ := report["findings"].([]any); len(findings) != len(testColumnFormatterFindings()) {
			t.Errorf("compact=%v: %d findings, want %d", compact, len(findings), len(testColumnFormatterFindings()))
		}
	}
}

func TestFormatJSONReproducibleIsByteIdentical(t *testing.T) {
	a := formatReproducible(t, time.Now().Add(-3*time.Second), false)
	b := formatReproducible(t, time.Now().Add(-17*time.Millisecond), false)
	if !bytes.Equal(a, b) {
		t.Errorf("reproducible reports differ:\n%s\n---\n%s", a, b)
	}
}
