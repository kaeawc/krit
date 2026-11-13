package rules

// Android Resource XML rules: RTL (right-to-left) rules.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// RtlHardcodedResource
// ---------------------------------------------------------------------------

// RtlHardcodedResourceRule mirrors Lint's RtlHardcoded: every Left/Right layout attribute (alignParent,
// margin, padding, drawable, listPreferredItemPadding, toXOf, alignX) gets its own finding that suggests the
// Start/End attribute, a Start/End twin already present is reported as redundant, and a symmetric
// padding or margin pair with equal values is skipped. left/right gravity values are reported too.
// Like Lint, an unset supportsRtl counts as RTL on: only an explicit supportsRtl="false" or a targetSdk below 17
// turns the Start/End advice off. The "Consider adding" wording for a minSdk below 17 is kept even then.
// A library module is judged by the apps that include it (see rtlResolveMains).
type RtlHardcodedResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// rtlAttributes is RtlDetector.ATTRIBUTES: the local names of the old (Left/Right) attributes.
var rtlAttributes = map[string]bool{
	"layout_alignParentLeft": true, "layout_alignParentRight": true,
	"layout_marginLeft": true, "layout_marginRight": true,
	"paddingLeft": true, "paddingRight": true,
	"drawableLeft": true, "drawableRight": true,
	"listPreferredItemPaddingLeft": true, "listPreferredItemPaddingRight": true,
	"layout_toLeftOf": true, "layout_toRightOf": true,
	"layout_alignLeft": true, "layout_alignRight": true,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *RtlHardcodedResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// rtlViewAttrsInOrder returns the android: attribute names of v in document order.
func rtlViewAttrsInOrder(v *android.View) []string {
	var names []string
	for name := range v.Attributes {
		if strings.HasPrefix(name, "android:") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		li, ci := v.PosOf(names[i])
		lj, cj := v.PosOf(names[j])
		if li != lj {
			return li < lj
		}
		if ci != cj {
			return ci < cj
		}
		return names[i] < names[j]
	})
	return names
}

func (r *RtlHardcodedResourceRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	modules := newRtlModuleCache(ctx, facts)
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		mains := rtlResolveMains(ctx, facts, layout.FilePath)
		module := modules.forLayout(layout.FilePath)
		target := rtlModuleTargetSdk(module)
		walkViewsUnlessIgnored(layout.RootView, []string{"RtlHardcoded"}, func(v *android.View) {
			if resourceToolsIgnoreMatches(v, "RtlHardcoded", r.RuleName) {
				return
			}
			for _, full := range rtlViewAttrsInOrder(v) {
				name := strings.TrimPrefix(full, "android:")
				value := v.Attributes[full]
				if name == "gravity" || name == "layout_gravity" {
					hasLeft, hasRight := strings.Contains(value, "left"), strings.Contains(value, "right")
					if !hasLeft && !hasRight {
						continue
					}
					if _, ok := rtlApplies(rtlEnabled, mains, module); !ok {
						continue
					}
					newWord, oldWord := "end", "right"
					if hasLeft {
						newWord, oldWord = "start", "left"
					}
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule,
						fmt.Sprintf("Use \"`%s`\" instead of \"`%s`\" to ensure correct behavior in right-to-left locales", newWord, oldWord)))
					continue
				}
				isPadding, isMargin := strings.HasPrefix(name, "padding"), strings.HasPrefix(name, "layout_margin")
				if isPadding || isMargin {
					if opposite, ok := v.Attributes["android:"+rtlOppositeDirection(name)]; ok && opposite == value {
						continue
					}
				}
				if !rtlAttributes[name] {
					continue
				}
				newName := rtlConvertOldToNew(name)
				if _, has := v.Attributes["android:"+newName]; has {
					if _, ok := rtlApplies(rtlEnabled|rtlModuleModern, mains, module); ok {
						ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule,
							fmt.Sprintf("Redundant attribute `%s`; already defining `%s` with `targetSdkVersion` %d", name, newName, target)))
					}
					continue
				}
				// A module that still supports API below 17 needs both attributes, whatever the RTL setting.
				if !module.modern {
					if _, ok := rtlApplies(rtlMinBelow17, mains, module); ok {
						ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule,
							fmt.Sprintf("Consider adding `android:%s=\"%s\"` to better support right-to-left layouts", newName, value)))
						continue
					}
				}
				if _, ok := rtlApplies(rtlEnabled|rtlMinAtLeast17, mains, module); ok {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, full, r.BaseRule,
						fmt.Sprintf("Consider replacing `%s` with `android:%s=\"%s\"` to better support right-to-left layouts", full, newName, value)))
				}
			}
		})
	}
}

// rtlModuleTargetSdk is the targetSdkVersion Lint prints in its redundant-attribute message.
func rtlModuleTargetSdk(module rtlModule) int {
	if module.target > 0 {
		return module.target
	}
	if module.minSdk > 0 {
		return module.minSdk
	}
	return 34
}

// ---------------------------------------------------------------------------
// RtlSymmetryResource
// ---------------------------------------------------------------------------

// RtlSymmetryResourceRule detects asymmetric padding or margin. If a view
// specifies paddingLeft but not paddingRight (or marginLeft but not
// marginRight), the layout will look unbalanced in RTL locales.
type RtlSymmetryResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RtlSymmetryResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RtlSymmetryResourceRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	modules := newRtlModuleCache(ctx, facts)
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if _, ok := rtlApplies(rtlEnabled, rtlResolveMains(ctx, facts, layout.FilePath), modules.forLayout(layout.FilePath)); !ok {
			continue
		}
		walkViewsUnlessIgnored(layout.RootView, []string{"RtlSymmetry"}, func(v *android.View) {
			if resourceToolsIgnoreMatches(v, "RtlSymmetry", r.RuleName) {
				return
			}
			for _, attr := range rtlSymmetryPaddingAttrs {
				if _, ok := v.Attributes["android:"+attr]; !ok {
					continue
				}
				opposite := rtlOppositeDirection(attr)
				if _, ok := v.Attributes["android:"+opposite]; ok {
					continue
				}
				counterpart := rtlConvertNewToOld(opposite)
				if strings.Contains(opposite, "Left") || strings.Contains(opposite, "Right") {
					counterpart = rtlConvertOldToNew(opposite)
				}
				if _, ok := v.Attributes["android:"+counterpart]; ok {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:"+attr, r.BaseRule,
					fmt.Sprintf("When you define `%s` you should probably also define `%s` for right-to-left symmetry", attr, opposite)))
			}
		})
	}
}

// rtlSymmetryPaddingAttrs are the android: padding attributes RtlDetector checks for a missing opposite side.
var rtlSymmetryPaddingAttrs = []string{"paddingLeft", "paddingStart", "paddingRight", "paddingEnd"}

// rtlOppositeDirection is RtlDetector.convertToOppositeDirection.
func rtlOppositeDirection(name string) string {
	switch {
	case strings.Contains(name, "Left"):
		return strings.ReplaceAll(name, "Left", "Right")
	case strings.Contains(name, "Right"):
		return strings.ReplaceAll(name, "Right", "Left")
	case strings.Contains(name, "Start"):
		return strings.ReplaceAll(name, "Start", "End")
	}
	return strings.ReplaceAll(name, "End", "Start")
}

// rtlConvertOldToNew is RtlDetector.convertOldToNew.
func rtlConvertOldToNew(name string) string {
	if strings.Contains(name, "Left") {
		return strings.ReplaceAll(name, "Left", "Start")
	}
	return strings.ReplaceAll(name, "Right", "End")
}

// rtlConvertNewToOld is RtlDetector.convertNewToOld.
func rtlConvertNewToOld(name string) string {
	if strings.Contains(name, "Start") {
		return strings.ReplaceAll(name, "Start", "Left")
	}
	return strings.ReplaceAll(name, "End", "Right")
}

// ---------------------------------------------------------------------------
// RtlSuperscriptResource
// ---------------------------------------------------------------------------

// RtlSuperscriptResourceRule detects superscript/subscript text styling that
// may break in RTL locales.
type RtlSuperscriptResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RtlSuperscriptResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RtlSuperscriptResourceRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	modules := newRtlModuleCache(ctx, facts)
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		if _, ok := rtlApplies(rtlEnabled, rtlResolveMains(ctx, facts, layout.FilePath), modules.forLayout(layout.FilePath)); !ok {
			continue
		}
		walkViewsUnlessIgnored(layout.RootView, []string{"RtlSuperscript"}, func(v *android.View) {
			textStyle := v.Attributes["android:textStyle"]
			if textStyle == "" {
				return
			}
			lower := strings.ToLower(textStyle)
			if strings.Contains(lower, "superscript") || strings.Contains(lower, "subscript") {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:textStyle", r.BaseRule,
					fmt.Sprintf("`%s` uses textStyle `%s` which may render incorrectly in RTL locales.", v.Type, textStyle)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RelativeOverlapResource
// ---------------------------------------------------------------------------

// RelativeOverlapResourceRule detects views inside a RelativeLayout that may
// overlap because multiple children use alignParentLeft (or alignParentStart)
// without different vertical positioning constraints.
type RelativeOverlapResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

var verticalConstraintAttrs = []string{
	"android:layout_below", "android:layout_above",
	"android:layout_alignTop", "android:layout_alignBottom",
	"android:layout_alignParentTop", "android:layout_alignParentBottom",
	"android:layout_centerVertical",
}

// startMarginDp returns the fixed dp start/left margin of a view, or 0 when absent or not in dp.
func startMarginDp(v *android.View) float64 {
	for _, attr := range []string{"android:layout_marginStart", "android:layout_marginLeft"} {
		if val := strings.TrimSpace(v.Attributes[attr]); strings.HasSuffix(val, "dp") || strings.HasSuffix(val, "dip") {
			return parseLayoutDp(val)
		}
	}
	return 0
}

// marginClearsSibling reports whether the later view's start margin pushes it past the earlier one.
func marginClearsSibling(earlier, later *android.View) bool {
	margin := startMarginDp(later)
	if margin <= 0 {
		return false
	}
	if width := parseLayoutDp(earlier.Attributes["android:layout_width"]); width > 0 {
		return margin >= width
	}
	return earlier.Attributes["android:layout_width"] == "wrap_content" && margin >= 48
}

func verticalConstraintKey(v *android.View) string {
	for _, attr := range verticalConstraintAttrs {
		if val := v.Attributes[attr]; val != "" {
			return attr + "=" + val
		}
	}
	return ""
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *RelativeOverlapResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *RelativeOverlapResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"RelativeOverlap"}, func(v *android.View) {
			if v.Type != "RelativeLayout" {
				return
			}
			// Collect children that align to the left
			type leftChild struct {
				view    *android.View
				vertKey string
			}
			var leftAligned []leftChild
			for _, child := range v.Children {
				if child.Attributes["android:layout_alignParentLeft"] == "true" ||
					child.Attributes["android:layout_alignParentStart"] == "true" {
					leftAligned = append(leftAligned, leftChild{
						view:    child,
						vertKey: verticalConstraintKey(child),
					})
				}
			}
			// Check for pairs with same vertical constraint key (including both empty)
			for i := 0; i < len(leftAligned); i++ {
				for j := i + 1; j < len(leftAligned); j++ {
					if leftAligned[i].vertKey == leftAligned[j].vertKey &&
						!marginClearsSibling(leftAligned[i].view, leftAligned[j].view) {
						ctx.Emit(baseFindingAt(layout.FilePath, leftAligned[j].view.Line, leftAligned[j].view.Col, r.BaseRule,
							fmt.Sprintf("`%s` (line %d) and `%s` (line %d) in `RelativeLayout` both use left/start alignment "+
								"without different vertical constraints and may overlap.",
								leftAligned[i].view.Type, leftAligned[i].view.Line,
								leftAligned[j].view.Type, leftAligned[j].view.Line)))
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NotSiblingResource
// ---------------------------------------------------------------------------

// NotSiblingResourceRule detects RelativeLayout constraints that reference IDs
// not belonging to a sibling view. When layout_below, layout_above, etc.
// reference an @id/ that is not a direct child of the same RelativeLayout,
// the constraint is invalid.
type NotSiblingResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android RTL resource rule. Detection flags start/end vs left/right
// attribute usage and bidi markers via attribute presence checks.
// Classified per roadmap/17.
func (r *NotSiblingResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NotSiblingResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	defined := lintDefinedIDs(idx)
	for _, layout := range idx.AllLayouts() {
		inFile := layoutIDs(layout.RootView)
		walkViewsUnlessIgnored(layout.RootView, []string{"NotSibling"}, func(v *android.View) {
			parent := notSiblingParentName(v.Type)
			if parent == "" {
				return
			}
			siblings := map[string]bool{}
			for _, child := range v.Children {
				if child.ID != "" {
					siblings[stripIDPrefix(child.ID)] = true
				}
				// The ids of an included layout are siblings too.
				if child.Type == "include" {
					if name, ok := strings.CutPrefix(child.Attributes["layout"], "@layout/"); ok {
						for _, included := range idx.LayoutConfigs[name] {
							walkViews(included.RootView, func(iv *android.View) {
								if iv.ID != "" {
									siblings[stripIDPrefix(iv.ID)] = true
								}
							})
						}
					}
				}
			}
			for _, child := range v.Children {
				if resourceToolsIgnoreMatches(child, "NotSibling") {
					continue
				}
				r.checkChild(ctx, layout.FilePath, parent, child, siblings, inFile, defined)
			}
		})
	}
}

// notSiblingParentName returns the tag name Lint prints for a parent whose children reference each other by id,
// or "" for any other view.
func notSiblingParentName(tag string) string {
	switch tag {
	case "androidx.constraintlayout.widget.ConstraintLayout", "android.support.constraint.ConstraintLayout":
		return "ConstraintLayout"
	case "android.support.percent.PercentRelativeLayout":
		return "RelativeLayout"
	}
	if frameworkViewName(tag) == "RelativeLayout" {
		return "RelativeLayout"
	}
	return ""
}

func stripIDPrefix(id string) string {
	return strings.TrimPrefix(strings.TrimPrefix(id, "@+id/"), "@id/")
}

// checkChild judges every layout_* id reference of child (and, in a ConstraintLayout, its
// constraint_referenced_ids) the way Lint's WrongIdDetector does.
func (r *NotSiblingResourceRule) checkChild(ctx *api.Context, path, parent string, child *android.View, siblings, inFile, defined map[string]bool) {
	own := stripIDPrefix(child.ID)
	attrs := make([]string, 0, len(child.Attributes))
	for attr := range child.Attributes {
		attrs = append(attrs, attr)
	}
	sort.Strings(attrs)
	for _, attr := range attrs {
		ns, local, ok := strings.Cut(attr, ":")
		if !ok || (ns != "android" && ns != "app") {
			continue
		}
		value := strings.TrimSpace(child.Attributes[attr])
		var refs []string
		switch {
		case strings.HasPrefix(local, "layout_") && (strings.HasPrefix(value, "@+id/") || strings.HasPrefix(value, "@id/")):
			refs = []string{value}
		case parent == "ConstraintLayout" && local == "constraint_referenced_ids":
			for _, token := range strings.Split(value, ",") {
				if token = strings.TrimSpace(token); token != "" {
					refs = append(refs, token)
				}
			}
		}
		for _, ref := range refs {
			name := stripIDPrefix(ref)
			switch {
			case siblings[name] && own != "" && name == own:
				// Lint compares the reference text with the declared id text, so only the same `@+id/` or `@id/` form is a self reference.
				if ref != strings.TrimSpace(child.ID) {
					continue
				}
				ctx.Emit(baseFindingAtAttr(path, child, attr, r.BaseRule,
					fmt.Sprintf("Cannot be relative to self: id=%s, %s=%s", own, local, own)))
			case !siblings[name] && (inFile[name] || defined[name] && !strings.HasPrefix(ref, "@+id/")):
				// An id nothing defines is UnknownId. An `@+id/` reference to an id another layout defines is
				// UnknownIdInLayout, so NotSibling only reports ids defined in this file or `@id/` references.
				ctx.Emit(baseFindingAtAttr(path, child, attr, r.BaseRule,
					fmt.Sprintf("`%s` is not a sibling in the same `%s`", ref, parent)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
