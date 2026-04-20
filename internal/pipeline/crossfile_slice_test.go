package pipeline

import "github.com/kaeawc/krit/internal/scanner"

// ApplySuppression is the slice-based form of applySuppressionColumns.
// Production code calls the columnar form directly in CrossFilePhase.Run;
// this helper exists so existing integration and cross-file tests can keep
// exercising suppression on []scanner.Finding fixtures.
func ApplySuppression(findings []scanner.Finding, files []*scanner.File) []scanner.Finding {
	if len(findings) == 0 {
		return findings
	}
	cols := scanner.CollectFindings(findings)
	kept := applySuppressionColumns(&cols, files)
	return kept.Findings()
}
