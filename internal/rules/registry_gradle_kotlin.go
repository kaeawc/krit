package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// registerGradleKotlinRules registers the Kotlin toolchain migration rules for Gradle build scripts.
func registerGradleKotlinRules() {
	{
		r := &KotlinOptionsDslRule{BaseRule: BaseRule{RuleName: "KotlinOptionsDsl", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects the removed kotlinOptions Gradle DSL, an error since Kotlin 2.2.0, and points to compilerOptions."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic, Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &JvmDefaultFlagRule{BaseRule: BaseRule{RuleName: "JvmDefaultFlag", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects the deprecated -Xjvm-default compiler argument and points to jvmDefault and -jvm-default (Kotlin 2.2.0)."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic, Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ObsoleteKotlinVersionFlagsRule{BaseRule: BaseRule{RuleName: "ObsoleteKotlinVersionFlags", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Kotlin compiler flags, language versions and gradle.properties keys that the project's Kotlin version made redundant or removed."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic, Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &KotlinAndroidPluginWithBuiltInKotlinRule{BaseRule: BaseRule{RuleName: "KotlinAndroidPluginWithBuiltInKotlin", RuleSetName: supplyChainRuleSet, Sev: "error", Desc: "Detects the org.jetbrains.kotlin.android plugin in a project on Android Gradle plugin 9.0 or newer, which has built-in Kotlin."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic, Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &ComposeCompilerRemovedFlagsRule{BaseRule: BaseRule{RuleName: "ComposeCompilerRemovedFlags", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Compose compiler Gradle plugin options and feature flags that are errors since Kotlin 2.4.0 or are being removed."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic, Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &KaptUsageRule{BaseRule: BaseRule{RuleName: "KaptUsage", RuleSetName: supplyChainRuleSet, Sev: "info", Desc: "Detects kapt plugins and kapt dependency configurations, and suggests KSP for processors that support it."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Scripts: api.ScriptsGradle, Implementation: r,
			Check: r.check,
		})
	}
}
