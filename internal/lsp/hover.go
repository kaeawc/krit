package lsp

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

type hoverRuleMeta struct {
	defaultActive bool
	fixLevel      string
}

// formatHoverColumns renders hover markdown for the rows in columns whose
// indices are listed in rowIndices (typically all rows whose Line == the
// hovered LSP line).
func formatHoverColumns(columns *scanner.FindingColumns, rowIndices []int) string {
	if columns == nil {
		return ""
	}
	var sb strings.Builder
	for i, row := range rowIndices {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(formatHoverRow(columns, row))
	}
	return sb.String()
}

func formatHoverRow(columns *scanner.FindingColumns, row int) string {
	rule := columns.RuleAt(row)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s/%s**\n\n", columns.RuleSetAt(row), rule))
	sb.WriteString(fmt.Sprintf("- Severity: `%s`\n", columns.SeverityAt(row)))
	if meta, ok := lookupHoverRuleMeta(rule); ok {
		defaultState := "opt-in"
		if meta.defaultActive {
			defaultState = "active"
		}
		sb.WriteString(fmt.Sprintf("- Default state: `%s`\n", defaultState))
		if meta.fixLevel != "" {
			sb.WriteString(fmt.Sprintf("- Auto-fix: `%s`\n", meta.fixLevel))
		} else {
			sb.WriteString("- Auto-fix: unavailable\n")
		}
	}
	sb.WriteString(fmt.Sprintf("- Finding: %s", columns.MessageAt(row)))
	return sb.String()
}

func lookupHoverRuleMeta(ruleName string) (hoverRuleMeta, bool) {
	for _, r := range v2.Registry {
		if r.ID != ruleName {
			continue
		}
		meta := hoverRuleMeta{
			defaultActive: rules.IsDefaultActive(r.ID),
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			meta.fixLevel = lvl.String()
		}
		return meta, true
	}
	return hoverRuleMeta{}, false
}
