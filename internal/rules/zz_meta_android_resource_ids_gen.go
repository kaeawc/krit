// Descriptor metadata for internal/rules/android_resource_ids.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AppCompatResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AppCompatResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CutPasteIdResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CutPasteIdResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DuplicateIdsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DuplicateIdsResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DuplicateIncludedIdsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DuplicateIncludedIdsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IllegalResourceRefResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "IllegalResourceRefResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InvalidIdResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InvalidIdResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InvalidResourceFolderResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InvalidResourceFolderResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingIdResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingIdResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingPrefixResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingPrefixResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NamespaceTypoResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NamespaceTypoResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResAutoResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ResAutoResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedNamespaceResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedNamespaceResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongCaseResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongCaseResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongFolderResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongFolderResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
