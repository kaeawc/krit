// Descriptor metadata for internal/rules/android_manifest_security.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AllowBackupManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AllowBackupManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *BackupRulesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "BackupRules",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CleartextTrafficRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CleartextTraffic",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DebuggableManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DebuggableManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedPreferenceActivityManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedPreferenceActivityManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedServiceManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedServiceManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ExportedWithoutPermissionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ExportedWithoutPermission",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InsecureBaseConfigurationManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InsecureBaseConfigurationManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MissingExportedFlagRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingExportedFlag",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ProtectedPermissionsManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ProtectedPermissionsManifest",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ServiceExportedManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ServiceExportedManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnprotectedSMSBroadcastReceiverManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnprotectedSMSBroadcastReceiverManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnsafeProtectedBroadcastReceiverManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnsafeProtectedBroadcastReceiverManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UseCheckPermissionManifestRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UseCheckPermissionManifest",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
