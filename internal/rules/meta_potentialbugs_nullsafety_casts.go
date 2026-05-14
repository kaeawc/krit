// Descriptor metadata for internal/rules/potentialbugs_nullsafety_casts.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CastNullableToNonNullableTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CastNullableToNonNullableType",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
	}
}

func (r *CastToNullableTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CastToNullableType",
		RuleSet:       "potential-bugs",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
		FixLevel:      "semantic",
	}
}

func (r *UnsafeCastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnsafeCast",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UnsafeCastRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *UnsafeCastRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[UnsafeCastRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *UnsafeCastRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}
