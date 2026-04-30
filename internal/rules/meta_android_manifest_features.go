// Descriptor metadata for internal/rules/android_manifest_features.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AppIndexingErrorManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AppIndexingErrorManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AppIndexingWarningManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AppIndexingWarningManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeviceAdminManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeviceAdminManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FullBackupContentManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FullBackupContentManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GoogleAppIndexingDeepLinkErrorManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GoogleAppIndexingDeepLinkErrorManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GoogleAppIndexingWarningManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GoogleAppIndexingWarningManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingLeanbackLauncherManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingLeanbackLauncherManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingLeanbackSupportManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingLeanbackSupportManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingRegisteredManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingRegisteredManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PermissionImpliesUnsupportedHardwareManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PermissionImpliesUnsupportedHardwareManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlCompatManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlCompatManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlEnabledManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlEnabledManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnsupportedChromeOsHardwareManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnsupportedChromeOsHardwareManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
