// Descriptor metadata for internal/rules/privacy_analytics.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AnalyticsCallWithoutConsentGateRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AnalyticsCallWithoutConsentGate",
		RuleSet:       "privacy",
		Severity:      "info",
		Description:   "Detects analytics event calls that are not guarded by a consent or GDPR check.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AnalyticsEventWithPiiParamNameRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AnalyticsEventWithPiiParamName",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects analytics event parameters whose key names match PII patterns like email, phone, or SSN.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *AnalyticsUserIdFromPiiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AnalyticsUserIdFromPii",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects user-ID setter calls whose argument is a PII property like email or phoneNumber.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *CrashlyticsCustomKeyWithPiiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CrashlyticsCustomKeyWithPii",
		RuleSet:       "privacy",
		Severity:      "warning",
		Description:   "Detects Crashlytics setCustomKey calls where the key name matches PII patterns.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FirebaseRemoteConfigDefaultsWithPii",
		RuleSet:       "privacy",
		Severity:      "info",
		Description:   "Detects Firebase Remote Config default map keys that match PII patterns.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
