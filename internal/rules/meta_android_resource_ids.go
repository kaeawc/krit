// Descriptor metadata for internal/rules/android_resource_ids.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AppCompatResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AppCompatResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *CutPasteIDResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CutPasteIdResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *DuplicateIDsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DuplicateIdsResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *DuplicateIncludedIDsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DuplicateIncludedIdsResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *IllegalResourceRefResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IllegalResourceRefResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *InvalidIDResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InvalidIdResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InvalidResourceFolderResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InvalidResourceFolderResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *MissingIDResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingIdResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *MissingPrefixResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingPrefixResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *NamespaceTypoResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NamespaceTypoResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ResAutoResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ResAutoResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *UnusedNamespaceResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedNamespaceResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *WrongCaseResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongCaseResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *WrongFolderResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongFolderResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}
