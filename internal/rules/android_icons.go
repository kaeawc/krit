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

// ConvertToWebpRule reports the largest PNG or JPEG of a res directory as convertible to WebP.
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

// IconExpectedSizeRule checks that launcher, action bar, tab, dialog, menu, notification and adaptive
// layer icons have the standard size for their density.
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

// iconFormatLabel names an image format the way Java ImageIO reports it: JPEG upper case, others lower case.
func iconFormatLabel(format string) string {
	if format == "jpg" {
		return "JPEG"
	}
	return format
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
