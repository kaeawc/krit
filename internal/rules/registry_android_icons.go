package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type iconRegistryRule interface {
	Name() string
	RuleSet() string
	Severity() string
	Description() string
	Confidence() float64
}

func registerAndroidIconRule(r iconRegistryRule, check func(*api.Context), opts ...func(*api.Rule)) {
	rule := &api.Rule{
		ID:             r.Name(),
		Category:       r.RuleSet(),
		Description:    r.Description(),
		Sev:            api.Severity(r.Severity()),
		Languages:      []scanner.Language{scanner.LangXML},
		AndroidDeps:    uint32(AndroidDepIcons),
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          check,
	}
	for _, opt := range opts {
		opt(rule)
	}
	api.Register(rule)
}

func iconRuleNeedsLibraryFacts(r *api.Rule) { r.NeedsLibraryFacts = true }

func registerAndroidIconsRules() {
	registerAndroidIconRule(&IconDensitiesRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconDensities", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconDensities", Brief: "Missing density variants for icon",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconDensities(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconDipSizeRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconDipSize", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconDipSize", Brief: "Icon dimensions don't match expected DPI ratios",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconDipSize(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconDuplicatesRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconDuplicates", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconDuplicates", Brief: "Same image across densities without scaling",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconDuplicates(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&GifUsageRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "GifUsage", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "GifUsage", Brief: "GIF file in resources",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckGifUsage(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&ConvertToWebpRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "ConvertToWebp", RuleSetName: androidRuleSet, Sev: "informational"},
		IssueID:  "ConvertToWebp", Brief: "Large PNG could be smaller as WebP",
		Category: ALCIcons, ALSeverity: ALSInformational, Priority: 3,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckConvertToWebp(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconMissingDensityFolderRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconMissingDensityFolder", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconMissingDensityFolder", Brief: "Missing density folder",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconMissingDensityFolder(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconExpectedSizeRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconExpectedSize", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconExpectedSize", Brief: "Launcher icon not at expected size",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconExpectedSize(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconExtensionRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconExtension", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconExtension", Brief: "Icon format does not match the file extension",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconExtension(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconLocationRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconLocation", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconLocation", Brief: "Image defined in density-independent drawable folder",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconLocation(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconMixedNinePatchRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconMixedNinePatch", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconMixedNinePatch", Brief: "Clashing PNG and 9-PNG files",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconMixedNinePatch(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconXMLAndPngRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconXmlAndPng", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconXmlAndPng", Brief: "Icon is specified both as .xml file and as a bitmap",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 7,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconXMLAndPng(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconNoDpiRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconNoDpi", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconNoDpi", Brief: "Icon in both nodpi and density-specific folder",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 4,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconNoDpi(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconDuplicatesConfigRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconDuplicatesConfig", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconDuplicatesConfig", Brief: "Identical icons across configuration folders",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 3,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconDuplicatesConfig(ctx.IconIndex, ctx.Collector)
	})

	registerAndroidIconRule(&IconColorsRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconColors", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconColors", Brief: "Icon colors do not follow the recommended visual style",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 7,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconColorsWithFacts(ctx.IconIndex, ctx.Collector, ctx.LibraryFacts)
	}, iconRuleNeedsLibraryFacts)

	registerAndroidIconRule(&IconLauncherShapeRule{AndroidRule: AndroidRule{
		BaseRule: BaseRule{RuleName: "IconLauncherShape", RuleSetName: androidRuleSet, Sev: "warning"},
		IssueID:  "IconLauncherShape", Brief: "Launcher icon has transparent corners",
		Category: ALCIcons, ALSeverity: ALSWarning, Priority: 5,
		Origin: "AOSP Android Lint",
	}}, func(ctx *api.Context) {
		CheckIconLauncherShape(ctx.IconIndex, ctx.Collector)
	})
}
