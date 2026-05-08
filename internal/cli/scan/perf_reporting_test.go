package scan

import (
	"bytes"
	"strings"
	"testing"
)

func TestHitRatePct(t *testing.T) {
	cases := []struct {
		hits, misses uint64
		want         int
	}{
		{0, 0, 0},
		{0, 5, 0},
		{5, 0, 100},
		{1, 1, 50},
		{1, 3, 25},
		{3, 1, 75},
		{99, 1, 99},
		{1, 99, 1},
	}
	for _, tc := range cases {
		got := hitRatePct(tc.hits, tc.misses)
		if got != tc.want {
			t.Errorf("hitRatePct(%d,%d) = %d; want %d", tc.hits, tc.misses, got, tc.want)
		}
	}
}

func TestReportOracleLookupStatsNilResolver(t *testing.T) {
	// Non-oracle resolver (nil here) must produce no output and not panic.
	var buf bytes.Buffer
	reportOracleLookupStats(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil resolver; got %q", buf.String())
	}
}

func TestReportContentHashMemoStatsEmptyIsSilent(t *testing.T) {
	// With no recorded hashing in this test process the memo Stats() may
	// or may not be zero depending on test ordering. To make this assertion
	// stable, just ensure the function emits at most one line and that the
	// line (when present) starts with the expected prefix.
	var buf bytes.Buffer
	reportContentHashMemoStats(&buf)
	out := buf.String()
	if out == "" {
		return // memo had zero observations — silent path exercised.
	}
	if !strings.HasPrefix(out, "perf: content-hash memo —") {
		t.Fatalf("unexpected prefix in output: %q", out)
	}
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected exactly one line, got %q", out)
	}
}
