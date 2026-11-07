package rules

import (
	"regexp"
	"strconv"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// BottomAppBarRule mirrors Lint's BottomAppBar: a BottomAppBar that is not a direct child of a CoordinatorLayout.
type BottomAppBarRule struct {
	LayoutResourceBase
	AndroidRule
}

// InvalidImeActionIDRule mirrors Lint's InvalidImeActionId: android:imeActionId that is not an integer.
type InvalidImeActionIDRule struct {
	LayoutResourceBase
	AndroidRule
}

// MenuTitleRule mirrors Lint's MenuTitle: menu items without a title.
type MenuTitleRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal layout structure.
func (r *BottomAppBarRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal layout structure.
func (r *InvalidImeActionIDRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal layout structure.
func (r *MenuTitleRule) Confidence() float64 { return api.ConfidenceHigh }

func emitAtViewName(ctx *api.Context, file string, v *android.View, msg string) {
	ctx.Emit(scanner.Finding{File: file, Line: v.Line, Col: v.Col + 1, Message: msg})
}

const (
	oldBottomAppBar      = "android.support.design.bottomappbar.BottomAppBar"
	newBottomAppBar      = "com.google.android.material.bottomappbar.BottomAppBar"
	oldCoordinatorLayout = "android.support.design.widget.CoordinatorLayout"
	newCoordinatorLayout = "androidx.coordinatorlayout.widget.CoordinatorLayout"
)

func (r *BottomAppBarRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"BottomAppBar"}, func(v, parent *android.View) {
			if v.Type != oldBottomAppBar && v.Type != newBottomAppBar {
				return
			}
			if parent != nil && (parent.Type == oldCoordinatorLayout || parent.Type == newCoordinatorLayout) {
				return
			}
			wrapper := newCoordinatorLayout
			if v.Type == oldBottomAppBar {
				wrapper = oldCoordinatorLayout
			}
			emitAtViewName(ctx, layout.FilePath, v, "This `BottomAppBar` must be wrapped in a `CoordinatorLayout` (`"+wrapper+"`)")
		})
	}
}

// resourceURLRe matches the reference forms Lint's ResourceUrl.parse accepts: @[+][pkg:]type/name and ?[pkg:][type/]name.
var resourceURLRe = regexp.MustCompile(`^(?:@\+?(?:[A-Za-z0-9_.]+:)?([a-z]+)/[A-Za-z0-9_.$]+|\?(?:[A-Za-z0-9_.]+:)?(?:(attr)/)?[A-Za-z0-9_.$]+)$`)

func (r *InvalidImeActionIDRule) check(ctx *api.Context) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsWithParentUnlessIgnored(layout.RootView, nil, []string{"InvalidImeActionId"}, func(v, _ *android.View) {
			value, ok := v.Attributes["android:imeActionId"]
			if !ok {
				return
			}
			line, col := v.PosOf("android:imeActionId")
			report := func(msg string) {
				ctx.Emit(scanner.Finding{File: layout.FilePath, Line: line, Col: col, Message: msg})
			}
			if m := resourceURLRe.FindStringSubmatch(value); m != nil {
				if m[1] != "integer" {
					report("Invalid resource type, expected integer value")
				}
				return
			}
			if _, err := strconv.ParseInt(value, 10, 32); err != nil {
				report(`"` + value + `" is not an integer`)
			}
		})
	}
}

func (r *MenuTitleRule) check(ctx *api.Context) {
	for _, menu := range ctx.ResourceIndex.AllMenus() {
		walkViewsUnlessIgnored(menu.RootView, []string{"MenuTitle"}, func(v *android.View) {
			if v.Type != "item" {
				return
			}
			if _, has := v.Attributes["android:title"]; has || v.Attributes["android:visible"] == "false" {
				return
			}
			emitAtViewName(ctx, menu.FilePath, v, "Menu items should specify a `title`")
		})
	}
}
