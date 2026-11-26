package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerThirdPartyDaggerCrossFileRules() {
	for _, rule := range []struct {
		id, brief string
		check     func(*api.Context)
	}{
		{daggerDoNotCallProviders, daggerDoNotCallProvidersMessage, emitDaggerProviderCalls},
		{daggerBindsTypeMismatch, daggerBindsTypeMismatchMessage, emitDaggerBindsTypeMismatch},
	} {
		registerDaggerCrossFileRule(rule.id, rule.brief, rule.check)
	}
}

func registerDaggerCrossFileRule(id, brief string, check func(*api.Context)) {
	r := &SlackAnnotationRule{BaseRule: BaseRule{RuleName: id, RuleSetName: moshiRuleSet,
		Sev: string(api.SeverityError), Desc: brief}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
		Needs:     api.NeedsCrossFile | api.NeedsResolver,
		Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		LanguageSupport: map[string]api.LanguageSupport{JavaLanguageSupportKey: {
			Status:   api.LanguageSupportSupported,
			Evidence: []string{"internal/rules/testdata/slack_lint_crossfile/dagger"},
		}},
		Confidence: r.Confidence(), Implementation: r, Check: check,
		LintIssue: true, DefaultActive: false, OptInReason: api.OptInReasonDomainSpecific, Maturity: api.MaturityExperimental,
	})
}
