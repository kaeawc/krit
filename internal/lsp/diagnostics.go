package lsp

import (
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// FindingColumnsToDiagnostics converts columnar findings to LSP diagnostics.
// Without the document content, finding columns are used as character offsets.
func FindingColumnsToDiagnostics(columns *scanner.FindingColumns) []Diagnostic {
	return FindingColumnsToDiagnosticsInContent(columns, nil)
}

// FindingColumnsToDiagnosticsInContent converts columnar findings to LSP diagnostics and maps their byte
// columns to the UTF-16 character offsets LSP requires, using the content the findings were computed from.
func FindingColumnsToDiagnosticsInContent(columns *scanner.FindingColumns, content []byte) []Diagnostic {
	if columns == nil {
		return nil
	}
	diags := make([]Diagnostic, 0, columns.Len())
	for row := 0; row < columns.Len(); row++ {
		diags = append(diags, rowToDiagnosticInContent(columns, row, content))
	}
	return diags
}

// byteColumnToUTF16 maps a 0-based byte column on a 0-based line to a UTF-16 code unit offset.
func byteColumnToUTF16(content []byte, line, byteCol int) int {
	start := lineStartOffset(content, line)
	end := min(start+byteCol, len(content))
	units := 0
	for off := start; off < end; {
		r, size := utf8.DecodeRune(content[off:end])
		units++
		if r > 0xffff {
			units++
		}
		off += size
	}
	return units + max(0, start+byteCol-end)
}

func rowToDiagnosticInContent(columns *scanner.FindingColumns, row int, content []byte) Diagnostic {
	severity := mapSeverity(columns.SeverityAt(row))

	// LSP lines and characters are 0-based; krit lines and columns are 1-based.
	findingLine := columns.LineAt(row)
	line := uint32(0)
	if findingLine > 0 {
		line = uint32(findingLine - 1)
	}
	findingCol := columns.ColumnAt(row)
	col := uint32(0)
	if findingCol > 0 {
		col = uint32(findingCol - 1)
		if content != nil {
			col = uint32(byteColumnToUTF16(content, int(line), int(col)))
		}
	}

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
