package rules

// Android Resource XML rules: Layout structure rules.
// Rules about view hierarchy, nesting, sizing.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
func (r *TooManyViewsResourceRule) Confidence() float64 { return 0.75 }

func (r *TooManyViewsResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	maxViews := r.MaxViews
	if maxViews <= 0 {
		maxViews = 80
	}
	for _, layout := range idx.Layouts {
		count := layout.ViewCount()
		if count > maxViews {
			ctx.Emit(resourceFinding(layout.FilePath, 1, r.BaseRule,
				fmt.Sprintf("Layout `%s` has %d views (threshold: %d). "+
					"Consider simplifying or using ViewStub/include.",
					layout.Name, count, maxViews)))
		}
	}
}

// ---------------------------------------------------------------------------
// TooDeepLayoutResource
// ---------------------------------------------------------------------------

// TooDeepLayoutResourceRule detects layouts that exceed a nesting depth
// threshold. Deep nesting causes expensive measure/layout passes.
type TooDeepLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
	MaxDepth int // default 10
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *TooDeepLayoutResourceRule) Confidence() float64 { return 0.75 }

func (r *TooDeepLayoutResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	maxDepth := r.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}
	for _, layout := range idx.Layouts {
		depth := layout.MaxDepth()
		if depth > maxDepth {
			ctx.Emit(resourceFinding(layout.FilePath, 1, r.BaseRule,
				fmt.Sprintf("Layout `%s` has nesting depth %d (threshold: %d). "+
					"Flatten with ConstraintLayout or merge tags.",
					layout.Name, depth, maxDepth)))
		}
	}
}

// ---------------------------------------------------------------------------
// UselessParentResource
// ---------------------------------------------------------------------------

// UselessParentResourceRule detects a ViewGroup with a single child that has
// no background, no padding, and no ID — it can be removed and the child
// promoted.
type UselessParentResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *UselessParentResourceRule) Confidence() float64 { return 0.75 }

func (r *UselessParentResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !android.IsLayoutView(v.Type) {
				return
			}
			if len(v.Children) != 1 {
				return
			}
			// Root views cannot be removed (unless merge is an option — separate rule)
			if v == layout.RootView {
				return
			}
			if v.Background != "" {
				return
			}
			if v.ID != "" {
				return
			}
			if hasAnyPadding(v) {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Useless parent `%s` with single child. Remove wrapper and promote the child.",
					v.Type)))
		})
	}
}

func hasAnyPadding(v *android.View) bool {
	paddingAttrs := []string{
		"android:padding", "android:paddingLeft", "android:paddingRight",
		"android:paddingTop", "android:paddingBottom",
		"android:paddingStart", "android:paddingEnd",
		"android:paddingHorizontal", "android:paddingVertical",
	}
	for _, attr := range paddingAttrs {
		if v.Attributes[attr] != "" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// UselessLeafResource
// ---------------------------------------------------------------------------

// UselessLeafResourceRule detects a leaf ViewGroup (no children, no background,
// no id) that can be removed.
type UselessLeafResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *UselessLeafResourceRule) Confidence() float64 { return 0.75 }

func (r *UselessLeafResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !android.IsLayoutView(v.Type) {
				return
			}
			if len(v.Children) != 0 {
				return
			}
			if v.Background != "" {
				return
			}
			if v.ID != "" {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Useless leaf `%s` has no children, no background, and no id. Remove it.",
					v.Type)))
		})
	}
}

// ---------------------------------------------------------------------------
// NestedScrollingResource
// ---------------------------------------------------------------------------

// NestedScrollingResourceRule detects ScrollView nested inside another
// ScrollView, which causes broken or unpredictable scrolling behavior.
type NestedScrollingResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *NestedScrollingResourceRule) Confidence() float64 { return 0.75 }

func (r *NestedScrollingResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		findNestedScrollViews(layout.RootView, false, layout.FilePath, r.BaseRule, ctx)
	}
}

func findNestedScrollViews(v *android.View, insideScroll bool, path string, rule BaseRule, ctx *v2.Context) {
	if v == nil {
		return
	}
	isScroll := android.IsScrollableView(v.Type)
	if isScroll && insideScroll {
		ctx.Emit(resourceFinding(path, v.Line, rule,
			fmt.Sprintf("Nested scrolling: `%s` inside another scrollable view. "+
				"This causes broken or unpredictable scrolling behavior.", v.Type)))
	}
	for _, child := range v.Children {
		findNestedScrollViews(child, insideScroll || isScroll, path, rule, ctx)
	}
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
func (r *ScrollViewCountResourceRule) Confidence() float64 { return 0.75 }

func (r *ScrollViewCountResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !android.IsScrollableView(v.Type) {
				return
			}
			if len(v.Children) > 1 {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` should have exactly one direct child, but has %d.",
						v.Type, len(v.Children))))
			}
		})
	}
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
func (r *ScrollViewSizeResourceRule) Confidence() float64 { return 0.75 }

func (r *ScrollViewSizeResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !android.IsScrollableView(v.Type) {
				return
			}
			isHorizontal := v.Type == "HorizontalScrollView"
			for _, child := range v.Children {
				if isHorizontal {
					if child.LayoutWidth == "match_parent" || child.LayoutWidth == "fill_parent" {
						ctx.Emit(resourceFinding(layout.FilePath, child.Line, r.BaseRule,
							fmt.Sprintf("`%s` child of `%s` should use `layout_width=\"wrap_content\"` instead of `%s`. "+
								"The parent scrolls horizontally so the child does not need to fill it.",
								child.Type, v.Type, child.LayoutWidth)))
					}
				} else {
					if child.LayoutHeight == "match_parent" || child.LayoutHeight == "fill_parent" {
						ctx.Emit(resourceFinding(layout.FilePath, child.Line, r.BaseRule,
							fmt.Sprintf("`%s` child of `%s` should use `layout_height=\"wrap_content\"` instead of `%s`. "+
								"The parent scrolls vertically so the child does not need to fill it.",
								child.Type, v.Type, child.LayoutHeight)))
					}
				}
			}
		})
	}
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
func (r *RequiredSizeResourceRule) Confidence() float64 { return 0.75 }

func (r *RequiredSizeResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	skipImplicit := experiment.Enabled("required-size-skip-implicit-dimensions")
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// <merge> is a no-op container that gets merged into its parent.
			// It never takes layout dimensions.
			// <include> can legitimately omit layout_width/height to inherit
			// from the included layout's root view.
			// <requestFocus> is a directive tag, not a view.
			// <tag> is a key-value storage directive, not a view.
			if v.Type == "merge" || v.Type == "include" ||
				v.Type == "requestFocus" || v.Type == "tag" {
				return
			}
			// TableRow (and AppCompat variants) gets its layout dimensions
			// automatically from TableLayout: width defaults to match_parent
			// and height to wrap_content. Requiring explicit values is noise.
			if skipImplicit {
				tname := v.Type
				if idx := strings.LastIndex(tname, "."); idx >= 0 {
					tname = tname[idx+1:]
				}
				if tname == "TableRow" {
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
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is missing both `android:layout_width` and `android:layout_height`.", v.Type)))
			} else if missingWidth {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is missing `android:layout_width`.", v.Type)))
			} else if missingHeight {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is missing `android:layout_height`.", v.Type)))
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
func (r *OrientationResourceRule) Confidence() float64 { return 0.75 }

func (r *OrientationResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	skipFixedHeightRow := experiment.Enabled("orientation-resource-skip-fixed-height-rows")
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			if _, ok := v.Attributes["android:orientation"]; ok {
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
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				"`LinearLayout` missing explicit `android:orientation`. The default is horizontal, which may not be intended."))
		})
	}
}

// ---------------------------------------------------------------------------
// AdapterViewChildrenResource
// ---------------------------------------------------------------------------

// AdapterViewChildrenResourceRule detects AdapterView subclasses (ListView,
// GridView, Spinner, Gallery, ExpandableListView) with direct children in XML.
// These views populate children from an adapter; XML children are invalid.
type AdapterViewChildrenResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var adapterViewTypes = map[string]bool{
	"ListView":           true,
	"GridView":           true,
	"Spinner":            true,
	"Gallery":            true,
	"ExpandableListView": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *AdapterViewChildrenResourceRule) Confidence() float64 { return 0.75 }

func (r *AdapterViewChildrenResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !adapterViewTypes[v.Type] {
				return
			}
			if len(v.Children) > 0 {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` cannot have children in XML. Its content is populated from an adapter.",
						v.Type)))
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
func (r *IncludeLayoutParamResourceRule) Confidence() float64 { return 0.75 }

func (r *IncludeLayoutParamResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "include" {
				return
			}
			hasWidth := v.Attributes["android:layout_width"] != ""
			hasHeight := v.Attributes["android:layout_height"] != ""
			// <include> can override layout_width/height, but both must be
			// specified together. Partial overrides are silently ignored.
			// If neither or both are set, the override is valid.
			if hasWidth == hasHeight {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				"`<include>` specifies only one of `layout_width`/`layout_height`. Partial overrides are silently ignored — specify both dimensions or neither."))
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
func (r *UseCompoundDrawablesResourceRule) Confidence() float64 { return 0.75 }

func (r *UseCompoundDrawablesResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "LinearLayout" {
				return
			}
			if len(v.Children) != 2 {
				return
			}
			c0, c1 := v.Children[0], v.Children[1]
			hasImage := (c0.Type == "ImageView" || c0.Type == "ImageButton") ||
				(c1.Type == "ImageView" || c1.Type == "ImageButton")
			hasText := c0.Type == "TextView" || c1.Type == "TextView"
			if hasImage && hasText {
				// Check that ImageView has no scaleType or complex sizing
				// that would prevent compound drawable replacement
				var imgView *android.View
				if c0.Type == "ImageView" || c0.Type == "ImageButton" {
					imgView = c0
				} else {
					imgView = c1
				}
				if imgView.Attributes["android:scaleType"] != "" {
					return
				}
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`LinearLayout` with `ImageView` + `TextView` can be replaced "+
						"by a single `TextView` with a compound drawable (e.g., `drawableStart`).")))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InconsistentLayoutResource
// ---------------------------------------------------------------------------

// InconsistentLayoutResourceRule detects layout XML files that differ
// significantly across configuration qualifiers (e.g. layout/ vs layout-land/).
// Different root view types, significantly different view counts (>50%), or
// different root child counts are flagged.
type InconsistentLayoutResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android layout resource rule. Detection flags layout-file anti-patterns
// (nesting depth, unnecessary containers, missing constraints) via
// structural checks on layout XML. Classified per roadmap/17.
func (r *InconsistentLayoutResourceRule) Confidence() float64 { return 0.75 }

func (r *InconsistentLayoutResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, configs := range idx.LayoutConfigs {
		if len(configs) < 2 {
			continue
		}

		// Gather root types, view counts, and child counts per qualifier.
		type configInfo struct {
			qualifier  string
			rootType   string
			viewCount  int
			childCount int
			filePath   string
			rootLine   int
		}
		var infos []configInfo
		for qualifier, layout := range configs {
			if layout.RootView == nil {
				continue
			}
			q := qualifier
			if q == "" {
				q = "default"
			}
			infos = append(infos, configInfo{
				qualifier:  q,
				rootType:   layout.RootView.Type,
				viewCount:  layout.ViewCount(),
				childCount: len(layout.RootView.Children),
				filePath:   layout.FilePath,
				rootLine:   layout.RootView.Line,
			})
		}
		if len(infos) < 2 {
			continue
		}

		first := infos[0]
		for i := 1; i < len(infos); i++ {
			other := infos[i]

			// Check root type mismatch.
			if first.rootType != other.rootType {
				ctx.Emit(resourceFinding(
					other.filePath, other.rootLine, r.BaseRule,
					fmt.Sprintf("Layout '%s' has inconsistent root view: '%s' in %s vs '%s' in %s",
						name, first.rootType, first.qualifier, other.rootType, other.qualifier),
				))
				continue
			}

			// Check root child count mismatch.
			if first.childCount != other.childCount {
				ctx.Emit(resourceFinding(
					other.filePath, other.rootLine, r.BaseRule,
					fmt.Sprintf("Layout '%s' has inconsistent root child count: %d in %s vs %d in %s",
						name, first.childCount, first.qualifier, other.childCount, other.qualifier),
				))
				continue
			}

			// Check significantly different view counts (>50% difference).
			if first.viewCount > 0 && other.viewCount > 0 {
				larger := first.viewCount
				smaller := other.viewCount
				if smaller > larger {
					larger, smaller = smaller, larger
				}
				diff := larger - smaller
				avg := (larger + smaller) / 2
				if avg > 0 && diff*100/avg > 50 {
					ctx.Emit(resourceFinding(
						other.filePath, other.rootLine, r.BaseRule,
						fmt.Sprintf("Layout '%s' has significantly different view counts: %d in %s vs %d in %s",
							name, first.viewCount, first.qualifier, other.viewCount, other.qualifier),
					))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
