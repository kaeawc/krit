package rules

// Android Resource XML rules: Layout structure rules.
// Rules about view hierarchy, nesting, sizing.

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// TooManyViewsResource
// ---------------------------------------------------------------------------

// TooManyViewsResourceRule detects layouts that exceed a view count threshold.
// Large layouts are slow to inflate and increase memory usage.
type TooManyViewsResourceRule struct {
	LayoutResourceBase
	AndroidRule
	MaxViews int // default 80
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *TooManyViewsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TooManyViewsResourceRule) check(ctx *api.Context) {
	maxViews := r.MaxViews
	if maxViews <= 0 {
		maxViews = 80
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if v := nthViewInDocumentOrder(layout.RootView, maxViews, "TooManyViews"); v != nil {
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				fmt.Sprintf("`%s` has more than %d views, bad for performance", path.Base(layout.FilePath), maxViews)))
		}
	}
}

// nthViewInDocumentOrder returns the n-th element of the layout in document order, or nil when the
// layout has fewer elements or the element or an ancestor ignores issue. Lint counts every element.
func nthViewInDocumentOrder(root *android.View, n int, issue string) *android.View {
	count := 0
	var found *android.View
	var walk func(v *android.View, ignored bool) bool
	walk = func(v *android.View, ignored bool) bool {
		ignored = ignored || resourceToolsIgnoreMatches(v, issue)
		count++
		if count == n {
			if !ignored {
				found = v
			}
			return true
		}
		for _, child := range v.Children {
			if walk(child, ignored) {
				return true
			}
		}
		return false
	}
	if root != nil {
		walk(root, false)
	}
	return found
}

// firstViewAtDepth returns the first element in document order whose depth (root is 1) equals depth.
func firstViewAtDepth(root *android.View, depth int, issue string) *android.View {
	var found *android.View
	var walk func(v *android.View, d int, ignored bool) bool
	walk = func(v *android.View, d int, ignored bool) bool {
		ignored = ignored || resourceToolsIgnoreMatches(v, issue)
		if d == depth {
			if !ignored {
				found = v
			}
			return true
		}
		for _, child := range v.Children {
			if walk(child, d+1, ignored) {
				return true
			}
		}
		return false
	}
	if root != nil {
		walk(root, 1, false)
	}
	return found
}

// ---------------------------------------------------------------------------
// TooDeepLayoutResource
// ---------------------------------------------------------------------------

// TooDeepLayoutResourceRule mirrors Lint's TooManyViewsDetector depth check: the first element that
// reaches the maximum depth is reported once per layout file.
type TooDeepLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
	MaxDepth int // default 10
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *TooDeepLayoutResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TooDeepLayoutResourceRule) check(ctx *api.Context) {
	maxDepth := r.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if v := firstViewAtDepth(layout.RootView, maxDepth, "TooDeepLayout"); v != nil {
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				fmt.Sprintf("`%s` has more than %d levels, bad for performance", path.Base(layout.FilePath), maxDepth)))
		}
	}
}

// ---------------------------------------------------------------------------
// UselessParentResource
// ---------------------------------------------------------------------------

// UselessParentResourceRule mirrors Lint's UselessViewDetector middle-layout check: a container
// with children that is the only child of another container, where one of the two can go.
type UselessParentResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UselessParentResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// uselessViewContainers is UselessViewDetector.CONTAINERS.
var uselessViewContainers = map[string]bool{
	"AbsoluteLayout": true, "FrameLayout": true, "GridLayout": true, "GridView": true, "ImageSwitcher": true,
	"LinearLayout": true, "RadioGroup": true, "RelativeLayout": true, "SlidingDrawer": true, "StackView": true,
	"TableLayout": true, "TableRow": true, "TextSwitcher": true, "ViewAnimator": true, "ViewFlipper": true,
	"ViewSwitcher": true,
}

func viewHasAttr(v *android.View, name string) bool {
	_, ok := v.Attributes[name]
	return ok
}

func viewHasDataBinding(v *android.View) bool {
	for _, value := range v.Attributes {
		if strings.HasPrefix(value, "@{") || strings.HasPrefix(value, "@={") {
			return true
		}
	}
	return false
}

var uselessViewPaddingAttrs = []string{
	"android:padding", "android:paddingLeft", "android:paddingRight", "android:paddingTop",
	"android:paddingBottom", "android:paddingStart", "android:paddingEnd",
}

func (r *UselessParentResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"UselessParent"}, func(v, parent *android.View) {
			if parent == nil || !uselessViewContainers[v.Type] || len(v.Children) == 0 {
				return
			}
			if parent.Type == "ScrollView" || parent.Type == "HorizontalScrollView" || parent.Type == "merge" {
				return
			}
			if len(parent.Children) != 1 || !uselessViewContainers[parent.Type] {
				return
			}
			hasBackground, parentBackground := viewHasAttr(v, "android:background"), viewHasAttr(parent, "android:background")
			if hasBackground && parentBackground {
				return
			}
			if viewHasAttr(v, "style") || viewHasAttr(parent, "style") {
				return
			}
			if parentBackground {
				for _, attr := range uselessViewPaddingAttrs {
					if viewHasAttr(v, attr) {
						return
					}
				}
			}
			if parent.Attributes["android:fitsSystemWindows"] == "true" || viewHasDataBinding(v) {
				return
			}
			msg := "This `" + v.Type + "` layout or its `" + parent.Type + "` parent is unnecessary"
			if viewHasAttr(v, "android:id") {
				msg = "This `" + v.Type + "` layout or its `" + parent.Type + "` parent is possibly unnecessary"
			}
			if hasBackground || parentBackground {
				msg += "; transfer the `background` attribute to the other view"
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule, msg))
		})
	}
}

// ---------------------------------------------------------------------------
// UselessLeafResource
// ---------------------------------------------------------------------------

// UselessLeafResourceRule mirrors Lint's UselessViewDetector leaf check: a container with no
// children, background, id, style or tag that is not the root.
type UselessLeafResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UselessLeafResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UselessLeafResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"UselessLeaf"}, func(v, parent *android.View) {
			if parent == nil || !uselessViewContainers[v.Type] || len(v.Children) != 0 {
				return
			}
			for _, attr := range []string{"android:id", "android:background", "style", "android:tag"} {
				if viewHasAttr(v, attr) {
					return
				}
			}
			if viewHasDataBinding(v) || parent.Attributes["android:fitsSystemWindows"] == "true" {
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				"This `"+v.Type+"` view is unnecessary (no children, no `background`, no `id`, no `style`)"))
		})
	}
}

// ---------------------------------------------------------------------------
// NestedScrollingResource
// ---------------------------------------------------------------------------

// NestedScrollingResourceRule detects a scrolling widget inside another one that scrolls in the
// same direction. Like Lint it knows only the plain ScrollView, ListView, GridView (vertical),
// Gallery and HorizontalScrollView (horizontal) tags, and it names the nearest such ancestor.
type NestedScrollingResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *NestedScrollingResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NestedScrollingResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		findNestedScrollViews(layout.RootView, nil, layout.FilePath, r.BaseRule, ctx)
	}
}

// findNestedScrollViews reports a scrolling widget whose nearest scrolling ancestor of the same direction exists.
func findNestedScrollViews(v *android.View, scrollAncestors []*android.View, path string, rule BaseRule, ctx *api.Context) {
	if v == nil || resourceToolsIgnoreMatches(v, "NestedScrolling") {
		return
	}
	axis := scrollViewAxis(v.Type)
	if axis != "" {
		for i := len(scrollAncestors) - 1; i >= 0; i-- {
			outer := scrollAncestors[i]
			if scrollViewAxis(outer.Type) != axis {
				continue
			}
			ctx.Emit(baseFindingAt(path, v.Line, v.Col, rule,
				fmt.Sprintf("The %sly scrolling `%s` should not contain another %sly scrolling widget (`%s`)",
					axis, outer.Type, axis, v.Type)))
			break
		}
		scrollAncestors = append(scrollAncestors[:len(scrollAncestors):len(scrollAncestors)], v)
	}
	for _, child := range v.Children {
		findNestedScrollViews(child, scrollAncestors, path, rule, ctx)
	}
}

// scrollViewAxis returns "vertical" or "horizontal" for the tags Lint treats as scrolling widgets, or "" for any other view.
func scrollViewAxis(tag string) string {
	switch tag {
	case "ScrollView", "ListView", "GridView":
		return "vertical"
	case "Gallery", "HorizontalScrollView":
		return "horizontal"
	}
	return ""
}

// ---------------------------------------------------------------------------
// ScrollViewCountResource
// ---------------------------------------------------------------------------

// ScrollViewCountResourceRule detects ScrollView or HorizontalScrollView with
// more than one direct child.
type ScrollViewCountResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *ScrollViewCountResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ScrollViewCountResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"ScrollViewCount"}, func(v *android.View) {
			if !isScrollViewCountTag(v.Type) {
				return
			}
			if countLayoutChildren(v) > 1 {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"A scroll view can have only one child"))
			}
		})
	}
}

// isScrollViewCountTag matches the tags Lint's ChildCountDetector visits.
func isScrollViewCountTag(tag string) bool {
	switch tag {
	case "ScrollView", "HorizontalScrollView", "androidx.core.widget.NestedScrollView",
		"android.support.v4.widget.NestedScrollView":
		return true
	}
	return false
}

// isLayoutMarkerTag mirrors Lint.isLayoutMarkerTag: tags that are not views.
func isLayoutMarkerTag(tag string) bool {
	switch tag {
	case "layout", "requestFocus", "data", "import", "variable", "tag", "aapt:attr":
		return true
	}
	return false
}

// countLayoutChildren counts the child elements of v that are views.
func countLayoutChildren(v *android.View) int {
	n := 0
	for _, child := range v.Children {
		if !isLayoutMarkerTag(child.Type) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// ScrollViewSizeResource
// ---------------------------------------------------------------------------

// ScrollViewSizeResourceRule detects ScrollView children that use match_parent
// on the scrolling axis. A vertical ScrollView's child should use
// wrap_content for layout_height; a HorizontalScrollView's child should use
// wrap_content for layout_width.
type ScrollViewSizeResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *ScrollViewSizeResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ScrollViewSizeResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"ScrollViewSize"}, func(v *android.View) {
			if v.Type != "ScrollView" && v.Type != "HorizontalScrollView" {
				return
			}
			attr := "layout_height"
			if v.Type == "HorizontalScrollView" {
				attr = "layout_width"
			}
			for _, child := range v.Children {
				value, ok := child.Attributes["android:"+attr]
				// Lint stops at the first child that has no such attribute.
				if !ok {
					return
				}
				if value != "match_parent" && value != "fill_parent" {
					continue
				}
				if hasWebViewChild(child) {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:"+attr, r.BaseRule,
					fmt.Sprintf("This %s should use `android:%s=\"wrap_content\"`", child.Type, attr)))
			}
		})
	}
}

func hasWebViewChild(v *android.View) bool {
	for _, child := range v.Children {
		if child.Type == "WebView" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// RequiredSizeResource
// ---------------------------------------------------------------------------

// RequiredSizeResourceRule detects views missing android:layout_width or
// android:layout_height.
type RequiredSizeResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *RequiredSizeResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RequiredSizeResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"RequiredSize"}, func(v, parent *android.View) {
			// <merge> is a no-op container that gets merged into its parent.
			// It never takes layout dimensions.
			// <include> can legitimately omit layout_width/height to inherit
			// from the included layout's root view.
			// <requestFocus> is a directive tag, not a view.
			// <tag> is a key-value storage directive, not a view.
			if v.Type == "merge" || v.Type == "include" ||
				v.Type == "requestFocus" || v.Type == "tag" ||
				v.Type == "layout" || v.Type == "data" ||
				v.Type == "variable" || v.Type == "import" {
				return
			}
			// Children of a TableLayout, TableRow or GridLayout get their size from the parent, as in Lint.
			if parent != nil {
				switch androidSimpleName(parent.Type) {
				case "TableLayout", "TableRow", "GridLayout":
					return
				}
			}
			// Views with a style attribute inherit layout_width/height from
			// the style — skip them since we can't resolve style contents.
			if v.Attributes["style"] != "" {
				return
			}
			missingWidth := v.Attributes["android:layout_width"] == ""
			missingHeight := v.Attributes["android:layout_height"] == ""
			if missingWidth && missingHeight {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"The required `layout_width` and `layout_height` attributes are missing"))
			} else if missingWidth {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"The required `layout_width` attribute is missing"))
			} else if missingHeight {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"The required `layout_height` attribute is missing"))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// OrientationResource
// ---------------------------------------------------------------------------

// OrientationResourceRule detects LinearLayout without an explicit
// android:orientation attribute. The default is horizontal, which is often
// not what developers intend.
type OrientationResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *OrientationResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OrientationResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	skipFixedHeightRow := experiment.Enabled("orientation-resource-skip-fixed-height-rows")
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"Orientation"}, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			if o, ok := v.Attributes["android:orientation"]; ok && o != "" {
				return
			}
			// style="..." may define orientation via a referenced style.
			// We cannot reliably resolve resources here, so suppress.
			if _, ok := v.Attributes["style"]; ok {
				return
			}
			// A LinearLayout with a fixed-dp or attribute-referenced
			// layout_height (e.g. "48dp", "?attr/actionBarSize") is
			// almost always a "row" where the default horizontal
			// orientation is intentional.
			if skipFixedHeightRow {
				h := v.Attributes["android:layout_height"]
				if h != "" && h != "match_parent" && h != "wrap_content" &&
					h != "fill_parent" && h != "0dp" {
					return
				}
			}
			msg := "Wrong orientation? No orientation specified, and the default is horizontal, yet this layout has multiple children where at least one has `layout_width=\"match_parent\"`"
			switch len(v.Children) {
			case 0:
				// AOSP heuristic: empty LinearLayouts populated dynamically
				// (signaled by android:id) — orientation matters when
				// children are added later.
				if v.ID == "" {
					return
				}
				msg = "No orientation specified, and the default is horizontal. This is a common source of bugs when children are added dynamically."
			case 1:
				// Single child cannot be pushed off-screen by a wrong
				// default orientation.
				return
			default:
				// AOSP heuristic: at least one non-last child has
				// match_parent/fill_parent width without layout_weight,
				// which would hide later siblings under horizontal layout.
				if !hasMatchParentChildBeforeLast(v.Children) {
					return
				}
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule, msg))
		})
	}
}

func hasMatchParentChildBeforeLast(children []*android.View) bool {
	for _, c := range children[:len(children)-1] {
		if c.LayoutWidth != "match_parent" && c.LayoutWidth != "fill_parent" {
			continue
		}
		if _, ok := c.Attributes["android:layout_weight"]; ok {
			continue
		}
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// AdapterViewChildrenResource
// ---------------------------------------------------------------------------

// AdapterViewChildrenResourceRule mirrors Lint's ChildCountDetector for
// AdapterViewChildren: a ListView or GridView with view children in XML. Lint
// ignores marker tags such as <requestFocus> and does not cover Spinner,
// Gallery or ExpandableListView.
type AdapterViewChildrenResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var adapterViewTypes = map[string]bool{
	"ListView": true,
	"GridView": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *AdapterViewChildrenResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AdapterViewChildrenResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"AdapterViewChildren"}, func(v *android.View) {
			if !adapterViewTypes[v.Type] {
				return
			}
			for _, child := range v.Children {
				if isLayoutMarkerTag(child.Type) {
					continue
				}
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"A list/grid should have no children declared in XML"))
				return
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IncludeLayoutParamResource
// ---------------------------------------------------------------------------

// IncludeLayoutParamResourceRule detects <include> elements that specify
// layout_width or layout_height. These attributes are ignored on <include>
// unless the included layout has a <merge> root.
type IncludeLayoutParamResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *IncludeLayoutParamResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *IncludeLayoutParamResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"IncludeLayoutParam"}, func(v *android.View) {
			if v.Type != "include" {
				return
			}
			var hasWidth, hasHeight bool
			var others []string
			for name := range v.Attributes {
				prefix, local, ok := strings.Cut(name, ":")
				if !ok || !strings.HasPrefix(local, "layout_") {
					continue
				}
				switch {
				case local == "layout_width":
					hasWidth = true
				case local == "layout_height":
					hasHeight = true
				case prefix == "android":
					others = append(others, name)
				}
			}
			// <include> applies its layout_* overrides only when both layout_width and layout_height are set.
			if hasWidth && hasHeight {
				return
			}
			missing := "`layout_width` is also specified"
			switch {
			case !hasWidth && !hasHeight:
				missing = "both `layout_width` and `layout_height` are also specified"
			case !hasHeight:
				missing = "`layout_height` is also specified"
			}
			// Lint names a lone width or height only when no other parameter is set.
			names := others
			if len(others) == 0 {
				switch {
				case hasWidth:
					names = []string{"android:layout_width"}
				case hasHeight:
					names = []string{"android:layout_height"}
				}
			}
			sort.Strings(names)
			for _, name := range names {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, name, r.BaseRule,
					fmt.Sprintf("Layout parameter `%s` ignored unless %s on `<include>` tag",
						strings.TrimPrefix(name, "android:"), missing)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UseCompoundDrawablesResource
// ---------------------------------------------------------------------------

// UseCompoundDrawablesResourceRule detects a LinearLayout with exactly one
// ImageView and one TextView as children. This pattern can be replaced with a
// single TextView using compound drawables (drawableLeft, drawableTop, etc.).
type UseCompoundDrawablesResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *UseCompoundDrawablesResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UseCompoundDrawablesResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"UseCompoundDrawables"}, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			if len(v.Children) != 2 {
				return
			}
			c0, c1 := v.Children[0], v.Children[1]
			// Mirrors UseCompoundDrawableDetector: an ImageView that is not weighted, clickable, focusable
			// or scaled, next to a TextView, in a LinearLayout with no background.
			canCombine := func(image *android.View) bool {
				for _, attr := range []string{"android:layout_weight", "android:clickable", "android:focusable"} {
					if viewHasAttr(image, attr) {
						return false
					}
				}
				return image.Attributes["android:scaleType"] == ""
			}
			pair := (c0.Type == "ImageView" && c1.Type == "TextView" && canCombine(c0)) ||
				(c1.Type == "ImageView" && c0.Type == "TextView" && canCombine(c1))
			if pair && !viewHasAttr(v, "android:background") {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					"This tag and its children can be replaced by one `<TextView/>` and a compound drawable"))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InconsistentLayoutResource
// ---------------------------------------------------------------------------

// InconsistentLayoutResourceRule detects a view id that one configuration of a
// layout (for example layout/) defines but another (layout-land/) does not.
// Code that calls findViewById on that id fails in the configuration where it
// is missing. Layout structure and view types may differ between
// configurations; only ids are compared.
type InconsistentLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Includes are not
// expanded, so an id contributed by an included layout is not seen.
func (r *InconsistentLayoutResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *InconsistentLayoutResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	// Names and qualifiers are visited in sorted order so anchors and finding order are stable.
	names := make([]string, 0, len(idx.LayoutConfigs))
	for name := range idx.LayoutConfigs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		configs := idx.LayoutConfigs[name]
		if len(configs) < 2 {
			continue
		}
		qualifiers := make([]string, 0, len(configs))
		for qualifier, layout := range configs {
			if layout != nil && layout.RootView != nil {
				qualifiers = append(qualifiers, qualifier)
			}
		}
		if len(qualifiers) < 2 {
			continue
		}
		sort.Strings(qualifiers)

		type idSite struct {
			line int
			col  int
		}
		present := make(map[string]map[string]idSite) // id -> qualifier -> first site
		var order []string
		for _, qualifier := range qualifiers {
			walkViewsUnlessIgnored(configs[qualifier].RootView, []string{"InconsistentLayout"}, func(v *android.View) {
				id := strings.TrimPrefix(strings.TrimPrefix(v.ID, "@+id/"), "@id/")
				// InvalidId already reports malformed ids.
				if id == "" || !isValidAndroidID(v.ID) {
					return
				}
				if present[id] == nil {
					present[id] = make(map[string]idSite)
					order = append(order, id)
				}
				if _, seen := present[id][qualifier]; !seen {
					present[id][qualifier] = idSite{line: v.Line, col: v.Col}
				}
			})
		}
		label := func(qualifier string) string {
			if qualifier == "" {
				return "layout"
			}
			return "layout-" + qualifier
		}
		sort.Strings(order)
		for _, id := range order {
			sites := present[id]
			if len(sites) == len(qualifiers) {
				continue
			}
			var anchor string
			anchored := false
			var missing []string
			for _, qualifier := range qualifiers {
				if _, ok := sites[qualifier]; ok {
					if !anchored {
						anchor, anchored = qualifier, true
					}
				} else {
					missing = append(missing, label(qualifier))
				}
			}
			site := sites[anchor]
			ctx.Emit(baseFindingAt(configs[anchor].FilePath, site.line, site.col, r.BaseRule,
				fmt.Sprintf("The id `%s` in layout `%s` is missing from the following layout configurations: %s (present in %s).",
					id, name, strings.Join(missing, ", "), label(anchor))))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GridLayoutResource
// ---------------------------------------------------------------------------

// GridLayoutResourceRule mirrors Lint's GridLayoutDetector for XML layouts:
// row and column indexes beyond the declared counts, and android: attributes
// on the v7 or AndroidX GridLayout, which expects the res-auto namespace.
type GridLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: the checks compare attribute values only.
func (r *GridLayoutResourceRule) Confidence() float64 { return api.ConfidenceHigh }

var gridLayoutAppAttrs = []string{"columnCount", "orientation", "rowCount", "useDefaultMargins", "alignmentMode", "columnOrderPreserved", "rowOrderPreserved"}
var gridLayoutChildAppAttrs = []string{"layout_column", "layout_columnSpan", "layout_gravity", "layout_row", "layout_rowSpan", "layout_rowWeight", "layout_columnWeight"}

func gridAttrInt(v *android.View, name string) int {
	raw, ok := v.Attributes["android:"+name]
	if !ok || raw == "" {
		return -1
	}
	n, err := strconv.ParseInt(raw, 0, 32)
	if err != nil {
		return -1
	}
	return int(n)
}

func (r *GridLayoutResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		appPrefix := "app"
		declared := false
		for name, value := range layout.RootView.Attributes {
			if strings.HasPrefix(name, "xmlns:") && value == "http://schemas.android.com/apk/res-auto" {
				appPrefix, declared = strings.TrimPrefix(name, "xmlns:"), true
			}
		}
		walkViewsUnlessIgnored(layout.RootView, []string{"GridLayout"}, func(v *android.View) {
			v7 := v.Type == "android.support.v7.widget.GridLayout" || v.Type == "androidx.gridlayout.widget.GridLayout"
			if v.Type != "GridLayout" && !v7 {
				return
			}
			rows, cols := gridAttrInt(v, "rowCount"), gridAttrInt(v, "columnCount")
			if rows != -1 || cols != -1 {
				for _, child := range v.Children {
					if cols != -1 {
						if col := gridAttrInt(child, "layout_column"); col != -1 && col >= cols {
							ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:layout_column", r.BaseRule,
								fmt.Sprintf("Column attribute (%d) exceeds declared grid column count (%d)", col, cols)))
						}
					}
					if rows != -1 {
						if row := gridAttrInt(child, "layout_row"); row != -1 && row > rows {
							ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:layout_row", r.BaseRule,
								fmt.Sprintf("Row attribute (%d) exceeds declared grid row count (%d)", row, rows)))
						}
					}
				}
			}
			if !v7 {
				return
			}
			wrong := func(el *android.View, name string) {
				if _, ok := el.Attributes["android:"+name]; !ok {
					return
				}
				msg := fmt.Sprintf("Wrong namespace; with v7 `GridLayout` you should use %s:%s", appPrefix, name)
				if !declared {
					msg += " (and add `xmlns:app=\"http://schemas.android.com/apk/res-auto\"` to your root element)"
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, el, "android:"+name, r.BaseRule, msg))
			}
			for _, name := range gridLayoutAppAttrs {
				wrong(v, name)
			}
			for _, child := range v.Children {
				for _, name := range gridLayoutChildAppAttrs {
					wrong(child, name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DeprecatedResource
// ---------------------------------------------------------------------------

// DeprecatedResourceRule mirrors the layout part of Lint's DeprecationDetector:
// attributes that inputType replaced, android:editable and AbsoluteLayout.
type DeprecatedResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: the checks compare attribute and tag names.
func (r *DeprecatedResourceRule) Confidence() float64 { return api.ConfidenceHigh }

//nolint:gosec // G101 false positive: attribute deprecation messages, not credentials.
var deprecatedLayoutAttrs = map[string]string{
	"android:autoText":    "Use `inputType` instead",
	"android:capitalize":  "Use `inputType` instead",
	"android:inputMethod": "Use `inputType` instead",
	"android:numeric":     "Use `inputType` instead",
	"android:password":    "Use `inputType` instead",
	"android:phoneNumber": "Use `inputType` instead",
	"android:editable":    "Use an `<EditText>` to make it editable",
}

func (r *DeprecatedResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"Deprecated"}, func(v *android.View) {
			if v.Type == "AbsoluteLayout" {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col+1, r.BaseRule, "`AbsoluteLayout` is deprecated"))
			}
			names := make([]string, 0, len(deprecatedLayoutAttrs))
			for attr := range deprecatedLayoutAttrs {
				if _, ok := v.Attributes[attr]; ok {
					names = append(names, attr)
				}
			}
			sort.Strings(names)
			for _, attr := range names {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
					fmt.Sprintf("`%s` is deprecated: %s", attr, deprecatedLayoutAttrs[attr])))
			}
		})
	}
}
