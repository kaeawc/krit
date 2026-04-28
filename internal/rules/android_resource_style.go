package rules

// Android Resource XML rules: Dimension/style rules.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *PxUsageResourceRule) Confidence() float64 { return 0.75 }

func (r *PxUsageResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for _, attr := range pxDimensionAttrs {
				val := v.Attributes[attr]
				if isPxValue(val) {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Avoid using `px` in `%s=\"%s\"`. Use `dp` or `sp` instead for density-independent sizing.",
							attr, val)))
				}
			}
		})
	}
	// Also check values/dimens
	for name, val := range idx.Dimensions {
		if isPxValue(val) {
			ctx.Emit(resourceFinding("res/values/dimens.xml", 0, r.BaseRule,
				fmt.Sprintf("Dimension `%s` uses `px` value `%s`. Use `dp` or `sp` instead.",
					name, val)))
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
func (r *SpUsageResourceRule) Confidence() float64 { return 0.75 }

func (r *SpUsageResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			val := v.Attributes["android:textSize"]
			if isDpValue(val) {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`android:textSize=\"%s\"` uses `dp`. Use `sp` instead so text respects the user's font size preference.",
						val)))
			}
		})
	}
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
func (r *SmallSpResourceRule) Confidence() float64 { return 0.75 }

func (r *SmallSpResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			textSize := v.Attributes["android:textSize"]
			if textSize == "" {
				return
			}
			sp, ok := parseSpValue(textSize)
			if !ok {
				return
			}
			if sp < 12 {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Text size `%s` is too small (below 12sp). Consider using at least 12sp for readability.",
						textSize)))
			}
		})
	}
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
func (r *InOrMmUsageResourceRule) Confidence() float64 { return 0.75 }

func (r *InOrMmUsageResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for _, attr := range pxDimensionAttrs {
				val := v.Attributes[attr]
				if unit, ok := isInOrMmValue(val); ok {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Avoid using `%s` units in `%s=\"%s\"`. Use `dp` or `sp` for density-independent sizing.",
							unit, attr, val)))
				}
			}
		})
	}
	// Also check values/dimens
	for name, val := range idx.Dimensions {
		if unit, ok := isInOrMmValue(val); ok {
			ctx.Emit(resourceFinding("res/values/dimens.xml", 0, r.BaseRule,
				fmt.Sprintf("Dimension `%s` uses `%s` value `%s`. Use `dp` or `sp` instead.",
					name, unit, val)))
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
func (r *NegativeMarginResourceRule) Confidence() float64 { return 0.75 }

func (r *NegativeMarginResourceRule) check(ctx *v2.Context) {
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
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Negative margin %s in `%s`. Negative margins can cause overlapping or clipping.",
						offenders[0], v.Type)))
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
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
// Suspicious0dpResource
// ---------------------------------------------------------------------------

// Suspicious0dpResourceRule detects 0dp dimension used on the wrong axis in a
// LinearLayout. In horizontal orientation, 0dp should be on layout_width (for
// weight distribution); 0dp on layout_height is suspicious (and vice versa).
type Suspicious0dpResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *Suspicious0dpResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViewsWithParent(layout.RootView, nil, func(v, parent *android.View) {
			if parent == nil || parent.Type != "LinearLayout" {
				return
			}
			orientation := parent.Attributes["android:orientation"]
			w := v.Attributes["android:layout_width"]
			h := v.Attributes["android:layout_height"]

			if orientation == "horizontal" {
				// In horizontal, 0dp height is suspicious
				if h == "0dp" && w != "0dp" {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						"Suspicious `0dp` on `layout_height` in horizontal `LinearLayout`. Did you mean `layout_width=\"0dp\"` (for weight)?"))
				}
			} else if orientation == "vertical" {
				// In vertical, 0dp width is suspicious
				if w == "0dp" && h != "0dp" {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						"Suspicious `0dp` on `layout_width` in vertical `LinearLayout`. Did you mean `layout_height=\"0dp\"` (for weight)?"))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DisableBaselineAlignmentResource
// ---------------------------------------------------------------------------

// DisableBaselineAlignmentResourceRule detects LinearLayout views that have
// children with layout_weight but the parent does not set
// baselineAligned="false". Baseline alignment is expensive with weighted children.
type DisableBaselineAlignmentResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *DisableBaselineAlignmentResourceRule) Confidence() float64 { return 0.75 }

func (r *DisableBaselineAlignmentResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	requireText := experiment.Enabled("disable-baseline-alignment-require-text-children")
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "LinearLayout" && v.Type != "AppCompatLinearLayout" {
				return
			}
			// baselineAligned only affects horizontal LinearLayouts.
			// Default orientation is horizontal, so treat missing as horizontal.
			orientation := v.Attributes["android:orientation"]
			if orientation == "vertical" {
				return
			}
			hasWeight := false
			for _, child := range v.Children {
				if child.Attributes["android:layout_weight"] != "" {
					hasWeight = true
					break
				}
			}
			if !hasWeight {
				return
			}
			if v.Attributes["android:baselineAligned"] == "false" {
				return
			}
			// Opt-in: only flag when at least one direct child is a
			// text-displaying view. LinearLayouts whose children are
			// nested containers (LinearLayout/FrameLayout) or images
			// have negligible baseline-alignment overhead and reporting
			// them is noise.
			if requireText && !hasDirectTextChild(v) {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("`%s` with weighted children should set `android:baselineAligned=\"false\"` for better performance.", v.Type)))
		})
	}
}

// hasDirectTextChild reports whether the view has at least one direct
// (non-descendant) child whose type is a Text-displaying view. Used by
// DisableBaselineAlignmentResource's experiment gate.
func hasDirectTextChild(v *android.View) bool {
	for _, child := range v.Children {
		if isTextDisplayingViewType(child.Type) {
			return true
		}
	}
	return false
}

// isTextDisplayingViewType returns true for view types whose baseline is
// set by a text element and participate in LinearLayout baseline alignment.
// Includes AppCompat / Material variants of each.
func isTextDisplayingViewType(t string) bool {
	// Strip package prefix if present, keeping only the final segment.
	if idx := strings.LastIndex(t, "."); idx >= 0 {
		t = t[idx+1:]
	}
	switch t {
	case "TextView", "AppCompatTextView", "MaterialTextView",
		"EditText", "AppCompatEditText", "TextInputEditText",
		"Button", "AppCompatButton", "MaterialButton",
		"CheckBox", "AppCompatCheckBox", "MaterialCheckBox",
		"RadioButton", "AppCompatRadioButton", "MaterialRadioButton",
		"ToggleButton", "Switch", "SwitchCompat", "SwitchMaterial",
		"Chip", "ChipGroup",
		"EmojiTextView", "EmojiEditText",
		"Spinner", "AppCompatSpinner",
		"AutoCompleteTextView", "MultiAutoCompleteTextView":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// InefficientWeightResource
// ---------------------------------------------------------------------------

// InefficientWeightResourceRule detects a LinearLayout that uses layout_weight
// on children but does not specify an android:orientation attribute. Without
// orientation, the default is horizontal, which can be unintentional.
type InefficientWeightResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *InefficientWeightResourceRule) Confidence() float64 { return 0.75 }

func (r *InefficientWeightResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "LinearLayout" && v.Type != "AppCompatLinearLayout" {
				return
			}
			// Check if any child uses layout_weight
			hasWeight := false
			for _, child := range v.Children {
				if child.Attributes["android:layout_weight"] != "" {
					hasWeight = true
					break
				}
			}
			if !hasWeight {
				return
			}
			// Check if orientation is missing
			if v.Attributes["android:orientation"] == "" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` uses `layout_weight` but is missing `android:orientation`. "+
						"Declare orientation explicitly when using weights.", v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NestedWeightsResource
// ---------------------------------------------------------------------------

// NestedWeightsResourceRule detects layout_weight inside a child LinearLayout
// that itself is inside a LinearLayout with weights. Nested weights cause
// exponential measure passes.
type NestedWeightsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *NestedWeightsResourceRule) Confidence() float64 { return 0.75 }

func (r *NestedWeightsResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		findNestedWeights(layout.RootView, layout.FilePath, r.BaseRule, &findings)
	}
	for _, f := range findings {
		ctx.Emit(f)
	}
}

func isLinearLayout(viewType string) bool {
	return viewType == "LinearLayout" || viewType == "AppCompatLinearLayout"
}

func hasWeightedChildren(v *android.View) bool {
	for _, child := range v.Children {
		if child.Attributes["android:layout_weight"] != "" {
			return true
		}
	}
	return false
}

// findNestedWeights flags a LinearLayout that simultaneously has layout_weight
// set on itself (parent weighs it) AND has weighted children (it weighs its
// own kids). That combination is what causes the exponential measure pass
// cascade. A LinearLayout merely containing weighted children is fine.
func findNestedWeights(v *android.View, path string, rule BaseRule, findings *[]scanner.Finding) {
	if v == nil {
		return
	}
	if isLinearLayout(v.Type) &&
		v.Attributes["android:layout_weight"] != "" &&
		hasWeightedChildren(v) {
		*findings = append(*findings, resourceFinding(path, v.Line, rule,
			fmt.Sprintf("Nested weights: `%s` has `layout_weight` set AND contains weighted children. "+
				"This causes exponential measure passes — restructure to avoid nesting weights.", v.Type)))
	}
	for _, child := range v.Children {
		findNestedWeights(child, path, rule, findings)
	}
}

// ---------------------------------------------------------------------------
// ObsoleteLayoutParamsResource
// ---------------------------------------------------------------------------

// ObsoleteLayoutParamsResourceRule detects layout_weight on children of
// non-LinearLayout parents. The layout_weight attribute is only meaningful
// inside a LinearLayout.
type ObsoleteLayoutParamsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *ObsoleteLayoutParamsResourceRule) Confidence() float64 { return 0.75 }

func (r *ObsoleteLayoutParamsResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		checkObsoleteParams(layout.RootView, layout.FilePath, r.BaseRule, &findings)
	}
	for _, f := range findings {
		ctx.Emit(f)
	}
}

func checkObsoleteParams(v *android.View, path string, rule BaseRule, findings *[]scanner.Finding) {
	if v == nil {
		return
	}
	isLinear := v.Type == "LinearLayout" || v.Type == "AppCompatLinearLayout"
	for _, child := range v.Children {
		if !isLinear {
			if child.Attributes["android:layout_weight"] != "" {
				*findings = append(*findings, resourceFinding(path, child.Line, rule,
					fmt.Sprintf("`android:layout_weight` on `%s` is only valid inside `LinearLayout`. "+
						"Parent is `%s`.", child.Type, v.Type)))
			}
		}
		checkObsoleteParams(child, path, rule, findings)
	}
}

// ---------------------------------------------------------------------------
// MergeRootFrameResource
// ---------------------------------------------------------------------------

// MergeRootFrameResourceRule detects a root FrameLayout that could be
// replaced with a <merge> tag to reduce one level of nesting when the
// layout is included in another layout.
type MergeRootFrameResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *MergeRootFrameResourceRule) Confidence() float64 { return 0.75 }

func (r *MergeRootFrameResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		root := layout.RootView
		if root == nil {
			continue
		}
		if root.Type != "FrameLayout" {
			continue
		}
		// Only flag if the FrameLayout has no background, no padding, no ID,
		// and uses match_parent for both dimensions (default root behavior).
		if root.Background != "" || root.ID != "" {
			continue
		}
		if hasAnyPadding(root) {
			continue
		}
		ctx.Emit(resourceFinding(layout.FilePath, root.Line, r.BaseRule,
			fmt.Sprintf("Root `FrameLayout` in `%s` can be replaced with `<merge>` tag to reduce nesting.",
				layout.Name)))
	}
}

// ---------------------------------------------------------------------------
// OverdrawResource
// ---------------------------------------------------------------------------

// OverdrawResourceRule detects a root layout with a background where an
// immediate child layout also has a background. This causes overdraw — the
// GPU paints pixels that are immediately covered by the child's background.
type OverdrawResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// isTransparentBackground returns true if the background value is transparent
// and thus does not contribute to overdraw.
func isTransparentBackground(bg string) bool {
	bg = strings.TrimSpace(bg)
	if bg == "" {
		return true
	}
	if bg == "@android:color/transparent" || bg == "@color/transparent" {
		return true
	}
	// Handle qualified references like @package:color/transparent
	if strings.HasSuffix(bg, "/transparent") {
		return true
	}
	// Literal transparent hex colors
	if bg == "#00000000" || bg == "#0000" {
		return true
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *OverdrawResourceRule) Confidence() float64 { return 0.75 }

func (r *OverdrawResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		root := layout.RootView
		if root == nil || root.Background == "" || isTransparentBackground(root.Background) {
			continue
		}
		for _, child := range root.Children {
			if child.Background == "" || isTransparentBackground(child.Background) {
				continue
			}
			if android.IsLayoutView(child.Type) {
				ctx.Emit(resourceFinding(layout.FilePath, child.Line, r.BaseRule,
					fmt.Sprintf("Possible overdraw: child `%s` has background `%s` and root `%s` also has background `%s`. "+
						"Remove one background to reduce overdraw.",
						child.Type, child.Background, root.Type, root.Background)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AlwaysShowActionResource
// ---------------------------------------------------------------------------

// AlwaysShowActionResourceRule detects showAsAction="always" in menu or layout
// XML. Using "always" can crowd the action bar on small screens. Prefer
// "ifRoom" to let the system decide.
type AlwaysShowActionResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *AlwaysShowActionResourceRule) Confidence() float64 { return 0.75 }

func (r *AlwaysShowActionResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			val := v.Attributes["app:showAsAction"]
			if val == "" {
				val = v.Attributes["android:showAsAction"]
			}
			if strings.Contains(val, "always") {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`showAsAction=\"%s\"` in `%s`. Use `ifRoom` instead of `always` to avoid crowding the action bar on small screens.",
						val, v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// StateListReachableResource
// ---------------------------------------------------------------------------

// StateListReachableResourceRule detects unreachable items in selector
// drawables. A non-last <item> with no state attributes (android:state_*)
// catches all states, making subsequent items unreachable.
//
// Stub: full implementation requires drawable XML parsing to inspect
// <selector> child items and their android:state_* attributes.
type StateListReachableResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *StateListReachableResourceRule) Confidence() float64 { return 0.75 }

func (r *StateListReachableResourceRule) check(_ *v2.Context) {
	// Drawable XML parsing is not yet available in ResourceIndex.
	// This is a placeholder that returns no findings until drawable XML
	// parsing support is added.
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
