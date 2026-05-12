package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerPrecompileDuplicateLabelRules() {
	r := &PrecompileDuplicateLabelRule{BaseRule: BaseRule{
		RuleName:    "DuplicateLabel",
		RuleSetName: api.CategoryPrecompile,
		Sev:         string(api.SeverityError),
		Desc:        "Detects repeated constant labels in a subject-bearing `when` expression. Mirrors kotlinc's DUPLICATE_LABEL_IN_WHEN.",
	}}
	api.Register(&api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		NodeTypes:      []string{"when_expression"},
		Languages:      []scanner.Language{scanner.LangKotlin},
		Confidence:     r.Confidence(),
		Level:          api.LevelFunction,
		KotlincAnalog:  "DUPLICATE_LABEL_IN_WHEN",
		Implementation: r,
		Check:          r.check,
		DefaultActive:  false,
	})
}
