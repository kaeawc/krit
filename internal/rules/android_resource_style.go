package rules

// Android Resource XML rules: Dimension/style rules.

import (
	"fmt"
	"regexp"
	"sort"
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
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"PxUsage"}, func(v *android.View) {
			for _, attr := range pxDimensionAttrs {
				val := v.Attributes[attr]
				if isPxValue(val) && !pxValueExempt(val) {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
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
	const msg = "Should use \"`sp`\" instead of \"`dp`\" for text sizes"
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"SpUsage"}, func(v, parent *android.View) {
			val := v.Attributes["android:textSize"]
			if val == "" {
				return
			}
			// Not in Lint: a fixed dp width and height box (icon badge, emoji swatch, avatar) is sized in dp, so its text is too.
			if hasFixedDpBox(idx, v) || (parent != nil && hasFixedDpBox(idx, parent)) {
				return
			}
			if isLintDpUnit(val) {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:textSize", r.BaseRule, msg))
				return
			}
			if resolved, loc, ok := resolveDimenReference(idx, val); ok && isLintDpUnit(resolved) && strings.HasPrefix(val, "@dimen/") {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:textSize", r.BaseRule,
					fmt.Sprintf("%s (`%s` is defined as `%s` in `%s`", msg, val, resolved, lintFileNameWithParent(loc.FilePath))))
			}
		})
	}
	// Style items: <item name="android:textSize">14dp</item> or <item name="textSize">14dp</item>.
	// Lint does not resolve @dimen references in style items.
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			if isTextSizeAttr(itemName) && isLintDpUnit(val) {
				ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule, msg))
			}
		}
	}
}

var lintDpUnitRegexp = regexp.MustCompile(`^\d+(\.\d+)?di?p$`)

// isLintDpUnit mirrors PxUsageDetector.isDpUnit: a non-negative number followed by dp or dip.
func isLintDpUnit(val string) bool {
	return lintDpUnitRegexp.MatchString(val)
}

// hasFixedDpBox reports whether both layout dimensions of v are dp values, literal or through @dimen.
func hasFixedDpBox(idx *android.ResourceIndex, v *android.View) bool {
	isFixed := func(val string) bool {
		if isDpValue(val) {
			return true
		}
		resolved, _, ok := resolveDimenReference(idx, val)
		return ok && isDpValue(resolved)
	}
	return isFixed(v.LayoutWidth) && isFixed(v.LayoutHeight)
}

// ---------------------------------------------------------------------------
// SmallSpResource
// ---------------------------------------------------------------------------

// SmallSpResourceRule mirrors Lint's SmallSp check: a literal sp size below 11sp in a textSize or
// layout_height attribute. Lint truncates the size to an integer, so 10.9sp is reported.
type SmallSpResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var lintSpUnitRegexp = regexp.MustCompile(`^\d+(\.\d+)?sp$`)

// Confidence reports a tier-2 (medium) base confidence.
func (r *SmallSpResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SmallSpResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"SmallSp"}, func(v *android.View) {
			for _, attr := range []string{"android:textSize", "android:layout_height"} {
				if val := v.Attributes[attr]; isSmallLintSp(val) {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule, smallSpMessage(val)))
				}
			}
		})
	}
	// Lint only matches style items whose name has no `android:` prefix, so `android:textSize` items are not reported.
	for _, style := range idx.Styles {
		for itemName, val := range style.Items {
			if (itemName == "textSize" || itemName == "layout_height") && isSmallLintSp(val) {
				ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule, smallSpMessage(val)))
			}
		}
	}
}

func smallSpMessage(val string) string {
	return fmt.Sprintf("Avoid using sizes smaller than `11sp`: `%s`", val)
}

// isSmallLintSp reports whether val is an sp size whose integer part is between 1 and 10.
func isSmallLintSp(val string) bool {
	if !lintSpUnitRegexp.MatchString(val) {
		return false
	}
	whole := val[:len(val)-2]
	if dot := strings.IndexByte(whole, '.'); dot >= 0 {
		whole = whole[:dot]
	}
	n, err := strconv.Atoi(whole)
	return err == nil && n > 0 && n < 11
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

// inOrMmValuePattern matches the whole value forms Lint's PxUsageDetector reports.
var inOrMmValuePattern = regexp.MustCompile(`^\d+(\.\d+)?(mm|in)$`)

func isInOrMmValue(val string) (string, bool) {
	val = strings.TrimSpace(val)
	if !inOrMmValuePattern.MatchString(val) {
		return "", false
	}
	return val[len(val)-2:], true
}

func inOrMmMessage(unit string) string {
	return fmt.Sprintf("Avoid using \"`%s`\" as units (it does not work accurately on all devices); use \"`dp`\" instead", unit)
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *InOrMmUsageResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InOrMmUsageResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"InOrMmUsage"}, func(v *android.View) {
			attrs := make([]string, 0, len(v.Attributes))
			for attr := range v.Attributes {
				attrs = append(attrs, attr)
			}
			sort.Strings(attrs)
			for _, attr := range attrs {
				val := v.Attributes[attr]
				unit, ok := isInOrMmValue(val)
				if !ok || inOrMmValueExempt(val, unit) {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule, inOrMmMessage(unit)))
			}
		})
	}
	dimenNames := make([]string, 0, len(idx.Dimensions))
	for name := range idx.Dimensions {
		dimenNames = append(dimenNames, name)
	}
	sort.Strings(dimenNames)
	for _, name := range dimenNames {
		val := idx.Dimensions[name]
		unit, ok := isInOrMmValue(val)
		if !ok || inOrMmValueExempt(val, unit) {
			continue
		}
		loc := idx.DimensionsLocation[name]
		path := loc.FilePath
		if path == "" {
			path = "res/values/dimens.xml"
		}
		ctx.Emit(baseFinding(path, loc.Line, r.BaseRule, inOrMmMessage(unit)))
	}
	for _, style := range idx.Styles {
		itemNames := make([]string, 0, len(style.Items))
		for itemName := range style.Items {
			itemNames = append(itemNames, itemName)
		}
		sort.Strings(itemNames)
		for _, itemName := range itemNames {
			val := style.Items[itemName]
			unit, ok := isInOrMmValue(val)
			if !ok || inOrMmValueExempt(val, unit) {
				continue
			}
			ctx.Emit(baseFinding(style.FilePath, style.ItemLines[itemName], r.BaseRule, inOrMmMessage(unit)))
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

// negativeMarginMayClip reports whether a child's negative margin is likely to
// be clipped; overlap-friendly parents and clipChildren=false parents are not.
func negativeMarginMayClip(parent *android.View) bool {
	if parent == nil || parent.Attributes["android:clipChildren"] == "false" {
		return false
	}
	simple := parent.Type
	if i := strings.LastIndex(simple, "."); i >= 0 {
		simple = simple[i+1:]
	}
	switch simple {
	case "ConstraintLayout", "FrameLayout", "RelativeLayout":
		return false
	}
	return true
}

// smallTightening reports whether a margin of at most 4dp on a wrap_content child of a vertical LinearLayout.
// Such margins tighten line spacing rather than overlap views.
func smallTightening(v, parent *android.View, val string) bool {
	if parent == nil || parent.Type != "LinearLayout" || parent.Attributes["android:orientation"] != "vertical" ||
		v.LayoutHeight != "wrap_content" {
		return false
	}
	val = strings.TrimSpace(val)
	return isDpValue(val) && parseLayoutDp(strings.TrimPrefix(val, "-")) <= 4
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
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"NegativeMargin"}, func(v, parent *android.View) {
			if !negativeMarginMayClip(parent) {
				return
			}
			// Combine all negative-margin attributes on a single view
			// into one finding to avoid duplicate (file,line,col,rule)
			// keys downstream.
			var offenders []string
			firstAttr := ""
			for _, attr := range marginAttrs {
				val := v.Attributes[attr]
				if isNegativeMargin(val) {
					if r.negativeMarginAllowed(v.Type, attr, val) || smallTightening(v, parent, val) {
						continue
					}
					if firstAttr == "" {
						firstAttr = attr
					}
					offenders = append(offenders, fmt.Sprintf("`%s=\"%s\"`", attr, val))
				}
			}
			if len(offenders) == 0 {
				return
			}
			if len(offenders) == 1 {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, firstAttr, r.BaseRule,
					fmt.Sprintf("Negative margin %s in `%s`. Negative margins can cause overlapping or clipping.",
						offenders[0], v.Type)))
				return
			}
			ctx.Emit(baseFindingAtAttr(layout.FilePath, v, firstAttr, r.BaseRule,
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
