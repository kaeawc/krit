package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Suspicious0dpResource
// ---------------------------------------------------------------------------

// Suspicious0dpResourceRule mirrors Lint's InefficientWeightDetector.checkWrong0Dp: in a
// LinearLayout, a child whose layout_width or layout_height starts with 0 must have a
// layout_weight, and the 0 must be on the orientation axis. Like Lint, the scan of a
// layout's children stops at a custom view, at a child with no 0 size, at a Space and at
// a child that is 0 on both axes.
type Suspicious0dpResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *Suspicious0dpResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"Suspicious0dp"}, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			r.checkChildren(ctx, layout.FilePath, v)
		})
	}
}

func (r *Suspicious0dpResourceRule) checkChildren(ctx *api.Context, path string, parent *android.View) {
	vertical := parent.Attributes["android:orientation"] == "vertical"
	for _, child := range parent.Children {
		if child.Type == "View" || child.Type == "view" || strings.Contains(child.Type, ".") {
			return
		}
		_, hasWeight := child.Attributes["android:layout_weight"]
		width, hasWidth := child.Attributes["android:layout_width"]
		height, hasHeight := child.Attributes["android:layout_height"]
		zeroWidth := hasWidth && strings.HasPrefix(width, "0")
		zeroHeight := hasHeight && strings.HasPrefix(height, "0")
		if !zeroWidth && !zeroHeight {
			return
		}
		if child.Type == "Space" || (zeroWidth && zeroHeight) {
			return
		}
		attr := "android:layout_height"
		if zeroWidth {
			attr = "android:layout_width"
		}
		msg := ""
		switch {
		case !hasWeight:
			msg = "Suspicious size: this will make the view invisible, should be used with `layout_weight`"
		case zeroWidth && vertical:
			msg = "Suspicious size: this will make the view invisible, probably intended for `layout_height`"
		case zeroHeight && !vertical:
			msg = "Suspicious size: this will make the view invisible, probably intended for `layout_width`"
		default:
			continue
		}
		ctx.Emit(baseFindingAtAttr(path, child, attr, r.BaseRule, msg))
	}
}

// ---------------------------------------------------------------------------
// DisableBaselineAlignmentResource
// ---------------------------------------------------------------------------

// DisableBaselineAlignmentResourceRule mirrors Lint's InefficientWeightDetector
// BASELINE_WEIGHTS check: a LinearLayout that is not vertical, sets no
// android:baselineAligned and distributes weight over nested layouts only.
type DisableBaselineAlignmentResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *DisableBaselineAlignmentResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// isLintLayoutTag mirrors Lint's SdkInfo.isLayout: a tag that ends in "Layout"
// or names one of the platform ViewGroups that do not.
func isLintLayoutTag(tag string) bool {
	if strings.HasSuffix(tag, "Layout") {
		return true
	}
	switch tag {
	case "MediaController", "SlidingDrawer", "RadioGroup", "TabHost", "ImageSwitcher", "TabWidget",
		"AdapterViewFlipper", "ViewSwitcher", "DialerFilter", "ViewFlipper", "ListView", "ViewAnimator",
		"ScrollView", "SearchView", "TableRow", "TextSwitcher", "GridView", "StackView",
		"HorizontalScrollView", "ExpandableListView":
		return true
	}
	return false
}

func (r *DisableBaselineAlignmentResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"DisableBaselineAlignment"}, func(v *android.View) {
			if v.Type != "LinearLayout" || len(v.Children) == 0 {
				return
			}
			if v.Attributes["android:orientation"] == "vertical" {
				return
			}
			if _, set := v.Attributes["android:baselineAligned"]; set {
				return
			}
			weighted := false
			for _, child := range v.Children {
				if _, ok := child.Attributes["android:layout_weight"]; ok {
					weighted = true
				}
				// Only nested layouts gain from skipping baseline alignment; RadioGroup keeps its baseline.
				nested := (isLintLayoutTag(child.Type) && child.Type != "RadioGroup") || child.Type == "fragment" || child.Type == "include"
				if !nested {
					return
				}
			}
			if !weighted {
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				"Set `android:baselineAligned=\"false\"` on this element for better performance"))
		})
	}
}

// ---------------------------------------------------------------------------
// InefficientWeightResource
// ---------------------------------------------------------------------------

// InefficientWeightResourceRule detects a LinearLayout whose only child with a
// layout_weight does not use 0dp along the layout direction. Like Lint it looks
// at plain LinearLayout tags, needs exactly one weighted child, and reports an
// undefined size too. A 0dp size lets the weight alone decide the size and
// skips a measure pass.
type InefficientWeightResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *InefficientWeightResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InefficientWeightResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"InefficientWeight"}, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			var weighted *android.View
			for _, child := range v.Children {
				if _, ok := child.Attributes["android:layout_weight"]; !ok {
					continue
				}
				if weighted != nil {
					return
				}
				weighted = child
			}
			if weighted == nil || resourceToolsIgnoreMatches(weighted, "InefficientWeight") {
				return
			}
			sizeAttr := "android:layout_width"
			if v.Attributes["android:orientation"] == "vertical" {
				sizeAttr = "android:layout_height"
			}
			size, hasSize := weighted.Attributes[sizeAttr]
			if !hasSize {
				// A style may supply the size, and styles are not resolved here.
				if _, styled := weighted.Attributes["style"]; styled {
					return
				}
				size = "(undefined)"
			}
			if strings.HasPrefix(size, "0") {
				return
			}
			ctx.Emit(baseFindingAtAttr(layout.FilePath, weighted, sizeAttr, r.BaseRule,
				fmt.Sprintf("Use a `%s` of `0dp` instead of `%s` for better performance",
					strings.TrimPrefix(sizeAttr, "android:"), size)))
		})
	}
}

// ---------------------------------------------------------------------------
// NestedWeightsResource
// ---------------------------------------------------------------------------

// NestedWeightsResourceRule detects a layout_weight on a child of a
// LinearLayout that is itself a weighted child of another LinearLayout. Like
// Lint it reports the first such child, on its layout_weight attribute.
type NestedWeightsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *NestedWeightsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NestedWeightsResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		insideWeight := map[*android.View]bool{}
		walkViewsUnlessIgnored(layout.RootView, []string{"NestedWeights"}, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			for _, child := range v.Children {
				if _, ok := child.Attributes["android:layout_weight"]; !ok {
					continue
				}
				insideWeight[child] = true
				inside, known := insideWeight[v]
				if !known {
					insideWeight[v] = false
					continue
				}
				if inside && !resourceToolsIgnoreMatches(child, "NestedWeights") {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:layout_weight", r.BaseRule,
						"Nested weights are bad for performance"))
					return
				}
			}
		})
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
	for _, f := range obsoleteLayoutParamFindings(ctx.ResourceIndex.AllLayouts(), r.BaseRule) {
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// MergeRootFrameResource
// ---------------------------------------------------------------------------

// MergeRootFrameResourceRule detects a root FrameLayout, in a layout that is
// pulled in with <include>, that could be replaced with a <merge> tag to
// reduce one level of nesting. Layouts never included are skipped because a
// <merge> root cannot be inflated without a parent (fragments, view holders).
type MergeRootFrameResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *MergeRootFrameResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// includedLayoutNames collects layout names referenced by <include layout="@layout/...">.
func includedLayoutNames(idx *android.ResourceIndex) map[string]bool {
	names := map[string]bool{}
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type == "include" {
				if ref, ok := strings.CutPrefix(v.Attributes["layout"], "@layout/"); ok {
					names[ref] = true
				}
			}
		})
	}
	return names
}

func (r *MergeRootFrameResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	// A <merge> root only inflates into a parent, so it is safe only for layouts pulled in with <include>.
	included := includedLayoutNames(idx)
	for _, layout := range idx.AllLayouts() {
		root := layout.RootView
		if root == nil || !included[layout.Name] {
			continue
		}
		if root.Type != "FrameLayout" || resourceToolsIgnoreMatches(root, "MergeRootFrame") {
			continue
		}
		// Lint skips a root with a layout_gravity unless it fills its parent in both directions.
		fills := isMatchParent(root.Attributes["android:layout_width"]) && isMatchParent(root.Attributes["android:layout_height"])
		if _, gravity := root.Attributes["android:layout_gravity"]; gravity && !fills {
			continue
		}
		if !mergeRootFrameCandidate(root) {
			continue
		}
		// A style can switch on fitsSystemWindows; styles are not resolved, so a styled root is left alone.
		if _, styled := root.Attributes["style"]; styled {
			continue
		}
		ctx.Emit(baseFindingAt(layout.FilePath, root.Line, root.Col, r.BaseRule,
			"This `<FrameLayout>` can be replaced with a `<merge>` tag"))
	}
}

// mergeRootFrameCandidate reports whether a root FrameLayout carries nothing a <merge> would drop:
// no background, foreground, fitsSystemWindows or android:padding* (Start and End are not checked, as in Lint).
func mergeRootFrameCandidate(root *android.View) bool {
	for _, attr := range []string{
		"android:background", "android:foreground",
		"android:padding", "android:paddingLeft", "android:paddingRight", "android:paddingTop", "android:paddingBottom",
	} {
		if _, ok := root.Attributes[attr]; ok {
			return false
		}
	}
	return root.Attributes["android:fitsSystemWindows"] != "true" && root.Background == ""
}

// ---------------------------------------------------------------------------
// OverdrawResource
// ---------------------------------------------------------------------------

func isMatchParent(size string) bool {
	return size == "match_parent" || size == "fill_parent"
}

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

// isFullBleedOpaqueRepaint reports whether child repaints root with the same
// opaque color; margins, elevation, shapes or partial sizes leave root visible.
func isFullBleedOpaqueRepaint(root, child *android.View) bool {
	bg := child.Background
	if bg != root.Background || strings.HasPrefix(bg, "@drawable/") || strings.HasPrefix(bg, "@mipmap/") {
		return false
	}
	if hex := strings.ToLower(strings.TrimPrefix(bg, "#")); hex != strings.ToLower(bg) {
		if (len(hex) == 8 && !strings.HasPrefix(hex, "ff")) || (len(hex) == 4 && !strings.HasPrefix(hex, "f")) {
			return false
		}
	}
	for attr := range child.Attributes {
		if strings.HasPrefix(attr, "android:layout_margin") || attr == "android:elevation" || attr == "app:cardElevation" {
			return false
		}
	}
	width, height := child.LayoutWidth, child.LayoutHeight
	if width == "" {
		width = child.Attributes["android:layout_width"]
	}
	if height == "" {
		height = child.Attributes["android:layout_height"]
	}
	return width == "match_parent" && height == "match_parent"
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *OverdrawResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OverdrawResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		root := layout.RootView
		if root == nil || root.Background == "" || isTransparentBackground(root.Background) ||
			resourceToolsIgnoreMatches(root, "Overdraw") {
			continue
		}
		for _, child := range root.Children {
			if child.Background == "" || isTransparentBackground(child.Background) {
				continue
			}
			if android.IsLayoutView(child.Type) && isFullBleedOpaqueRepaint(root, child) {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:background", r.BaseRule,
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

// AlwaysShowActionResourceRule detects menus that pin too many actions to the
// action bar. Like Android Lint it judges each menu file on its own: it reports
// one finding, on the first showAsAction="always" item, when the menu has more
// than one showAsAction item and either more than two "always" items or no
// "ifRoom" item.
type AlwaysShowActionResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android style/theme resource rule. Detection flags style inheritance
// anti-patterns and attribute mismatches via structural checks on style
// XML. Classified per roadmap/17.
func (r *AlwaysShowActionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// showAsActionFlags reports whether a showAsAction value carries the "always"
// and "ifRoom" flags. Lint counts every flag of a "|" separated value.
func showAsActionFlags(val string) (always, ifRoom bool) {
	for _, flag := range strings.Split(val, "|") {
		switch strings.TrimSpace(flag) {
		case "always":
			always = true
		case "ifRoom":
			ifRoom = true
		}
	}
	return always, ifRoom
}

func (r *AlwaysShowActionResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, menu := range idx.AllMenus() {
		var total, ifRoom int
		var always []struct {
			view *android.View
			attr string
		}
		// Lint skips the whole file once any showAsAction attribute is suppressed.
		fileIgnored := false
		var walk func(v *android.View, inherited bool)
		walk = func(v *android.View, inherited bool) {
			if v == nil {
				return
			}
			ignored := inherited || resourceToolsIgnoreMatches(v, "AlwaysShowAction")
			for _, attr := range []string{"app:showAsAction", "android:showAsAction"} {
				val := v.Attributes[attr]
				if val == "" {
					continue
				}
				if ignored {
					fileIgnored = true
					continue
				}
				total++
				hasAlways, hasIfRoom := showAsActionFlags(val)
				if hasAlways {
					always = append(always, struct {
						view *android.View
						attr string
					}{v, attr})
				}
				if hasIfRoom {
					ifRoom++
				}
			}
			for _, child := range v.Children {
				walk(child, ignored)
			}
		}
		walk(menu.RootView, false)
		if fileIgnored || len(always) == 0 || total <= 1 || (len(always) <= 2 && ifRoom > 0) {
			continue
		}
		first := always[0]
		ctx.Emit(baseFindingAtAttr(menu.FilePath, first.view, first.attr, r.BaseRule,
			"Prefer \"`ifRoom`\" instead of \"`always`\""))
	}
}

// ---------------------------------------------------------------------------
// StateListReachableResource
// ---------------------------------------------------------------------------

// StateListReachableResourceRule detects unreachable items in selector
// drawables. Selector items are matched top-down and chosen when the
// runtime state set is a superset of the item's state qualifiers, so an
// item whose qualifier set is a superset of any earlier item's is
// unreachable. Like Lint it reports only the first such item of a file.
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
	for _, merged := range idx.DrawableSelectors {
		// The index merges same-named selectors from qualified folders, each of which is its own state list.
		byFile := make(map[string][]android.SelectorItem)
		var files []string
		for _, item := range merged {
			if _, seen := byFile[item.FilePath]; !seen {
				files = append(files, item.FilePath)
			}
			byFile[item.FilePath] = append(byFile[item.FilePath], item)
		}
		for _, file := range files {
			r.checkItems(ctx, byFile[file])
		}
	}
}

func (r *StateListReachableResourceRule) checkItems(ctx *api.Context, items []android.SelectorItem) {
	// Lint scans earlier items first and stops at the first masked item, so a state list gets at most one report.
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
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
				Message:    fmt.Sprintf("This item is unreachable because a previous item (item #%d) is a more general match than this one", i+1),
				Confidence: r.Confidence(),
			})
			return
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
// Only the `@+id/` form allocates a new id; `@id/...` is safe inside a style.
func aaptCrashIsGeneratedID(val string) bool {
	return strings.HasPrefix(strings.TrimSpace(val), "@+id/")
}

func (r *AaptCrashRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, style := range idx.Styles {
		val, ok := style.Items["android:id"]
		if !ok || !aaptCrashIsGeneratedID(val) {
			continue
		}
		val = strings.TrimSpace(val)
		name := strings.TrimPrefix(val, "@+id/")
		ctx.Emit(baseFinding(style.FilePath, style.ItemLines["android:id"], r.BaseRule,
			fmt.Sprintf("This construct can potentially crash `aapt` during a build. Change `%s` to `@id/%s` and define the id explicitly using `<item type=\"id\" name=\"%s\"/>` instead.",
				val, name, name)))
	}
}
