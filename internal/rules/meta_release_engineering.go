// Descriptor metadata for internal/rules/release_engineering.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *AllProjectsBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AllProjectsBlock",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *BuildConfigDebugInLibraryRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BuildConfigDebugInLibrary",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *BuildConfigDebugInvertedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "BuildConfigDebugInverted",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *CommentedOutCodeBlockRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CommentedOutCodeBlock",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *CommentedOutImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "CommentedOutImport",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *ConventionPluginDeadCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ConventionPluginDeadCode",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *ModuleTemplateConformanceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ModuleTemplateConformance",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonProjectPolicy,
		CustomApply: api.TypedCustomApply(func(rule *ModuleTemplateConformanceRule, cfg api.ConfigSource) {
			adapter, ok := cfg.(*ConfigAdapter)
			if !ok {
				return
			}
			rule.Template = adapter.Unwrap().ModuleTemplate()
		}),
	}
}

func (r *DebugToastInProductionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DebugToastInProduction",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *GradleBuildContainsTodoRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GradleBuildContainsTodo",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *HardcodedEnvironmentNameRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedEnvironmentName",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *HardcodedLocalhostURLRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedLocalhostUrl",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *HardcodedLogTagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "HardcodedLogTag",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *MergeConflictMarkerLeftoverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MergeConflictMarkerLeftover",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *NonASCIIIdentifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NonAsciiIdentifier",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonOpinionated,
	}
}

func (r *OpenForTestingCallerInNonTestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OpenForTestingCallerInNonTest",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *PrintStackTraceInProductionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PrintStackTraceInProduction",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *PrintlnInProductionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PrintlnInProduction",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *TestFixtureAccessedFromProductionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestFixtureAccessedFromProduction",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *TestOnlyImportInProductionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TestOnlyImportInProduction",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}

func (r *TimberTreeNotPlantedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "TimberTreeNotPlanted",
		RuleSet:       "release-engineering",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
	}
}

func (r *VisibleForTestingCallerInNonTestRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "VisibleForTestingCallerInNonTest",
		RuleSet:       "release-engineering",
		DefaultActive: true,
	}
}
