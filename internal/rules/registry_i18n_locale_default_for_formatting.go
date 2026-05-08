package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerI18nLocaleDefaultForFormattingRules() {
	r := &LocaleGetDefaultForFormattingRule{BaseRule: BaseRule{
		RuleName:    "LocaleGetDefaultForFormatting",
		RuleSetName: "i18n",
		Sev:         "warning",
		Desc:        "Detects Locale.getDefault() passed to a formatter used for persistence or network IO; use Locale.ROOT or Locale.US instead.",
	}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		NodeTypes:      []string{"call_expression"},
		Languages:      []scanner.Language{scanner.LangKotlin},
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          r.check,
		DefaultActive:  true,
	})
}
