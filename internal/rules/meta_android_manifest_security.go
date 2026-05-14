// Descriptor metadata for internal/rules/android_manifest_security.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AllowBackupManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AllowBackupManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *BackupRulesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BackupRules",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *CleartextTrafficRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CleartextTraffic",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *DebuggableManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DebuggableManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *DeepLinkMissingAutoVerifyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeepLinkMissingAutoVerify",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *ExportedPreferenceActivityManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedPreferenceActivityManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ExportedServiceManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedServiceManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *ExportedWithoutPermissionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExportedWithoutPermission",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InsecureBaseConfigurationManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InsecureBaseConfigurationManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *NetworkSecurityConfigDebugOverridesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NetworkSecurityConfigDebugOverrides",
		RuleSet:       "security",
		DefaultActive: true,
	}
}

func (r *MissingExportedFlagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingExportedFlag",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ProtectedPermissionsManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ProtectedPermissionsManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *ServiceExportedManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ServiceExportedManifest",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonAndroidOnly,
	}
}

func (r *UnprotectedSMSBroadcastReceiverManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnprotectedSMSBroadcastReceiverManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UnsafeProtectedBroadcastReceiverManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnsafeProtectedBroadcastReceiverManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *UseCheckPermissionManifestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UseCheckPermissionManifest",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
