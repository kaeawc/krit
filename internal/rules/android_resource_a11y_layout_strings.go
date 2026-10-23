package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// LayoutClickableWithoutMinSize
// ---------------------------------------------------------------------------

type LayoutClickableWithoutMinSizeRule struct {
	LayoutResourceBase
	AndroidRule
}

func (r *LayoutClickableWithoutMinSizeRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LayoutClickableWithoutMinSizeRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:clickable"] != "true" {
				return
			}
			w := parseLayoutDp(v.LayoutWidth)
			h := parseLayoutDp(v.LayoutHeight)
			if (w > 0 && w < 48) || (h > 0 && h < 48) {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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

func (r *LayoutEditTextMissingImportanceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LayoutEditTextMissingImportanceRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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

func (r *LayoutImportantForAccessibilityNoRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LayoutImportantForAccessibilityNoRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:importantForAccessibility"] != "no" {
				return
			}
			if v.Attributes["android:clickable"] == "true" ||
				v.Attributes["android:focusable"] == "true" {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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

func (r *LayoutAutofillHintMismatchRule) Confidence() float64 { return api.ConfidenceMedium }

// specificAutofillInputType maps inputType flags that unambiguously identify an
// autofillable field to the autofill hint Android expects for them.
//
// We deliberately exclude generic, ambiguous input types:
//   - "number" (and numeric variants) is used for OTPs, quantities, country
//     codes, PINs, passphrases, etc.; it does NOT imply a credit-card field.
//   - "text" / "textPersonName" is routinely used for search boxes and free
//     text, so requiring a personName hint produces noise.
//
// Only specific, single-purpose types remain here so we can confidently assert
// both the expected hint (for the missing-hint case) and a contradiction (for
// the genuine-mismatch case). Prefer false-negatives over false-positives.
var specificAutofillInputType = map[string]string{
	"textEmailAddress":  "emailAddress",
	"textPassword":      "password",
	"textWebPassword":   "password",
	"numberPassword":    "password",
	"phone":             "phone",
	"textPostalAddress": "postalAddress",
}

// autofillHintCompatibleInputTypes lists, for a given autofillHints value, the
// inputType flags that are consistent with it. A field whose autofillHints is
// set to one of these keys but whose inputType contains none of the compatible
// flags (and is itself a specific, contradicting type) is a genuine mismatch.
var autofillHintCompatibleInputTypes = map[string]map[string]bool{
	"emailAddress":  {"textEmailAddress": true},
	"password":      {"textPassword": true, "textWebPassword": true, "numberPassword": true, "textVisiblePassword": true},
	"phone":         {"phone": true},
	"postalAddress": {"textPostalAddress": true, "textMultiLine": true},
	"creditCardNumber": {
		"number": true, "numberPassword": true, "phone": true,
	},
}

// isExemptFromAutofill reports whether the field opts out of autofill entirely.
func isExemptFromAutofill(v *android.View) bool {
	switch v.Attributes["android:importantForAutofill"] {
	case "no", "noExcludeDescendants":
		return true
	}
	return false
}

// inputTypeFlags splits an android:inputType value (which may be a
// pipe-combined flag set like "number|textNoSuggestions") into its flags.
func inputTypeFlags(inputType string) []string {
	parts := strings.Split(inputType, "|")
	flags := make([]string, 0, len(parts))
	for _, p := range parts {
		if f := strings.TrimSpace(p); f != "" {
			flags = append(flags, f)
		}
	}
	return flags
}

func (r *LayoutAutofillHintMismatchRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			inputType := v.Attributes["android:inputType"]
			if inputType == "" {
				return
			}
			// Fields explicitly opted out of autofill never mismatch.
			if isExemptFromAutofill(v) {
				return
			}

			flags := inputTypeFlags(inputType)
			autofillHints := strings.TrimSpace(v.Attributes["android:autofillHints"])

			// Case 1: genuine mismatch — an autofillHints value is set, but the
			// inputType is a specific type that contradicts it (e.g.
			// autofillHints="password" on inputType="phone"). Only assert this
			// when every flag is a known specific type and none is compatible
			// with the declared hint; otherwise stay silent.
			if autofillHints != "" {
				compatible, known := autofillHintCompatibleInputTypes[autofillHints]
				if !known {
					return
				}
				hasContradiction := false
				for _, f := range flags {
					if compatible[f] {
						return // at least one flag is consistent — no mismatch
					}
					if _, specific := specificAutofillInputType[f]; specific {
						hasContradiction = true
					}
				}
				if hasContradiction {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`%s` has `autofillHints=\"%s\"` but `inputType=\"%s\"` is inconsistent with it. "+
							"Align the autofill hint with the input type.", v.Type, autofillHints, inputType)))
				}
				return
			}

			// Case 2: missing hint — only suggest one when the inputType is a
			// specific, unambiguously autofillable type. Generic types
			// (number, text, textPersonName, ...) are intentionally ignored.
			for _, f := range flags {
				if expectedHint, ok := specificAutofillInputType[f]; ok {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("`%s` has `inputType=\"%s\"` but no `android:autofillHints`. "+
							"Add `autofillHints=\"%s\"` for autofill support.", v.Type, inputType, expectedHint)))
					return
				}
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

func (r *LayoutMinTouchTargetInButtonRowRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LayoutMinTouchTargetInButtonRowRule) check(ctx *api.Context) {
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
						ctx.Emit(baseFinding(layout.FilePath, child.Line, r.BaseRule,
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

func (r *StringNotSelectableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringNotSelectableRule) check(ctx *api.Context) {
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
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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

func (r *StringRepeatedInContentDescriptionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *StringRepeatedInContentDescriptionRule) check(ctx *api.Context) {
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
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
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

func (r *StringSpanInContentDescriptionRule) Confidence() float64 { return api.ConfidenceMedium }

var htmlTagInStringRe = regexp.MustCompile(`<(b|i|u|em|strong|br|a|span|font|big|small|sup|sub|strike|tt)\b`)

func (r *StringSpanInContentDescriptionRule) check(ctx *api.Context) {
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
			ctx.Emit(baseFinding(loc.FilePath, loc.Line, r.BaseRule,
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
