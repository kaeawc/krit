// Descriptor metadata for internal/rules/privacy_permissions.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AdMobInitializedBeforeConsentRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AdMobInitializedBeforeConsent",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *BiometricAuthNotFallingBackToDeviceCredentialRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BiometricAuthNotFallingBackToDeviceCredential",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ClipboardOnSensitiveInputTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ClipboardOnSensitiveInputType",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ContactsAccessWithoutPermissionUIRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ContactsAccessWithoutPermissionUi",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *LocationBackgroundWithoutRationaleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LocationBackgroundWithoutRationale",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *ScreenshotNotBlockedOnLoginScreenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ScreenshotNotBlockedOnLoginScreen",
		RuleSet:       "privacy",
		DefaultActive: true,
	}
}
