package rules

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// VectorRasterRule mirrors Lint's VectorRaster: a vector drawable larger than 200dp in either dimension.
type VectorRasterRule struct {
	LayoutResourceBase
	AndroidRule
}

// VectorPathRule mirrors Lint's VectorPath: a vector path of 3000 or more characters.
type VectorPathRule struct {
	LayoutResourceBase
	AndroidRule
}

// InvalidVectorPathRule mirrors Lint's InvalidVectorPath: path numbers that crash older devices.
type InvalidVectorPathRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal drawable attributes.
func (r *VectorRasterRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal drawable attributes.
func (r *VectorPathRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal drawable attributes.
func (r *InvalidVectorPathRule) Confidence() float64 { return api.ConfidenceHigh }

// vectorDipSize parses a dp or dip size, returning -1 for anything else.
func vectorDipSize(value string) int {
	if value == "" || !unicode.IsDigit(rune(value[0])) {
		return -1
	}
	switch {
	case strings.HasSuffix(value, "dp"):
		value = strings.TrimSuffix(value, "dp")
	case strings.HasSuffix(value, "dip"):
		value = strings.TrimSuffix(value, "dip")
	default:
		return -1
	}
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return -1
	}
	return int(n)
}

func (r *VectorRasterRule) check(ctx *api.Context) {
	for _, vd := range ctx.ResourceIndex.VectorDrawables {
		if vd.Width == nil || vd.Height == nil {
			continue
		}
		report := vd.Height
		if vectorDipSize(vd.Width.Value) > 200 {
			report = vd.Width
		} else if vectorDipSize(vd.Height.Value) <= 200 {
			continue
		}
		ctx.Emit(scanner.Finding{File: vd.FilePath, Line: report.ValueLine, Col: report.ValueCol,
			Message: "Limit vector icons sizes to 200×200 to keep icon drawing fast; see https://developer.android.com/studio/write/vector-asset-studio#when for more"})
	}
}

// vectorPathData resolves a pathData value: a literal, or the text of a project string resource.
func vectorPathData(ctx *api.Context, value string) (string, bool) {
	if !strings.HasPrefix(value, "@") {
		return value, true
	}
	name, ok := strings.CutPrefix(value, "@string/")
	if !ok {
		return "", false
	}
	text, found := ctx.ResourceIndex.Strings[name]
	return text, found
}

func projectMinSdkOrZero(ctx *api.Context) int {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	if facts == nil {
		return 0
	}
	return facts.Profile.MinSdkVersion
}

func (r *VectorPathRule) check(ctx *api.Context) {
	minSdk := projectMinSdkOrZero(ctx)
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	for _, vd := range ctx.ResourceIndex.VectorDrawables {
		if vd.RootTag != "vector" {
			continue
		}
		// Gradle builds with a minimum SDK below 21 rasterize vectors to PNG, which Lint treats as fine.
		if minSdk > 0 && minSdk < 21 && vd.FolderVersion < 21 && facts != nil && facts.Profile.HasGradle {
			continue
		}
		for _, attr := range vd.PathData {
			path, ok := vectorPathData(ctx, attr.Value)
			if !ok {
				continue
			}
			// Lint counts a run of whitespace as one character.
			length, prevSpace := 0, false
			for _, c := range path {
				space := unicode.IsSpace(c)
				if !space || !prevSpace {
					length++
				}
				prevSpace = space
			}
			if length < 3000 {
				continue
			}
			ctx.Emit(scanner.Finding{File: vd.FilePath, Line: attr.ValueLine, Col: attr.ValueCol,
				Message: fmt.Sprintf("Very long vector path (%d characters), which is bad for performance. Considering reducing precision, removing minor details or rasterizing vector.", length)})
		}
	}
}

// vectorFindNextStart returns the index of the next path command letter at or after end, other than an exponent marker.
func vectorFindNextStart(s string, end int) int {
	for ; end < len(s); end++ {
		c := s[end]
		if (c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') && c != 'e' && c != 'E' {
			return end
		}
	}
	return end
}

func (r *InvalidVectorPathRule) check(ctx *api.Context) {
	minSdk := projectMinSdkOrZero(ctx)
	if minSdk == 0 {
		minSdk = 1
	}
	if minSdk >= 23 {
		return
	}
	for _, vd := range ctx.ResourceIndex.VectorDrawables {
		if vd.RootTag != "vector" {
			continue
		}
		for _, attr := range vd.PathData {
			path, ok := vectorPathData(ctx, attr.Value)
			if !ok {
				continue
			}
			start, end := 0, 1
			for end < len(path) {
				end = vectorFindNextStart(path, end)
				trimStart := start
				if trimStart < end && unicode.IsSpace(rune(path[trimStart])) {
					trimStart++
				}
				trimEnd := end
				if trimEnd > trimStart && unicode.IsSpace(rune(path[trimEnd-1])) {
					trimEnd--
				}
				if trimEnd > trimStart {
					r.checkFloats(ctx, vd, attr, path, trimStart, trimEnd, minSdk)
				}
				start = end
				end++
			}
		}
	}
}

func (r *InvalidVectorPathRule) report(ctx *api.Context, vd android.VectorDrawable, attr android.VectorAttr, path string, from, to int, msg string) {
	line, col := attr.ValueLine, attr.ValueCol
	if path == attr.Value {
		// The path is the attribute text itself, so point at the number.
		line, col = attr.ValueLine+strings.Count(path[:from], "\n"), attr.ValueCol+from
		if i := strings.LastIndexByte(path[:from], '\n'); i >= 0 {
			col = from - i
		}
	}
	ctx.Emit(scanner.Finding{File: vd.FilePath, Line: line, Col: col, Message: msg})
}

func (r *InvalidVectorPathRule) checkFloats(ctx *api.Context, vd android.VectorDrawable, attr android.VectorAttr, s string, start, end, minSdk int) {
	if s[start] == 'z' || s[start] == 'Z' {
		return
	}
	startPosition := start + 1
	for startPosition < end {
		foundSeparator, seenDot, isExponential, hasExponential, endWithNegOrDot := false, false, false, false, false
		currentIndex := startPosition
		for ; currentIndex < end; currentIndex++ {
			isPrevExponential := isExponential
			isExponential = false
			switch s[currentIndex] {
			case '\t', '\n', ' ', ',':
				foundSeparator = true
			case '-':
				if currentIndex != startPosition && !isPrevExponential {
					foundSeparator, endWithNegOrDot = true, true
				}
			case '.':
				if !seenDot {
					seenDot = true
				} else {
					foundSeparator, endWithNegOrDot = true, true
				}
			case 'e', 'E':
				isExponential, hasExponential = true, true
			}
			if foundSeparator {
				break
			}
		}
		if hasExponential && startPosition < currentIndex && minSdk < 21 {
			number := s[startPosition:currentIndex]
			msg := fmt.Sprintf("Avoid scientific notation (`%s`) in vector paths because it can lead to crashes on some devices.", number)
			if v, err := strconv.ParseFloat(number, 64); err == nil {
				msg += " Use `" + strconv.FormatFloat(v, 'f', -1, 64) + "` instead."
			}
			r.report(ctx, vd, attr, s, startPosition, currentIndex, msg)
		}
		if (strings.HasPrefix(s[startPosition:], ".") || strings.HasPrefix(s[startPosition:], "-.")) && minSdk < 23 {
			number := s[startPosition:currentIndex]
			replace := "-0." + number[2:]
			if strings.HasPrefix(number, ".") {
				replace = "0" + number
			}
			r.report(ctx, vd, attr, s, startPosition, currentIndex,
				fmt.Sprintf("Use %s instead of %s to avoid crashes on some devices", replace, number))
		}
		if endWithNegOrDot {
			startPosition = currentIndex
		} else {
			startPosition = currentIndex + 1
		}
	}
}
