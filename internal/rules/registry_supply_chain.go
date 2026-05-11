package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerSupplyChainRules() {

	// --- from supply_chain.go ---
	{
		r := &CompileSdkMismatchAcrossModulesRule{BaseRule: BaseRule{RuleName: "CompileSdkMismatchAcrossModules", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Android modules whose compileSdk is lower than the maximum compileSdk in the project."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &GradleWrapperValidationActionRule{BaseRule: BaseRule{RuleName: "GradleWrapperValidationAction", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects GitHub Actions Gradle setup steps missing wrapper validation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &JvmTargetMismatchRule{BaseRule: BaseRule{RuleName: "JvmTargetMismatch", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Kotlin JVM target and Java compatibility settings that disagree."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &KotlinVersionMismatchAcrossModulesRule{BaseRule: BaseRule{RuleName: "KotlinVersionMismatchAcrossModules", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects modules whose Kotlin JVM target or toolchain differs from the project majority."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MissingGradleChecksumsRule{BaseRule: BaseRule{RuleName: "MissingGradleChecksums", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Gradle dependency locking declarations without a sibling gradle.lockfile."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_dependencies.go ---
	{
		r := &DependencyFromBintrayRule{BaseRule: BaseRule{RuleName: "DependencyFromBintray", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects Gradle repositories hosted on retired Bintray endpoints."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencySnapshotInReleaseRule{BaseRule: BaseRule{RuleName: "DependencySnapshotInRelease", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects Gradle dependencies that use SNAPSHOT versions in release builds."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyVerificationDisabledRule{BaseRule: BaseRule{RuleName: "DependencyVerificationDisabled", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects Gradle dependency verification disabled in gradle.properties."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyWithoutGroupRule{BaseRule: BaseRule{RuleName: "DependencyWithoutGroup", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects legacy Gradle dependency coordinates that omit the group."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependenciesInRootProjectRule{
			BaseRule:              BaseRule{RuleName: "DependenciesInRootProject", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects dependency declarations directly in the root Gradle project."},
			AllowedConfigurations: []string{"classpath", "detektPlugins"},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyFromHTTPRule{
			BaseRule:      BaseRule{RuleName: "DependencyFromHttp", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects Gradle Maven/Ivy repositories fetched over plaintext HTTP."},
			AllowLoopback: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DependencyFromJcenterRule{BaseRule: BaseRule{RuleName: "DependencyFromJcenter", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects Gradle repositories that still use JCenter."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_catalog_buildsrc_mismatch.go ---
	{
		r := &VersionCatalogBuildSrcMismatchRule{BaseRule: BaseRule{RuleName: "VersionCatalogBuildSrcMismatch", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects buildSrc/build-logic dependency coordinates whose version disagrees with libs.versions.toml."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_catalog_raw_version_in_build.go ---
	{
		r := &VersionCatalogRawVersionInBuildRule{
			BaseRule: BaseRule{RuleName: "VersionCatalogRawVersionInBuild", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects build.gradle(.kts) dependency literals whose group:name coordinate is already declared in libs.versions.toml."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_catalog.go ---
	{
		r := &VersionCatalogUnusedRule{
			BaseRule:              BaseRule{RuleName: "VersionCatalogUnused", RuleSetName: supplyChainRuleSet, Sev: "info", Desc: "Detects libs.versions.toml aliases not referenced by any build script or convention plugin."},
			ScanConventionPlugins: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_catalog_duplicate_version.go ---
	{
		r := &VersionCatalogDuplicateVersionRule{
			BaseRule: BaseRule{RuleName: "VersionCatalogDuplicateVersion", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects [versions] aliases in libs.versions.toml that share the same literal version string."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}

	// --- from supply_drift.go ---
	{
		r := &ApplyPluginTwiceRule{
			BaseRule: BaseRule{RuleName: "ApplyPluginTwice", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects a Gradle plugin applied in both plugins { } and apply(plugin = ...) in the same build file."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixIdiomatic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ConventionPluginAppliedToWrongTargetRule{
			BaseRule: BaseRule{RuleName: "ConventionPluginAppliedToWrongTarget", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects convention plugins applied to incompatible Gradle module targets."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ConfigurationsAllSideEffectRule{
			BaseRule:                 BaseRule{RuleName: "ConfigurationsAllSideEffect", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects configurations.all blocks that mutate dependency resolution globally."},
			AllowInConventionPlugins: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
