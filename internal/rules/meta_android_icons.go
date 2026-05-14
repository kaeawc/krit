// Descriptor metadata for internal/rules/android_icons.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ConvertToWebpRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConvertToWebp",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *GifUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GifUsage",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconDensitiesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconDensities",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconDipSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconDipSize",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconDuplicatesConfigRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconDuplicatesConfig",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconDuplicatesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconDuplicates",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconExpectedSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconExpectedSize",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconExtensionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconExtension",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconLocationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconLocation",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconMixedNinePatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconMixedNinePatch",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconMissingDensityFolderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconMissingDensityFolder",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconColorsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconColors",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconLauncherShapeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconLauncherShape",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconXMLAndPngRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconXmlAndPng",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *IconNoDpiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IconNoDpi",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}
