// Descriptor metadata for internal/rules/compose.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ComposeColumnRowInScrollableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeColumnRowInScrollable",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeDerivedStateMisuseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeDerivedStateMisuse",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeDisposableEffectMissingDisposeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeDisposableEffectMissingDispose",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeLambdaCapturesUnstableStateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeLambdaCapturesUnstableState",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeLaunchedEffectWithoutKeysRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeLaunchedEffectWithoutKeys",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeModifierBackgroundAfterClipRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeModifierBackgroundAfterClip",
		RuleSet:       "compose",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ComposeModifierClickableBeforePaddingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeModifierClickableBeforePadding",
		RuleSet:       "compose",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ComposeModifierFillAfterSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeModifierFillAfterSize",
		RuleSet:       "compose",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ComposeModifierPassedThenChainedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeModifierPassedThenChained",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeMutableDefaultArgumentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeMutableDefaultArgument",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeMutableStateInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeMutableStateInComposition",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposePreviewAnnotationMissingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposePreviewAnnotationMissing",
		RuleSet:       "compose",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ComposePreviewWithBackingStateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposePreviewWithBackingState",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeRememberSaveableNonParcelableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeRememberSaveableNonParcelable",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeRememberWithoutKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeRememberWithoutKey",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeSideEffectInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeSideEffectInComposition",
		RuleSet:       "compose",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ComposeSideEffectInCompositionRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *ComposeSideEffectInCompositionRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ComposeSideEffectInCompositionRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *ComposeSideEffectInCompositionRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}

func (r *ComposeStatefulDefaultParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeStatefulDefaultParameter",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeStringResourceInsideLambdaRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeStringResourceInsideLambda",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}

func (r *ComposeUnstableParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeUnstableParameter",
		RuleSet:       "compose",
		DefaultActive: true,
	}
}
