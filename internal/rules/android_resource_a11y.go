package rules

// Android Resource XML rules: Accessibility + i18n rules.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// HardcodedValuesResource
// ---------------------------------------------------------------------------

// HardcodedValuesResourceRule detects android:text with literal strings instead
// of @string/ references. Hardcoded text breaks internationalization.
type HardcodedValuesResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *HardcodedValuesResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// hardcodedTextAttrs lists the android: attributes Lint's HardcodedValuesDetector reads.
var hardcodedTextAttrs = []string{"text", "contentDescription", "hint", "label", "prompt", "textOn", "textOff", "title", "description"}

// isHardcodedSampleText reports placeholder text that Android Studio's layout templates insert.
func isHardcodedSampleText(value, tag string) bool {
	switch value {
	case "Hello World!", "Large Text", "Medium Text", "Small Text", "New Text":
		return true
	}
	return value == "New "+tag
}

func (r *HardcodedValuesResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	layouts := append(idx.AllLayouts(), idx.AllMenus()...)
	for _, layout := range layouts {
		walkViewsUnlessIgnored(layout.RootView, []string{"HardcodedText"}, func(v *android.View) {
			for _, name := range hardcodedTextAttrs {
				attr := "android:" + name
				value, ok := v.Attributes[attr]
				if !ok || value == "" || value[0] == '@' || value[0] == '?' || isHardcodedSampleText(value, v.Type) {
					continue
				}
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
					fmt.Sprintf("Hardcoded string \"%s\", should use `@string` resource", value)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MissingContentDescriptionResource
// ---------------------------------------------------------------------------

// MissingContentDescriptionResourceRule detects ImageView (and ImageButton)
// elements that lack a contentDescription attribute, which is required for
// screen readers.
type MissingContentDescriptionResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// contentDescriptionTags lists the tags Lint's AccessibilityDetector checks for a missing description.
var contentDescriptionTags = map[string]bool{
	"ImageView":   true,
	"ImageButton": true,
	"android.support.design.widget.FloatingActionButton":                    true,
	"com.google.android.material.floatingactionbutton.FloatingActionButton": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *MissingContentDescriptionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingContentDescriptionResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"ContentDescription"}, func(v *android.View) {
			desc, hasDesc := v.Attributes["android:contentDescription"]
			if hasDesc {
				if _, hasHint := v.Attributes["android:hint"]; hasHint {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:contentDescription", r.BaseRule,
						"Do not set both `contentDescription` and `hint`: the `contentDescription` will mask the `hint`"))
				}
			}
			if !contentDescriptionTags[v.Type] {
				return
			}
			switch {
			case !hasDesc:
				if v.Attributes["android:importantForAccessibility"] != "no" {
					ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
						"Missing `contentDescription` attribute on image"))
				}
			case desc == "" || desc == "TODO":
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:contentDescription", r.BaseRule,
					"Empty `contentDescription` attribute on image"))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LabelForResource
// ---------------------------------------------------------------------------

// LabelForResourceRule mirrors Lint's LabelForDetector: an EditText, AutoCompleteTextView or
// MultiAutoCompleteTextView needs an android:hint or a labelFor view, but not both, and a
// TextView that labels another view needs its own text or contentDescription.
type LabelForResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *LabelForResourceRule) Confidence() float64 { return api.ConfidenceMedium }

// labelForFieldTags are the tags LabelForDetector.getApplicableElements visits.
var labelForFieldTags = map[string]bool{"EditText": true, "AutoCompleteTextView": true, "MultiAutoCompleteTextView": true}

const (
	labelForPrefix       = "Missing accessibility label: "
	labelForEitherMsg    = "provide either a view with an `android:labelFor` that references this view or provide an `android:hint`"
	labelForBothMsg      = labelForEitherMsg + ", but not both"
	labelForPreJBMR1Msg  = "where minSdk < 17, you should provide an `android:hint`"
	labelForNeedsTextMsg = "when using `android:labelFor`, you must also define an `android:text` or an `android:contentDescription`"
	labelForMinSdk       = 17
)

func (r *LabelForResourceRule) check(ctx *api.Context) {
	projectMin := 0
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil {
		projectMin = facts.Profile.MinSdkVersion
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		// An unknown minSdk counts as modern.
		minSdk := resourceModuleMinSdk(layout.FilePath)
		if minSdk == 0 {
			minSdk = projectMin
		}
		var fields []*android.View
		labels := map[string]bool{}
		var findings []scanner.Finding
		var walk func(v *android.View, ignored bool)
		walk = func(v *android.View, ignored bool) {
			if v == nil {
				return
			}
			ignored = ignored || resourceToolsIgnoreMatches(v, "LabelFor")
			if target, ok := v.Attributes["android:labelFor"]; ok {
				labels[target] = true
				if v.Type == "TextView" && !ignored && v.Attributes["android:text"] == "" && v.Attributes["android:contentDescription"] == "" {
					findings = append(findings, baseFindingAtAttr(layout.FilePath, v, "android:labelFor", r.BaseRule, labelForPrefix+labelForNeedsTextMsg))
				}
			}
			if labelForFieldTags[v.Type] && !ignored {
				if hint, ok := v.Attributes["android:hint"]; ok && hint == "" {
					findings = append(findings, baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule, "Empty `android:hint` attribute"))
				}
				fields = append(fields, v)
			}
			for _, child := range v.Children {
				walk(child, ignored)
			}
		}
		walk(layout.RootView, false)
		for _, v := range fields {
			msg, stop := labelForFieldMessage(v, labels, minSdk)
			if stop {
				break
			}
			if msg != "" {
				findings = append(findings, baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule, labelForPrefix+msg))
			}
		}
		for _, f := range findings {
			ctx.Emit(f)
		}
	}
}

// labelForFieldMessage returns the report for a text field, or "" when the hint and label are
// consistent. stop is true for a well-formed field: Lint returns from its end-of-file loop there,
// so it never checks the fields after it.
func labelForFieldMessage(v *android.View, labels map[string]bool, minSdk int) (msg string, stop bool) {
	_, hasHint := v.Attributes["android:hint"]
	labelled := labelForIsLabelled(v.ID, labels)
	oldSdk := minSdk != 0 && minSdk < labelForMinSdk
	switch {
	case hasHint && labelled:
		if oldSdk {
			return "", false
		}
		return labelForBothMsg, false
	case !hasHint && !labelled:
		if oldSdk {
			return labelForPreJBMR1Msg, false
		}
		return labelForEitherMsg, false
	case labelled && oldSdk:
		return labelForPreJBMR1Msg, false
	}
	return "", true
}

// labelForIsLabelled reports whether some labelFor names the id, in either @id/ or @+id/ form.
func labelForIsLabelled(id string, labels map[string]bool) bool {
	if labels[id] {
		return true
	}
	if name, ok := strings.CutPrefix(id, "@+id/"); ok {
		return labels["@id/"+name]
	}
	if name, ok := strings.CutPrefix(id, "@id/"); ok {
		return labels["@+id/"+name]
	}
	return false
}

// ---------------------------------------------------------------------------
// ClickableViewAccessibilityResource
// ---------------------------------------------------------------------------

// ClickableViewAccessibilityResourceRule detects views that have
// android:clickable="true" but are missing a contentDescription. Clickable
// views need a content description so screen readers can announce them.
type ClickableViewAccessibilityResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// viewIDName strips the @id/@+id prefix from an android:id value.
func viewIDName(id string) string {
	id = strings.TrimPrefix(id, "@+id/")
	return strings.TrimPrefix(id, "@id/")
}

// labelForTargets collects ids that another view labels through android:labelFor.
func labelForTargets(root *android.View) map[string]bool {
	targets := map[string]bool{}
	walkViews(root, func(v *android.View) {
		if t := v.Attributes["android:labelFor"]; t != "" {
			targets[viewIDName(t)] = true
		}
	})
	return targets
}

// hasAccessibleName reports whether the view exposes text or is hidden from accessibility.
func hasAccessibleName(v *android.View) bool {
	if v.Text != "" || v.Attributes["android:text"] != "" || v.Attributes["android:hint"] != "" {
		return true
	}
	switch v.Attributes["android:importantForAccessibility"] {
	case "no", "noHideDescendants":
		return true
	}
	return false
}

// isIconOnlyClickable reports whether the view type has no text of its own to speak:
// image views, image/floating action buttons, icon views, or a childless plain View.
func isIconOnlyClickable(v *android.View) bool {
	simple := v.Type
	if idx := strings.LastIndex(simple, "."); idx >= 0 {
		simple = simple[idx+1:]
	}
	if len(v.Children) > 0 {
		return false
	}
	if simple == "View" {
		return true
	}
	for _, suffix := range []string{"ImageView", "ImageButton", "FloatingActionButton", "IconView", "Icon"} {
		if strings.HasSuffix(simple, suffix) {
			return true
		}
	}
	return false
}

// isInputBlockingScrim reports whether a childless plain View is clickable only to swallow touches.
func isInputBlockingScrim(v *android.View) bool {
	if v.Type != "View" || len(v.Children) > 0 {
		return false
	}
	if v.Attributes["android:focusable"] == "false" {
		return true
	}
	if v.LayoutWidth == "match_parent" && v.LayoutHeight == "match_parent" {
		return true
	}
	// A zero-sized or invisible plain View has nothing for a screen reader to announce.
	if isZeroDimension(v.LayoutWidth) || isZeroDimension(v.LayoutHeight) {
		return true
	}
	return v.Background == "" && v.Attributes["android:foreground"] == ""
}

func isZeroDimension(value string) bool {
	switch value {
	case "0dp", "0px", "0dip", "0sp", "0":
		return true
	}
	return false
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ClickableViewAccessibilityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ClickableViewAccessibilityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		labelTargets := labelForTargets(layout.RootView)
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:clickable"] != "true" {
				return
			}
			if v.ContentDescription != "" {
				return
			}
			// Text, hint, a labelFor label, or a hidden view already settles the accessible name.
			if hasAccessibleName(v) || labelTargets[viewIDName(v.ID)] {
				return
			}
			// Only icon-only views lack a name of their own; text widgets and groups get one from text.
			if !isIconOnlyClickable(v) {
				return
			}
			if isInputBlockingScrim(v) {
				return
			}
			// Source code sets the description at runtime (for example through an icon setter).
			if v.ID != "" && ctx.DescribedViewNames[viewIDBindingName(viewIDName(v.ID))] {
				return
			}
			// tools:ignore is acceptable
			if resourceToolsIgnoreMatches(v, "ContentDescription") {
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				fmt.Sprintf("`%s` is clickable but missing `android:contentDescription`. "+
					"Add a description for accessibility.", v.Type)))
		})
	}
}

// ---------------------------------------------------------------------------
// BackButtonResource
// ---------------------------------------------------------------------------

// BackButtonResourceRule detects buttons with text "Back" or "@string/back".
// Android uses system navigation for back; explicit back buttons are
// discouraged as they conflict with the platform's navigation pattern.
type BackButtonResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *BackButtonResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *BackButtonResourceRule) check(ctx *api.Context) {
	for _, f := range runButtonDetector(ctx, buttonIssueBack, r.BaseRule) {
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// ButtonCaseResource
// ---------------------------------------------------------------------------

// ButtonCaseResourceRule detects OK/Cancel buttons with wrong capitalization.
// The standard capitalization is "OK" (not "Ok"/"ok") and "Cancel" (not
// "CANCEL"/"cancel") when using hardcoded text instead of string resources.
type ButtonCaseResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ButtonCaseResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonCaseResourceRule) check(ctx *api.Context) {
	for _, f := range runButtonDetector(ctx, buttonIssueCase, r.BaseRule) {
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// ButtonOrderResource
// ---------------------------------------------------------------------------

// ButtonOrderResourceRule checks that in dialog layouts, the OK/confirm button
// appears after the Cancel/dismiss button. Android design guidelines place the
// positive action on the right (higher line number in layout).
type ButtonOrderResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ButtonOrderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonOrderResourceRule) check(ctx *api.Context) {
	for _, f := range runButtonDetector(ctx, buttonIssueOrder, r.BaseRule) {
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// ButtonStyleResource
// ---------------------------------------------------------------------------

// ButtonStyleResourceRule detects dialog-style buttons without borderless style.
// Buttons in dialog or alert layouts should use a Borderless button style.
type ButtonStyleResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ButtonStyleResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonStyleResourceRule) check(ctx *api.Context) {
	for _, f := range runButtonDetector(ctx, buttonIssueStyle, r.BaseRule) {
		ctx.Emit(f)
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
