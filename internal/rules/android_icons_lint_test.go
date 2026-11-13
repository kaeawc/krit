package rules

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/scanner"
)

// iconProject builds a res directory in a temp module and scans it.
type iconProject struct {
	t    *testing.T
	root string
	res  string
}

func newIconProject(t *testing.T) *iconProject {
	t.Helper()
	root := t.TempDir()
	return &iconProject{t: t, root: root, res: filepath.Join(root, "res")}
}

func (p *iconProject) path(rel string) string {
	full := filepath.Join(p.res, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		p.t.Fatal(err)
	}
	return full
}

// pixels returns the color of one pixel, so a test can shape the image.
type pixels func(x, y, w, h int) color.NRGBA

func solid(c color.NRGBA) pixels { return func(int, int, int, int) color.NRGBA { return c } }

// png writes an image whose fill color encodes tag, so differently tagged files never share bytes.
func (p *iconProject) png(rel string, w, h int, tag uint8, shape pixels) {
	p.t.Helper()
	if shape == nil {
		shape = solid(color.NRGBA{R: tag, G: 7, B: 9, A: 255})
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, shape(x, y, w, h))
		}
	}
	f, err := os.Create(p.path(rel))
	if err != nil {
		p.t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		p.t.Fatal(err)
	}
}

func (p *iconProject) write(rel, content string) {
	p.t.Helper()
	if err := os.WriteFile(p.path(rel), []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

func (p *iconProject) scan() *android.IconIndex {
	p.t.Helper()
	idx, err := android.ScanIconDirs(p.res)
	if err != nil {
		p.t.Fatal(err)
	}
	return idx
}

// run executes a check and returns "relative/path: message" lines, sorted.
func (p *iconProject) run(check func(*android.IconIndex, *scanner.FindingCollector)) []string {
	p.t.Helper()
	c := scanner.NewFindingCollector(0)
	check(p.scan(), c)
	var out []string
	for _, f := range c.Columns().Findings() {
		rel, err := filepath.Rel(p.res, f.File)
		if err != nil {
			rel = f.File
		}
		out = append(out, filepath.ToSlash(rel)+": "+f.Message)
	}
	sort.Strings(out)
	return out
}

func expectLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestIconDipSize_VarianceUsesStandardDeviation(t *testing.T) {
	p := newIconProject(t)
	// 48, 53 and 48 dp: the spread is within a tenth of the average.
	p.png("drawable-mdpi/near.png", 48, 48, 1, nil)
	p.png("drawable-hdpi/near.png", 80, 80, 2, nil)
	p.png("drawable-xhdpi/near.png", 96, 96, 3, nil)
	// 48, 133 and 48 dp: reported at the variant with the smallest dp size.
	p.png("drawable-mdpi/far.png", 48, 48, 4, nil)
	p.png("drawable-hdpi/far.png", 200, 200, 5, nil)
	p.png("drawable-xhdpi/far.png", 96, 96, 6, nil)
	expectLines(t, p.run(CheckIconDipSize),
		"drawable-xhdpi/far.png: The image `far.png` varies significantly in its density-independent (dip) size across the various density versions: "+
			"drawable-hdpi/far.png: 133x133 dp (200x200 px), drawable-mdpi/far.png: 48x48 dp (48x48 px), drawable-xhdpi/far.png: 48x48 dp (96x96 px)")
}

func TestIconDipSize_GroupsByFolderTypeAndConfiguration(t *testing.T) {
	p := newIconProject(t)
	// drawable and mipmap copies of one name are compared separately, as are -v21 variants.
	p.png("drawable-mdpi/icon.png", 32, 32, 1, nil)
	p.png("drawable-xhdpi/icon.png", 64, 64, 2, nil)
	p.png("mipmap-mdpi/icon.png", 32, 32, 3, nil)
	p.png("mipmap-xhdpi/icon.png", 64, 64, 4, nil)
	p.png("drawable-mdpi-v21/icon.png", 32, 32, 5, nil)
	p.png("drawable-hdpi-v21/icon.png", 48, 48, 6, nil)
	expectLines(t, p.run(CheckIconDipSize))
}

func TestIconDipSize_SuspiciousFileName(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-hdpi/logo_24dp.png", 36, 36, 1, nil)
	p.png("drawable-xhdpi/logo_24dp.png", 48, 48, 2, nil)
	p.png("drawable-mdpi/wide_24dp.png", 12, 12, 3, nil)
	expectLines(t, p.run(CheckIconDipSize),
		"drawable-mdpi/wide_24dp.png: Suspicious file name `wide_24dp.png`: The implied 24 `dp` size does not match the actual `dp` size "+
			"(pixel size 12×12 in a `drawable-mdpi` folder computes to 12×12 `dp`)")
}

func TestIconDensities_MatchesByBaseNameAndOnlyRequiredDrawableFolders(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-mdpi/a.png", 8, 8, 1, nil)
	p.png("drawable-hdpi/a.png", 8, 8, 2, nil)
	p.png("drawable-xhdpi/a.png", 8, 8, 3, nil)
	p.png("drawable-xxhdpi/a.png", 8, 8, 4, nil)
	// An xml in one density stands for the bitmaps of the same base name in the others.
	p.write("drawable-hdpi/shape.xml", "<shape/>")
	p.png("drawable-mdpi/shape.png", 8, 8, 5, nil)
	// xxxhdpi counts toward the names every folder should have but is not itself checked, and mipmap
	// folders are not compared.
	p.png("drawable-xxxhdpi/big.png", 8, 8, 6, nil)
	p.png("mipmap-mdpi/m.png", 8, 8, 7, nil)
	expectLines(t, p.run(CheckIconDensities),
		"drawable-hdpi: Missing the following drawables in `drawable-hdpi`: big.png (found in drawable-xxxhdpi)",
		"drawable-mdpi: Missing the following drawables in `drawable-mdpi`: big.png (found in drawable-xxxhdpi)",
		"drawable-xhdpi: Missing the following drawables in `drawable-xhdpi`: big.png, shape.png, shape.xml",
		"drawable-xxhdpi: Missing the following drawables in `drawable-xxhdpi`: big.png, shape.png, shape.xml",
	)
}

func TestIconDensities_AbbreviatesPastFiveNames(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-mdpi/keep.png", 8, 8, 1, nil)
	p.png("drawable-hdpi/keep.png", 8, 8, 2, nil)
	for i, n := range []string{"a", "b", "c", "d", "e", "f"} {
		p.png("drawable-mdpi/"+n+".png", 8, 8, uint8(10+i), nil)
	}
	expectLines(t, p.run(CheckIconDensities),
		"drawable-hdpi: Missing the following drawables in `drawable-hdpi`: a.png, b.png, c.png, d.png, e.png... (1 more)")
}

func TestIconNoDpi_ReportsAtLastSortedCopyAndMatchesBaseNames(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-nodpi/one.png", 8, 8, 1, nil)
	p.png("drawable-nodpi/two.png", 8, 8, 2, nil)
	p.png("drawable-hdpi/one.png", 8, 8, 3, nil)
	p.png("drawable-mdpi/two.gif", 8, 8, 4, nil)
	expectLines(t, p.run(CheckIconNoDpi),
		"drawable-mdpi/two.png: The following images appear in both `-nodpi` and in a density folder: one.png, two.gif, two.png")
}

func TestIconExpectedSize_ByRoleAndFolder(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-mdpi/ic_menu_ok.png", 32, 32, 1, nil)
	p.png("drawable-hdpi/ic_menu_bad.png", 49, 49, 2, nil)
	p.png("drawable-mdpi/ic_stat_ok.png", 24, 24, 3, nil)
	p.png("drawable-xhdpi/ic_stat_bad.png", 60, 60, 4, nil)
	p.png("drawable-mdpi/ic_tab_bad.png", 40, 40, 5, nil)
	p.png("drawable-mdpi/ic_dialog_bad.png", 40, 40, 6, nil)
	p.png("drawable-mdpi/ic_action_ok.png", 32, 32, 7, nil)
	p.png("mipmap-xxxhdpi/ic_launcher.png", 192, 192, 8, nil)
	p.png("mipmap-xxhdpi/ic_launcher_foreground.png", 300, 300, 9, nil)
	// Numeric and named density qualifiers outside the five buckets use the dpi value.
	p.png("drawable-420dpi/ic_tab_dpi.png", 32, 32, 10, nil)
	p.png("drawable-tvdpi/ic_tab_tv.png", 32, 32, 11, nil)
	// No density, nine-patch and unrelated names are not sized.
	p.png("drawable/ic_tab_plain.png", 99, 99, 12, nil)
	p.png("drawable-mdpi/ic_tab_nine.9.png", 99, 99, 13, nil)
	p.png("drawable-mdpi/photo.png", 99, 99, 14, nil)
	expectLines(t, p.run(CheckIconExpectedSize),
		"drawable-420dpi/ic_tab_dpi.png: Incorrect icon size for `drawable-420dpi/ic_tab_dpi.png`: expected 84x84, but was 32x32",
		"drawable-hdpi/ic_menu_bad.png: Incorrect icon size for `drawable-hdpi/ic_menu_bad.png`: expected 48x48, but was 49x49",
		"drawable-mdpi/ic_dialog_bad.png: Incorrect icon size for `drawable-mdpi/ic_dialog_bad.png`: expected 32x32, but was 40x40",
		"drawable-mdpi/ic_tab_bad.png: Incorrect icon size for `drawable-mdpi/ic_tab_bad.png`: expected 32x32, but was 40x40",
		"drawable-tvdpi/ic_tab_tv.png: Incorrect icon size for `drawable-tvdpi/ic_tab_tv.png`: expected 42x42, but was 32x32",
		"drawable-xhdpi/ic_stat_bad.png: Incorrect icon size for `drawable-xhdpi/ic_stat_bad.png`: expected 48x48, but was 60x60",
		"mipmap-xxhdpi/ic_launcher_foreground.png: Incorrect icon size for `mipmap-xxhdpi/ic_launcher_foreground.png`: expected 324x324, but was 300x300",
	)
}

func TestIconExpectedSize_ManifestIconCountsAsLauncher(t *testing.T) {
	p := newIconProject(t)
	testWriteManifest(t, p.root, `<application android:icon="@mipmap/app" android:roundIcon="@drawable/app_round"><activity android:icon="@drawable/main"/></application>`)
	p.png("mipmap-mdpi/app.png", 40, 40, 1, nil)
	p.png("drawable-mdpi/main.png", 40, 40, 2, nil)
	p.png("drawable-mdpi/app_round.png", 48, 48, 3, nil)
	p.png("drawable-mdpi/app.png", 40, 40, 4, nil) // a drawable named like the mipmap launcher is not one
	expectLines(t, p.run(CheckIconExpectedSize),
		"drawable-mdpi/main.png: Incorrect icon size for `drawable-mdpi/main.png`: expected 48x48, but was 40x40",
		"mipmap-mdpi/app.png: Incorrect icon size for `mipmap-mdpi/app.png`: expected 48x48, but was 40x40",
	)
}

func holeAt(cx, cy int) pixels {
	return func(x, y, _, _ int) color.NRGBA {
		if x == cx && y == cy {
			return color.NRGBA{}
		}
		return color.NRGBA{R: 200, A: 255}
	}
}

func cornerAlpha(a uint8) pixels {
	return func(x, y, _, _ int) color.NRGBA {
		if x < 2 && y < 2 {
			return color.NRGBA{R: 200, A: a}
		}
		return color.NRGBA{R: 200, A: 255}
	}
}

func disc(x, y, w, h int) color.NRGBA {
	dx, dy := (float64(x)+0.5-float64(w)/2)/(float64(w)/2), (float64(y)+0.5-float64(h)/2)/(float64(h)/2)
	if dx*dx+dy*dy <= 1 {
		return color.NRGBA{B: 200, A: 255}
	}
	return color.NRGBA{}
}

func TestIconLauncherShape_NeedsOneFullyTransparentPixel(t *testing.T) {
	p := newIconProject(t)
	const msg = launcherShapeMessage
	p.png("mipmap-mdpi/ic_launcher_solid.png", 48, 48, 1, nil)
	p.png("mipmap-mdpi/ic_launcher_faint.png", 48, 48, 2, cornerAlpha(30))
	p.png("mipmap-mdpi/ic_launcher_clear.png", 48, 48, 3, cornerAlpha(0))
	p.png("mipmap-mdpi/ic_launcher_hole.png", 48, 48, 4, holeAt(24, 24))
	p.png("mipmap-mdpi/ic_launcher_foreground.png", 48, 48, 5, nil)
	p.png("mipmap-mdpi/ic_launcherv2.png", 48, 48, 6, nil)
	p.png("mipmap-mdpi/ic_other.png", 48, 48, 7, nil)
	expectLines(t, p.run(CheckIconLauncherShape),
		"mipmap-mdpi/ic_launcher_faint.png: "+msg,
		"mipmap-mdpi/ic_launcher_solid.png: "+msg,
		"mipmap-mdpi/ic_launcherv2.png: "+msg,
	)
}

func TestIconLauncherShape_RoundAndManifestIcons(t *testing.T) {
	p := newIconProject(t)
	testWriteManifest(t, p.root, `<application android:icon="@mipmap/app" android:roundIcon="@mipmap/app_circle"/>`)
	p.png("mipmap-mdpi/app.png", 48, 48, 1, nil)
	p.png("mipmap-mdpi/app_circle.png", 48, 48, 2, nil)
	p.png("mipmap-hdpi/app_circle.png", 72, 72, 3, disc)
	p.png("mipmap-mdpi/ic_launcher_round.png", 48, 48, 4, cornerAlpha(0))
	p.png("mipmap-hdpi/ic_launcher_round.png", 72, 72, 5, disc)
	expectLines(t, p.run(CheckIconLauncherShape),
		"mipmap-mdpi/app.png: "+launcherShapeMessage,
		"mipmap-mdpi/app_circle.png: "+roundIconMessage,
		"mipmap-mdpi/ic_launcher_round.png: "+roundIconMessage,
	)
}

func TestConvertToWebp_ReportsOnlyTheLargestEligibleImage(t *testing.T) {
	p := newIconProject(t)
	testWriteManifest(t, p.root, testMinSdk("21"))
	big := func(x, y, _, _ int) color.NRGBA {
		v := uint8((x*37 + y*91 + (x*y)%251) % 256)
		return color.NRGBA{R: v, G: v ^ 0x5a, B: v ^ 0xc3, A: 255}
	}
	p.png("drawable-mdpi/small.png", 8, 8, 1, nil)
	p.png("drawable-hdpi/medium.png", 64, 64, 2, big)
	p.png("drawable-xhdpi/large.png", 128, 128, 3, big)
	p.png("drawable-xxhdpi/ic_launcher_foreground.png", 256, 256, 4, big)
	p.png("drawable-xxhdpi/bubble.9.png", 256, 256, 5, big)
	expectLines(t, p.run(func(idx *android.IconIndex, c *scanner.FindingCollector) { CheckConvertToWebp(idx, c, nil) }),
		"drawable-xhdpi/large.png: One or more images in this project can be converted to the WebP format which typically results in smaller file sizes, even for lossless conversion")
}

func TestConvertToWebp_MinSdkFromGradleOverridesManifest(t *testing.T) {
	p := newIconProject(t)
	testWriteManifest(t, p.root, testMinSdk("21"))
	p.png("drawable-mdpi/a.png", 8, 8, 1, nil)
	facts := &librarymodel.Facts{}
	facts.Profile.MinSdkVersion = 16
	got := p.run(func(idx *android.IconIndex, c *scanner.FindingCollector) { CheckConvertToWebp(idx, c, facts) })
	expectLines(t, got)
	facts.Profile.MinSdkVersion = 18
	if got := p.run(func(idx *android.IconIndex, c *scanner.FindingCollector) { CheckConvertToWebp(idx, c, facts) }); len(got) != 1 {
		t.Fatalf("expected a finding at minSdk 18, got %v", got)
	}
}

func TestIconDuplicates_AcrossFoldersAndNames(t *testing.T) {
	p := newIconProject(t)
	// Different names, different folders: one IconDuplicates finding at the last path.
	p.png("drawable-mdpi/a.png", 10, 10, 1, nil)
	p.png("mipmap-hdpi/b.png", 10, 10, 1, nil)
	// One name in several folders: IconDuplicatesConfig, not IconDuplicates.
	p.png("drawable-en/same.png", 10, 10, 2, nil)
	p.png("drawable-fr/same.png", 10, 10, 2, nil)
	// Names that all carry a dp size are expected to repeat.
	p.png("drawable-mdpi/x_24dp.png", 10, 10, 3, nil)
	p.png("drawable-hdpi/y_24dp.png", 10, 10, 3, nil)
	// A name next to a differently named copy makes the whole group IconDuplicates.
	p.png("drawable-de/mixed.png", 10, 10, 4, nil)
	p.png("drawable-it/mixed.png", 10, 10, 4, nil)
	p.png("drawable-it/other.png", 10, 10, 4, nil)
	expectLines(t, p.run(CheckIconDuplicates),
		"drawable-it/other.png: The following unrelated icon files have identical contents: mixed.png, mixed.png, other.png",
		"mipmap-hdpi/b.png: The following unrelated icon files have identical contents: a.png, b.png",
	)
	expectLines(t, p.run(CheckIconDuplicatesConfig),
		"drawable-fr/same.png: The `same.png` icon has identical contents in the following configuration folders: drawable-en, drawable-fr")
}

func TestGifUsage_UsesTheFileExtension(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-mdpi/png_named_gif.gif", 8, 8, 1, nil)
	p.png("drawable-mdpi/png_named_png.png", 8, 8, 2, nil)
	expectLines(t, p.run(CheckGifUsage),
		"drawable-mdpi/png_named_gif.gif: Using the `.gif` format for bitmaps is discouraged")
}

func TestIconExtension_FormatNamesFollowImageIO(t *testing.T) {
	p := newIconProject(t)
	p.png("drawable-mdpi/looks_like.gif", 8, 8, 1, nil)
	p.png("drawable-mdpi/looks_like.jpg", 8, 8, 2, nil)
	expectLines(t, p.run(CheckIconExtension),
		"drawable-mdpi/looks_like.gif: Misleading file extension; named `.gif` but the file format is `png`",
		"drawable-mdpi/looks_like.jpg: Misleading file extension; named `.jpg` but the file format is `png`",
	)
}

func TestIconLauncherShape_RoundCheckMatchesJavaOvalOnSmallDiscs(t *testing.T) {
	p := newIconProject(t)
	// A 16px disc differs from Java's half-pixel shifted oval on over 4% of pixels; a 48px one does not.
	p.png("mipmap-mdpi/ic_launcher_a_round.png", 16, 16, 1, disc)
	p.png("mipmap-mdpi/ic_launcher_b_round.png", 48, 48, 2, disc)
	expectLines(t, p.run(CheckIconLauncherShape),
		"mipmap-mdpi/ic_launcher_a_round.png: "+roundIconMessage)
}
