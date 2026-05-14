// Descriptor metadata for internal/rules/style_braces.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *BracesOnIfStatementsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BracesOnIfStatements",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.StringOption(api.StringOptionSpec[BracesOnIfStatementsRule]{
				Name:        "multiLine",
				Default:     "",
				Description: "Brace policy for multi-line if: never/always/necessary/consistent.",
				Apply:       func(r *BracesOnIfStatementsRule, v string) { r.MultiLine = v },
			}),
			api.StringOption(api.StringOptionSpec[BracesOnIfStatementsRule]{
				Name:        "singleLine",
				Default:     "",
				Description: "Brace policy for single-line if: never/always/necessary/consistent.",
				Apply:       func(r *BracesOnIfStatementsRule, v string) { r.SingleLine = v },
			}),
		},
	}
}

func (r *BracesOnWhenStatementsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BracesOnWhenStatements",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
		Options: []api.ConfigOption{
			api.StringOption(api.StringOptionSpec[BracesOnWhenStatementsRule]{
				Name:        "multiLine",
				Default:     "",
				Description: "Brace policy for multi-line when branches.",
				Apply:       func(r *BracesOnWhenStatementsRule, v string) { r.MultiLine = v },
			}),
			api.StringOption(api.StringOptionSpec[BracesOnWhenStatementsRule]{
				Name:        "singleLine",
				Default:     "",
				Description: "Brace policy for single-line when branches.",
				Apply:       func(r *BracesOnWhenStatementsRule, v string) { r.SingleLine = v },
			}),
		},
	}
}

func (r *MandatoryBracesLoopsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MandatoryBracesLoops",
		RuleSet:       "style",
		DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		FixLevel:      "cosmetic",
	}
}
