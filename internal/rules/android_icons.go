package rules

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/scanner"
)

// IconResourceBase provides the AndroidDepIcons declaration shared by every
// icon rule. The v2 dispatcher supplies an IconIndex to these rules.
type IconResourceBase struct{}

func (IconResourceBase) AndroidDependencies() AndroidDataDependency {
	return AndroidDepIcons
}

// IconNoDpiRule detects icons in both drawable-nodpi and a density-specific folder.
type IconNoDpiRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconNoDpiRule) Confidence() float64 { return 0.75 }

// IconDuplicatesConfigRule detects identical icons across different configuration folders.
type IconDuplicatesConfigRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDuplicatesConfigRule) Confidence() float64 { return 0.75 }

// IconDensitiesRule checks for missing density variants.
type IconDensitiesRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDensitiesRule) Confidence() float64 { return 0.75 }

// IconDipSizeRule checks that icon dimensions match expected DPI ratios.
type IconDipSizeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDipSizeRule) Confidence() float64 { return 0.75 }

// IconDuplicatesRule detects identical images across densities.
type IconDuplicatesRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconDuplicatesRule) Confidence() float64 { return 0.75 }

// GifUsageRule detects GIF files in resources.
type GifUsageRule struct {
	IconResourceBase
	AndroidRule
}

func (r *GifUsageRule) Confidence() float64 { return 0.75 }

// ConvertToWebpRule detects large PNGs that could be smaller as WebP.
type ConvertToWebpRule struct {
	IconResourceBase
	AndroidRule
}

func (r *ConvertToWebpRule) Confidence() float64 { return 0.75 }

// IconMissingDensityFolderRule detects missing density folders.
type IconMissingDensityFolderRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconMissingDensityFolderRule) Confidence() float64 { return 0.75 }

// IconExpectedSizeRule checks that launcher icons have expected sizes.
type IconExpectedSizeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconExpectedSizeRule) Confidence() float64 { return 0.75 }

// IconExtensionRule detects icon files whose extension does not match the contents.
type IconExtensionRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconExtensionRule) Confidence() float64 { return 0.75 }

// IconLocationRule detects bitmap icons placed in density-independent drawable folders.
type IconLocationRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconLocationRule) Confidence() float64 { return 0.75 }

// IconMixedNinePatchRule detects PNG and 9-patch files with the same resource name.
type IconMixedNinePatchRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconMixedNinePatchRule) Confidence() float64 { return 0.75 }

// IconXmlAndPngRule detects drawable XML and bitmap files with the same resource name.
type IconXmlAndPngRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconXmlAndPngRule) Confidence() float64 { return 0.75 }

// IconColorsRule checks notification and action bar icon color constraints.
type IconColorsRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconColorsRule) Confidence() float64 { return 0.75 }

// IconLauncherShapeRule checks launcher icon shape transparency.
type IconLauncherShapeRule struct {
	IconResourceBase
	AndroidRule
}

func (r *IconLauncherShapeRule) Confidence() float64 { return 0.75 }

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

// CheckIconDensities reports icons that exist in some densities but not others.
func CheckIconDensities(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue // need at least 2 densities to compare
		}
		present := make(map[string]bool)
		var samplePath string
		for _, ic := range icons {
			present[ic.Density] = true
			if samplePath == "" {
				samplePath = ic.Path
			}
		}
		// Check which standard densities are missing
		var missing []string
		for _, d := range android.KnownDensities {
			if !present[d.Name] {
				missing = append(missing, d.Name)
			}
		}
		if len(missing) > 0 && len(missing) < len(android.KnownDensities) {
			findings = append(findings, scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconDensities",
				Severity: "warning",
				Message:  fmt.Sprintf("Icon '%s' is missing density variants: %s", name, strings.Join(missing, ", ")),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// CheckIconDipSize reports icons whose dimensions don't match expected DPI ratios.
func CheckIconDipSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		// Need at least 2 densities with known dimensions
		var withDims []*android.IconFile
		for _, ic := range icons {
			if ic.Width > 0 && ic.Height > 0 {
				withDims = append(withDims, ic)
			}
		}
		if len(withDims) < 2 {
			continue
		}

		// Compute expected base dp size from the first icon
		ref := withDims[0]
		refMul := android.DensityMultiplier(ref.Density)
		if refMul == 0 {
			continue
		}
		baseDp := float64(ref.Width) / refMul

		for _, ic := range withDims[1:] {
			mul := android.DensityMultiplier(ic.Density)
			if mul == 0 {
				continue
			}
			expectedW := baseDp * mul
			expectedH := (float64(ref.Height) / refMul) * mul

			// Allow 1px tolerance
			if math.Abs(float64(ic.Width)-expectedW) > 1 || math.Abs(float64(ic.Height)-expectedH) > 1 {
				findings = append(findings, scanner.Finding{
					File:     ic.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDipSize",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' in %s is %dx%d but expected ~%.0fx%.0f based on %s variant",
						name, ic.Density, ic.Width, ic.Height, expectedW, expectedH, ref.Density),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
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

// CheckIconMissingDensityFolder reports when standard density folders are missing entirely.
// It requires that at least one drawable-* or mipmap-* folder exists.
func CheckIconMissingDensityFolder(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil || len(idx.Densities) == 0 {
		return
	}

	// Required densities if any density folder exists
	required := []string{"mdpi", "hdpi", "xhdpi", "xxhdpi"}

	var samplePath string
	for _, icons := range idx.Densities {
		if len(icons) > 0 {
			samplePath = icons[0].Path
			break
		}
	}

	for _, d := range required {
		if _, exists := idx.Densities[d]; !exists {
			c.Append(scanner.Finding{
				File:     samplePath,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconMissingDensityFolder",
				Severity: "warning",
				Message:  fmt.Sprintf("Missing density folder for '%s'. Provide resources for all common densities.", d),
			})
		}
	}
}

// CheckIconExpectedSize reports launcher icons that are not at the expected size for their density.
func CheckIconExpectedSize(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}

	for _, ic := range idx.Icons {
		if ic.Width == 0 || ic.Height == 0 {
			continue
		}
		isLauncher := false
		for _, prefix := range launcherPrefixes {
			if ic.Name == prefix || strings.HasPrefix(ic.Name, prefix+"_") {
				isLauncher = true
				break
			}
		}
		if !isLauncher {
			continue
		}

		expected, ok := expectedLauncherSizes[ic.Density]
		if !ok {
			continue
		}

		if ic.Width != expected || ic.Height != expected {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconExpectedSize",
				Severity: "warning",
				Message: fmt.Sprintf("Launcher icon '%s' in %s is %dx%d but expected %dx%d (48dp base)",
					ic.Name, ic.Density, ic.Width, ic.Height, expected, expected),
				BinaryFix: &scanner.BinaryFix{
					Type:        scanner.BinaryFixCreateFile,
					SourcePath:  ic.Path,
					TargetPath:  ic.Path,
					Description: fmt.Sprintf("Resize launcher icon to %dx%d (cannot auto-resize without quality loss)", expected, expected),
					HintOnly:    true,
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

// CheckIconXmlAndPng reports resources that exist as both XML vector and raster (PNG/JPG/WebP) formats.
func CheckIconXmlAndPng(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.IconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		hasXml := false
		hasRaster := false
		var samplePath string
		for _, ic := range icons {
			switch ic.Format {
			case "xml":
				hasXml = true
			case "png", "jpg", "webp", "gif":
				hasRaster = true
			}
			if samplePath == "" {
				samplePath = ic.Path
			}
		}
		if hasXml && hasRaster {
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

// CheckIconNoDpi reports icons that appear in both drawable-nodpi and a
// density-specific folder. An icon in nodpi should not also exist in a
// density-specific folder, as the density variant will shadow the nodpi one.
func CheckIconNoDpi(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	// Get nodpi icons
	nodpiIcons := idx.Densities["nodpi"]
	if len(nodpiIcons) == 0 {
		return
	}
	nodpiNames := make(map[string]*android.IconFile)
	for _, ic := range nodpiIcons {
		nodpiNames[ic.Name] = ic
	}

	// Check if any nodpi icon also exists in a density-specific folder
	var findings []scanner.Finding
	for density, icons := range idx.Densities {
		if density == "nodpi" {
			continue
		}
		for _, ic := range icons {
			if nodpiIC, ok := nodpiNames[ic.Name]; ok {
				findings = append(findings, scanner.Finding{
					File:     nodpiIC.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconNoDpi",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' exists in both drawable-nodpi and drawable-%s. "+
						"The density-specific variant will shadow the nodpi version.",
						ic.Name, density),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// CheckIconDuplicatesConfig reports identical icons across different
// configuration folders (e.g., drawable-en and drawable-fr). Icons with the
// same hash in different config folders are wasteful duplicates.
func CheckIconDuplicatesConfig(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	byName := idx.ConfigIconsByName()
	var findings []scanner.Finding

	for name, icons := range byName {
		if len(icons) < 2 {
			continue
		}
		// Group by hash
		hashMap := make(map[string][]string) // hash -> configs
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
		for hash, configs := range hashMap {
			if len(configs) > 1 {
				sort.Strings(configs)
				findings = append(findings, scanner.Finding{
					File:     pathMap[hash],
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconDuplicatesConfig",
					Severity: "warning",
					Message: fmt.Sprintf("Icon '%s' is identical across configurations %s (duplicate resources)",
						name, strings.Join(configs, ", ")),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	c.AppendAll(findings)
}

// actionBarIconPrefixes identifies action bar icons by name prefix.
var actionBarIconPrefixes = []string{"ic_action_", "ic_menu_"}

var notificationIconPrefixes = []string{"ic_notification_", "ic_stat_"}

type iconColorKind int

const (
	iconColorUnknown iconColorKind = iota
	iconColorActionBar
	iconColorNotificationGray
	iconColorNotificationWhite
)

func classifyIconColorCheck(ic *android.IconFile, facts *librarymodel.Facts) iconColorKind {
	folderVersion := iconFolderVersion(ic.Path)
	minSdk, hasMinSdk := iconProjectMinSdk(facts)
	isAndroid30 := iconIsAndroid30(folderVersion, minSdk, hasMinSdk)
	isAndroid23 := iconIsAndroid23(folderVersion, minSdk, hasMinSdk)
	for _, prefix := range actionBarIconPrefixes {
		if strings.HasPrefix(ic.Name, prefix) {
			if (folderVersion != -1 && folderVersion < 11) || !isAndroid30 {
				return iconColorUnknown
			}
			return iconColorActionBar
		}
	}
	for _, prefix := range notificationIconPrefixes {
		if strings.HasPrefix(ic.Name, prefix) {
			switch {
			case folderVersion != -1 && folderVersion < 9:
				return iconColorUnknown
			case isAndroid30:
				return iconColorNotificationWhite
			case isAndroid23:
				return iconColorNotificationGray
			default:
				return iconColorUnknown
			}
		}
	}
	return iconColorUnknown
}

func iconProjectMinSdk(facts *librarymodel.Facts) (int, bool) {
	if facts == nil || facts.Profile.MinSdkVersion <= 0 {
		return 0, false
	}
	return facts.Profile.MinSdkVersion, true
}

func iconIsAndroid30(folderVersion, minSdk int, hasMinSdk bool) bool {
	if folderVersion >= 11 {
		return true
	}
	if hasMinSdk {
		return minSdk >= 11
	}
	return folderVersion == -1
}

func iconIsAndroid23(folderVersion, minSdk int, hasMinSdk bool) bool {
	if iconIsAndroid30(folderVersion, minSdk, hasMinSdk) {
		return false
	}
	if folderVersion == 9 || folderVersion == 10 {
		return true
	}
	return hasMinSdk && minSdk >= 9 && minSdk < 11
}

func iconFolderVersion(path string) int {
	folder := filepath.Base(filepath.Dir(path))
	for _, qualifier := range strings.Split(folder, "-")[1:] {
		if len(qualifier) > 1 && qualifier[0] == 'v' {
			version, err := strconv.Atoi(qualifier[1:])
			if err == nil {
				return version
			}
		}
	}
	return -1
}

func isExactGray(r, g, b uint32) bool {
	return r == g && r == b
}

func isExactWhite(r, g, b uint32) bool {
	return r == 255 && g == 255 && b == 255
}

func pixelRGBA8(img image.Image, x, y int) (uint32, uint32, uint32, uint32) {
	r, g, b, a := img.At(x, y).RGBA()
	return r >> 8, g >> 8, b >> 8, a >> 8
}

func pixelColorKey(img image.Image, x, y int) uint32 {
	r, g, b, a := pixelRGBA8(img, x, y)
	return a<<24 | r<<16 | g<<8 | b
}

func hasDifferentNeighbor(img image.Image, bounds image.Rectangle, x, y int, colorKey uint32) bool {
	if x < bounds.Max.X-1 && colorKey != pixelColorKey(img, x+1, y) {
		return true
	}
	if x > bounds.Min.X && colorKey != pixelColorKey(img, x-1, y) {
		return true
	}
	if y < bounds.Max.Y-1 && colorKey != pixelColorKey(img, x, y+1) {
		return true
	}
	if y > bounds.Min.Y && colorKey != pixelColorKey(img, x, y-1) {
		return true
	}
	return false
}

// CheckIconColors checks that notification and action bar icons use the same
// color constraints as Android lint's IconColors detector.
func CheckIconColors(idx *android.IconIndex, c *scanner.FindingCollector) {
	CheckIconColorsWithFacts(idx, c, nil)
}

// CheckIconColorsWithFacts checks IconColors using project SDK facts when the
// Android pipeline has Gradle-derived profile data available.
func CheckIconColorsWithFacts(idx *android.IconIndex, c *scanner.FindingCollector, facts *librarymodel.Facts) {
	if idx == nil {
		return
	}
	var findings []scanner.Finding
	icons := append([]*android.IconFile{}, idx.Icons...)
	icons = append(icons, idx.ConfigIcons...)
	for _, ic := range icons {
		if (ic.Format != "png" && ic.Format != "jpg" && ic.Format != "gif") || ic.Width == 0 || ic.Height == 0 ||
			strings.HasSuffix(filepath.Base(ic.Path), ".9.png") {
			continue
		}
		kind := classifyIconColorCheck(ic, facts)
		if kind == iconColorUnknown {
			continue
		}

		f, err := os.Open(ic.Path)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}

		bounds := img.Bounds()
	checkPixels:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r8, g8, b8, a8 := pixelRGBA8(img, x, y)
				if a8 == 0 {
					continue
				}

				if kind == iconColorActionBar {
					if !isExactGray(r8, g8, b8) {
						findings = append(findings, scanner.Finding{
							File:     ic.Path,
							Line:     1,
							Col:      1,
							RuleSet:  androidRuleSet,
							Rule:     "IconColors",
							Severity: "warning",
							Message: fmt.Sprintf("Action Bar icon '%s' should use a single gray color "+
								"(#333333 for light themes with 60%%/30%% opacity for enabled/disabled, "+
								"and #FFFFFF with opacity 80%%/30%% for dark themes).", ic.Name),
						})
						break checkPixels
					}
					continue
				}

				if kind == iconColorNotificationGray {
					if !isExactGray(r8, g8, b8) {
						findings = append(findings, scanner.Finding{
							File:     ic.Path,
							Line:     1,
							Col:      1,
							RuleSet:  androidRuleSet,
							Rule:     "IconColors",
							Severity: "warning",
							Message:  fmt.Sprintf("Notification icon '%s' should not use colors.", ic.Name),
						})
						break checkPixels
					}
					continue
				}

				if isExactWhite(r8, g8, b8) {
					continue
				}
				if isExactGray(r8, g8, b8) && hasDifferentNeighbor(img, bounds, x, y, pixelColorKey(img, x, y)) {
					continue
				}
				findings = append(findings, scanner.Finding{
					File:     ic.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconColors",
					Severity: "warning",
					Message:  fmt.Sprintf("Notification icon '%s' must be entirely white.", ic.Name),
				})
				break checkPixels
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

// CheckIconLauncherShape checks launcher icon PNGs for transparent corners,
// which may indicate a non-adaptive icon format. Adaptive icons should fill
// the entire canvas without transparent corners.
func CheckIconLauncherShape(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		if ic.Format != "png" || ic.Width == 0 || ic.Height == 0 {
			continue
		}
		isLauncher := false
		for _, prefix := range launcherPrefixes {
			if ic.Name == prefix || strings.HasPrefix(ic.Name, prefix+"_") {
				isLauncher = true
				break
			}
		}
		if !isLauncher {
			continue
		}

		f, err := os.Open(ic.Path)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}

		bounds := img.Bounds()
		w := bounds.Max.X - bounds.Min.X
		h := bounds.Max.Y - bounds.Min.Y
		if w < 4 || h < 4 {
			continue
		}

		// Check the four corners (2x2 pixel area each)
		corners := [][2]int{
			{bounds.Min.X, bounds.Min.Y},
			{bounds.Max.X - 2, bounds.Min.Y},
			{bounds.Min.X, bounds.Max.Y - 2},
			{bounds.Max.X - 2, bounds.Max.Y - 2},
		}

		transparentCorners := 0
		for _, corner := range corners {
			_, _, _, a := img.At(corner[0], corner[1]).RGBA()
			if a>>8 < 50 {
				transparentCorners++
			}
		}

		if transparentCorners >= 3 {
			findings = append(findings, scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "IconLauncherShape",
				Severity: "warning",
				Message: fmt.Sprintf("Launcher icon '%s' in %s has transparent corners, indicating a non-adaptive icon. "+
					"Consider using adaptive icon format (mipmap-anydpi-v26) for better display across devices.",
					ic.Name, ic.Density),
			})
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}
