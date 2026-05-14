// Descriptor metadata for internal/rules/supply_chain.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *CompileSdkMismatchAcrossModulesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CompileSdkMismatchAcrossModules",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *ApplyPluginTwiceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ApplyPluginTwice",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *ConventionPluginAppliedToWrongTargetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConventionPluginAppliedToWrongTarget",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ConventionPluginAppliedToWrongTargetRule]{
				Name:        "pluginTargetMap",
				Default:     []string{},
				Description: "Convention plugin target overrides as id=android, id=jvm, or id=any entries.",
				Apply:       func(r *ConventionPluginAppliedToWrongTargetRule, v []string) { r.PluginTargetMap = v },
			}),
		},
	}
}

func (r *ConfigurationsAllSideEffectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConfigurationsAllSideEffect",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[ConfigurationsAllSideEffectRule]{
				Name:        "allowInConventionPlugins",
				Default:     true,
				Description: "Allow configurations.all side effects in build-logic/ and buildSrc/ convention plugin paths.",
				Apply:       func(r *ConfigurationsAllSideEffectRule, v bool) { r.AllowInConventionPlugins = v },
			}),
		},
	}
}

func (r *DependencyFromBintrayRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencyFromBintray",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *DependencyFromHTTPRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencyFromHttp",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[DependencyFromHTTPRule]{
				Name:        "allowLoopback",
				Default:     true,
				Description: "Allow plaintext HTTP repositories hosted on localhost or loopback IP addresses.",
				Apply:       func(r *DependencyFromHTTPRule, v bool) { r.AllowLoopback = v },
			}),
			api.StringListOption(api.StringListOptionSpec[DependencyFromHTTPRule]{
				Name:        "allowedHosts",
				Default:     []string{},
				Description: "Exact repository hostnames to permit over HTTP.",
				Apply:       func(r *DependencyFromHTTPRule, v []string) { r.AllowedHosts = v },
			}),
			api.StringListOption(api.StringListOptionSpec[DependencyFromHTTPRule]{
				Name:        "allowedUrls",
				Default:     []string{},
				Description: "Repository URL prefixes to permit over HTTP.",
				Apply:       func(r *DependencyFromHTTPRule, v []string) { r.AllowedUrls = v },
			}),
		},
	}
}

func (r *DependencyFromJcenterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencyFromJcenter",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *DependencySnapshotInReleaseRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencySnapshotInRelease",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[DependencySnapshotInReleaseRule]{
				Name:        "allowedSnapshots",
				Default:     []string{},
				Description: "Allowed group:name coordinate patterns for temporary snapshot dependencies. Supports * wildcards.",
				Apply:       func(r *DependencySnapshotInReleaseRule, v []string) { r.AllowedSnapshots = v },
			}),
			api.StringOption(api.StringOptionSpec[DependencySnapshotInReleaseRule]{
				Name:        "suppressUntil",
				Default:     "",
				Description: "YYYY-MM-DD date through which this rule is suppressed for temporary snapshot testing.",
				Apply:       func(r *DependencySnapshotInReleaseRule, v string) { r.SuppressUntil = v },
			}),
		},
	}
}

func (r *DependencyVerificationDisabledRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencyVerificationDisabled",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.BoolOption(api.BoolOptionSpec[DependencyVerificationDisabledRule]{
				Name:        "allowLenient",
				Default:     false,
				Description: "Allow org.gradle.dependency.verification=lenient while still flagging off.",
				Apply:       func(r *DependencyVerificationDisabledRule, v bool) { r.AllowLenient = v },
			}),
		},
	}
}

func (r *DependencyWithoutGroupRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependencyWithoutGroup",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *DependenciesInRootProjectRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DependenciesInRootProject",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[DependenciesInRootProjectRule]{
				Name:        "allowedConfigurations",
				Default:     []string{"classpath", "detektPlugins"},
				Description: "Dependency configurations that may legitimately appear in the root project.",
				Apply:       func(r *DependenciesInRootProjectRule, v []string) { r.AllowedConfigurations = v },
			}),
		},
	}
}

func (r *GradleWrapperValidationActionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleWrapperValidationAction",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *JvmTargetMismatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JvmTargetMismatch",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *KotlinVersionMismatchAcrossModulesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KotlinVersionMismatchAcrossModules",
		RuleSet:       "supply-chain",
		DefaultActive: true,
	}
}

func (r *MissingGradleChecksumsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MissingGradleChecksums",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *VersionCatalogBuildSrcMismatchRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VersionCatalogBuildSrcMismatch",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *VersionCatalogDuplicateVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VersionCatalogDuplicateVersion",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *VersionCatalogRawVersionInBuildRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VersionCatalogRawVersionInBuild",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
	}
}

func (r *VersionCatalogUnusedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VersionCatalogUnused",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason: api.OptInReasonDomainSpecific,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[VersionCatalogUnusedRule]{
				Name:        "ignoredAliases",
				Default:     []string{},
				Description: "Alias names (or globs with leading/trailing *) to skip when reporting unused entries.",
				Apply:       func(r *VersionCatalogUnusedRule, v []string) { r.IgnoredAliases = v },
			}),
			api.BoolOption(api.BoolOptionSpec[VersionCatalogUnusedRule]{
				Name:        "scanConventionPlugins",
				Default:     true,
				Description: "Also scan build-logic/ and buildSrc/ Kotlin sources for catalog accessor references.",
				Apply:       func(r *VersionCatalogUnusedRule, v bool) { r.ScanConventionPlugins = v },
			}),
		},
	}
}
