package firchecks

import "github.com/kaeawc/krit/internal/scanner"

// FirFinding is the per-finding JSON shape emitted by krit-fir.
type FirFinding struct {
	Path       string  `json:"path"`
	Line       int     `json:"line"`
	Col        int     `json:"col"`
	Rule       string  `json:"rule"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Confidence float64 `json:"confidence"`
}

// CheckResponse is the JSON envelope returned by krit-fir for a "check" request.
type CheckResponse struct {
	ID        int64             `json:"id"`
	Succeeded int               `json:"succeeded"`
	Skipped   int               `json:"skipped"`
	Findings  []FirFinding      `json:"findings"`
	Crashed   map[string]string `json:"crashed"`
}

// ToScannerFinding converts a FirFinding to a scanner.Finding. The RuleSet
// is set to the FIR category name so SARIF / JSON output treats it the same
// as a tree-sitter finding from that category.
func ToScannerFinding(f FirFinding) scanner.Finding {
	sev := f.Severity
	if sev == "" {
		sev = "warning"
	}
	return scanner.Finding{
		File:       f.Path,
		Line:       f.Line,
		Col:        f.Col,
		RuleSet:    "fir",
		Rule:       f.Rule,
		Severity:   sev,
		Message:    f.Message,
		Confidence: f.Confidence,
	}
}
