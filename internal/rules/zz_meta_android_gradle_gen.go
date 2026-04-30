// Descriptor metadata for internal/rules/android_gradle.go.
// NOTE: Meta() for NewerVersionAvailableRule is hand-written in meta_newer_version_available.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AndroidGradlePluginVersionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AndroidGradlePluginVersion",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeprecatedDependencyRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DeprecatedDependency",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

// DynamicVersionRule is also registered as rule ID "GradleDynamicVersion"; Meta() only represents the primary ID.
func (r *DynamicVersionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DynamicVersion",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleDeprecatedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradleDeprecated",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleGetterRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradleGetter",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleIdeErrorRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradleIdeError",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleOldTargetApiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OldTargetApi",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "threshold",
				Type:        v2.OptInt,
				Default:     33,
				Description: "",
				Apply:       func(target interface{}, value interface{}) { target.(*GradleOldTargetApiRule).Threshold = value.(int) },
			},
		},
	}
}

func (r *GradleOverridesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradleOverrides",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradlePathRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradlePath",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

// GradlePluginCompatibilityRule is also registered as rule ID "GradleCompatible"; Meta() only represents the primary ID.
func (r *GradlePluginCompatibilityRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GradlePluginCompatibility",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MavenLocalRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MavenLocal",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MinSdkTooLowRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MinSdkTooLow",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "threshold",
				Type:        v2.OptInt,
				Default:     21,
				Description: "",
				Apply:       func(target interface{}, value interface{}) { target.(*MinSdkTooLowRule).Threshold = value.(int) },
			},
		},
	}
}

func (r *RemoteVersionRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RemoteVersion",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

// StringIntegerRule is also registered as rule ID "StringShouldBeInt"; Meta() only represents the primary ID.
func (r *StringIntegerRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StringInteger",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
