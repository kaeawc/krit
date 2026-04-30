// Descriptor metadata for internal/rules/android_source.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *FragmentConstructorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FragmentConstructor",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GetSignaturesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GetSignatures",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LogTagLengthRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LongLogTag",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *LogTagMismatchRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LogTagMismatch",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NonInternationalizedSmsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NonInternationalizedSms",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ServiceCastRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ServiceCast",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SparseArrayRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseSparseArrays",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}

func (r *ToastRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ShowToast",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *UseValueOfRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseValueOf",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.9,
	}
}
