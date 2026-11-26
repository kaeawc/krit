package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerThirdPartyResourceAliasRules() {
	base := func(id, desc string) ResourceImportAliasRule {
		return ResourceImportAliasRule{BaseRule: BaseRule{RuleName: id, RuleSetName: moshiRuleSet, Sev: string(api.SeverityError), Desc: desc}}
	}
	full := &FullyQualifiedResourceRule{base("FullyQualifiedResource", "Resources should use an import alias instead of being fully qualified.")}
	missing := &MissingResourceImportAliasRule{base("MissingResourceImportAlias", "Missing import alias for R class.")}
	wrong := &WrongResourceImportAliasRule{base("WrongResourceImportAlias", "Wrong import alias for this R class.")}
	for _, rule := range []struct {
		base  *ResourceImportAliasRule
		impl  any
		nodes []string
		check func(*api.Context)
	}{
		{&full.ResourceImportAliasRule, full, []string{"navigation_expression"}, full.checkFullyQualified},
		{&missing.ResourceImportAliasRule, missing, []string{"import_list"}, missing.checkMissingAlias},
		{&wrong.ResourceImportAliasRule, wrong, []string{"import_list"}, wrong.checkWrongAlias},
	} {
		r := rule.base
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
			NodeTypes:      rule.nodes,
			Languages:      []scanner.Language{scanner.LangKotlin},
			FileGate:       resourceRFileGate,
			Confidence:     r.Confidence(),
			Implementation: rule.impl,
			Check:          rule.check,
			LintIssue:      true,
			DefaultActive:  false,
			OptInReason:    api.OptInReasonAndroidOnly,
			Maturity:       api.MaturityExperimental,
		})
	}
}
