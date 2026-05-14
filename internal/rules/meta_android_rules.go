// Descriptor metadata for internal/rules/android.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ContentDescriptionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ContentDescription",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ExportedServiceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedService",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *HardcodedTextRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedText",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *LogDetectorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LogConditional",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ObsoleteLayoutParamsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObsoleteLayoutParam",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *PrivateKeyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PackagedPrivateKey",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SdCardPathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SdCardPath",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SetJavaScriptEnabledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SetJavaScriptEnabled",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *ViewHolderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ViewHolder",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WakelockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Wakelock",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}
