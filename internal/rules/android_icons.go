package rules

import (
	"fmt"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"path/filepath"
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
var launcherPrefixes = []string{"ic_launcher", "ic_launcher_round"}

// isLauncherIconName reports whether name is a legacy launcher icon such as ic_launcher or
// ic_launcher_debug. Adaptive icon layers (foreground, background, monochrome) are 108dp, not 48dp,
// and are expected to be transparent, so they are not launcher icons here.
func isLauncherIconName(name string) bool {
	for _, layer := range []string{"_foreground", "_background", "_monochrome"} {
		if strings.HasSuffix(name, layer) {
			return false
		}
	}
	for _, prefix := range launcherPrefixes {
		if name == prefix || strings.HasPrefix(name, prefix+"_") {
			return true
		}
	}
	return false
}

// expectedLauncherSizes maps density to the expected launcher icon pixel size (48dp base).
var expectedLauncherSizes = map[string]int{
	"ldpi":    36,
	"mdpi":    48,
	"hdpi":    72,
	"xhdpi":   96,
	"xxhdpi":  144,
	"xxxhdpi": 192,
}

// CheckIconDuplicates reports different file names with identical contents in one density folder, like Lint.
func CheckIconDuplicates(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	var findings []scanner.Finding
	type densityHash struct{ density, hash string }
	sameFolder := make(map[densityHash][]*android.IconFile)
	for _, ic := range idx.Icons {
		if ic.Hash == "" || ic.Format == "xml" {
			continue
		}
		key := densityHash{ic.Density, ic.Hash}
		sameFolder[key] = append(sameFolder[key], ic)
	}
	for _, icons := range sameFolder {
		names := make(map[string]*android.IconFile)
		for _, ic := range icons {
			fileName := filepath.Base(ic.Path)
			if _, ok := names[fileName]; !ok {
				names[fileName] = ic
			}
		}
		if len(names) < 2 {
			continue
		}
		sorted := make([]string, 0, len(names))
		for n := range names {
			sorted = append(sorted, n)
		}
		sort.Strings(sorted)
		findings = append(findings, scanner.Finding{
			File:     names[sorted[len(sorted)-1]].Path,
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconDuplicates",
			Severity: "warning",
			Message:  "The following unrelated icon files have identical contents: " + strings.Join(sorted, ", "),
		})
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
				Message:  "Using the `.gif` format for bitmaps is discouraged",
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
		// A nine-patch must stay a .9.png: the packaging tool only reads nine-patch data from PNG files.
		if ic.Format == "png" && ic.Size >= webpThresholdBytes && !ic.IsNinePatch() {
			c.Append(scanner.Finding{
				File:     ic.Path,
				Line:     1,
				Col:      1,
				RuleSet:  androidRuleSet,
				Rule:     "ConvertToWebp",
				Severity: "warning",
				Message:  "One or more images in this project can be converted to the WebP format which typically results in smaller file sizes, even for lossless conversion",
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
				Message: fmt.Sprintf("Misleading file extension; named `.%s` but the file format is `%s`",
					strings.TrimPrefix(strings.ToLower(filepath.Ext(ic.Path)), "."), iconFormatLabel(ic.Format)),
			})
		}
	}
}

// iconFormatLabel names an image format the way Lint prints it.
func iconFormatLabel(format string) string {
	if format == "jpg" {
		return "JPEG"
	}
	return strings.ToUpper(format)
}

// CheckIconLocation reports bitmap drawables placed directly in the densityless drawable folder, as Lint does.
func CheckIconLocation(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	for _, ic := range append(append([]*android.IconFile(nil), idx.Icons...), idx.ConfigIcons...) {
		if ic.Format == "xml" || filepath.Base(filepath.Dir(ic.Path)) != "drawable" {
			continue
		}
		c.Append(scanner.Finding{
			File:     ic.Path,
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconLocation",
			Severity: "warning",
			Message:  fmt.Sprintf("Found bitmap drawable `res/drawable/%s` in densityless folder", filepath.Base(ic.Path)),
		})
	}
}

// lintDrawableExts are the extensions Lint treats as drawable files.
var lintDrawableExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".xml": true, ".webp": true}

// iconFolderFiles lists drawable and mipmap folders (including configuration-qualified ones) with their files.
func iconFolderFiles(idx *android.IconIndex) map[string][]string {
	folders := make(map[string][]string)
	add := func(ic *android.IconFile) {
		if !lintDrawableExts[strings.ToLower(filepath.Ext(ic.Path))] {
			return
		}
		dir := filepath.Dir(ic.Path)
		folders[dir] = append(folders[dir], filepath.Base(ic.Path))
	}
	for _, ic := range idx.Icons {
		add(ic)
	}
	for _, ic := range idx.ConfigIcons {
		add(ic)
	}
	return folders
}

// lintDisplayPath renders a resource path relative to its module like Lint, e.g. src/main/res/drawable/a.xml.
func lintDisplayPath(path string) string {
	slashed := filepath.ToSlash(path)
	if i := strings.LastIndex(slashed, "/src/"); i >= 0 {
		return slashed[i+1:]
	}
	if i := strings.LastIndex(slashed, "/res/"); i >= 0 {
		return slashed[i+1:]
	}
	return slashed
}

// primaryChainFile returns the file Lint reports at: it sorts the chained locations and the last one is the head.
func primaryChainFile(paths []string) string {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	return sorted[len(sorted)-1]
}

// CheckIconMixedNinePatch reports X.png next to X.9.png in one folder, like Lint, once per name.
func CheckIconMixedNinePatch(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	clashes := make(map[string][]string)
	for dir, files := range iconFolderFiles(idx) {
		present := make(map[string]bool, len(files))
		for _, f := range files {
			present[f] = true
		}
		for _, f := range files {
			name, ok := strings.CutSuffix(f, ".9.png")
			if !ok || !present[name+".png"] {
				continue
			}
			clashes[name] = append(clashes[name], filepath.Join(dir, name+".png"), filepath.Join(dir, f))
		}
	}
	names := make([]string, 0, len(clashes))
	for name := range clashes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c.Append(scanner.Finding{
			File:     primaryChainFile(clashes[name]),
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconMixedNinePatch",
			Severity: "warning",
			Message:  fmt.Sprintf("The files `%[1]s.png` and `%[1]s.9.png` clash; both will map to `@drawable/%[1]s`", name),
		})
	}
}

// CheckIconXMLAndPng reports drawables defined both as density independent .xml and as bitmaps, like Lint.
func CheckIconXMLAndPng(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	xmlNames := make(map[string]bool)
	bitmapNames := make(map[string]bool)
	for dir, files := range iconFolderFiles(idx) {
		for _, f := range files {
			base := strings.TrimSuffix(f, filepath.Ext(f))
			if strings.EqualFold(filepath.Ext(f), ".xml") {
				if !strings.Contains(filepath.Base(dir), "-v") {
					xmlNames[base] = true
				}
			} else {
				bitmapNames[base] = true
			}
		}
	}
	byName := make(map[string][]string)
	for dir, files := range iconFolderFiles(idx) {
		for _, f := range files {
			base := strings.TrimSuffix(f, filepath.Ext(f))
			if xmlNames[base] && bitmapNames[base] {
				byName[base] = append(byName[base], filepath.Join(dir, f))
			}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		paths := byName[name]
		sort.Strings(paths)
		var listed []string
		seenXML, seenBitmap := false, false
		for _, p := range paths {
			isXML := strings.EqualFold(filepath.Ext(p), ".xml")
			if isXML && !seenXML {
				seenXML = true
				listed = append(listed, lintDisplayPath(p))
			} else if !isXML && !seenBitmap {
				seenBitmap = true
				listed = append(listed, lintDisplayPath(p))
			}
		}
		sort.Strings(listed)
		c.Append(scanner.Finding{
			File:     primaryChainFile(paths),
			Line:     1,
			Col:      1,
			RuleSet:  androidRuleSet,
			Rule:     "IconXmlAndPng",
			Severity: "warning",
			Message:  "The following images appear both as density independent `.xml` files and as bitmap files: " + strings.Join(listed, ", "),
		})
	}
}
