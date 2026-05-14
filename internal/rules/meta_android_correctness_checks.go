// Descriptor metadata for internal/rules/android_correctness_checks.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AccidentalOctalRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AccidentalOctal",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *AppCompatMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AppCompatMethod",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *CustomViewStyleableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CustomViewStyleable",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *DeprecatedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Deprecated",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *InnerclassSeparatorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InnerclassSeparator",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *LocalSuppressRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocalSuppress",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ObjectAnimatorBindingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObjectAnimatorBinding",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *OnClickRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OnClick",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *OverrideAbstractRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OverrideAbstract",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ParcelCreatorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ParcelCreator",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *PluralsCandidateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PluralsCandidate",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *PropertyEscapeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PropertyEscape",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *RangeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Range",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ResourceAsColorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ResourceAsColor",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ResourceTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ResourceType",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ShortAlarmRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ShortAlarm",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SupportAnnotationUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SupportAnnotationUsage",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SwitchIntDefRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SwitchIntDef",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *TextViewEditsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TextViewEdits",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WrongViewCastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongViewCast",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}
