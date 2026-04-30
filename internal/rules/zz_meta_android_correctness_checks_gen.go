// Descriptor metadata for internal/rules/android_correctness_checks.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AccidentalOctalRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AccidentalOctal",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AppCompatMethodRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AppCompatMethod",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CustomViewStyleableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CustomViewStyleable",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *DeprecatedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Deprecated",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InnerclassSeparatorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InnerclassSeparator",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocalSuppressRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocalSuppress",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ObjectAnimatorBindingRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObjectAnimatorBinding",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OnClickRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OnClick",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OverrideAbstractRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OverrideAbstract",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ParcelCreatorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ParcelCreator",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PluralsCandidateRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PluralsCandidate",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PropertyEscapeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PropertyEscape",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RangeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Range",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResourceAsColorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ResourceAsColor",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResourceTypeRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ResourceType",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ShortAlarmRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ShortAlarm",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SupportAnnotationUsageRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SupportAnnotationUsage",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SwitchIntDefRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SwitchIntDef",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TextViewEditsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TextViewEdits",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongViewCastRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongViewCast",
		RuleSet:       "android-lint",
		Severity:      "",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
