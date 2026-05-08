// Descriptor metadata for internal/rules/potentialbugs_nullsafety_bangbang.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *MapGetWithNotNullAssertionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MapGetWithNotNullAssertionOperator",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		FixLevel:      "semantic",
	}
}

func (r *UnsafeCallOnNullableTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnsafeCallOnNullableType",
		RuleSet:       "potential-bugs",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[UnsafeCallOnNullableTypeRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *UnsafeCallOnNullableTypeRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[UnsafeCallOnNullableTypeRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *UnsafeCallOnNullableTypeRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}
