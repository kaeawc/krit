package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *ReturnThisRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ReturnThis", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"An override is checked when the overridden @ReturnThis method is declared in the same file; library and other-file supertypes are not followed.",
		}}
}

func (r *UseSdkSuppressRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseSdkSuppress", RuleSet: "android-lint", DefaultActive: true}
}

func (r *AnnotateVersionCheckRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AnnotateVersionCheck", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Only plain SDK_INT comparisons are checked (not SdkExtensions.getExtensionVersion, getBuildSdkInt, when branches or bare SDK_INT field aliases); the library module comes from the nearest build script's plugins.",
		}}
}
