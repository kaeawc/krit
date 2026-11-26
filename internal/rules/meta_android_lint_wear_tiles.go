package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *SquareAndRoundTilePreviewsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SquareAndRoundTilePreviews", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: tilePreviewLimitations}
}

func (r *TilePreviewImageFormatRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TilePreviewImageFormat", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: tilePreviewLimitations}
}

func (r *StartActivityAndCollapseDeprecatedRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "StartActivityAndCollapseDeprecated", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The receiver must be proven a TileService: an implicit or `this` receiver inside a class whose same-file supertype chain reaches TileService, or a receiver declared as one; the argument must be an Intent constructor, a variable declared or initialized as one, or an Intent builder call on one.",
			"The target SDK (error from 34, warning below) is read from the build scripts; without one the finding is a warning.",
		}}
}

var tilePreviewLimitations = []string{
	"Preview images are read from the drawable folders of every source set of the manifest's module; PNG, JPEG and GIF headers are decoded, a WebP file counts as 0x0 as it does for Lint (no WebP reader on its classpath), and other formats are skipped.",
	"Each AndroidManifest.xml is checked on its own rather than as part of the merged manifest.",
}
