package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// RtlCompatResourceRule ports Android Lint's RtlCompat (RtlDetector.COMPAT). It
// applies when an app that includes the module has an <application>, targets API 17+ and has not turned RTL
// off with supportsRtl="false" (an unset attribute counts as on):
//   - a textAlignment attribute needs a matching gravity or layout_gravity on
//     API levels below 17, and an existing gravity must agree with it;
//   - a start/end attribute needs its left/right twin on API levels below 17.
//
// It replaces RtlCompatManifest, which checked a different thing (a missing
// supportsRtl) under Lint's RtlCompat name; that id is kept as a suppression alias.
type RtlCompatResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: SDK levels come from
// Gradle or the module manifest and may differ per variant.
func (r *RtlCompatResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// rtlCompatStartEndAttrs are the start/end attributes from Lint's RtlDetector.ATTRIBUTES.
var rtlCompatStartEndAttrs = []string{
	"layout_alignParentStart", "layout_alignParentEnd", "layout_marginStart", "layout_marginEnd",
	"paddingStart", "paddingEnd", "drawableStart", "drawableEnd",
	"listPreferredItemPaddingStart", "listPreferredItemPaddingEnd",
	"layout_toStartOf", "layout_toEndOf", "layout_alignStart", "layout_alignEnd",
}

// rtlCompatTextAlignmentGravity maps a textAlignment value to the gravity it implies, or "".
func rtlCompatTextAlignmentGravity(value string) string {
	switch {
	case value == "center":
		return "center_horizontal"
	case strings.HasSuffix(value, "Start"):
		return "start"
	case strings.HasSuffix(value, "End"):
		return "end"
	}
	return ""
}

// rtlCompatLeftRight converts a start/end attribute name to its left/right twin.
func rtlCompatLeftRight(name string) string {
	if strings.Contains(name, "Start") {
		return strings.Replace(name, "Start", "Left", 1)
	}
	return strings.Replace(name, "End", "Right", 1)
}

// rtlCompatOpposite swaps the direction in name (Left/Right, then Start/End).
func rtlCompatOpposite(name string) string {
	switch {
	case strings.Contains(name, "Left"):
		return strings.Replace(name, "Left", "Right", 1)
	case strings.Contains(name, "Right"):
		return strings.Replace(name, "Right", "Left", 1)
	case strings.Contains(name, "Start"):
		return strings.Replace(name, "Start", "End", 1)
	}
	return strings.Replace(name, "End", "Start", 1)
}

func (r *RtlCompatResourceRule) check(ctx *api.Context) {
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	modules := newRtlModuleCache(ctx, facts)
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		mains := rtlResolveMains(ctx, facts, layout.FilePath)
		module := modules.forLayout(layout.FilePath)
		if _, ok := rtlApplies(rtlEnabled, mains, module); !ok {
			continue
		}
		// The module still supports API below 17 and an including app does too.
		main, preSeventeen := rtlApplies(rtlEnabled|rtlMinBelow17, mains, module)
		preSeventeen = preSeventeen && !module.modern
		walkViewsUnlessIgnored(layout.RootView, []string{"RtlCompat"}, func(v *android.View) {
			r.checkTextAlignment(ctx, layout.FilePath, v, main.minSdk, preSeventeen)
			if preSeventeen {
				r.checkStartEnd(ctx, layout.FilePath, v, main.minSdk)
			}
		})
	}
}

func (r *RtlCompatResourceRule) checkTextAlignment(ctx *api.Context, path string, v *android.View, minSdk int, preSeventeen bool) {
	value, ok := v.Attributes["android:textAlignment"]
	if !ok {
		return
	}
	expected := rtlCompatTextAlignmentGravity(value)
	if expected == "" {
		return
	}
	gravityAttr := "android:gravity"
	gravity, has := v.Attributes[gravityAttr]
	if !has {
		gravityAttr = "android:layout_gravity"
		gravity, has = v.Attributes[gravityAttr]
	}
	if !has {
		if preSeventeen {
			ctx.Emit(baseFindingAtAttr(path, v, "android:textAlignment", r.BaseRule, fmt.Sprintf(
				"To support older versions than API 17 (project specifies %d) you must **also** specify `gravity` or `layout_gravity=\"%s\"`",
				minSdk, expected)))
		}
		return
	}
	for _, part := range strings.Split(gravity, "|") {
		part = strings.TrimSpace(part)
		if part == expected || part == "center" && expected == "center_horizontal" ||
			part == "fill" && expected == "fill_horizontal" {
			return
		}
	}
	ctx.Emit(baseFindingAtAttr(path, v, gravityAttr, r.BaseRule, fmt.Sprintf(
		"Inconsistent alignment specification between `textAlignment` and `gravity` attributes: was `%s`, expected `%s`",
		gravity, expected)))
}

func (r *RtlCompatResourceRule) checkStartEnd(ctx *api.Context, path string, v *android.View, minSdk int) {
	for _, name := range rtlCompatStartEndAttrs {
		attr := "android:" + name
		value, ok := v.Attributes[attr]
		if !ok {
			continue
		}
		// Lint returns early for padding and margin pairs that carry the same value on both sides.
		if strings.HasPrefix(name, "padding") || strings.HasPrefix(name, "layout_margin") {
			if opposite, has := v.Attributes["android:"+rtlCompatOpposite(name)]; has && opposite == value {
				continue
			}
		}
		old := rtlCompatLeftRight(name)
		if _, has := v.Attributes["android:"+old]; has {
			continue
		}
		ctx.Emit(baseFindingAtAttr(path, v, attr, r.BaseRule, fmt.Sprintf(
			"To support older versions than API 17 (project specifies %d) you should **also** add `android:%s=\"%s\"`",
			minSdk, old, value)))
	}
}
