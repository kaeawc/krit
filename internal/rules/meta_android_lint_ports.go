// Descriptor metadata for the Android Lint ports in android_lint_*.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *UnspecifiedImmutableFlagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnspecifiedImmutableFlag", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WeekBasedYearRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WeekBasedYear", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UsableSpaceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UsableSpace", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SetWorldReadableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SetWorldReadable", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SetWorldWritableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SetWorldWritable", RuleSet: "android-lint", DefaultActive: true}
}

func (r *CustomX509TrustManagerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CustomX509TrustManager", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TrustAllX509TrustManagerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TrustAllX509TrustManager", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WakelockTimeoutRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WakelockTimeout", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidWakeLockTagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidWakeLockTag", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ApplySharedPrefRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ApplySharedPref", RuleSet: "android-lint", DefaultActive: true}
}

func (r *StaticFieldLeakRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "StaticFieldLeak", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SuspiciousImportRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SuspiciousImport", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UsingHTTPRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UsingHttp", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ProxyPasswordRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ProxyPassword", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ExactAlarmRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ExactAlarm", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SelectedPhotoAccessRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SelectedPhotoAccess", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DataExtractionRulesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DataExtractionRules", RuleSet: "android-lint", DefaultActive: true}
}

func (r *RedundantLabelRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RedundantLabel", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MissingTvBannerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingTvBanner", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsupportedTvHardwareRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsupportedTvHardware", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ImpliedTouchscreenHardwareRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ImpliedTouchscreenHardware", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TypographyDashesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TypographyDashes", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TypographyEllipsisRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TypographyEllipsis", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TypographyFractionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TypographyFractions", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TypographyOtherRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TypographyOther", RuleSet: "android-lint", DefaultActive: true}
}

func (r *TypographyQuotesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TypographyQuotes", RuleSet: "android-lint", DefaultActive: false, OptInReason: api.OptInReasonAndroidOnly}
}

func (r *SignatureOrSystemPermissionsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SignatureOrSystemPermissions", RuleSet: "android-lint", DefaultActive: true}
}

func (r *HighSamplingRateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "HighSamplingRate", RuleSet: "android-lint", DefaultActive: true}
}

func (r *KeyboardInaccessibleWidgetRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "KeyboardInaccessibleWidget", RuleSet: "android-lint", DefaultActive: true}
}

func (r *BottomAppBarRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BottomAppBar", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidImeActionIDRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidImeActionId", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MenuTitleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MenuTitle", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UsingC2DMRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UsingC2DM", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidPermissionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidPermission", RuleSet: "android-lint", DefaultActive: true}
}

func (r *CoarseFineLocationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CoarseFineLocation", RuleSet: "android-lint", DefaultActive: true}
}

func (r *LeanbackUsesWifiRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LeanbackUsesWifi", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnnecessaryRequiredFeatureRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnnecessaryRequiredFeature", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidManifestAttributeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidManifestAttribute", RuleSet: "android-lint", DefaultActive: true}
}

func (r *EllipsizeMaxLinesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "EllipsizeMaxLines", RuleSet: "android-lint", DefaultActive: true}
}

func (r *NetworkSecurityConfigRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NetworkSecurityConfig", RuleSet: "android-lint", DefaultActive: true}
}

func (r *PinSetExpiryRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "PinSetExpiry", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MissingBackupPinRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingBackupPin", RuleSet: "android-lint", DefaultActive: true}
}

func (r *AcceptsUserCertificatesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AcceptsUserCertificates", RuleSet: "android-lint", DefaultActive: true}
}

func (r *VectorRasterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "VectorRaster", RuleSet: "android-lint", DefaultActive: true}
}

func (r *VectorPathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "VectorPath", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidVectorPathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidVectorPath", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnspecifiedRegisterReceiverFlagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnspecifiedRegisterReceiverFlag", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SSLCertificateSocketFactoryGetInsecureRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SSLCertificateSocketFactoryGetInsecure", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SSLCertificateSocketFactoryCreateSocketRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SSLCertificateSocketFactoryCreateSocket", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DeletedProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DeletedProvider", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WebViewClientOnReceivedSslErrorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WebViewClientOnReceivedSslError", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MissingInflatedIDRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingInflatedId", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MissingTranslationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingTranslation", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ExtraTranslationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ExtraTranslation", RuleSet: "android-lint", DefaultActive: true}
}

func (r *MissingDefaultResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingDefaultResource", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UntranslatableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "Untranslatable", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DuplicateDefinitionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DuplicateDefinition", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UnsafeDynamicallyLoadedCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeDynamicallyLoadedCode", RuleSet: "android-lint", DefaultActive: true}
}

func (r *BadHostnameVerifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BadHostnameVerifier", RuleSet: "android-lint", DefaultActive: true}
}

func (r *HardwareIDsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "HardwareIds", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DeprecatedProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DeprecatedProvider", RuleSet: "android-lint", DefaultActive: true}
}

func (r *ResourceCycleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ResourceCycle", RuleSet: "android-lint", DefaultActive: true}
}

func (r *InvalidWearFeatureAttributeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidWearFeatureAttribute", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WearStandaloneAppFlagRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearStandaloneAppFlag", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WearRecentsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearRecents", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WearSplashScreenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearSplashScreen", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WatchFaceEditorRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WatchFaceEditor", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WatchFaceFormatDeclaresHasNoCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WatchFaceFormatDeclaresHasNoCode", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WatchFaceFormatMissingVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WatchFaceFormatMissingVersion", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WatchFaceFormatInvalidVersionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WatchFaceFormatInvalidVersion", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WearPasswordInputRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearPasswordInput", RuleSet: "android-lint", DefaultActive: true}
}

func (r *NotInterpolatedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NotInterpolated", RuleSet: "android-lint", DefaultActive: true}
}

func (r *BomWithoutPlatformRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BomWithoutPlatform", RuleSet: "android-lint", DefaultActive: true}
}

func (r *AnnotationProcessorOnCompilePathRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "AnnotationProcessorOnCompilePath", RuleSet: "android-lint", DefaultActive: true}
}

func (r *UseOfBundledGooglePlayServicesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseOfBundledGooglePlayServices", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DevModeObsoleteRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DevModeObsolete", RuleSet: "android-lint", DefaultActive: true}
}

func (r *HighAppVersionCodeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "HighAppVersionCode", RuleSet: "android-lint", DefaultActive: true}
}

func (r *DataBindingWithoutKaptRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DataBindingWithoutKapt", RuleSet: "android-lint", DefaultActive: true}
}

func (r *SuspiciousIndentationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SuspiciousIndentation", RuleSet: "android-lint", DefaultActive: true}
}
