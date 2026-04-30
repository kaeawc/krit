// Descriptor metadata for internal/rules/android_manifest_structure.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *DuplicateActivityManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DuplicateActivityManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DuplicateUsesFeatureManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DuplicateUsesFeatureManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleOverridesManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradleOverridesManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *IntentFilterExportRequiredRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "IntentFilterExportRequired",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InvalidUsesTagAttributeManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InvalidUsesTagAttributeManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ManifestOrderManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ManifestOrderManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ManifestTypoRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ManifestTypoManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MipmapLauncherRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MipmapLauncher",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingApplicationIconRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingApplicationIconManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingVersionManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingVersionManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MockLocationManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MockLocationManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MultipleUsesSdkManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MultipleUsesSdkManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SystemPermissionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SystemPermission",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *TargetNewerRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "TargetNewer",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UniquePermissionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UniquePermission",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnpackedNativeCodeManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnpackedNativeCodeManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UsesSdkManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UsesSdkManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *WrongManifestParentManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "WrongManifestParentManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
