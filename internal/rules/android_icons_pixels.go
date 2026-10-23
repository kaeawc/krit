package rules

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/scanner"
)

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
