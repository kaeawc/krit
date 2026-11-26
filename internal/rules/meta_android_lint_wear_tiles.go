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
			"The receiver must be proven a TileService: an implicit, `this`, `super` or labeled receiver inside a class whose same-file supertype chain reaches TileService (an extension function decides from its receiver type), or a receiver declared as one with an import, star import or qualified name; a TileService subclass in another file is followed only with type information. The argument must be an Intent constructor, a variable declared or initialized as one, an Intent builder call on one, `this` in a scope lambda on one, or (with type information) any Intent-typed expression.",
			"The target SDK (error from 34, warning below) is the file's module's, from its build script; without one the finding is a warning.",
		}}
}

var tilePreviewLimitations = []string{
	"Preview images are read from the drawable folders of every source set of the manifest's module; PNG, JPEG and GIF headers are decoded, a WebP file counts as 0x0 as it does for Lint (no WebP reader on its classpath), and other formats are skipped.",
	"Each AndroidManifest.xml is checked on its own rather than as part of the merged manifest.",
}
