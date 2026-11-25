package rules

// registerAndroidLintSecurityProjectPorts registers the Android Lint 32.4.1 Security checks that read the manifest
// and project folders, in android_lint_security_project.go.
func registerAndroidLintSecurityProjectPorts() {
	tile := &TileProviderPermissionsRule{AndroidRule: lintAndroidRule("TileProviderPermissions",
		"TileProvider does not set permission", ALCSecurity, ALSWarning, 6)}
	registerLintManifestRule(tile.AndroidRule, tile, tile.Confidence(), tile.check)
	native := &UnsafeNativeCodeLocationRule{AndroidRule: lintAndroidRule("UnsafeNativeCodeLocation",
		"Native code outside library directory", ALCSecurity, ALSWarning, 4)}
	registerLintResourceRule(native.AndroidRule, native, native.Confidence(), native.check)
}
