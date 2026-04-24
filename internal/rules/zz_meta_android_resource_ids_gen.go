// Descriptor metadata for internal/rules/android_resource_ids.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AppCompatResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AppCompatResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CutPasteIdResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CutPasteIdResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DuplicateIdsResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DuplicateIdsResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DuplicateIncludedIdsResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DuplicateIncludedIdsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IllegalResourceRefResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "IllegalResourceRefResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InvalidIdResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "InvalidIdResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InvalidResourceFolderResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "InvalidResourceFolderResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingIdResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingIdResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingPrefixResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingPrefixResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NamespaceTypoResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NamespaceTypoResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ResAutoResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ResAutoResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedNamespaceResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnusedNamespaceResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongCaseResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WrongCaseResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongFolderResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "WrongFolderResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
