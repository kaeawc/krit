// Descriptor metadata for internal/rules/android_resource_values.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *ExtraTextResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExtraTextResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GoogleApiKeyInResourcesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GoogleApiKeyInResources",
		RuleSet:       "security",
		Severity:      "warning",
		Description:   "Detects Google API keys embedded directly in XML resource files",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ImpliedQuantityResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ImpliedQuantityResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InconsistentArraysResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InconsistentArraysResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *LocaleConfigStaleResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LocaleConfigStale",
		RuleSet:       "android-lint",
		Severity:      "info",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingQuantityResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingQuantityResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OnClickResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OnClickResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringFormatCountResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringFormatCountResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringFormatInvalidResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringFormatInvalidResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringFormatMatchesResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringFormatMatchesResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringFormatTrivialResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringFormatTrivialResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StringTrailingWhitespaceResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringTrailingWhitespace",
		RuleSet:       "android-lint",
		Severity:      "info",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.85,
	}
}

func (r *StringNotLocalizableResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringNotLocalizableResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TextFieldsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TextFieldsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedAttributeResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedAttributeResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedQuantityResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedQuantityResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WebViewInScrollViewResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WebViewInScrollViewResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongRegionResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongRegionResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
