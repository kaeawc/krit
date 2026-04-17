package lsp

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// FindingsToDiagnostics converts a slice of krit findings to LSP diagnostics.
func FindingsToDiagnostics(findings []scanner.Finding) []Diagnostic {
	diags := make([]Diagnostic, 0, len(findings))
	for _, f := range findings {
		diags = append(diags, FindingToDiagnostic(f))
	}
	return diags
}

// FindingToDiagnostic converts a single krit finding to an LSP diagnostic.
func FindingToDiagnostic(f scanner.Finding) Diagnostic {
	severity := mapSeverity(f.Severity)

	// LSP lines and characters are 0-based; krit lines are 1-based, cols are 0-based.
	line := uint32(0)
	if f.Line > 0 {
		line = uint32(f.Line - 1)
	}
	col := uint32(f.Col)

	return Diagnostic{
		Range: Range{
			Start: Position{Line: line, Character: col},
			End:   Position{Line: line, Character: col},
		},
		Severity: severity,
		Code:     f.RuleSet + "/" + f.Rule,
		Source:   "krit",
		Message:  f.Message,
	}
}

// mapSeverity converts a krit severity string to an LSP severity int.
// LSP: 1=Error, 2=Warning, 3=Information, 4=Hint
func mapSeverity(sev string) int {
	switch sev {
	case "error":
		return 1
	case "warning":
		return 2
	case "info":
		return 3
	default:
		return 2 // default to warning
	}
}
