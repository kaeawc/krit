package lsp

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

type hoverRuleMeta struct {
	defaultActive bool
	fixLevel      string
}

func formatHoverContent(findings []scanner.Finding) string {
	var sb strings.Builder
	for i, f := range findings {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(formatHoverFinding(f))
	}
	return sb.String()
}

func formatHoverFinding(f scanner.Finding) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s/%s**\n\n", f.RuleSet, f.Rule))
	sb.WriteString(fmt.Sprintf("- Severity: `%s`\n", f.Severity))
	if meta, ok := lookupHoverRuleMeta(f.Rule); ok {
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
	sb.WriteString(fmt.Sprintf("- Finding: %s", f.Message))
	return sb.String()
}

func lookupHoverRuleMeta(ruleName string) (hoverRuleMeta, bool) {
	for _, r := range rules.Registry {
		if r.Name() != ruleName {
			continue
		}
		meta := hoverRuleMeta{
			defaultActive: rules.IsDefaultActive(r.Name()),
		}
		if fr, ok := r.(rules.FixableRule); ok && fr.IsFixable() {
			meta.fixLevel = rules.GetFixLevel(r).String()
		}
		return meta, true
	}
	return hoverRuleMeta{}, false
}
