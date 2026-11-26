package rules

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
	registerLintRule(splash.AndroidRule, splash, splash.Confidence(), splash.check, []string{"class_declaration"}, 0, nil)

	vector := &VectorDrawableCompatRule{AndroidRule: lintAndroidRule("VectorDrawableCompat", "Using VectorDrawableCompat",
		ALCCorrectness, ALSError, 5)}
	registerLintResourceRule(vector.AndroidRule, vector, vector.Confidence(), vector.check)

	translucent := &TranslucentOrientationRule{AndroidRule: lintAndroidRule("TranslucentOrientation",
		"Mixing screenOrientation and translucency", ALCCorrectness, ALSWarning, 8)}
	registerLintResourceRule(translucent.AndroidRule, translucent, translucent.Confidence(), translucent.check)
}
