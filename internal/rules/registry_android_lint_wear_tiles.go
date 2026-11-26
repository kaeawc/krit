package rules

import "github.com/kaeawc/krit/internal/rules/api"

// registerAndroidLintWearTileChecks registers the Android Lint 32.4.1 Wear tile preview checks and the
// TileService.startActivityAndCollapse(Intent) check.
func registerAndroidLintWearTileChecks() {
	previews := &SquareAndRoundTilePreviewsRule{AndroidRule: lintAndroidRule("SquareAndRoundTilePreviews",
		"TileProvider does not have round and square previews", ALCIcons, ALSWarning, 6)}
	registerLintManifestRule(previews.AndroidRule, previews, previews.Confidence(), previews.check)

	format := &TilePreviewImageFormatRule{AndroidRule: lintAndroidRule("TilePreviewImageFormat",
		"Tile preview is not compliant with standards", ALCIcons, ALSError, 6)}
	registerLintManifestRule(format.AndroidRule, format, format.Confidence(), format.check)

	collapse := &StartActivityAndCollapseDeprecatedRule{AndroidRule: lintAndroidRule("StartActivityAndCollapseDeprecated",
		"TileService.startActivityAndCollapse(Intent) is deprecated", ALCCorrectness, ALSError, 6)}
	registerLintRule(collapse.AndroidRule, collapse, collapse.Confidence(), collapse.check, lintCallNodeTypes[:2],
		api.NeedsTypeInfo|api.NeedsOracleCallTargets, &api.JavaFactProfile{ReceiverTypesForCallees: []string{tileCollapseMethod}})
}
