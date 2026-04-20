package lsp

import "github.com/kaeawc/krit/internal/scanner"

// Test-only slice wrappers over the columnar LSP helpers. Production code
// holds Document.Findings as FindingColumns and calls the *Columns entry
// points directly; these helpers let the existing server_test.go and
// quickfix_preview_test.go fixtures keep passing []scanner.Finding.

// FindingsToDiagnostics converts a slice of findings to LSP diagnostics.
func FindingsToDiagnostics(findings []scanner.Finding) []Diagnostic {
	diags := make([]Diagnostic, 0, len(findings))
	for _, f := range findings {
		diags = append(diags, FindingToDiagnostic(f))
	}
	return diags
}

// FindingToDiagnostic converts a single finding to an LSP diagnostic.
// Preserves the direct severity mapping so unrecognized severity strings
// fall back to "warning" rather than being coerced to "info" by the
// columnar round-trip.
func FindingToDiagnostic(f scanner.Finding) Diagnostic {
	severity := mapSeverity(f.Severity)

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

// findingsToCodeActions is the slice form of findingColumnsToCodeActions.
func findingsToCodeActions(uri string, content string, findings []scanner.Finding) []CodeAction {
	columns := scanner.CollectFindings(findings)
	return findingColumnsToCodeActions(uri, content, &columns)
}

// formatHoverContent renders hover markdown from a slice of findings.
func formatHoverContent(findings []scanner.Finding) string {
	columns := scanner.CollectFindings(findings)
	rows := make([]int, columns.Len())
	for i := range rows {
		rows[i] = i
	}
	return formatHoverColumns(&columns, rows)
}
