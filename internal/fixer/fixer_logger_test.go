package fixer

import (
	"log/slog"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestApplyFixesDroppedOverlapLogsViaPkgLog drives the same overlap
// scenario as TestApplyFixesDetailedColumns_ReportsDroppedOverlapLikeSlicePath
// and verifies the per-fix and summary warnings route through the
// package Logger (not the standard log package).
func TestApplyFixesDroppedOverlapLogsViaPkgLog(t *testing.T) {
	prev := pkgLog
	cap := logger.NewCapture(slog.LevelDebug)
	SetLogger(cap)
	t.Cleanup(func() { SetLogger(prev) })

	path := writeTestFile(t, "abcdefghij")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartByte:   2,
			EndByte:     6,
			Replacement: "XXXX",
			ByteMode:    true,
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartByte:   4,
			EndByte:     8,
			Replacement: "YYYY",
			ByteMode:    true,
		}),
	}

	if _, err := ApplyFixesDetailed(t.Context(), path, findings, "", false); err != nil {
		t.Fatalf("ApplyFixesDetailed: %v", err)
	}

	if !cap.HasMessage("dropped overlapping fix") {
		t.Errorf("expected per-fix warning record, got %+v", cap.Records())
	}
	if !cap.HasMessage("fixes dropped due to overlapping conflicts") {
		t.Errorf("expected summary warning record, got %+v", cap.Records())
	}
	warns := cap.FilterLevel(slog.LevelWarn)
	if len(warns) < 2 {
		t.Errorf("expected at least 2 Warn records (per-fix + summary), got %d", len(warns))
	}
}
