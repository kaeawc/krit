// Descriptor metadata for internal/rules/thirdparty_inclusive.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *InclusiveNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            slackInclusiveNaming,
		RuleSet:       moshiRuleSet,
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[InclusiveNamingRule]{
				Name: "blockList",
				Description: "Words a name must not contain, matched against the lower-cased name (slack-lints `block-list`; " +
					"comma-separated entries are split too). Empty, the default, reports nothing.",
				Apply: func(r *InclusiveNamingRule, v []string) { r.BlockList = v },
			}),
		},
	}
}
