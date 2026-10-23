package rules

import (
	"fmt"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// IconResourceBase provides the AndroidDepIcons declaration shared by every
// icon rule. The dispatcher supplies an IconIndex to these rules.
type IconResourceBase struct{}

func (IconResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepIcons
}

// IconNoDpiRule detects icons in both drawable-nodpi and a density-specific folder.
type IconNoDpiRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconNoDpiRule) Confidence() float64 { return api.ConfidenceMedium }

// IconDuplicatesConfigRule detects identical icons across different configuration folders.
type IconDuplicatesConfigRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDuplicatesConfigRule) Confidence() float64 { return api.ConfidenceMedium }

// IconDensitiesRule checks for missing density variants.
type IconDensitiesRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDensitiesRule) Confidence() float64 { return api.ConfidenceMedium }

// IconDipSizeRule checks that icon dimensions match expected DPI ratios.
type IconDipSizeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDipSizeRule) Confidence() float64 { return api.ConfidenceMedium }

// IconDuplicatesRule detects identical images across densities.
type IconDuplicatesRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDuplicatesRule) Confidence() float64 { return api.ConfidenceMedium }

// GifUsageRule detects GIF files in resources.
type GifUsageRule struct {
	IconResourceBase
	AndroidRule
}

func (r *GifUsageRule) Confidence() float64 { return api.ConfidenceMedium }

// ConvertToWebpRule detects large PNGs that could be smaller as WebP.
type ConvertToWebpRule struct {
	IconResourceBase
	AndroidRule
}

func (r *ConvertToWebpRule) Confidence() float64 { return api.ConfidenceMedium }

// IconMissingDensityFolderRule detects missing density folders.
type IconMissingDensityFolderRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconMissingDensityFolderRule) Confidence() float64 { return api.ConfidenceMedium }

// IconExpectedSizeRule checks that launcher icons have expected sizes.
type IconExpectedSizeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconExpectedSizeRule) Confidence() float64 { return api.ConfidenceMedium }

// IconExtensionRule detects icon files whose extension does not match the contents.
type IconExtensionRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconExtensionRule) Confidence() float64 { return api.ConfidenceMedium }

// IconLocationRule detects bitmap icons placed in density-independent drawable folders.
type IconLocationRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconLocationRule) Confidence() float64 { return api.ConfidenceMedium }

// IconMixedNinePatchRule detects PNG and 9-patch files with the same resource name.
type IconMixedNinePatchRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconMixedNinePatchRule) Confidence() float64 { return api.ConfidenceMedium }

// IconXMLAndPngRule detects drawable XML and bitmap files with the same resource name.
type IconXMLAndPngRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconXMLAndPngRule) Confidence() float64 { return api.ConfidenceMedium }

// IconColorsRule checks notification and action bar icon color constraints.
type IconColorsRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconColorsRule) Confidence() float64 { return api.ConfidenceMedium }

// IconLauncherShapeRule checks launcher icon shape transparency.
type IconLauncherShapeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconLauncherShapeRule) Confidence() float64 { return api.ConfidenceMedium }

// --- IconIndex-backed check functions ---

// webpThresholdBytes is the minimum PNG size to suggest WebP conversion (10 KB).
const webpThresholdBytes = 10 * 1024

// launcherPrefixes identifies launcher icon resources by name prefix.
var launcherPrefixes = []string{"ic_launcher", "ic_launcher_round", "ic_launcher_foreground"}

// expectedLauncherSizes maps density to the expected launcher icon pixel size (48dp base).
var expectedLauncherSizes = map[string]int{
	"ldpi":    36,
	"mdpi":    48,
	"hdpi":    72,
	"xhdpi":   96,
	"xxhdpi":  144,
	"xxxhdpi": 192,
}

// CheckIconDuplicates reports icons that are identical across densities (same hash).
func CheckIconDuplicates(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		// Group by hash
		hashMap := make(map[string][]string) // hash -> densities
		pathMap := make(map[string]string)   // hash -> first path
		for _, ic := range icons {
			if ic.Hash == "" {
				continue
			}
			hashMap[ic.Hash] = append(hashMap[ic.Hash], ic.Density)
			if _, ok := pathMap[ic.Hash]; !ok {
				pathMap[ic.Hash] = ic.Path
			}
		}
		for hash, densities := range hashMap {
			if len(densities) > 1 {
				sort.Strings(densities)
				findings = append(findings, scanner.Finding{
					File:     pathMap[hash],
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDuplicates",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' is identical across densities %s (no scaling applied)",
						name, strings.Join(densities, ", ")),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// CheckGifUsage reports any GIF files in the resource directories.
func CheckGifUsage(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range idx.Icons {
		if ic.Format == "gif" {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "GifUsage",
				Severity: "warning",
				Message:  fmt.Sprintf("GIF file '%s' in resources. Use animated WebP or vector drawable instead.", ic.Name),
				BinaryFix: &scanner.BinaryFix{
					Type:         scanner.BinaryFixConvertWebP,
					SourcePath:   ic.Path,
					TargetPath:   "", // will generate .webp alongside
					Description:  "Convert GIF to WebP format",
					DeleteSource: true,
				},
			})
		}
	}
}

// CheckConvertToWebp reports large PNG files that could be converted to WebP.
func CheckConvertToWebp(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range idx.Icons {
		if ic.Format == "png" && ic.Size >= webpThresholdBytes {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "ConvertToWebp",
				Severity: "informational",
				Message: fmt.Sprintf("PNG file '%s' is %d bytes. Consider converting to WebP for smaller size.",
					ic.Name, ic.Size),
				BinaryFix: &scanner.BinaryFix{
					Type:         scanner.BinaryFixConvertWebP,
					SourcePath:   ic.Path,
					TargetPath:   "", // will generate .webp alongside
					Description:  "Convert to WebP format and remove original PNG",
					DeleteSource: true,
				},
			})
		}
	}
}

// CheckIconExtension reports icon files where the file extension doesn't match the detected content format.
func CheckIconExtension(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range idx.Icons {
		extFmt := ic.ExtensionFormat()
		if extFmt == "" || ic.Format == "" {
			continue
		}
		if extFmt != ic.Format {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconExtension",
				Severity: "warning",
				Message: fmt.Sprintf("Icon '%s' has extension '%s' but content is %s",
					ic.Name, extFmt, ic.Format),
			})
		}
	}
}

// CheckIconLocation reports launcher icons placed in drawable-* folders instead of mipmap-*.
func CheckIconLocation(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range idx.Icons {
		if !strings.HasPrefix(ic.Name, "ic_launcher") {
			continue
		}
		if strings.Contains(ic.Path, "drawable") {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconLocation",
				Severity: "warning",
				Message: fmt.Sprintf("Launcher icon '%s' should be in mipmap-* folder, not drawable-*",
					ic.Name),
			})
		}
	}
}

// CheckIconMixedNinePatch reports icons that have a mix of nine-patch (.9.png) and non-nine-patch variants.
func CheckIconMixedNinePatch(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		hasNinePatch := false
		hasRegular := false
		var samplePath string
		for _, ic := range icons {
			if ic.IsNinePatch() {
				hasNinePatch = true
			} else {
				hasRegular = true
			}
			if samplePath == "" {
				samplePath = ic.Path
			}
		}
		if hasNinePatch && hasRegular {
			findings = append(findings, scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconMixedNinePatch",
				Severity: "warning",
				Message: fmt.Sprintf("Icon '%s' has a mix of nine-patch and non-nine-patch variants",
					name),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// CheckIconXMLAndPng reports resources that exist as both XML vector and raster (PNG/JPG/WebP) formats.
func CheckIconXMLAndPng(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		hasXML := false
		hasRaster := false
		var samplePath string
		for _, ic := range icons {
			switch ic.Format {
			case "xml":
				hasXML = true
			case "png", "jpg", "webp", "gif":
				hasRaster = true
			}
			if samplePath == "" {
				samplePath = ic.Path
			}
		}
		if hasXML && hasRaster {
			findings = append(findings, scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconXmlAndPng",
				Severity: "warning",
				Message: fmt.Sprintf("Icon '%s' exists as both XML vector and raster image",
					name),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}
