// Descriptor metadata for internal/rules/android_resource_a11y.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *BackButtonResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BackButtonResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ButtonCaseResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ButtonCaseResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ButtonOrderResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ButtonOrderResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ButtonStyleResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ButtonStyleResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ClickableViewAccessibilityResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ClickableViewAccessibilityResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *HardcodedValuesResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedValuesResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *LabelForResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LabelForResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LayoutAutofillHintMismatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutAutofillHintMismatch",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LayoutClickableWithoutMinSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutClickableWithoutMinSize",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LayoutEditTextMissingImportanceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutEditTextMissingImportance",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LayoutImportantForAccessibilityNoRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutImportantForAccessibilityNo",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LayoutMinTouchTargetInButtonRowRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutMinTouchTargetInButtonRow",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MissingContentDescriptionResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingContentDescriptionResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *StringNotSelectableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringNotSelectable",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringRepeatedInContentDescriptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringRepeatedInContentDescription",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringSpanInContentDescriptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringSpanInContentDescription",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
