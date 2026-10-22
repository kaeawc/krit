package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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

func (r *Suspicious0dpResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViewsWithParent(layout.RootView, nil, func(v, parent *android.View) {
			if parent == nil || parent.Type != "LinearLayout" {
				return
			}
			orientation := parent.Attributes["android:orientation"]
			w := v.Attributes["android:layout_width"]
			h := v.Attributes["android:layout_height"]

			switch orientation {
			case "horizontal":
				// In horizontal, 0dp height is suspicious
				if h == "0dp" && w != "0dp" {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						"Suspicious `0dp` on `layout_height` in horizontal `LinearLayout`. Did you mean `layout_width=\"0dp\"` (for weight)?"))
				}
			case "vertical":
				// In vertical, 0dp width is suspicious
				if w == "0dp" && h != "0dp" {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *DisableBaselineAlignmentResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DisableBaselineAlignmentResourceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *InefficientWeightResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InefficientWeightResourceRule) check(ctx *api.Context) {
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
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *NestedWeightsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NestedWeightsResourceRule) check(ctx *api.Context) {
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
		*findings = append(*findings, baseFinding(path, v.Line, rule,
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
func (r *ObsoleteLayoutParamsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ObsoleteLayoutParamsResourceRule) check(ctx *api.Context) {
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
				*findings = append(*findings, baseFinding(path, child.Line, rule,
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
func (r *MergeRootFrameResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MergeRootFrameResourceRule) check(ctx *api.Context) {
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
		ctx.Emit(baseFinding(layout.FilePath, root.Line, r.BaseRule,
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
func (r *OverdrawResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OverdrawResourceRule) check(ctx *api.Context) {
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
				ctx.Emit(baseFinding(layout.FilePath, child.Line, r.BaseRule,
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
func (r *AlwaysShowActionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AlwaysShowActionResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			val := v.Attributes["app:showAsAction"]
			if val == "" {
				val = v.Attributes["android:showAsAction"]
			}
			if strings.Contains(val, "always") {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
// drawables. Selector items are matched top-down and chosen when the
// runtime state set is a superset of the item's state qualifiers, so an
// item whose qualifier set is a superset of any earlier item's is
// unreachable.
type StateListReachableResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *StateListReachableResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StateListReachableResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	if idx == nil {
		return
	}
	for _, items := range idx.DrawableSelectors {
		for j := 1; j < len(items); j++ {
			for i := 0; i < j; i++ {
				if !stateAttrsSubsetOf(items[i].StateAttrs, items[j].StateAttrs) {
					continue
				}
				ctx.Emit(scanner.Finding{
					File:       items[j].FilePath,
					Line:       items[j].Line,
					Col:        1,
					RuleSet:    r.RuleSetName,
					Rule:       r.RuleName,
					Severity:   r.Sev,
					Message:    fmt.Sprintf("This selector item is unreachable because item #%d is a more general match.", i+1),
					Confidence: r.Confidence(),
				})
				break
			}
		}
	}
}

// stateAttrsSubsetOf reports whether a's qualifiers are a subset of b's —
// meaning b is matched only when a is also matched, so b is unreachable.
func stateAttrsSubsetOf(a, b map[string]string) bool {
	for name, value := range a {
		if bv, ok := b[name]; !ok || bv != value {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// AaptCrash
// ---------------------------------------------------------------------------

// AaptCrashRule detects a `<style>` that sets `android:id` to a freshly
// generated id (`@+id/...`). Declaring a new id inside a style definition
// crashes many versions of the resource packaging tool during the build, so
// this is a Fatal correctness check. The fix is to declare the id explicitly
// in a values file (`<item type="id" name="..."/>`) and reference it with the
// non-generating `@id/...` form from the style.
type AaptCrashRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. The crash trigger is
// read from the parsed style index (style name -> item name -> value), not a
// text scan, but values resources can carry project-specific shapes, so this
// is classified medium per roadmap/17.
func (r *AaptCrashRule) Confidence() float64 { return api.ConfidenceMedium }

// aaptCrashIsGeneratedID reports whether val is an id-generating reference.
// Only the `@+id/` (and namespace-qualified `@+android:id/`) forms allocate a
// new id; the plain `@id/...` reference does not and is safe inside a style.
func aaptCrashIsGeneratedID(val string) bool {
	v := strings.TrimSpace(val)
	return strings.HasPrefix(v, "@+id/") || strings.HasPrefix(v, "@+android:id/")
}

func (r *AaptCrashRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, style := range idx.Styles {
		val, ok := style.Items["android:id"]
		if !ok {
			continue
		}
		if !aaptCrashIsGeneratedID(val) {
			continue
		}
		ctx.Emit(baseFinding(style.FilePath, style.ItemLines["android:id"], r.BaseRule,
			fmt.Sprintf("Style `%s` sets `android:id` to a dynamically generated id `%s`, "+
				"which can crash the resource packaging tool at build time. Declare the id "+
				"explicitly with `<item type=\"id\" name=\"...\"/>` and reference it with `@id/...`.",
				style.Name, strings.TrimSpace(val))))
	}
}
