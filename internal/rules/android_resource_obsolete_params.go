package rules

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// obsoleteParamLayouts maps a layout_* attribute to the layout class that reads it, as in
// Lint's ObsoleteLayoutParamsDetector.getLayoutForAttribute.
var obsoleteParamLayouts = buildObsoleteParamLayouts()

func buildObsoleteParamLayouts() map[string]string {
	m := map[string]string{"layout_x": "AbsoluteLayout", "layout_y": "AbsoluteLayout", "layout_weight": "LinearLayout", "layout_span": "TableRow"}
	for _, a := range []string{"layout_column", "layout_columnSpan", "layout_row", "layout_rowSpan"} {
		m[a] = "GridLayout"
	}
	for _, a := range []string{
		"layout_alignLeft", "layout_alignStart", "layout_alignRight", "layout_alignEnd", "layout_alignTop",
		"layout_alignBottom", "layout_alignParentTop", "layout_alignParentBottom", "layout_alignParentLeft",
		"layout_alignParentStart", "layout_alignParentRight", "layout_alignParentEnd", "layout_alignWithParentIfMissing",
		"layout_alignBaseline", "layout_centerInParent", "layout_centerVertical", "layout_centerHorizontal",
		"layout_toRightOf", "layout_toEndOf", "layout_toLeftOf", "layout_toStartOf", "layout_below", "layout_above",
	} {
		m[a] = "RelativeLayout"
	}
	return m
}

// obsoleteParamSubclasses lists the framework views, by tag, that extend each layout class
// (the platform widgets.txt hierarchy Lint reads). A tag with no dot that is neither the
// class nor listed here is not a subclass.
var obsoleteParamSubclasses = map[string][]string{
	"LinearLayout":   {"ActionMenuView", "RadioGroup", "SearchView", "TabWidget", "TableLayout", "TableRow", "NumberPicker", "ZoomControls"},
	"RelativeLayout": {"DialerFilter", "TwoLineListItem"},
	"AbsoluteLayout": {"WebView"},
}

// obsoleteParamValidForParent mirrors isValidParamForParent: custom (dotted) tags and <view>
// may be anything, framework tags must be the layout class or one of its subclasses.
func obsoleteParamValidForParent(parentTag, layout string) bool {
	if strings.Contains(parentTag, ".") || parentTag == "view" || parentTag == layout {
		return true
	}
	for _, sub := range obsoleteParamSubclasses[layout] {
		if sub == parentTag {
			return true
		}
	}
	return false
}

// obsoleteParamAllowed reports whether attr (a layout_* name) is read by parentTag.
func obsoleteParamAllowed(parentTag, attr, layout string) bool {
	return obsoleteParamValidForParent(parentTag, layout) ||
		(attr == "layout_column" && obsoleteParamValidForParent(parentTag, "TableRow"))
}

type obsoleteParamInclude struct {
	file      string
	parentTag string
}

type obsoleteParamPending struct {
	layoutName string
	path       string
	view       *android.View
	attrKey    string
	attr       string
	layout     string
}

// obsoleteLayoutParamFindings ports Lint's ObsoleteLayoutParamsDetector. Params on a layout's
// root (or directly under <merge>) are judged by the parents that <include> the layout.
func obsoleteLayoutParamFindings(layouts []*android.Layout, rule BaseRule) []scanner.Finding {
	var findings []scanner.Finding
	var pending []obsoleteParamPending
	includes := map[string][]obsoleteParamInclude{}
	for _, layout := range layouts {
		walkObsoleteParams(layout, layout.RootView, nil, false, includes, &pending, &findings, rule)
	}
	if len(includes) == 0 {
		return findings
	}
	for _, p := range pending {
		incs := includes[p.layoutName]
		if len(incs) == 0 {
			continue
		}
		parts := make([]string, 0, len(incs))
		valid := false
		for _, inc := range incs {
			if obsoleteParamAllowed(inc.parentTag, p.attr, p.layout) {
				valid = true
				break
			}
			parts = append(parts, fmt.Sprintf("included from within a `%s` in `%s`", inc.parentTag, inc.file))
		}
		if valid {
			continue
		}
		findings = append(findings, baseFindingAtAttr(p.path, p.view, p.attrKey, rule,
			fmt.Sprintf("Invalid layout param '`%s`' (%s)", p.attr, strings.Join(parts, ","))))
	}
	return findings
}

func walkObsoleteParams(layout *android.Layout, v, parent *android.View, parentIgnored bool,
	includes map[string][]obsoleteParamInclude, pending *[]obsoleteParamPending, findings *[]scanner.Finding, rule BaseRule) {
	if v == nil {
		return
	}
	ignored := parentIgnored || resourceToolsIgnoreMatches(v, "ObsoleteLayoutParam")
	if v.Type == "include" && parent != nil && parent.Type != "merge" && !strings.Contains(parent.Type, ".") {
		if name, ok := strings.CutPrefix(v.Attributes["layout"], "@layout/"); ok {
			includes[name] = append(includes[name], obsoleteParamInclude{
				file:      filepath.Base(filepath.Dir(layout.FilePath)) + "/" + filepath.Base(layout.FilePath),
				parentTag: parent.Type,
			})
		}
	}
	if !ignored {
		checkObsoleteParamAttrs(layout, v, parent, pending, findings, rule)
	}
	for _, child := range v.Children {
		walkObsoleteParams(layout, child, v, ignored, includes, pending, findings, rule)
	}
}

func checkObsoleteParamAttrs(layout *android.Layout, v, parent *android.View,
	pending *[]obsoleteParamPending, findings *[]scanner.Finding, rule BaseRule) {
	keys := make([]string, 0, len(v.Attributes))
	for k := range v.Attributes {
		if strings.HasPrefix(k, "android:layout_") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		attr := strings.TrimPrefix(key, "android:")
		want, ok := obsoleteParamLayouts[attr]
		if !ok {
			continue
		}
		switch {
		case parent == nil || parent.Type == "merge":
			*pending = append(*pending, obsoleteParamPending{
				layoutName: layout.Name, path: layout.FilePath, view: v, attrKey: key, attr: attr, layout: want,
			})
		case !obsoleteParamAllowed(parent.Type, attr, want):
			*findings = append(*findings, baseFindingAtAttr(layout.FilePath, v, key, rule,
				fmt.Sprintf("Invalid layout param in a `%s`: `%s`", parent.Type, attr)))
		}
	}
}
