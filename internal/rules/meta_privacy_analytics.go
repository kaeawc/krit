// Descriptor metadata for internal/rules/privacy_analytics.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AnalyticsCallWithoutConsentGateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnalyticsCallWithoutConsentGate",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *AnalyticsEventWithPiiParamNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnalyticsEventWithPiiParamName",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *AnalyticsUserIDFromPiiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AnalyticsUserIdFromPii",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *CrashlyticsCustomKeyWithPiiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CrashlyticsCustomKeyWithPii",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *FirebaseRemoteConfigDefaultsWithPiiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FirebaseRemoteConfigDefaultsWithPii",
		RuleSet:       "privacy",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}
