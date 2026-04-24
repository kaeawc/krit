// Descriptor metadata for internal/rules/android_manifest_features.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AppIndexingErrorManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AppIndexingErrorManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AppIndexingWarningManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AppIndexingWarningManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeviceAdminManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DeviceAdminManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FullBackupContentManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "FullBackupContentManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GoogleAppIndexingDeepLinkErrorManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GoogleAppIndexingDeepLinkErrorManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GoogleAppIndexingWarningManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GoogleAppIndexingWarningManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingLeanbackLauncherManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingLeanbackLauncherManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingLeanbackSupportManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingLeanbackSupportManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingRegisteredManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingRegisteredManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PermissionImpliesUnsupportedHardwareManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "PermissionImpliesUnsupportedHardwareManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlCompatManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlCompatManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlEnabledManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlEnabledManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnsupportedChromeOsHardwareManifestRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnsupportedChromeOsHardwareManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
