// Descriptor metadata for internal/rules/android_source.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *FragmentConstructorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FragmentConstructor",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *GetSignaturesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GetSignatures",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LogTagLengthRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LongLogTag",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *LogTagMismatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogTagMismatch",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *NonInternationalizedSmsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NonInternationalizedSms",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ServiceCastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ServiceCast",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *SparseArrayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseSparseArrays",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ToastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ShowToast",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UseValueOfRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseValueOf",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
