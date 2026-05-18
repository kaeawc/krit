// Descriptor metadata for internal/rules/android_resource_values.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *ExtraTextResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExtraTextResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GoogleAPIKeyInResourcesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GoogleApiKeyInResources",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *ImpliedQuantityResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ImpliedQuantityResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InconsistentArraysResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InconsistentArraysResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *LocaleConfigStaleResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocaleConfigStale",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *LocaleFolderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocaleFolder",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MissingQuantityResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingQuantityResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *OnClickResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OnClickResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringFormatCountResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringFormatCountResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringFormatInvalidResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringFormatInvalidResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringFormatMatchesResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringFormatMatchesResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringFormatTrivialResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringFormatTrivialResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringTrailingWhitespaceResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringTrailingWhitespace",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *StringResourceMissingPositionalRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringResourceMissingPositional",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StringNotLocalizableResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringNotLocalizableResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *TextFieldsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TextFieldsResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UnusedAttributeResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedAttributeResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UnusedQuantityResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedQuantityResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UseAlpha2Rule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseAlpha2",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WebViewInScrollViewResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WebViewInScrollViewResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *WrongRegionResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongRegionResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}
