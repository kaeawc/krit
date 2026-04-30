// Descriptor metadata for internal/rules/android_source_extra.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *GridLayoutRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GridLayout",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *InstantiatableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Instantiatable",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutInflationRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LayoutInflation",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *LibraryCustomViewRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LibraryCustomView",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocaleFolderRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocaleFolder",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MangledCRLFRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MangledCRLF",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *MissingPermissionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingPermission",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NfcTechWhitespaceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NfcTechWhitespace",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ProguardRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Proguard",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ProguardSplitRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ProguardSplit",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResourceNameRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ResourceName",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *RtlAwareRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlAware",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlFieldAccessRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlFieldAccess",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TrulyRandomRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TrulyRandom",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnknownIdInLayoutRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnknownIdInLayout",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *UseAlpha2Rule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseAlpha2",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0,
	}
}

func (r *ViewConstructorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ViewConstructor",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ViewTagRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ViewTag",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *WrongConstantRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongConstant",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *WrongImportRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongImport",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "idiomatic",
		Confidence:    0.95,
	}
}
