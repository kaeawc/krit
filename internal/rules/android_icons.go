package rules

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// IconResourceBase provides the no-op Check and AndroidDepIcons declaration
// shared by every icon rule. Icon rules operate on the IconIndex via a
// package-level CheckIcon* function rather than implementing it themselves.
type IconResourceBase struct{}

func (IconResourceBase) Check(*scanner.File) []scanner.Finding { return nil }
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

// --- Resource-based check functions (called with an IconIndex, not during AST dispatch) ---

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
func CheckIconDensities(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// CheckIconDipSize reports icons whose dimensions don't match expected DPI ratios.
func CheckIconDipSize(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// CheckIconDuplicates reports icons that are identical across densities (same hash).
func CheckIconDuplicates(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// CheckGifUsage reports any GIF files in the resource directories.
func CheckGifUsage(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		if ic.Format == "gif" {
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckConvertToWebp reports large PNG files that could be converted to WebP.
func CheckConvertToWebp(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		if ic.Format == "png" && ic.Size >= webpThresholdBytes {
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckIconMissingDensityFolder reports when standard density folders are missing entirely.
// It requires that at least one drawable-* or mipmap-* folder exists.
func CheckIconMissingDensityFolder(idx *android.IconIndex) []scanner.Finding {
	if idx == nil || len(idx.Densities) == 0 {
		return nil
	}

	// Required densities if any density folder exists
	required := []string{"mdpi", "hdpi", "xhdpi", "xxhdpi"}

	var findings []scanner.Finding
	var samplePath string
	for _, icons := range idx.Densities {
		if len(icons) > 0 {
			samplePath = icons[0].Path
			break
		}
	}

	for _, d := range required {
		if _, exists := idx.Densities[d]; !exists {
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckIconExpectedSize reports launcher icons that are not at the expected size for their density.
func CheckIconExpectedSize(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding

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
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckIconExtension reports icon files where the file extension doesn't match the detected content format.
func CheckIconExtension(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		extFmt := ic.ExtensionFormat()
		if extFmt == "" || ic.Format == "" {
			continue
		}
		if extFmt != ic.Format {
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckIconLocation reports launcher icons placed in drawable-* folders instead of mipmap-*.
func CheckIconLocation(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		if !strings.HasPrefix(ic.Name, "ic_launcher") {
			continue
		}
		if strings.Contains(ic.Path, "drawable") {
			findings = append(findings, scanner.Finding{
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
	return findings
}

// CheckIconMixedNinePatch reports icons that have a mix of nine-patch (.9.png) and non-nine-patch variants.
func CheckIconMixedNinePatch(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// CheckIconXmlAndPng reports resources that exist as both XML vector and raster (PNG/JPG/WebP) formats.
func CheckIconXmlAndPng(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// CheckIconNoDpi reports icons that appear in both drawable-nodpi and a
// density-specific folder. An icon in nodpi should not also exist in a
// density-specific folder, as the density variant will shadow the nodpi one.
func CheckIconNoDpi(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	// Get nodpi icons
	nodpiIcons := idx.Densities["nodpi"]
	if len(nodpiIcons) == 0 {
		return nil
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
	return findings
}

// CheckIconDuplicatesConfig reports identical icons across different
// configuration folders (e.g., drawable-en and drawable-fr). Icons with the
// same hash in different config folders are wasteful duplicates.
func CheckIconDuplicatesConfig(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
	return findings
}

// actionBarIconPrefixes identifies action bar / notification icons by name prefix.
var actionBarIconPrefixes = []string{"ic_action_", "ic_menu_", "ic_notification_", "ic_stat_"}

// CheckIconColors checks that action bar icons use primarily white/gray colors
// as recommended by Material Design guidelines. Samples pixels from the image
// and flags if too many are non-white/gray/transparent.
func CheckIconColors(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
	}
	var findings []scanner.Finding
	for _, ic := range idx.Icons {
		if ic.Format != "png" || ic.Width == 0 || ic.Height == 0 {
			continue
		}
		isActionBar := false
		for _, prefix := range actionBarIconPrefixes {
			if strings.HasPrefix(ic.Name, prefix) {
				isActionBar = true
				break
			}
		}
		if !isActionBar {
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
		totalPixels := 0
		nonStandardPixels := 0

		// Sample every 2nd pixel for performance
		for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
			for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
				r, g, b, a := img.At(x, y).RGBA()
				// Convert from 16-bit to 8-bit
				r8, g8, b8, a8 := r>>8, g>>8, b>>8, a>>8

				// Skip fully transparent pixels
				if a8 < 10 {
					continue
				}

				totalPixels++

				// Check if pixel is white/gray (R==G==B with high values, or any gray)
				isGray := math.Abs(float64(r8)-float64(g8)) < 30 &&
					math.Abs(float64(g8)-float64(b8)) < 30 &&
					math.Abs(float64(r8)-float64(b8)) < 30
				isWhite := r8 > 200 && g8 > 200 && b8 > 200

				if !isGray && !isWhite {
					nonStandardPixels++
				}
			}
		}

		if totalPixels > 0 {
			ratio := float64(nonStandardPixels) / float64(totalPixels)
			if ratio > 0.3 {
				findings = append(findings, scanner.Finding{
					File:     ic.Path,
					Line:     1,
					Col:      1,
					RuleSet:  androidRuleSet,
					Rule:     "IconColors",
					Severity: "warning",
					Message: fmt.Sprintf("Action bar icon '%s' uses non-standard colors (%.0f%% non-white/gray pixels). "+
						"Material Design recommends white/gray for action bar icons.",
						ic.Name, ratio*100),
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	return findings
}

// CheckIconLauncherShape checks launcher icon PNGs for transparent corners,
// which may indicate a non-adaptive icon format. Adaptive icons should fill
// the entire canvas without transparent corners.
func CheckIconLauncherShape(idx *android.IconIndex) []scanner.Finding {
	if idx == nil {
		return nil
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
		for _, c := range corners {
			_, _, _, a := img.At(c[0], c[1]).RGBA()
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
	return findings
}

// RunAllIconChecks runs all icon lint checks against the given IconIndex and returns combined findings.
func RunAllIconChecks(idx *android.IconIndex) []scanner.Finding {
	var all []scanner.Finding
	all = append(all, CheckIconDensities(idx)...)
	all = append(all, CheckIconDipSize(idx)...)
	all = append(all, CheckIconDuplicates(idx)...)
	all = append(all, CheckGifUsage(idx)...)
	all = append(all, CheckConvertToWebp(idx)...)
	all = append(all, CheckIconMissingDensityFolder(idx)...)
	all = append(all, CheckIconExpectedSize(idx)...)
	all = append(all, CheckIconExtension(idx)...)
	all = append(all, CheckIconLocation(idx)...)
	all = append(all, CheckIconMixedNinePatch(idx)...)
	all = append(all, CheckIconXmlAndPng(idx)...)
	all = append(all, CheckIconNoDpi(idx)...)
	all = append(all, CheckIconDuplicatesConfig(idx)...)
	all = append(all, CheckIconColors(idx)...)
	all = append(all, CheckIconLauncherShape(idx)...)
	return all
}
