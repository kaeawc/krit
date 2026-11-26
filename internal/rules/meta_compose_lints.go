package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *ComposeCompositionLocalUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeCompositionLocalUsage", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"The compositionLocalOf/staticCompositionLocalOf callee must resolve to androidx.compose.runtime; Lint matches the name alone, so it also reports same-named local functions.",
		},
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ComposeCompositionLocalUsageRule]{
				Name:        "allowedCompositionLocals",
				Description: "Property names allowed to declare a CompositionLocal (compose-lints `allowed-composition-locals`).",
				Apply:       func(r *ComposeCompositionLocalUsageRule, v []string) { r.AllowedCompositionLocals = v },
			}),
		}}
}

func (r *ComposeCompositionLocalGetterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeCompositionLocalGetter", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"The compositionLocalOf/staticCompositionLocalOf callee must resolve to androidx.compose.runtime; Lint matches the name alone, so it also reports same-named local functions.",
		}}
}
