package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const customPatternRulesKey = "customRules"

type customPatternRule struct {
	FlatDispatchBase
	BaseRule
	pattern            *regexp.Regexp
	message            string
	match              string
	ignorePlaceholders bool
}

func registerCustomPatternRulesFromConfig(cfg *config.Config) {
	removeCustomPatternRules()
	if cfg == nil {
		return
	}
	raw, ok := cfg.Data()[customPatternRulesKey]
	if !ok {
		return
	}
	items, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, item := range items {
		spec, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		rule := customPatternRuleFromSpec(spec)
		if rule == nil {
			continue
		}
		api.Register(rule)
		if !rule.DefaultActive {
			DefaultInactive[rule.ID] = true
		} else {
			delete(DefaultInactive, rule.ID)
		}
	}
}

func removeCustomPatternRules() {
	dst := api.Registry[:0]
	for _, r := range api.Registry {
		if _, ok := r.Implementation.(*customPatternRule); ok {
			delete(DefaultInactive, r.ID)
			continue
		}
		dst = append(dst, r)
	}
	api.Registry = dst
}

func customPatternRuleFromSpec(spec map[string]interface{}) *api.Rule {
	id := stringField(spec, "id")
	patternText := stringField(spec, "pattern")
	message := stringField(spec, "message")
	if id == "" || patternText == "" || message == "" {
		return nil
	}
	pattern, err := regexp.Compile(patternText)
	if err != nil {
		return nil
	}
	category := stringField(spec, "ruleset")
	if category == "" {
		category = "custom"
	}
	description := stringField(spec, "description")
	if description == "" {
		description = "Config-defined pattern rule."
	}
	severity := api.Severity(stringField(spec, "severity"))
	if severity == "" {
		severity = api.SeverityWarning
	}
	match := stringField(spec, "match")
	if match == "" {
		match = "nodeText"
	}
	nodeTypes := stringListField(spec, "nodeTypes")
	if len(nodeTypes) == 0 {
		nodeTypes = []string{"string_literal", "line_string_literal", "multi_line_string_literal"}
	}
	confidence := floatField(spec, "confidence", 0.75)
	defaultActive := boolField(spec, "defaultActive", true)
	impl := &customPatternRule{
		BaseRule:           BaseRule{RuleName: id, RuleSetName: category, Sev: string(severity), Desc: description},
		pattern:            pattern,
		message:            message,
		match:              match,
		ignorePlaceholders: boolField(spec, "ignorePlaceholders", false),
	}
	return &api.Rule{
		ID:             id,
		Category:       category,
		Description:    description,
		Sev:            severity,
		NodeTypes:      nodeTypes,
		Languages:      languageListField(spec, "languages"),
		Confidence:     confidence,
		DefaultActive:  defaultActive,
		Implementation: impl,
		Check: func(ctx *api.Context) {
			impl.check(ctx)
		},
	}
}

func (r *customPatternRule) check(ctx *api.Context) {
	if r == nil || r.pattern == nil {
		return
	}
	candidate := ctx.File.FlatNodeText(ctx.Idx)
	if r.match == "stringLiteralBody" {
		body, ok := kotlinStringLiteralBody(candidate)
		if !ok {
			return
		}
		candidate = body
	}
	candidate = strings.TrimSpace(candidate)
	if r.ignorePlaceholders && secretLooksLikePlaceholder(candidate) {
		return
	}
	if !r.pattern.MatchString(candidate) {
		return
	}
	ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1, r.message)
}

func stringField(m map[string]interface{}, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func stringListField(m map[string]interface{}, key string) []string {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

func boolField(m map[string]interface{}, key string, def bool) bool {
	if b, ok := m[key].(bool); ok {
		return b
	}
	return def
}

func floatField(m map[string]interface{}, key string, def float64) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	}
	return def
}

func languageListField(m map[string]interface{}, key string) []scanner.Language {
	names := stringListField(m, key)
	if len(names) == 0 {
		return nil
	}
	out := make([]scanner.Language, 0, len(names))
	for _, name := range names {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "kotlin":
			out = append(out, scanner.LangKotlin)
		case "java":
			out = append(out, scanner.LangJava)
		case "xml":
			out = append(out, scanner.LangXML)
		case "gradle":
			out = append(out, scanner.LangGradle)
		case "version-catalog":
			out = append(out, scanner.LangVersionCatalog)
		}
	}
	return out
}
