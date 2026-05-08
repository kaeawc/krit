// Descriptor metadata for internal/rules/android_source_extra.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *GridLayoutRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GridLayout",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *InstantiatableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Instantiatable",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *LayoutInflationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayoutInflation",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LibraryCustomViewRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LibraryCustomView",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *LocaleFolderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocaleFolder",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *MangledCRLFRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MangledCRLF",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *MissingPermissionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingPermission",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *NfcTechWhitespaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NfcTechWhitespace",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ProguardRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Proguard",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ProguardSplitRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ProguardSplit",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ResourceNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ResourceName",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *RtlAwareRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlAware",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *RtlFieldAccessRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlFieldAccess",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *TrulyRandomRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TrulyRandom",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *UnknownIDInLayoutRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnknownIdInLayout",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *UseAlpha2Rule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseAlpha2",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *ViewConstructorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ViewConstructor",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ViewTagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ViewTag",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WrongConstantRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongConstant",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *WrongImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongImport",
		RuleSet:       "android-lint",
		DefaultActive: true,
		FixLevel:      "idiomatic",
	}
}
