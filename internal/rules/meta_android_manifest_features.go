// Descriptor metadata for internal/rules/android_manifest_features.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *DeviceAdminManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeviceAdminManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *FullBackupContentManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FullBackupContentManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GoogleAppIndexingDeepLinkErrorManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GoogleAppIndexingDeepLinkErrorManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *GoogleAppIndexingWarningManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GoogleAppIndexingWarningManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MissingLeanbackLauncherManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingLeanbackLauncherManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MissingLeanbackSupportManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingLeanbackSupportManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *MissingRegisteredManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingRegisteredManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *PermissionImpliesUnsupportedHardwareManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PermissionImpliesUnsupportedHardwareManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *PermissionImpliesUnsupportedChromeOsHardwareManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PermissionImpliesUnsupportedChromeOsHardwareManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RtlEnabledManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlEnabledManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UnsupportedChromeOsHardwareManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnsupportedChromeOsHardwareManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
