package rules

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	launcherShapeMessage = "Launcher icons should not fill every pixel of their square region; see the design guide for details"
	roundIconMessage     = "Launcher icon used as round icon did not have a circular shape"
)

// CheckIconLauncherShape mirrors Lint: a launcher bitmap must have at least one fully transparent pixel,
// and an icon used as a round icon must be circular. Launcher icons are ic_launcher* names plus the icons
// the manifest assigns.
func CheckIconLauncherShape(idx *android.IconIndex, c *scanner.FindingCollector) {
	if idx == nil {
		return
	}
	var findings []scanner.Finding
	for _, ic := range idx.AllIcons() {
		name := filepath.Base(ic.Path)
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".9.png") {
			continue
		}
		folder := iconFolderOf(ic)
		base := iconBaseName(name)
		if !isLintLauncherIcon(idx, folder, base) {
			continue
		}
		img := decodeIconImage(ic.Path)
		if img == nil {
			continue
		}
		if msg := launcherShapeProblem(img, isLintRoundIcon(idx, folder, base)); msg != "" {
			findings = append(findings, iconFinding("IconLauncherShape", ic.Path, msg))
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].File < findings[j].File })
	c.AppendAll(findings)
}

func decodeIconImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

// launcherShapeProblem returns the message for a launcher bitmap that breaks the shape guidance.
func launcherShapeProblem(img image.Image, round bool) string {
	if round && !isRoundImage(img) {
		return roundIconMessage
	}
	if hasTransparentPixel(img) {
		return ""
	}
	return launcherShapeMessage
}

func pixelAlpha8(img image.Image, x, y int) uint32 {
	_, _, _, a := img.At(x, y).RGBA()
	return a >> 8
}

func hasTransparentPixel(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if pixelAlpha8(img, x, y) == 0 {
				return true
			}
		}
	}
	return false
}

// opaqueBounds returns the bounding box (x0, y0, x1, y1 inclusive) of the pixels that are not fully
// transparent, in image-local coordinates.
func opaqueBounds(img image.Image) (int, int, int, int, bool) {
	b := img.Bounds()
	minX, minY, maxX, maxY := math.MaxInt, math.MaxInt, 0, 0
	found := false
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			if pixelAlpha8(img, b.Min.X+x, b.Min.Y+y) == 0 {
				continue
			}
			found = true
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	return minX, minY, maxX, maxY, found
}

// isRoundImage mirrors Lint's isRound: the opaque area is about as wide as tall and fills an ellipse
// over its bounding box with fewer than 4% of the pixels differing.
func isRoundImage(img image.Image) bool {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	minX, minY, maxX, maxY, found := opaqueBounds(img)
	if !found {
		return false
	}
	bw, bh := maxX-minX+1, maxY-minY+1
	if absInt(bw-bh) > width/10 {
		return false
	}
	oval := newJavaOval(minX, minY, bw, bh)
	mismatches := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			a := pixelAlpha8(img, b.Min.X+x, b.Min.Y+y)
			if a > 0 && a < 64 {
				continue
			}
			if (a < 64) == oval.contains(x, y) {
				mismatches++
			}
		}
	}
	return float64(100*mismatches)/float64(width*height) < 4.0
}

// javaOval approximates Graphics.fillOval with default rendering hints. Java snaps the curve to
// quarter-pixel positions, which in effect moves the ellipse half a pixel right and down of its
// bounds; a pixel is filled when its centre lies strictly inside.
type javaOval struct {
	cx, cy, rx, ry float64
}

func newJavaOval(x, y, w, h int) javaOval {
	return javaOval{
		cx: float64(x) + float64(w)/2 + 0.5, cy: float64(y) + float64(h)/2 + 0.5,
		rx: float64(w) / 2, ry: float64(h) / 2,
	}
}

func (o javaOval) contains(x, y int) bool {
	dx, dy := (float64(x)+0.5-o.cx)/o.rx, (float64(y)+0.5-o.cy)/o.ry
	return dx*dx+dy*dy < 1
}
