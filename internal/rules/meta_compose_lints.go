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

func (r *ComposeM2ApiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeM2Api", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"Names are resolved through imports, not types: a member read on a local variable of an M2 type (colors.primary) is not reported, and a qualified chain counts only as an M2 class or object member (ButtonDefaults.buttonColors()), a MaterialTheme.colors/typography/shapes member, or an extension imported from androidx.compose.material.",
			"allowedM2Apis matches the source name; Lint matches the JVM name, which is mangled for functions with inline-class parameters unless enable-mangling-workaround is set.",
		},
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ComposeM2ApiRule]{
				Name:        "allowedM2Apis",
				Description: "androidx.compose.material API names that are not reported (compose-lints `allowed-m2-apis`).",
				Apply:       func(r *ComposeM2ApiRule, v []string) { r.AllowedM2Apis = v },
			}),
		}}
}

func (r *ComposeItemKeyHashCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeItemKeyHashCode", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated}
}

func (r *ComposeModifierComposedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeModifierComposed", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"The receiver must be provably a Modifier: Modifier, this or the implicit receiver in a Modifier extension, a parameter or local declared as Modifier, or a call chain rooted in one of those.",
		}}
}

func (r *ComposePreviewPublicRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposePreviewPublic", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"Multipreview annotations resolve to top-level annotation classes in the analyzed sources or to the AndroidX and Wear tooling multipreviews; other library multipreviews are not recognized.",
		}}
}

func (r *ComposePreviewNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposePreviewNaming", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"Only top-level annotation classes are checked, and nested multipreviews resolve as for ComposePreviewPublic.",
		}}
}

func (r *ComposeModifierMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeModifierMissing", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"Expression-bodied composables without a declared return type are skipped: their return type needs type inference.",
			"Previews are recognized as for ComposePreviewPublic.",
		},
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ComposeModifierMissingRule]{
				Name:        "contentEmitters",
				Description: "Extra composables known to emit content (compose-lints `content-emitters`).",
				Apply:       func(r *ComposeModifierMissingRule, v []string) { r.ContentEmitters = v },
			}),
			api.StringOption(api.StringOptionSpec[ComposeModifierMissingRule]{
				Name:        "visibilityThreshold",
				Default:     "only_public",
				Description: "Which composables need a Modifier: only_public, public_and_internal or all (compose-lints `visibility-threshold`).",
				Apply:       func(r *ComposeModifierMissingRule, v string) { r.VisibilityThreshold = v },
			}),
		}}
}

func (r *ComposeModifierWithoutDefaultRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeModifierWithoutDefault", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated, FixLevel: "idiomatic"}
}

func (r *ComposeParameterOrderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposeParameterOrder", RuleSet: composeLintsRuleSet, DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated,
		KnownLimitations: []string{
			"A trailing function is a function type, a type alias of one declared in the analyzed sources, or java.lang.Runnable, Callable, Comparator or a java.util.function type; library type aliases of function types are not expanded.",
			"compose-lints' reordering autofix is not ported: moving parameters breaks positional callers.",
		}}
}
