// Descriptor metadata for internal/rules/android.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *ContentDescriptionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ContentDescription",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedServiceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedService",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *HardcodedTextRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "HardcodedText",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LogDetectorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LogConditional",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ObsoleteLayoutParamsRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObsoleteLayoutParam",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PrivateKeyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PackagedPrivateKey",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SdCardPathRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SdCardPath",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SetJavaScriptEnabledRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SetJavaScriptEnabled",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ViewHolderRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ViewHolder",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WakelockRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Wakelock",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
