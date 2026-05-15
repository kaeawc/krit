package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func implicitDefaultLocaleOracleCallTarget(ctx *api.Context, idx uint32) (target string, oracleAvailable bool) {
	if ctx.Resolver == nil || ctx.File == nil {
		return "", false
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok || cr.Oracle() == nil {
		return "", false
	}
	target = oracleLookupCallTargetFlat(cr.Oracle(), ctx.File, idx)
	target = strings.TrimSpace(target)
	if target == "" || target == flatCallExpressionName(ctx.File, idx) {
		return "", false
	}
	return target, true
}

func implicitDefaultLocaleIsStringFormatTarget(target string) bool {
	target = strings.TrimSpace(strings.ReplaceAll(target, "#", "."))
	if target == "" {
		return false
	}
	return strings.HasPrefix(target, "kotlin.text.format") ||
		strings.HasPrefix(target, "kotlin.text.StringsKt.format") ||
		strings.Contains(target, ".kotlin.text.StringsKt.format") ||
		strings.HasPrefix(target, "java.lang.String.format")
}

func init() {
	registerAccessibilityRules()
	registerAndroidRules()
	registerAndroidCorrectnessRules()
	registerAndroidCorrectnessChecksRules()
	registerAndroidGradleRules()
	registerAndroidIconsRules()
	registerAndroidManifestFeaturesRules()
	registerAndroidManifestI18nRules()
	registerAndroidManifestSecurityRules()
	registerAndroidManifestStructureRules()
	registerAndroidResourceA11yRules()
	registerAndroidResourceIDsRules()
	registerAndroidResourceLayoutRules()
	registerAndroidResourceRtlRules()
	registerAndroidResourceStyleRules()
	registerAndroidResourceValuesRules()
	registerAndroidSecurityRules()
	registerAndroidSourceRules()
	registerAndroidSourceExtraRules()
	registerAndroidUsabilityRules()
	registerCommentsRules()
	registerComplexityRules()
	registerComposeRules()
	registerCoroutinesRules()
	registerDatabaseRules()
	registerDeadcodeRules()
	registerDeadcodeModuleRules()
	registerDiHygieneRules()
	registerEmptyblocksRules()
	registerExceptionsRules()
	registerHotspotRules()
	registerI18nLocaleDefaultForFormattingRules()
	registerI18nMarkupRules()
	registerI18nPluralsRules()
	registerI18nStringConcatRules()
	registerI18nStringContainsHTMLWithoutCDATARules()
	registerI18nStringTemplateRules()
	registerI18nTextDirectionRules()
	registerI18nUnicodeNormalizationRules()
	registerI18nUpperLowerRules()
	registerLayerDependencyViolationRules()
	registerLibraryRules()
	registerLicensingRules()
	registerModuleDependencyCycleRules()
	registerNamingRules()
	registerObservabilityRules()
	registerPackageDependencyCycleRules()
	registerPackageNamingConventionDriftRules()
	registerPerformanceRules()
	registerPotentialbugsExceptionsRules()
	registerPotentialbugsLifecycleRules()
	registerPotentialbugsMiscRules()
	registerPotentialbugsNullsafetyBangbangRules()
	registerPotentialbugsNullsafetyCastsRules()
	registerPotentialbugsNullsafetyRedundantRules()
	registerPotentialbugsPropertiesRules()
	registerPotentialbugsSmartCastRules()
	registerPotentialbugsTypesRules()
	registerPrecompileUnreachableCodeRules()
	registerPrecompileDuplicateLabelRules()
	registerPrecompileDeprecatedSymbolUsedErrorRules()
	registerPrecompileDuplicateDeclarationRules()
	registerCorrectnessOverrideRules()
	registerCorrectnessAbstractRules()
	registerPrivacyAnalyticsRules()
	registerPrivacyPermissionsRules()
	registerPrivacyStorageRules()
	registerPublicToInternalLeakyAbstractionRules()
	registerReleaseEngineeringRules()
	registerResourceCostRules()
	registerSecurityRules()
	registerStyleBracesRules()
	registerStyleClassesRules()
	registerStyleExpressionsRules()
	registerStyleExpressionsExtraRules()
	registerStyleForbiddenRules()
	registerStyleFormatRules()
	registerStyleIdiomaticRules()
	registerStyleIdiomaticDataRules()
	registerStyleRedundantRules()
	registerStyleUnnecessaryRules()
	registerStyleUnusedRules()
	registerSupplyChainRules()
	registerTestingQualityRules()

	applySecurityTaxonomy()
}
