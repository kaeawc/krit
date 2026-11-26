package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *WearableActionDuplicateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearableActionDuplicate", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: wearConfigLimitations}
}

func (r *WearableConfigurationActionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearableConfigurationAction", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: wearConfigLimitations}
}

func (r *WatchFaceForAndroidXRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WatchFaceForAndroidX", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The androidx.wear.watchface:watchface dependency is read from the build scripts and version catalog, not the resolved classpath, and counts for every module of the scan.",
		}}
}

func (r *WearBackNavigationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearBackNavigation", RuleSet: "android-lint", DefaultActive: true}
}

func (r *WearMaterialThemeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WearMaterialTheme", RuleSet: "android-lint", DefaultActive: true}
}

var wearConfigLimitations = []string{
	"Each AndroidManifest.xml is checked on its own; Lint checks the merged manifest, so an activity or metadata declared only in a variant or library manifest is not seen.",
	"The androidx.wear.watchface:watchface dependency is read from the build scripts and version catalog; the check is skipped only when they prove it absent.",
}
