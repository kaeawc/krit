package rules

// Android Resource XML rules: Accessibility + i18n rules.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
func (r *HardcodedValuesResourceRule) Confidence() float64 { return 0.75 }

func (r *HardcodedValuesResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.HasHardcodedText() {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Hardcoded text `%s` in `%s`. Use a `@string/` resource reference instead for internationalization.",
						truncate(v.Text, 40), v.Type)))
			}
			// Also check android:hint
			if hint := v.Attributes["android:hint"]; hint != "" &&
				!strings.HasPrefix(hint, "@string/") &&
				!strings.HasPrefix(hint, "@android:string/") {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Hardcoded hint `%s` in `%s`. Use a `@string/` resource reference instead.",
						truncate(hint, 40), v.Type)))
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

var imageViewTypes = map[string]bool{
	"ImageView":   true,
	"ImageButton": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *MissingContentDescriptionResourceRule) Confidence() float64 { return 0.75 }

func (r *MissingContentDescriptionResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if !imageViewTypes[v.Type] {
				return
			}
			if v.HasContentDescription() {
				return
			}
			// tools:ignore="ContentDescription" is acceptable
			if v.Attributes["tools:ignore"] != "" &&
				strings.Contains(v.Attributes["tools:ignore"], "ContentDescription") {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("`%s` missing `android:contentDescription` attribute. "+
					"Set a description for accessibility or use `tools:ignore=\"ContentDescription\"`.",
					v.Type)))
		})
	}
}

// ---------------------------------------------------------------------------
// LabelForResource
// ---------------------------------------------------------------------------

// LabelForResourceRule detects EditText views without a corresponding
// labelFor from a sibling view, which is needed for accessibility.
type LabelForResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *LabelForResourceRule) Confidence() float64 { return 0.75 }

func (r *LabelForResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	var findings []scanner.Finding
	for _, layout := range idx.Layouts {
		checkLabelFor(layout.RootView, layout.FilePath, r.BaseRule, &findings)
	}
	for _, f := range findings {
		ctx.Emit(f)
	}
}

func checkLabelFor(v *android.View, path string, rule BaseRule, findings *[]scanner.Finding) {
	if v == nil {
		return
	}
	// Collect labelFor targets from siblings (children of this view)
	labelForTargets := make(map[string]bool)
	for _, child := range v.Children {
		if lf := child.Attributes["android:labelFor"]; lf != "" {
			target := strings.TrimPrefix(lf, "@+id/")
			target = strings.TrimPrefix(target, "@id/")
			labelForTargets[target] = true
		}
	}
	// Check if any EditText child is missing a corresponding labelFor
	for _, child := range v.Children {
		if child.Type == "EditText" || child.Type == "AppCompatEditText" {
			// Skip EditText with android:hint — the hint serves as the
			// accessibility label (TalkBack announces it).
			if child.Attributes["android:hint"] != "" {
				continue
			}
			// Skip EditText with contentDescription — it's already labeled.
			if child.Attributes["android:contentDescription"] != "" {
				continue
			}
			if child.ID != "" {
				id := strings.TrimPrefix(child.ID, "@+id/")
				id = strings.TrimPrefix(id, "@id/")
				if !labelForTargets[id] {
					*findings = append(*findings, resourceFinding(path, child.Line, rule,
						fmt.Sprintf("No sibling has `android:labelFor` pointing to `%s`. "+
							"Add a label for accessibility.", child.ID)))
				}
			} else {
				// EditText without an ID can't be referenced by labelFor
				*findings = append(*findings, resourceFinding(path, child.Line, rule,
					fmt.Sprintf("`%s` has no `android:id`, so no label can reference it via `labelFor`. "+
						"Add an id and a corresponding label for accessibility.", child.Type)))
			}
		}
	}
	// Recurse into children
	for _, child := range v.Children {
		checkLabelFor(child, path, rule, findings)
	}
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

// containerViewTypes are view types where contentDescription is typically
// not meaningful — the children of the container provide the accessible
// content. Marking these clickable to intercept taps is a common pattern
// and shouldn't require contentDescription.
var containerViewTypes = map[string]bool{
	"LinearLayout":         true,
	"RelativeLayout":       true,
	"FrameLayout":          true,
	"ConstraintLayout":     true,
	"CoordinatorLayout":    true,
	"GridLayout":           true,
	"TableLayout":          true,
	"ScrollView":           true,
	"HorizontalScrollView": true,
	"NestedScrollView":     true,
	"ViewPager":            true,
	"ViewPager2":           true,
	"RecyclerView":         true,
	"CardView":             true,
	"MaterialCardView":     true,
}

func isContainerView(viewType string) bool {
	// Strip fully-qualified prefixes to the simple name
	simple := viewType
	if idx := strings.LastIndex(simple, "."); idx >= 0 {
		simple = simple[idx+1:]
	}
	return containerViewTypes[simple]
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ClickableViewAccessibilityResourceRule) Confidence() float64 { return 0.75 }

func (r *ClickableViewAccessibilityResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:clickable"] != "true" {
				return
			}
			if v.ContentDescription != "" {
				return
			}
			// Container views provide accessibility via their children.
			if isContainerView(v.Type) {
				return
			}
			// tools:ignore is acceptable
			if v.Attributes["tools:ignore"] != "" &&
				strings.Contains(v.Attributes["tools:ignore"], "ContentDescription") {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *BackButtonResourceRule) Confidence() float64 { return 0.75 }

func (r *BackButtonResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "Button" && v.Type != "AppCompatButton" &&
				v.Type != "com.google.android.material.button.MaterialButton" {
				return
			}
			text := v.Text
			if strings.EqualFold(text, "back") || text == "@string/back" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Button with text `%s` detected. Avoid explicit back buttons; use the system navigation bar instead.",
						text)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ButtonCaseResource
// ---------------------------------------------------------------------------

// ButtonCaseResourceRule detects OK/Cancel buttons with wrong capitalization.
// On Android, button text should be "OK" (not "Ok"/"ok") and "CANCEL" (not
// "Cancel"/"cancel") when using hardcoded text instead of string resources.
type ButtonCaseResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ButtonCaseResourceRule) Confidence() float64 { return 0.75 }

func (r *ButtonCaseResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "Button" && v.Type != "AppCompatButton" {
				return
			}
			text := v.Attributes["android:text"]
			if text == "" || strings.HasPrefix(text, "@string/") || strings.HasPrefix(text, "@android:string/") {
				return
			}
			lower := strings.ToLower(text)
			if lower == "ok" && text != "OK" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Button text `%s` should be `OK` (all caps).", text)))
			}
			if lower == "cancel" && text != "CANCEL" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Button text `%s` should be `CANCEL` (all caps on Android).", text)))
			}
		})
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

func isOKButton(v *android.View) bool {
	t := strings.ToLower(v.Text)
	return t == "ok" || t == "confirm" || t == "@string/ok" || t == "@string/confirm"
}

func isCancelButton(v *android.View) bool {
	t := strings.ToLower(v.Text)
	return t == "cancel" || t == "dismiss" || t == "@string/cancel" || t == "@string/dismiss"
}

// Confidence reports a tier-2 (medium) base confidence. Android accessibility resource rule. Detection flags missing
// contentDescription, importantForAccessibility, and focusable attributes
// via structural checks. Classified per roadmap/17.
func (r *ButtonOrderResourceRule) Confidence() float64 { return 0.75 }

func (r *ButtonOrderResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			// Look at children for button pairs
			var okBtn, cancelBtn *android.View
			for _, child := range v.Children {
				if child.Type != "Button" && child.Type != "AppCompatButton" &&
					child.Type != "com.google.android.material.button.MaterialButton" {
					continue
				}
				if isOKButton(child) {
					okBtn = child
				}
				if isCancelButton(child) {
					cancelBtn = child
				}
			}
			if okBtn != nil && cancelBtn != nil && cancelBtn.Line > okBtn.Line {
				ctx.Emit(resourceFinding(layout.FilePath, cancelBtn.Line, r.BaseRule,
					fmt.Sprintf("Cancel button (line %d) appears after OK button (line %d). "+
						"Android convention places Cancel before OK.", cancelBtn.Line, okBtn.Line)))
			}
		})
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
func (r *ButtonStyleResourceRule) Confidence() float64 { return 0.75 }

func (r *ButtonStyleResourceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for name, layout := range idx.Layouts {
		if !strings.HasPrefix(name, "dialog_") && !strings.HasPrefix(name, "alert_") {
			continue
		}
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "Button" && v.Type != "AppCompatButton" {
				return
			}
			style := v.Attributes["style"]
			if style != "" && (strings.Contains(style, "Borderless") || strings.Contains(style, "borderless")) {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Button in dialog layout `%s` should use a borderless style (e.g., `?android:attr/borderlessButtonStyle`).", name)))
		})
	}
}

// ---------------------------------------------------------------------------
// LayoutClickableWithoutMinSize
// ---------------------------------------------------------------------------

type LayoutClickableWithoutMinSizeRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutClickableWithoutMinSizeRule) Confidence() float64 { return 0.75 }

func (r *LayoutClickableWithoutMinSizeRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:clickable"] != "true" {
				return
			}
			w := parseLayoutDp(v.LayoutWidth)
			h := parseLayoutDp(v.LayoutHeight)
			if (w > 0 && w < 48) || (h > 0 && h < 48) {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is clickable with a dimension below 48dp. "+
						"Use at least 48dp for touch targets.", v.Type)))
			}
		})
	}
}

func parseLayoutDp(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "wrap_content" || value == "match_parent" || value == "0dp" || value == "" {
		return 0
	}
	value = strings.TrimSuffix(value, "dp")
	value = strings.TrimSuffix(value, "dip")
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return f
}

// ---------------------------------------------------------------------------
// LayoutEditTextMissingImportance
// ---------------------------------------------------------------------------

type LayoutEditTextMissingImportanceRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutEditTextMissingImportanceRule) Confidence() float64 { return 0.75 }

func (r *LayoutEditTextMissingImportanceRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "EditText" && v.Type != "AppCompatEditText" &&
				v.Type != "TextInputEditText" {
				return
			}
			if v.Attributes["android:importantForAutofill"] != "" {
				return
			}
			ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("`%s` missing `android:importantForAutofill`. "+
					"Set `yes` or `no` explicitly for autofill accessibility (API 26+).", v.Type)))
		})
	}
}

// ---------------------------------------------------------------------------
// LayoutImportantForAccessibilityNo
// ---------------------------------------------------------------------------

type LayoutImportantForAccessibilityNoRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutImportantForAccessibilityNoRule) Confidence() float64 { return 0.75 }

func (r *LayoutImportantForAccessibilityNoRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:importantForAccessibility"] != "no" {
				return
			}
			if v.Attributes["android:clickable"] == "true" ||
				v.Attributes["android:focusable"] == "true" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` has `importantForAccessibility=\"no\"` but is clickable or focusable. "+
						"This hides an interactive element from assistive technology.", v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LayoutAutofillHintMismatch
// ---------------------------------------------------------------------------

type LayoutAutofillHintMismatchRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutAutofillHintMismatchRule) Confidence() float64 { return 0.75 }

var inputTypeToAutofillHint = map[string]string{
	"textEmailAddress":  "emailAddress",
	"textPassword":      "password",
	"textPersonName":    "personName",
	"phone":             "phone",
	"textPostalAddress": "postalAddress",
	"number":            "creditCardNumber",
}

func (r *LayoutAutofillHintMismatchRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			inputType := v.Attributes["android:inputType"]
			if inputType == "" {
				return
			}
			expectedHint, ok := inputTypeToAutofillHint[inputType]
			if !ok {
				return
			}
			autofillHints := v.Attributes["android:autofillHints"]
			if autofillHints == "" {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` has `inputType=\"%s\"` but no `android:autofillHints`. "+
						"Add `autofillHints=\"%s\"` for autofill support.", v.Type, inputType, expectedHint)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LayoutMinTouchTargetInButtonRow
// ---------------------------------------------------------------------------

type LayoutMinTouchTargetInButtonRowRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutMinTouchTargetInButtonRowRule) Confidence() float64 { return 0.75 }

func (r *LayoutMinTouchTargetInButtonRowRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "LinearLayout" && v.Type != "AppCompatLinearLayout" {
				return
			}
			orientation := v.Attributes["android:orientation"]
			if orientation != "horizontal" {
				return
			}
			if countDirectButtonChildren(v) < 2 {
				return
			}
			for _, child := range v.Children {
				if !isButtonLikeViewType(child.Type) {
					continue
				}
				// A styled button may inherit minHeight from the project style
				// graph. Until style resolution is wired into this rule, do not
				// report styled buttons based only on local layout attributes.
				if child.Attributes["style"] != "" {
					continue
				}
				h := child.LayoutHeight
				if h == "wrap_content" || h == "" {
					minH := child.Attributes["android:minHeight"]
					if minH == "" || parseLayoutDp(minH) < 48 {
						ctx.Emit(resourceFinding(layout.FilePath, child.Line, r.BaseRule,
							fmt.Sprintf("`%s` in `%s` with `wrap_content` height and no `minHeight >= 48dp`. "+
								"Ensure a minimum 48dp touch target.", child.Type, v.Type)))
					}
				}
			}
		})
	}
}

func countDirectButtonChildren(v *android.View) int {
	if v == nil {
		return 0
	}
	count := 0
	for _, child := range v.Children {
		if isButtonLikeViewType(child.Type) {
			count++
		}
	}
	return count
}

func isButtonLikeViewType(t string) bool {
	switch t {
	case "Button", "AppCompatButton", "com.google.android.material.button.MaterialButton", "MaterialButton", "ImageButton":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// StringNotSelectable
// ---------------------------------------------------------------------------

type StringNotSelectableRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *StringNotSelectableRule) Confidence() float64 { return 0.75 }

func (r *StringNotSelectableRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "TextView" && v.Type != "AppCompatTextView" {
				return
			}
			if v.Attributes["android:textIsSelectable"] != "false" {
				return
			}
			text := v.Attributes["android:text"]
			if text == "" {
				return
			}
			stringValue := text
			if strings.HasPrefix(text, "@string/") {
				name := strings.TrimPrefix(text, "@string/")
				if s, ok := idx.Strings[name]; ok {
					stringValue = s
				}
			}
			if containsURLOrPhone(stringValue) {
				ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` with `textIsSelectable=\"false\"` contains URLs or phone numbers. "+
						"Allow selection for assistive technology users to copy content.", v.Type)))
			}
		})
	}
}

var urlOrPhoneRe = regexp.MustCompile(`https?://|tel:|(\+?\d[\d\-\s]{6,}\d)`)

func containsURLOrPhone(s string) bool {
	return urlOrPhoneRe.MatchString(s)
}

// ---------------------------------------------------------------------------
// StringRepeatedInContentDescription
// ---------------------------------------------------------------------------

type StringRepeatedInContentDescriptionRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *StringRepeatedInContentDescriptionRule) Confidence() float64 { return 0.75 }

func (r *StringRepeatedInContentDescriptionRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.ContentDescription == "" {
				return
			}
			cd := resolveStringValue(idx, v.ContentDescription)
			for _, sibling := range v.Children {
				sibText := resolveStringValue(idx, sibling.Text)
				if sibText != "" && strings.EqualFold(cd, sibText) {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`contentDescription` on `%s` duplicates visible text of child `%s`. "+
							"TalkBack reads both, causing stutter.", v.Type, sibling.Type)))
				}
			}
		})
		walkViewsWithParent(layout.RootView, nil, func(v *android.View, parent *android.View) {
			if parent == nil || v.ContentDescription == "" {
				return
			}
			cd := resolveStringValue(idx, v.ContentDescription)
			for _, sibling := range parent.Children {
				if sibling == v {
					continue
				}
				sibText := resolveStringValue(idx, sibling.Text)
				if sibText != "" && strings.EqualFold(cd, sibText) {
					ctx.Emit(resourceFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`contentDescription` on `%s` duplicates visible text of sibling `%s`. "+
							"TalkBack reads both, causing stutter.", v.Type, sibling.Type)))
				}
			}
		})
	}
}

func resolveStringValue(idx *android.ResourceIndex, raw string) string {
	if strings.HasPrefix(raw, "@string/") {
		name := strings.TrimPrefix(raw, "@string/")
		if s, ok := idx.Strings[name]; ok {
			return s
		}
	}
	return raw
}

// ---------------------------------------------------------------------------
// StringSpanInContentDescription
// ---------------------------------------------------------------------------

type StringSpanInContentDescriptionRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *StringSpanInContentDescriptionRule) Confidence() float64 { return 0.75 }

var htmlTagInStringRe = regexp.MustCompile(`<(b|i|u|em|strong|br|a|span|font|big|small|sup|sub|strike|tt)\b`)

func (r *StringSpanInContentDescriptionRule) check(ctx *v2.Context) {
	idx := ctx.ResourceIndex
	cdStrings := collectContentDescriptionStringNames(idx)

	for name := range cdStrings {
		value, ok := idx.Strings[name]
		if !ok {
			continue
		}
		if htmlTagInStringRe.MatchString(value) {
			loc, locOk := idx.StringsLocation[name]
			if !locOk {
				continue
			}
			ctx.Emit(resourceFinding(loc.FilePath, loc.Line, r.BaseRule,
				fmt.Sprintf("String `%s` used as `contentDescription` contains HTML markup. "+
					"TalkBack reads raw tags aloud.", name)))
		}
	}
}

func collectContentDescriptionStringNames(idx *android.ResourceIndex) map[string]bool {
	names := make(map[string]bool)
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			cd := v.ContentDescription
			if strings.HasPrefix(cd, "@string/") {
				names[strings.TrimPrefix(cd, "@string/")] = true
			}
		})
	}
	return names
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
