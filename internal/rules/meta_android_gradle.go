// Descriptor metadata for internal/rules/android_gradle.go.
// NOTE: Meta() for NewerVersionAvailableRule is hand-written in meta_newer_version_available.go.

package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func (r *AndroidGradlePluginVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AndroidGradlePluginVersion",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *DeprecatedDependencyRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DeprecatedDependency",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

// DynamicVersionRule is also registered as rule ID "GradleDynamicVersion"; Meta() only represents the primary ID.
func (r *DynamicVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DynamicVersion",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GradleDeprecatedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleDeprecated",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GradleGetterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleGetter",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GradleIdeErrorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleIdeError",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GradleOldTargetAPIRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OldTargetApi",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[GradleOldTargetAPIRule]{
				Name:        "threshold",
				Default:     33,
				Description: "",
				Apply:       func(r *GradleOldTargetAPIRule, v int) { r.Threshold = v },
			}),
		},
	}
}

func (r *ExpiredTargetSdkVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ExpiredTargetSdkVersion",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[ExpiredTargetSdkVersionRule]{
				Name:        "floor",
				Default:     defaultExpiredTargetSdkFloor,
				Description: "Minimum acceptable targetSdkVersion under the project's enforced compliance policy.",
				Apply:       func(r *ExpiredTargetSdkVersionRule, v int) { r.Floor = v },
			}),
		},
	}
}

func (r *GradleOverridesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleOverrides",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *GradlePathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradlePath",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

// GradlePluginCompatibilityRule is also registered as rule ID "GradleCompatible"; Meta() only represents the primary ID.
func (r *GradlePluginCompatibilityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradlePluginCompatibility",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MavenLocalRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MavenLocal",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MinSdkTooLowRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MinSdkTooLow",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[MinSdkTooLowRule]{
				Name:        "threshold",
				Default:     21,
				Description: "",
				Apply:       func(r *MinSdkTooLowRule, v int) { r.Threshold = v },
			}),
		},
	}
}

func (r *RemoteVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RemoteVersion",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

// StringIntegerRule is also registered as rule ID "StringShouldBeInt"; Meta() only represents the primary ID.
func (r *StringIntegerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StringInteger",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}
