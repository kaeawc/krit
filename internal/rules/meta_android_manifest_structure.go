// Descriptor metadata for internal/rules/android_manifest_structure.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *DuplicateActivityManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DuplicateActivityManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *DuplicateUsesFeatureManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DuplicateUsesFeatureManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *GradleOverridesManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleOverridesManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *IntentFilterExportRequiredRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "IntentFilterExportRequired",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *InvalidUsesTagAttributeManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InvalidUsesTagAttributeManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ManifestOrderManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ManifestOrderManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ManifestTypoRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ManifestTypoManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MipmapLauncherRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MipmapLauncher",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MissingApplicationIconRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingApplicationIconManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MissingVersionManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingVersionManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MockLocationManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MockLocationManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MultipleUsesSdkManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MultipleUsesSdkManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *SystemPermissionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SystemPermission",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *TargetNewerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TargetNewer",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UniquePermissionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UniquePermission",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *UnpackedNativeCodeManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnpackedNativeCodeManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UsesSdkManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UsesSdkManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *WrongManifestParentManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "WrongManifestParentManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
