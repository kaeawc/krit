package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *ScopedStorageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ScopedStorage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Each AndroidManifest.xml is checked with its own module's target SDK and its own MANAGE_EXTERNAL_STORAGE and requestLegacyExternalStorage declarations; Lint reads the main project's merged manifest.",
		}}
}

func (r *SelectableTextRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SelectableText", RuleSet: "android-lint", DefaultActive: false, OptInReason: api.OptInReasonAndroidOnly}
}

func (r *CustomSplashScreenRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CustomSplashScreen", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The supertype chain is followed through same-file classes, the resolver and known framework and AndroidX subclasses of Activity, Fragment and View; a class whose chain leaves those is not reported.",
		}}
}

func (r *VectorDrawableCompatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "VectorDrawableCompat", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"minSdk and vectorDrawables.useSupportLibrary are read from literal assignments in the module's build script, and a module without a literal minSdk is not checked; the module path in the message comes from the nearest settings script and is `build.gradle` without one.",
		}}
}

func (r *TranslucentOrientationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TranslucentOrientation", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Themes set at runtime with setTheme() in an activity are not followed.",
		}}
}
