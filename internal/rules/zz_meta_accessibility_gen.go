// Descriptor metadata for internal/rules/accessibility.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AnimatorDurationIgnoresScaleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AnimatorDurationIgnoresScale",
		RuleSet:       "a11y",
		Severity:      "info",
		Description:   "Detects animator durations that ignore the system ANIMATOR_DURATION_SCALE accessibility setting.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeClickableWithoutMinTouchTargetRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeClickableWithoutMinTouchTarget",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects clickable Compose modifiers with explicit touch target dimensions below the 48dp minimum.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeDecorativeImageContentDescriptionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeDecorativeImageContentDescription",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects decorative images with null contentDescription that are not hidden from TalkBack.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeIconButtonMissingContentDescriptionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeIconButtonMissingContentDescription",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects Icon or IconButton composables missing a contentDescription for screen readers.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeRawTextLiteralRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeRawTextLiteral",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects Compose Text() calls using hardcoded string literals instead of stringResource() for i18n.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeSemanticsMissingRoleRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeSemanticsMissingRole",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects interactive Compose modifiers (clickable, toggleable, selectable) without an explicit accessibility role.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ComposeTextFieldMissingLabelRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ComposeTextFieldMissingLabel",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects TextField or OutlinedTextField composables missing a label parameter for accessibility.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ToastForAccessibilityAnnouncementRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ToastForAccessibilityAnnouncement",
		RuleSet:       "a11y",
		Severity:      "warning",
		Description:   "Detects Toast.makeText used in accessibility-related functions instead of announceForAccessibility.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
