// Descriptor metadata for internal/rules/android_source.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *FragmentConstructorRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "FragmentConstructor",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GetSignaturesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GetSignatures",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LogTagLengthRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LongLogTag",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *LogTagMismatchRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LogTagMismatch",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NonInternationalizedSmsRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NonInternationalizedSms",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ServiceCastRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ServiceCast",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SparseArrayRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UseSparseArrays",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *ToastRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "ShowToast",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *UseValueOfRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UseValueOf",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}
