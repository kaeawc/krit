// Descriptor metadata for internal/rules/android_gradle.go.
// NOTE: Meta() for NewerVersionAvailableRule is hand-written in meta_newer_version_available.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *AndroidGradlePluginVersionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "AndroidGradlePluginVersion",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DeprecatedDependencyRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
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
func (r *DynamicVersionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DynamicVersion",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleDeprecatedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradleDeprecated",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleGetterRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradleGetter",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleIdeErrorRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradleIdeError",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradleOldTargetApiRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "OldTargetApi",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "threshold",
				Type:        registry.OptInt,
				Default:     33,
				Description: "",
				Apply:       func(target interface{}, value interface{}) { target.(*GradleOldTargetApiRule).Threshold = value.(int) },
			},
		},
	}
}

func (r *GradleOverridesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradleOverrides",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *GradlePathRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
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
func (r *GradlePluginCompatibilityRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "GradlePluginCompatibility",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MavenLocalRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MavenLocal",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MinSdkTooLowRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MinSdkTooLow",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "threshold",
				Type:        registry.OptInt,
				Default:     21,
				Description: "",
				Apply:       func(target interface{}, value interface{}) { target.(*MinSdkTooLowRule).Threshold = value.(int) },
			},
		},
	}
}

func (r *RemoteVersionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
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
func (r *StringIntegerRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "StringInteger",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
