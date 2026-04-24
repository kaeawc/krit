// Descriptor metadata for internal/rules/android_resource_a11y.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *BackButtonResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "BackButtonResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ButtonCaseResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ButtonCaseResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ButtonOrderResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ButtonOrderResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ButtonStyleResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ButtonStyleResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ClickableViewAccessibilityResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ClickableViewAccessibilityResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedValuesResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "HardcodedValuesResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LabelForResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LabelForResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutAutofillHintMismatchRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutAutofillHintMismatch",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutClickableWithoutMinSizeRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutClickableWithoutMinSize",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutEditTextMissingImportanceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutEditTextMissingImportance",
		RuleSet:       "android-lint",
		Severity:      "info",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutImportantForAccessibilityNoRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutImportantForAccessibilityNo",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutMinTouchTargetInButtonRowRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutMinTouchTargetInButtonRow",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingContentDescriptionResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingContentDescriptionResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringNotSelectableRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "StringNotSelectable",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringRepeatedInContentDescriptionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "StringRepeatedInContentDescription",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringSpanInContentDescriptionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "StringSpanInContentDescription",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
