// Descriptor metadata for internal/rules/accessibility.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AnimatorDurationIgnoresScaleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnimatorDurationIgnoresScale",
		RuleSet:       "a11y",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *ComposeClickableWithoutMinTouchTargetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeClickableWithoutMinTouchTarget",
		RuleSet:       "a11y",
		DefaultActive: true,
	}
}

func (r *ComposeDecorativeImageContentDescriptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeDecorativeImageContentDescription",
		RuleSet:       "a11y",
		DefaultActive: true,
	}
}

func (r *ComposeIconButtonMissingContentDescriptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeIconButtonMissingContentDescription",
		RuleSet:       "a11y",
		DefaultActive: true,
	}
}

func (r *ComposeRawTextLiteralRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeRawTextLiteral",
		RuleSet:       "a11y",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ComposeRawTextLiteralRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *ComposeRawTextLiteralRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ComposeRawTextLiteralRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *ComposeRawTextLiteralRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}

func (r *ComposeSemanticsMissingRoleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeSemanticsMissingRole",
		RuleSet:       "a11y",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ComposeSemanticsMissingRoleRule]{
				Name:        "customPreviewWildcard",
				Default:     true,
				Description: "Treat annotations whose simple name ends with Preview/Previews as preview scaffolding.",
				Apply:       func(r *ComposeSemanticsMissingRoleRule, v bool) { r.CustomPreviewWildcard = v },
			}),
			api.StringListOption(api.StringListOptionSpec[ComposeSemanticsMissingRoleRule]{
				Name:        "customPreviewPrefixes",
				Description: "Annotation prefixes to treat as preview scaffolding, e.g. Custom matches CustomPreview.",
				Apply:       func(r *ComposeSemanticsMissingRoleRule, v []string) { r.CustomPreviewPrefixes = v },
			}),
		},
	}
}

func (r *ComposeTextFieldMissingLabelRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeTextFieldMissingLabel",
		RuleSet:       "a11y",
		DefaultActive: true,
	}
}

func (r *ToastForAccessibilityAnnouncementRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ToastForAccessibilityAnnouncement",
		RuleSet:       "a11y",
		DefaultActive: true,
	}
}
