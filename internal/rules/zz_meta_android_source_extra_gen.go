// Descriptor metadata for internal/rules/android_source_extra.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *GridLayoutRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GridLayout",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *InstantiatableRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "Instantiatable",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LayoutInflationRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayoutInflation",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *LibraryCustomViewRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LibraryCustomView",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocaleFolderRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LocaleFolder",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MangledCRLFRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MangledCRLF",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}

func (r *MissingPermissionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingPermission",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NfcTechWhitespaceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NfcTechWhitespace",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ProguardRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "Proguard",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ProguardSplitRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ProguardSplit",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResourceNameRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ResourceName",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *RtlAwareRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlAware",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlFieldAccessRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlFieldAccess",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TrulyRandomRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "TrulyRandom",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnknownIdInLayoutRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnknownIdInLayout",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *UseAlpha2Rule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UseAlpha2",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0,
	}
}

func (r *ViewConstructorRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ViewConstructor",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ViewTagRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ViewTag",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *WrongConstantRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WrongConstant",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *WrongImportRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WrongImport",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
