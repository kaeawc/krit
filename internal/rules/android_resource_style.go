package rules

// Android Resource XML rules: Dimension/style rules.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// PxUsageResource
// ---------------------------------------------------------------------------

// PxUsageResourceRule detects dimension values specified in `px` instead of `dp`
// or `sp`. Pixel values do not scale across screen densities.
type PxUsageResourceRule struct {
	ResourceBase
	AndroidRule
}

func (PxUsageResourceRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesDimensions
}

// pxDimensionAttrs lists layout attributes that should use dp/sp, not px.
var pxDimensionAttrs = []string{
	"android:layout_width", "android:layout_height",
	"android:layout_margin", "android:layout_marginLeft",
	"android:layout_marginRight", "android:layout_marginTop",
	"android:layout_marginBottom", "android:layout_marginStart",
	"android:layout_marginEnd", "android:padding",
	"android:paddingLeft", "android:paddingRight",
	"android:paddingTop", "android:paddingBottom",
	"android:paddingStart", "android:paddingEnd",
	"android:textSize", "android:layout_marginHorizontal",
	"android:layout_marginVertical", "android:paddingHorizontal",
	"android:paddingVertical",
}

func isPxValue(val string) bool {
	val = strings.TrimSpace(val)
	if !strings.HasSuffix(val, "px") {
		return false
	}
	// Exclude "dp", "sp", "dip" — only match raw "px"
	prefix := val[:len(val)-2]
	if prefix == "" {
		return false
	}
	// Must be a number before px
	for _, c := range prefix {
		if (c < '0' || c > '9') && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

// pxValueExempt mirrors AOSP PxUsageDetector: `0px` (and any other numeric
// value evaluating to 0, e.g. `0.0px`) and exactly `1px` are intentionally
// allowed. 0px is density-independent because 0 * any-scale is 0; 1px is the
// classic hairline divider idiom — see AOSP issue 55722.
func pxValueExempt(val string) bool {
	val = strings.TrimSpace(val)
	if val == "1px" {
		return true
	}
	if !strings.HasSuffix(val, "px") {
		return false
	}
	n, ok := parseLeadingNumber(val[:len(val)-2])
	return ok && n == 0
}

// inOrMmValueExempt mirrors AOSP: a 0-valued `0mm` / `0in` is allowed.
func inOrMmValueExempt(val, unit string) bool {
	val = strings.TrimSpace(val)
	if !strings.HasSuffix(val, unit) {
		return false
	}
	n, ok := parseLeadingNumber(val[:len(val)-len(unit)])
	return ok && n == 0
}

func parseLeadingNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// resolveDimenReference resolves an @dimen/... (or @package:dimen/...)
// reference against the project's dimensions map. Returns the literal value
// and the location of the <dimen> declaration when found.
func resolveDimenReference(idx *android.ResourceIndex, val string) (string, android.StringLocation, bool) {
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "@") {
		return "", android.StringLocation{}, false
	}
	rest := val[1:]
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[i+1:]
	}
	const prefix = "dimen/"
	if !strings.HasPrefix(rest, prefix) {
		return "", android.StringLocation{}, false
	}
	name := rest[len(prefix):]
	if name == "" {
		return "", android.StringLocation{}, false
	}
	resolved, ok := idx.Dimensions[name]
	if !ok {
		return "", android.StringLocation{}, false
	}
	loc := idx.DimensionsLocation[name]
	return resolved, loc, true
}

// isTextSizeAttr matches the AOSP ATTR_TEXT_SIZE / "android:textSize" pair.
func isTextSizeAttr(name string) bool {
	return name == "android:textSize" || name == "textSize"
}

// isLayoutHeightAttr matches the AOSP ATTR_LAYOUT_HEIGHT / "android:layout_height" pair.
func isLayoutHeightAttr(name string) bool {
	return name == "android:layout_height" || name == "layout_height"
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *PxUsageResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *PxUsageResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for _, attr := range pxDimensionAttrs {
				val := v.Attributes[attr]
				if isPxValue(val) && !pxValueExempt(val) {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Avoid using `px` in `%s=\"%s\"`. Use `dp` or `sp` instead for density-independent sizing.",
							attr, val)))
				}
			}
		})
	}
	// values/dimens.xml dimensions
	for name, val := range idx.Dimensions {
		if !isPxValue(val) || pxValueExempt(val) {
			continue
		}
		loc := idx.DimensionsLocation[name]
		path := loc.FilePath
		if path == "" {
			path = "res/values/dimens.xml"
		}
		ctx.Emit(baseFinding(path, loc.Line, r.BaseRule,
			fmt.Sprintf("Dimension `%s` uses `px` value `%s`. Use `dp` or `sp` instead.",
				name, val)))
	}
	// values/styles.xml <style><item> entries (parity with AOSP checkStyleItem).
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			if !isPxValue(val) || pxValueExempt(val) {
				continue
			}
			ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule,
				fmt.Sprintf("Style `%s` item `%s` uses `px` value `%s`. Use `dp` or `sp` instead.",
					style.Name, itemName, val)))
		}
	}
}

// ---------------------------------------------------------------------------
// SpUsageResource
// ---------------------------------------------------------------------------

// SpUsageResourceRule detects android:textSize using dp instead of sp.
// Text sizes should use sp (scaled pixels) so they respect the user's font
// size preference.
type SpUsageResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func isDpValue(val string) bool {
	val = strings.TrimSpace(val)
	if !strings.HasSuffix(val, "dp") && !strings.HasSuffix(val, "dip") {
		return false
	}
	var prefix string
	if strings.HasSuffix(val, "dip") {
		prefix = val[:len(val)-3]
	} else {
		prefix = val[:len(val)-2]
	}
	if prefix == "" {
		return false
	}
	for _, c := range prefix {
		if (c < '0' || c > '9') && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *SpUsageResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SpUsageResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			val := v.Attributes["android:textSize"]
			if val == "" {
				return
			}
			if isDpValue(val) {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`android:textSize=\"%s\"` uses `dp`. Use `sp` instead so text respects the user's font size preference.",
						val)))
				return
			}
			if resolved, loc, ok := resolveDimenReference(idx, val); ok && isDpValue(resolved) {
				origin := dimenOriginSuffix(loc, resolved)
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`android:textSize=\"%s\"` resolves to a `dp` value%s. Use `sp` instead so text respects the user's font size preference.",
						val, origin)))
			}
		})
	}
	// Style items: <item name="android:textSize">14dp</item> or <item name="textSize">14dp</item>
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			if !isTextSizeAttr(itemName) {
				continue
			}
			if isDpValue(val) {
				ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule,
					fmt.Sprintf("Style `%s` item `%s=\"%s\"` uses `dp`. Use `sp` instead so text respects the user's font size preference.",
						style.Name, itemName, val)))
				continue
			}
			if resolved, loc, ok := resolveDimenReference(idx, val); ok && isDpValue(resolved) {
				origin := dimenOriginSuffix(loc, resolved)
				ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule,
					fmt.Sprintf("Style `%s` item `%s=\"%s\"` resolves to a `dp` value%s. Use `sp` instead so text respects the user's font size preference.",
						style.Name, itemName, val, origin)))
			}
		}
	}
}

// dimenOriginSuffix produces a short " (foo is N in path)"-style suffix
// pointing at the resolved dimension declaration. Empty when the source
// location is unknown.
func dimenOriginSuffix(loc android.StringLocation, resolved string) string {
	if loc.FilePath == "" {
		return fmt.Sprintf(" (%s)", resolved)
	}
	return fmt.Sprintf(" (`%s` in `%s`)", resolved, loc.FilePath)
}

// ---------------------------------------------------------------------------
// SmallSpResource
// ---------------------------------------------------------------------------

// SmallSpResourceRule detects android:textSize values below 12sp, which may
// be too small to read comfortably on most devices.
type SmallSpResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func parseSpValue(val string) (float64, bool) {
	val = strings.TrimSpace(val)
	if !strings.HasSuffix(val, "sp") {
		return 0, false
	}
	numStr := val[:len(val)-2]
	var num float64
	_, err := fmt.Sscanf(numStr, "%f", &num)
	if err != nil {
		return 0, false
	}
	return num, true
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *SmallSpResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SmallSpResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			textSize := v.Attributes["android:textSize"]
			if textSize == "" {
				return
			}
			if handleSpLiteral(ctx, r.BaseRule, layout.FilePath, v.Line, textSize, "", "android:textSize") {
				return
			}
			if resolved, loc, ok := resolveDimenReference(idx, textSize); ok {
				origin := dimenOriginSuffix(loc, resolved)
				handleSpLiteral(ctx, r.BaseRule, layout.FilePath, v.Line, resolved, origin, "android:textSize=\""+textSize+"\"")
			}
		})
	}
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			if !isTextSizeAttr(itemName) && !isLayoutHeightAttr(itemName) {
				continue
			}
			line := style.ItemLines[itemName]
			label := fmt.Sprintf("style `%s` item `%s`", style.Name, itemName)
			if handleSpLiteral(ctx, r.BaseRule, style.FilePath, line, val, "", label) {
				continue
			}
			if resolved, loc, ok := resolveDimenReference(idx, val); ok {
				origin := dimenOriginSuffix(loc, resolved)
				handleSpLiteral(ctx, r.BaseRule, style.FilePath, line, resolved, origin,
					fmt.Sprintf("%s=\"%s\"", label, val))
			}
		}
	}
}

// handleSpLiteral parses val as an sp literal; when below 12sp, emits a
// SmallSp finding. The returned bool means "val was a recognized sp value"
// — callers use it to skip the @dimen fallback, since a literal sp value
// has already been fully evaluated whether or not a finding was emitted.
func handleSpLiteral(ctx *api.Context, rule BaseRule, path string, line int, val, origin, label string) (handled bool) {
	sp, ok := parseSpValue(val)
	if !ok {
		return false
	}
	if sp < 12 {
		ctx.Emit(baseFinding(path, line, rule,
			fmt.Sprintf("Text size `%s`%s is too small (below 12sp) for `%s`. Consider using at least 12sp for readability.",
				val, origin, label)))
	}
	return true
}

// ---------------------------------------------------------------------------
// InOrMmUsageResource
// ---------------------------------------------------------------------------

// InOrMmUsageResourceRule detects dimension values using `in` (inches) or `mm`
// (millimeters) units. These units map to exact physical sizes and do not
// scale well across different screen sizes and densities.
type InOrMmUsageResourceRule struct {
	ResourceBase
	AndroidRule
}

func (InOrMmUsageResourceRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesDimensions
}

func isInOrMmValue(val string) (string, bool) {
	val = strings.TrimSpace(val)
	if strings.HasSuffix(val, "mm") {
		numStr := val[:len(val)-2]
		if numStr != "" && isNumeric(numStr) {
			return "mm", true
		}
	}
	if strings.HasSuffix(val, "in") {
		numStr := val[:len(val)-2]
		if numStr != "" && isNumeric(numStr) {
			return "in", true
		}
	}
	return "", false
}

func isNumeric(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *InOrMmUsageResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InOrMmUsageResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for _, attr := range pxDimensionAttrs {
				val := v.Attributes[attr]
				unit, ok := isInOrMmValue(val)
				if !ok || inOrMmValueExempt(val, unit) {
					continue
				}
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Avoid using `%s` units in `%s=\"%s\"`. Use `dp` or `sp` for density-independent sizing.",
						unit, attr, val)))
			}
		})
	}
	for name, val := range idx.Dimensions {
		unit, ok := isInOrMmValue(val)
		if !ok || inOrMmValueExempt(val, unit) {
			continue
		}
		loc := idx.DimensionsLocation[name]
		path := loc.FilePath
		if path == "" {
			path = "res/values/dimens.xml"
		}
		ctx.Emit(baseFinding(path, loc.Line, r.BaseRule,
			fmt.Sprintf("Dimension `%s` uses `%s` value `%s`. Use `dp` or `sp` instead.",
				name, unit, val)))
	}
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			unit, ok := isInOrMmValue(val)
			if !ok || inOrMmValueExempt(val, unit) {
				continue
			}
			ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule,
				fmt.Sprintf("Style `%s` item `%s` uses `%s` value `%s`. Use `dp` or `sp` instead.",
					style.Name, itemName, unit, val)))
		}
	}
}

// ---------------------------------------------------------------------------
// NegativeMarginResource
// ---------------------------------------------------------------------------

// NegativeMarginResourceRule detects negative margin values, which can cause
// views to overlap or be clipped unexpectedly.
type NegativeMarginResourceRule struct {
	LayoutResourceBase
	AndroidRule
	AllowedNegativeMargins []string
}

var marginAttrs = []string{
	"android:layout_margin",
	"android:layout_marginLeft", "android:layout_marginRight",
	"android:layout_marginTop", "android:layout_marginBottom",
	"android:layout_marginStart", "android:layout_marginEnd",
	"android:layout_marginHorizontal", "android:layout_marginVertical",
}

func isNegativeMargin(val string) bool {
	val = strings.TrimSpace(val)
	return strings.HasPrefix(val, "-")
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *NegativeMarginResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NegativeMarginResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// Combine all negative-margin attributes on a single view
			// into one finding to avoid duplicate (file,line,col,rule)
			// keys downstream.
			var offenders []string
			for _, attr := range marginAttrs {
				val := v.Attributes[attr]
				if isNegativeMargin(val) {
					if r.negativeMarginAllowed(v.Type, attr, val) {
						continue
					}
					offenders = append(offenders, fmt.Sprintf("`%s=\"%s\"`", attr, val))
				}
			}
			if len(offenders) == 0 {
				return
			}
			if len(offenders) == 1 {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Negative margin %s in `%s`. Negative margins can cause overlapping or clipping.",
						offenders[0], v.Type)))
				return
			}
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Negative margins in `%s`: %s. Negative margins can cause overlapping or clipping.",
					v.Type, strings.Join(offenders, ", "))))
		})
	}
}

func (r *NegativeMarginResourceRule) negativeMarginAllowed(viewType, attr, val string) bool {
	for _, pattern := range r.AllowedNegativeMargins {
		if negativeMarginPatternMatches(pattern, viewType, attr, val) {
			return true
		}
	}
	return false
}

func negativeMarginPatternMatches(pattern, viewType, attr, val string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	candidates := []string{
		viewType + ":" + attr + "=" + val,
		viewType + ":" + attr,
		viewType + ":*=" + val,
		attr + "=" + val,
		attr,
		val,
	}
	for _, candidate := range candidates {
		if pattern == candidate {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
