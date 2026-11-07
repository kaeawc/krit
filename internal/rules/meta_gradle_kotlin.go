package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func (r *KotlinOptionsDslRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KotlinOptionsDsl",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		FixLevel:      "semantic",
		KnownLimitations: []string{
			"Reads build.gradle(.kts) only; convention plugins in .kt files are not scanned.",
			"Severity follows the Kotlin version found in build scripts and the version catalog; an unknown version reports a medium-confidence warning.",
			"The fix covers kotlinOptions inside a Kotlin task configuration when every statement is jvmTarget, freeCompilerArgs +=, a boolean flag, languageVersion or apiVersion; android { kotlinOptions {} } is report-only because it moves to kotlin { compilerOptions {} }.",
		},
	}
}

func (r *JvmDefaultFlagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "JvmDefaultFlag",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		FixLevel:      "semantic",
		KnownLimitations: []string{
			"Reports only when the Kotlin version is known to be 2.2.0 or newer; an unknown version is skipped because older compilers reject -jvm-default.",
			"The fix rewrites -Xjvm-default=all-compatibility, all and disable; other values have no documented mapping and are report-only.",
			"Only string literals in build scripts are matched; arguments built from variables are not.",
		},
	}
}

func (r *ObsoleteKotlinVersionFlagsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObsoleteKotlinVersionFlags",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason:   api.OptInReasonDomainSpecific,
		FixLevel:      "semantic",
		RelatedRules:  []string{"JvmDefaultFlag", "KotlinOptionsDsl"},
		KnownLimitations: []string{
			"Reports a construct only when the Kotlin version is known and has reached the version that made it obsolete; an unknown version is skipped.",
			"A script that pins an older languageVersion keeps its feature flags until that version catches up.",
			"-Xjvm-default is reported by JvmDefaultFlag, not by this rule.",
			"gradle.properties keys are checked from the settings script next to the file; a gradle.properties without a settings script is not read.",
			"The fix deletes a flag that is a plain list element or the only argument of a freeCompilerArgs statement, and a removed gradle.properties key; language versions and context receivers are report-only.",
		},
	}
}

func (r *KotlinAndroidPluginWithBuiltInKotlinRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KotlinAndroidPluginWithBuiltInKotlin",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		FixLevel:      "semantic",
		RelatedRules:  []string{"KotlinOptionsDsl", "KaptUsage"},
		KnownLimitations: []string{
			"Reports only when the Android Gradle plugin version is known to be 9.0 or newer, from a plugin version literal or the version catalog.",
			"A project with android.builtInKotlin=false in an applicable gradle.properties is skipped, unless the script applies com.android.built-in-kotlin.",
			"Plugin declarations with apply false are not reported, and plugins applied from convention plugins are not seen.",
			"The fix deletes the plugin line only when the script has no kapt plugin, kotlinOptions or sourceSets block that needs a migration first.",
		},
	}
}

func (r *ComposeCompilerRemovedFlagsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ComposeCompilerRemovedFlags",
		RuleSet:       "supply-chain",
		DefaultActive: true,
		FixLevel:      "semantic",
		KnownLimitations: []string{
			"Severity follows the Kotlin version found in build scripts and the version catalog; an unknown version reports a medium-confidence warning.",
			"OptimizeNonSkippingGroups and PausableComposition are reported only from Kotlin 2.4.0, where they are deprecated.",
			"The fix deletes only `enableStrongSkippingMode = true` and `enableIntrinsicRemember = true` lines, which restate the defaults.",
			"Options set in convention plugins written in .kt files are not scanned.",
		},
	}
}

func (r *KaptUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "KaptUsage",
		RuleSet:       "supply-chain",
		DefaultActive: false,
		OptInReason:   api.OptInReasonProjectPolicy,
		RelatedRules:  []string{"KotlinAndroidPluginWithBuiltInKotlin"},
		KnownLimitations: []string{
			"Only Room, Moshi codegen, Glide, Dagger and Hilt are named as having a KSP variant; other processors get a generic pointer.",
			"A catalog reference such as kapt(libs.room.compiler) is resolved through the version catalog; other forms get the generic message.",
			"No autofix: moving a processor to KSP can need a library version bump and a plugin change.",
			"kapt configurations are matched inside dependencies blocks only.",
		},
	}
}
