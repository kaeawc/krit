package rules

// Android Resource XML rules: Accessibility + i18n rules.

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
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

func (r *HardcodedValuesResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.HasHardcodedText() {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Hardcoded text `%s` in `%s`. Use a `@string/` resource reference instead for internationalization.",
						truncate(v.Text, 40), v.Type)))
			}
			// Also check android:hint
			if hint := v.Attributes["android:hint"]; hint != "" &&
				!strings.HasPrefix(hint, "@string/") &&
				!strings.HasPrefix(hint, "@android:string/") {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *MissingContentDescriptionResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingContentDescriptionResourceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *LabelForResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LabelForResourceRule) check(ctx *api.Context) {
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
					*findings = append(*findings, baseFinding(path, child.Line, rule,
						fmt.Sprintf("No sibling has `android:labelFor` pointing to `%s`. "+
							"Add a label for accessibility.", child.ID)))
				}
			} else {
				// EditText without an ID can't be referenced by labelFor
				*findings = append(*findings, baseFinding(path, child.Line, rule,
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
func (r *ClickableViewAccessibilityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ClickableViewAccessibilityResourceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "Button" && v.Type != "AppCompatButton" &&
				v.Type != "com.google.android.material.button.MaterialButton" {
				return
			}
			text := v.Text
			if strings.EqualFold(text, "back") || text == "@string/back" {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *ButtonCaseResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonCaseResourceRule) check(ctx *api.Context) {
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
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("Button text `%s` should be `OK` (all caps).", text)))
			}
			if lower == "cancel" && text != "CANCEL" {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
func (r *ButtonOrderResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonOrderResourceRule) check(ctx *api.Context) {
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
				ctx.Emit(baseFinding(layout.FilePath, cancelBtn.Line, r.BaseRule,
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
func (r *ButtonStyleResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ButtonStyleResourceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
				fmt.Sprintf("Button in dialog layout `%s` should use a borderless style (e.g., `?android:attr/borderlessButtonStyle`).", name)))
		})
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
