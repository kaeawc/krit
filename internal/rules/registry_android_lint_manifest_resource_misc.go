package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintManifestResourceMiscChecks registers the Android Lint 32.4.1 ScopedStorage, SelectableText,
// CustomSplashScreen, VectorDrawableCompat and TranslucentOrientation checks.
func registerAndroidLintManifestResourceMiscChecks() {
	storage := &ScopedStorageRule{AndroidRule: lintAndroidRule("ScopedStorage", "Affected by scoped storage", ALCCorrectness, ALSWarning, 8)}
	registerLintManifestRule(storage.AndroidRule, storage, storage.Confidence(), storage.check)

	selectable := &SelectableTextRule{AndroidRule: lintAndroidRule("SelectableText", "Dynamic text should probably be selectable",
		ALCUsability, ALSWarning, 7)}
	registerLintResourceRule(selectable.AndroidRule, selectable, selectable.Confidence(), selectable.check)

	splash := &CustomSplashScreenRule{AndroidRule: lintAndroidRule("CustomSplashScreen", "Application-defined Launch Screen",
		ALCCorrectness, ALSWarning, 5)}
	// Registered like registerLintRule, plus OptionalResolver: the supertype chain is followed through the resolver
	// when one is available.
	api.Register(&api.Rule{
		ID: splash.RuleName, Category: splash.RuleSetName, Description: splash.Description(), Sev: api.Severity(splash.Sev),
		NodeTypes: []string{"class_declaration"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: splash.Confidence(), Implementation: splash, Check: lintSkipTestSources(splash.check),
		FileGate: lintFileMentionsGate(splash), NeedsLibraryFacts: true, OptionalResolver: true,
	})

	vector := &VectorDrawableCompatRule{AndroidRule: lintAndroidRule("VectorDrawableCompat", "Using VectorDrawableCompat",
		ALCCorrectness, ALSError, 5)}
	registerLintResourceRule(vector.AndroidRule, vector, vector.Confidence(), vector.check)

	translucent := &TranslucentOrientationRule{AndroidRule: lintAndroidRule("TranslucentOrientation",
		"Mixing screenOrientation and translucency", ALCCorrectness, ALSWarning, 8)}
	registerLintResourceRule(translucent.AndroidRule, translucent, translucent.Confidence(), translucent.check)
}
