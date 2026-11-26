package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintWearConfigChecks registers the Android Lint 32.4.1 watch face configuration, back navigation and
// MaterialTheme checks.
func registerAndroidLintWearConfigChecks() {
	duplicate := &WearableActionDuplicateRule{AndroidRule: lintAndroidRule("WearableActionDuplicate",
		"Duplicate watch face configuration activities found", ALCCorrectness, ALSWarning, 5)}
	registerWearConfigManifestRule(duplicate.AndroidRule, duplicate, duplicate.Confidence(), duplicate.check)

	action := &WearableConfigurationActionRule{AndroidRule: lintAndroidRule("WearableConfigurationAction",
		"Wear configuration action metadata must match an activity", ALCCorrectness, ALSWarning, 5)}
	registerWearConfigManifestRule(action.AndroidRule, action, action.Confidence(), action.check)

	androidx := &WatchFaceForAndroidXRule{AndroidRule: lintAndroidRule("WatchFaceForAndroidX",
		"AndroidX watch faces must use action `WATCH_FACE_EDITOR`", ALCCorrectness, ALSWarning, 5)}
	registerWearConfigManifestRule(androidx.AndroidRule, androidx, androidx.Confidence(), androidx.check)

	back := &WearBackNavigationRule{AndroidRule: lintAndroidRule("WearBackNavigation", "Wear: Disabling Back navigation",
		ALCUsability, ALSWarning, 5)}
	registerLintResourceRule(back.AndroidRule, back, back.Confidence(), back.check)

	theme := &WearMaterialThemeRule{AndroidRule: lintAndroidRule("WearMaterialTheme",
		"Using not non-Wear `MaterialTheme` in a Wear OS project", ALCCorrectness, ALSError, 6)}
	api.Register(&api.Rule{
		ID: theme.RuleName, Category: theme.RuleSetName, Description: theme.Description(), Sev: api.Severity(theme.Sev),
		Needs: api.NeedsManifest | api.NeedsParsedFiles | api.NeedsConcurrent, AndroidDeps: uint32(AndroidDepManifest),
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: theme.Confidence(), Implementation: theme, Check: theme.check,
	})
}

// registerWearConfigManifestRule registers a manifest Lint port that also reads the project's dependencies.
func registerWearConfigManifestRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: confidence, Implementation: impl,
		NeedsLibraryFacts: true, Check: check,
	})
}
