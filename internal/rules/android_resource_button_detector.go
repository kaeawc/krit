package rules

// A port of Lint's ButtonDetector, shared by the BackButton, ButtonCase,
// ButtonOrder and ButtonStyle rules. Lint runs it in two phases over layouts
// and values; the port does the same so the findings and message wording
// match.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type buttonIssue int

const (
	buttonIssueBack buttonIssue = iota
	buttonIssueCase
	buttonIssueOrder
	buttonIssueStyle
)

var buttonIssueIDs = map[buttonIssue]string{
	buttonIssueBack:  "BackButton",
	buttonIssueCase:  "ButtonCase",
	buttonIssueOrder: "ButtonOrder",
	buttonIssueStyle: "ButtonStyle",
}

const (
	buttonBarButtonStyleMessage = "Buttons in button bars should be borderless; use `style=\"?android:attr/buttonBarButtonStyle\"` (and `?android:attr/buttonBarStyle` on the parent)"
	backButtonMessage           = "Back buttons are not standard on Android; see design guide's navigation section"
	selectableItemBackground    = "?android:attr/selectableItemBackground"
)

type buttonDetector struct {
	idx   *android.ResourceIndex
	issue buttonIssue
	rule  BaseRule
	out   []scanner.Finding
	// ignore holds buttons that an order finding already covers.
	ignore map[*android.View]bool
	// applicable holds "@string/name" references to OK, Cancel and Back strings.
	applicable map[string]bool
	// keyToLabel maps a string name to its label for files that define such strings.
	keyToLabel map[string]string
	parents    map[*android.View]*android.View
}

// runButtonDetector returns the findings of one of the four Lint button issues.
func runButtonDetector(ctx *api.Context, issue buttonIssue, rule BaseRule) []scanner.Finding {
	idx := ctx.ResourceIndex
	if idx == nil {
		return nil
	}
	d := &buttonDetector{
		idx: idx, issue: issue, rule: rule,
		ignore:     map[*android.View]bool{},
		applicable: map[string]bool{},
		keyToLabel: map[string]string{},
		parents:    map[*android.View]*android.View{},
	}
	d.scanStrings()
	layouts := idx.AllLayouts()
	for _, layout := range layouts {
		d.indexParents(layout.RootView, nil)
	}
	for _, layout := range layouts {
		d.phaseOne(layout)
	}
	if len(d.applicable) > 0 {
		for _, layout := range layouts {
			d.phaseTwo(layout)
		}
	}
	return d.out
}

func (d *buttonDetector) indexParents(v, parent *android.View) {
	if v == nil {
		return
	}
	d.parents[v] = parent
	for _, child := range v.Children {
		d.indexParents(child, v)
	}
}

// stripButtonLabel trims a label and removes one pair of surrounding quotes.
func stripButtonLabel(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// scanStrings finds default-locale OK, Cancel and Back strings, reports their
// capitalization and records which string files hold them.
func (d *buttonDetector) scanStrings() {
	names := make([]string, 0, len(d.idx.StringSources))
	for name := range d.idx.StringSources {
		names = append(names, name)
	}
	sort.Strings(names)
	labelFiles := map[string]bool{}
	for _, name := range names {
		src := d.idx.StringSources[name]
		text := strings.TrimSpace(src.Value)
		for _, label := range []string{"Cancel", "OK", "Back"} {
			if len(text) < len(label) || !strings.EqualFold(text[:len(label)], label) {
				continue
			}
			if !strings.EqualFold(stripButtonLabel(text), label) {
				break
			}
			d.applicable["@string/"+name] = true
			labelFiles[src.FilePath] = true
			if label != "Back" && stripButtonLabel(text) != label && d.issue == buttonIssueCase {
				tip := "@android:string/" + strings.ToLower(label)
				d.out = append(d.out, baseFindingAt(src.FilePath, src.Line, 0, d.rule,
					fmt.Sprintf("The standard Android way to capitalize %s is \"%s\" (tip: use `%s` instead)", text, label, tip)))
			}
			break
		}
	}
	for _, name := range names {
		src := d.idx.StringSources[name]
		if labelFiles[src.FilePath] {
			if label := stripButtonLabel(src.Value); label != "" {
				d.keyToLabel[name] = label
			}
		}
	}
}

func (d *buttonDetector) ignored(v *android.View) bool {
	for cur := v; cur != nil; cur = d.parents[cur] {
		if resourceToolsIgnoreMatches(cur, buttonIssueIDs[d.issue]) {
			return true
		}
	}
	return false
}

func (d *buttonDetector) walkButtons(layout *android.Layout, fn func(*android.View)) {
	var walk func(v *android.View)
	walk = func(v *android.View) {
		if v == nil {
			return
		}
		if v.Type == "Button" {
			fn(v)
		}
		for _, child := range v.Children {
			walk(child)
		}
	}
	walk(layout.RootView)
}

func (d *buttonDetector) emit(layout *android.Layout, v *android.View, attr, msg string) {
	if d.ignored(v) {
		return
	}
	if attr != "" {
		d.out = append(d.out, baseFindingAtAttr(layout.FilePath, v, attr, d.rule, msg))
		return
	}
	d.out = append(d.out, baseFindingAt(layout.FilePath, v.Line, v.Col, d.rule, msg))
}

func (d *buttonDetector) phaseOne(layout *android.Layout) {
	d.walkButtons(layout, func(v *android.View) {
		if d.issue == buttonIssueStyle && d.isInButtonBar(v) {
			if _, hasStyle := v.Attributes["style"]; !hasStyle &&
				v.Attributes["android:background"] != selectableItemBackground &&
				!parentDefinesSelectableItem(v, d.parents) {
				d.emit(layout, v, "", buttonBarButtonStyleMessage)
			}
		}
		text := v.Attributes["android:text"]
		switch text {
		case "Cancel", "@android:string/cancel":
			if d.isWrongPosition(v, true) {
				d.reportPosition(layout, v, true)
			}
		case "OK", "@android:string/ok":
			if d.isWrongPosition(v, false) {
				d.reportPosition(layout, v, false)
			}
		case "@android:string/yes", "@android:string/no":
			if d.issue != buttonIssueCase {
				return
			}
			yes := text == "@android:string/yes"
			returned, wanted, tip := "Cancel", "No", "@android:string/cancel"
			if yes {
				returned, wanted, tip = "OK", "Yes", "@android:string/ok"
			}
			d.emit(layout, v, "android:text", fmt.Sprintf(
				"%s actually returns \"%s\", not \"%s\"; use %s instead or create a local string resource for %s",
				text, returned, wanted, tip, wanted))
		}
	})
}

func (d *buttonDetector) phaseTwo(layout *android.Layout) {
	d.walkButtons(layout, func(v *android.View) {
		text := v.Attributes["android:text"]
		if !d.applicable[text] {
			return
		}
		switch strings.ToLower(d.keyToLabel[strings.TrimPrefix(text, "@string/")]) {
		case "cancel":
			if d.isWrongPosition(v, true) {
				d.reportPosition(layout, v, true)
			}
		case "ok":
			if d.isWrongPosition(v, false) {
				d.reportPosition(layout, v, false)
			}
		case "back":
			if d.issue == buttonIssueBack {
				d.emit(layout, v, "", backButtonMessage)
			}
		}
	})
}

func parentDefinesSelectableItem(v *android.View, parents map[*android.View]*android.View) bool {
	for cur := v; cur != nil; cur = parents[cur] {
		if cur.Attributes["android:background"] == selectableItemBackground {
			return true
		}
	}
	return false
}

// isInButtonBar reports whether a button sits in a bar of two or more
// buttons, or in a parent styled with buttonBarStyle.
func (d *buttonDetector) isInButtonBar(v *android.View) bool {
	parent := d.parents[v]
	if parent == nil {
		return false
	}
	if strings.Contains(parent.Attributes["style"], "buttonBarStyle") {
		return true
	}
	if len(parent.Children) < 2 {
		return false
	}
	if parent.Type != "LinearLayout" && parent.Type != "TableRow" {
		return false
	}
	if parent.Attributes["android:orientation"] == "vertical" {
		return false
	}
	for _, child := range parent.Children {
		if child.Type != "Button" {
			return false
		}
	}
	return true
}

func (d *buttonDetector) isWrongPosition(v *android.View, cancel bool) bool {
	parent := d.parents[v]
	if parent == nil || len(parent.Children) < 2 {
		return false
	}
	switch parent.Type {
	case "LinearLayout", "TableRow":
		if parent.Attributes["android:orientation"] == "vertical" {
			return false
		}
		for i, child := range parent.Children {
			if child != v {
				continue
			}
			if cancel {
				return i > 0
			}
			return i < len(parent.Children)-1
		}
	case "RelativeLayout", "android.support.percent.PercentRelativeLayout":
		attrTrue := func(name string) bool { return v.Attributes["android:"+name] == "true" }
		if cancel {
			if toRight, ok := v.Attributes["android:layout_toRightOf"]; ok && d.isButtonID(parent, toRight) {
				return true
			}
			return attrTrue("layout_alignParentRight") && !attrTrue("layout_alignParentLeft")
		}
		if _, ok := v.Attributes["android:layout_toLeftOf"]; ok && d.isButtonID(parent, v.Attributes["android:layout_toRightOf"]) {
			return true
		}
		return attrTrue("layout_alignParentLeft") && !attrTrue("layout_alignParentRight")
	}
	return false
}

func (d *buttonDetector) isButtonID(parent *android.View, ref string) bool {
	for _, sibling := range parent.Children {
		if idReferencesMatch(sibling.Attributes["android:id"], ref) {
			return sibling.Type == "Button"
		}
	}
	return false
}

// idReferencesMatch compares two id references such as @+id/ok and @id/ok.
func idReferencesMatch(a, b string) bool {
	trim := func(s string) string {
		s = strings.TrimPrefix(s, "@+id/")
		return strings.TrimPrefix(s, "@id/")
	}
	return a != "" && b != "" && trim(a) == trim(b)
}

func (d *buttonDetector) reportPosition(layout *android.Layout, v *android.View, cancel bool) {
	if d.issue != buttonIssueOrder || d.ignore[v] {
		return
	}
	parent := d.parents[v]
	for _, child := range parent.Children {
		d.ignore[child] = true
	}
	msg := "OK button should be on the right"
	if cancel {
		msg = "Cancel button should be on the left"
	}
	if parent.Type == "LinearLayout" || parent.Type == "TableRow" {
		labels := d.labelList(parent)
		was := strings.Join(labels, " | ")
		sortButtonLabels(labels)
		msg += fmt.Sprintf(" (was \"%s\", should be \"%s\")", was, strings.Join(labels, " | "))
	}
	d.emit(layout, v, "", msg)
}

func (d *buttonDetector) labelList(parent *android.View) []string {
	var labels []string
	for _, child := range parent.Children {
		if child.Type == "Button" {
			labels = append(labels, d.buttonLabel(child.Attributes["android:text"]))
		}
	}
	return labels
}

func (d *buttonDetector) buttonLabel(text string) string {
	label := text
	switch {
	case strings.HasPrefix(text, "@android:string/"):
		switch text {
		case "@android:string/ok":
			label = "OK"
		case "@android:string/cancel":
			label = "Cancel"
		}
	case strings.HasPrefix(text, "@string/"):
		if mapped, ok := d.keyToLabel[strings.TrimPrefix(text, "@string/")]; ok {
			label = mapped
		}
	}
	if strings.Contains(label, " ") && !strings.Contains(label, "\"") {
		label = "\"" + label + "\""
	}
	return label
}

// sortButtonLabels moves Cancel to the front and OK to the end, as Lint does.
func sortButtonLabels(labels []string) {
	for i := 0; i < len(labels); i++ {
		switch {
		case strings.EqualFold(labels[i], "Cancel") && i > 0:
			labels[0], labels[i] = labels[i], labels[0]
		case strings.EqualFold(labels[i], "OK") && i < len(labels)-1:
			labels[len(labels)-1], labels[i] = labels[i], labels[len(labels)-1]
		}
	}
}
