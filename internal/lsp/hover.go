package lsp

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// formatSymbolHover renders the markdown block that describes the symbol
// under the cursor using oracle metadata: declaration signature, kind, and
// click-through location. Returns "" when no matching declaration exists.
func formatSymbolHover(idx *oracle.Index, name string) string {
	candidates := idx.FindDeclarationBySimpleName(name)
	if len(candidates) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, decl := range candidates {
		if decl == nil {
			continue
		}
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sig := decl.Signature
		if sig == "" {
			sig = decl.FQN
		}
		fmt.Fprintf(&sb, "```kotlin\n%s\n```\n", sig)
		fmt.Fprintf(&sb, "- FQN: `%s`\n", decl.FQN)
		if decl.Kind != "" {
			fmt.Fprintf(&sb, "- Kind: `%s`\n", decl.Kind)
		}
		if decl.File != "" {
			loc := declLocation(decl)
			fmt.Fprintf(&sb, "- Defined in: [%s](%s)", decl.File, loc.URI)
		}
	}
	return sb.String()
}

type hoverRuleMeta struct {
	defaultActive bool
	fixLevel      string
	descriptor    api.RuleDescriptor
}

// formatHoverColumns renders hover markdown for the rows in columns whose
// indices are listed in rowIndices (typically all rows whose Line == the
// hovered LSP line).
func formatHoverColumns(columns *scanner.FindingColumns, rowIndices []int, cfg *config.Config) string {
	if columns == nil {
		return ""
	}
	var sb strings.Builder
	for i, row := range rowIndices {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(formatHoverRow(columns, row, cfg))
	}
	return sb.String()
}

func formatHoverRow(columns *scanner.FindingColumns, row int, cfg *config.Config) string {
	rule := columns.RuleAt(row)
	ruleSet := columns.RuleSetAt(row)
	currentSeverity := currentRuleSeverity(cfg, ruleSet, rule, columns.SeverityAt(row))
	var sb strings.Builder
	if meta, ok := lookupHoverRuleMeta(rule); ok {
		desc := meta.descriptor
		category := desc.RuleSet
		if category == "" {
			category = ruleSet
		}
		defaultSeverity := desc.Severity
		if defaultSeverity == "" {
			defaultSeverity = columns.SeverityAt(row)
		}
		fmt.Fprintf(&sb, "**%s** - %s - default: %s - current: %s\n\n", rule, category, defaultSeverity, currentSeverity)
		if desc.Description != "" {
			sb.WriteString(desc.Description)
			sb.WriteString("\n\n")
		}
		if desc.DocsURL != "" {
			fmt.Fprintf(&sb, "[Open rule docs](%s)\n\n", desc.DocsURL)
		}
	}
	if sb.Len() == 0 {
		fmt.Fprintf(&sb, "**%s/%s**\n\n", ruleSet, rule)
	}
	fmt.Fprintf(&sb, "- Severity: `%s`\n", currentSeverity)
	if meta, ok := lookupHoverRuleMeta(rule); ok {
		defaultState := "opt-in"
		if meta.defaultActive {
			defaultState = "active"
		}
		fmt.Fprintf(&sb, "- Default state: `%s`\n", defaultState)
		if meta.fixLevel != "" {
			fmt.Fprintf(&sb, "- Auto-fix: `%s`\n", meta.fixLevel)
		} else {
			sb.WriteString("- Auto-fix: unavailable\n")
		}
	}
	fmt.Fprintf(&sb, "- Finding: %s", columns.MessageAt(row))
	return sb.String()
}

func currentRuleSeverity(cfg *config.Config, ruleSet, rule, fallback string) string {
	if cfg == nil {
		return fallback
	}
	return cfg.GetString(ruleSet, rule, "severity", fallback)
}

func lookupHoverRuleMeta(ruleName string) (hoverRuleMeta, bool) {
	for _, r := range api.Registry {
		if r.ID != ruleName {
			continue
		}
		meta := hoverRuleMeta{
			defaultActive: rules.IsDefaultActive(r.ID),
		}
		if desc, ok := rules.MetaForRule(r); ok {
			meta.descriptor = desc
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			meta.fixLevel = lvl.String()
		}
		return meta, true
	}
	return hoverRuleMeta{}, false
}
