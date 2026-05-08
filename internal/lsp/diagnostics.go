package lsp

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// FindingColumnsToDiagnostics converts columnar findings to LSP diagnostics.
func FindingColumnsToDiagnostics(columns *scanner.FindingColumns) []Diagnostic {
	if columns == nil {
		return nil
	}
	diags := make([]Diagnostic, 0, columns.Len())
	for row := 0; row < columns.Len(); row++ {
		diags = append(diags, rowToDiagnostic(columns, row))
	}
	return diags
}

// rowToDiagnostic builds a Diagnostic from a single columnar row.
func rowToDiagnostic(columns *scanner.FindingColumns, row int) Diagnostic {
	severity := mapSeverity(columns.SeverityAt(row))

	// LSP lines and characters are 0-based; krit lines are 1-based, cols are 0-based.
	findingLine := columns.LineAt(row)
	line := uint32(0)
	if findingLine > 0 {
		line = uint32(findingLine - 1)
	}
	col := uint32(columns.ColumnAt(row))

	return Diagnostic{
		Range: Range{
			Start: Position{Line: line, Character: col},
			End:   Position{Line: line, Character: col},
		},
		Severity: severity,
		Code:     columns.RuleSetAt(row) + "/" + columns.RuleAt(row),
		Source:   "krit",
		Message:  columns.MessageAt(row),
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
