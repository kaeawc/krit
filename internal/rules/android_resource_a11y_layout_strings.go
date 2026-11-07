package rules

import (
	"fmt"
	"regexp"
	"slices"
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

// AndroidDependencies adds the dimension values so @dimen sizes can be resolved.
func (LayoutClickableWithoutMinSizeRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesDimensions
}

func (r *LayoutClickableWithoutMinSizeRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:clickable"] != "true" {
				return
			}
			w := parseLayoutDp(resolveLayoutDimen(idx, v.LayoutWidth))
			h := parseLayoutDp(resolveLayoutDimen(idx, v.LayoutHeight))
			if (w > 0 && w < 48) || (h > 0 && h < 48) {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:clickable", r.BaseRule,
					fmt.Sprintf("`%s` is clickable with a dimension below 48dp. "+
						"Use at least 48dp for touch targets.", v.Type)))
			}
		})
	}
}

// styleItemValue returns the value a project style, or one of its project parents, sets for item such as
// android:minHeight. The style reference is a view's `style` attribute.
func styleItemValue(idx *android.ResourceIndex, styleRef, item string) (string, bool) {
	if idx == nil {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSpace(styleRef), "@style/")
	for depth := 0; depth < 16 && name != ""; depth++ {
		style := idx.Styles[name]
		if style == nil {
			return "", false
		}
		if value, ok := style.Items[item]; ok {
			return strings.TrimSpace(value), true
		}
		if parent := strings.TrimPrefix(strings.TrimSpace(style.Parent), "@style/"); parent != "" {
			name = parent
		} else if dot := strings.LastIndex(name, "."); dot > 0 {
			name = name[:dot]
		} else {
			return "", false
		}
	}
	return "", false
}

// viewAttrOrStyle returns a view attribute, falling back to its style.
func viewAttrOrStyle(idx *android.ResourceIndex, v *android.View, attr string) string {
	if value := v.Attributes[attr]; value != "" {
		return value
	}
	if style := v.Attributes["style"]; style != "" {
		if value, ok := styleItemValue(idx, style, attr); ok {
			return value
		}
	}
	return ""
}

// resolveLayoutDimen replaces an @dimen/name reference with the literal declared in values, when known.
func resolveLayoutDimen(idx *android.ResourceIndex, value string) string {
	if resolved, _, ok := resolveDimenReference(idx, value); ok {
		return resolved
	}
	return value
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

// AndroidDependencies adds styles (read with the dimension values) so a style can supply the autofill attributes.
func (LayoutEditTextMissingImportanceRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesDimensions
}

func (r *LayoutEditTextMissingImportanceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		excluded := map[*android.View]bool{}
		markAutofillExcluded(idx, layout.RootView, false, excluded)
		walkViewsUnlessIgnored(layout.RootView, []string{"Autofill"}, func(v *android.View) {
			switch androidSimpleName(v.Type) {
			case "EditText", "AppCompatEditText", "TextInputEditText":
			default:
				return
			}
			// An ancestor with a *ExcludeDescendants value already decided for this field.
			if excluded[v] {
				return
			}
			// Declared hints already say the field takes part in autofill.
			if viewAttrOrStyle(idx, v, "android:importantForAutofill") != "" || viewAttrOrStyle(idx, v, "android:autofillHints") != "" {
				return
			}
			// Only a field with an input type or hint looks like something autofill would fill or save.
			inputType := viewAttrOrStyle(idx, v, "android:inputType")
			if inputType == "" && v.Attributes["android:hint"] == "" {
				return
			}
			if isReadOnlyEditText(idx, v, inputType) {
				return
			}
			if isPasswordInputType(inputType) {
				ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
					fmt.Sprintf("`%s` is a password field with no `android:autofillHints`. "+
						"Add `android:autofillHints=\"password\"` so password managers can fill it (API 26+).", v.Type)))
				return
			}
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				fmt.Sprintf("`%s` has no `android:importantForAutofill`, so it defaults to `auto` and the autofill service decides whether to fill it. "+
					"Set `yes` or `no` to choose explicitly (API 26+).", v.Type)))
		})
	}
}

// markAutofillExcluded records every view below one that sets importantForAutofill to a *ExcludeDescendants value.
func markAutofillExcluded(idx *android.ResourceIndex, v *android.View, underExcluding bool, out map[*android.View]bool) {
	if v == nil {
		return
	}
	if underExcluding {
		out[v] = true
	}
	switch viewAttrOrStyle(idx, v, "android:importantForAutofill") {
	case "noExcludeDescendants", "yesExcludeDescendants":
		underExcluding = true
	}
	for _, child := range v.Children {
		markAutofillExcluded(idx, child, underExcluding, out)
	}
}

// isReadOnlyEditText reports whether the user cannot type into the field, so autofill never fills it.
func isReadOnlyEditText(idx *android.ResourceIndex, v *android.View, inputType string) bool {
	for _, attr := range []string{"android:enabled", "android:focusable", "android:editable"} {
		if viewAttrOrStyle(idx, v, attr) == "false" {
			return true
		}
	}
	return slices.Contains(inputTypeFlags(inputType), "none")
}

// isPasswordInputType reports whether an inputType value marks a password field.
func isPasswordInputType(inputType string) bool {
	for _, flag := range inputTypeFlags(inputType) {
		if strings.HasSuffix(flag, "Password") {
			return true
		}
	}
	return false
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
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:importantForAccessibility"] != "no" {
				return
			}
			// A non-focusable view with no text or description only intercepts touches.
			if v.Attributes["android:focusable"] == "false" && v.Text == "" && v.ContentDescription == "" &&
				v.Attributes["android:text"] == "" && v.Attributes["android:hint"] == "" {
				return
			}
			if v.Attributes["android:clickable"] == "true" ||
				v.Attributes["android:focusable"] == "true" {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:importantForAccessibility", r.BaseRule,
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

// AndroidDependencies adds strings (hint text) and styles (read with the dimension values) to the layouts.
func (LayoutAutofillHintMismatchRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesStrings | AndroidDepValuesDimensions
}

// specificAutofillInputType maps inputType flags that unambiguously identify an
// autofillable field to the autofill hint Android expects for them.
//
// We deliberately exclude generic, ambiguous input types:
//   - "number" (and numeric variants) is used for OTPs, quantities, country
//     codes, PINs, passphrases, etc.; it does NOT imply a credit-card field.
//   - "text" / "textPersonName" is routinely used for search boxes and free
//     text, so requiring a personName hint produces noise.
//
// "numberPassword" is also excluded: it is usually a PIN, not an account password.
//
// Only specific, single-purpose types remain here so we can confidently assert
// both the expected hint (for the missing-hint case) and a contradiction (for
// the genuine-mismatch case). Prefer false-negatives over false-positives.
var specificAutofillInputType = map[string]string{
	"textEmailAddress":  "emailAddress",
	"textPassword":      "password",
	"textWebPassword":   "password",
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
func isExemptFromAutofill(idx *android.ResourceIndex, v *android.View) bool {
	switch viewAttrOrStyle(idx, v, "android:importantForAutofill") {
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

// looksLikeCredentialField reports whether the id or hint names a sign-in
// field; other fields may hold someone else's data, so a hint is not implied.
func looksLikeCredentialField(idx *android.ResourceIndex, v *android.View) bool {
	text := strings.ToLower(viewIDName(v.ID) + " " + resolveStringValue(idx, v.Attributes["android:hint"]))
	text = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, text)
	for _, kw := range []string{"signin", "login", "username", "password"} {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func (r *LayoutAutofillHintMismatchRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"Autofill"}, func(v *android.View) {
			inputType := viewAttrOrStyle(idx, v, "android:inputType")
			if inputType == "" {
				return
			}
			// Fields explicitly opted out of autofill never mismatch.
			if isExemptFromAutofill(idx, v) {
				return
			}

			flags := inputTypeFlags(inputType)
			autofillHints := strings.TrimSpace(viewAttrOrStyle(idx, v, "android:autofillHints"))

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
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:autofillHints", r.BaseRule,
						fmt.Sprintf("`%s` has `autofillHints=\"%s\"` but `inputType=\"%s\"` is inconsistent with it. "+
							"Align the autofill hint with the input type.", v.Type, autofillHints, inputType)))
				}
				return
			}

			// Case 2: missing hint — only suggest one when the inputType is a
			// specific, unambiguously autofillable type. Generic types
			// (number, text, textPersonName, ...) are intentionally ignored.
			if !looksLikeCredentialField(idx, v) {
				return
			}
			for _, f := range flags {
				if expectedHint, ok := specificAutofillInputType[f]; ok {
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:inputType", r.BaseRule,
						fmt.Sprintf("Missing autofill hint: `%s` has `inputType=\"%s\"` but declares no `android:autofillHints`. "+
							"Add `autofillHints=\"%s\"` so autofill services can fill it.", v.Type, inputType, expectedHint)))
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

// AndroidDependencies adds styles (read with the dimension values) so a button style can supply its minHeight.
func (LayoutMinTouchTargetInButtonRowRule) AndroidDependencies() AndroidDataDependency {
	return AndroidDepLayout | AndroidDepValuesDimensions
}

func (r *LayoutMinTouchTargetInButtonRowRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			switch androidSimpleName(v.Type) {
			case "LinearLayout", "AppCompatLinearLayout", "LinearLayoutCompat":
			default:
				return
			}
			// LinearLayout is horizontal when no orientation is set.
			if orientation := v.Attributes["android:orientation"]; orientation != "horizontal" && orientation != "" {
				return
			}
			if countDirectButtonChildren(v) < 2 {
				return
			}
			for _, child := range v.Children {
				if !isButtonLikeViewType(child.Type) {
					continue
				}
				// A styled button may inherit minHeight from the project style graph.
				// Report only when the style (or its parents) sets it, since a style that
				// does not may still inherit a framework minHeight.
				styleMinH := ""
				if styleRef := child.Attributes["style"]; styleRef != "" {
					value, ok := styleItemValue(idx, styleRef, "android:minHeight")
					if !ok {
						continue
					}
					styleMinH = value
				}
				h := child.LayoutHeight
				if h == "wrap_content" || h == "" {
					minH := child.Attributes["android:minHeight"]
					if minH == "" {
						minH = styleMinH
					}
					// Button, AppCompatButton and MaterialButton get a 48dp minHeight from the theme.
					themed := androidSimpleName(child.Type) != "ImageButton"
					// A resource or theme reference cannot be resolved here, so stay silent.
					if strings.HasPrefix(minH, "@") || strings.HasPrefix(minH, "?") {
						continue
					}
					if (minH == "" && !themed) || (minH != "" && parseLayoutDp(minH) < 48) {
						ctx.Emit(baseFindingAtAttr(layout.FilePath, child, "android:minHeight", r.BaseRule,
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
	switch androidSimpleName(t) {
	case "Button", "AppCompatButton", "MaterialButton", "ImageButton":
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
	for _, layout := range idx.AllLayouts() {
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
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, "android:textIsSelectable", r.BaseRule,
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
	for _, layout := range idx.AllLayouts() {
		r.visit(ctx, idx, layout.FilePath, layout.RootView, nil, false)
	}
}

// hiddenFromAccessibility reports whether TalkBack skips the view itself.
func hiddenFromAccessibility(v *android.View) bool {
	switch v.Attributes["android:importantForAccessibility"] {
	case "no", "noHideDescendants":
		return true
	}
	return false
}

// visit reports duplicate content descriptions; ancestorHides is true when an
// ancestor is noHideDescendants, which makes the whole subtree unreachable.
func (r *StringRepeatedInContentDescriptionRule) visit(ctx *api.Context, idx *android.ResourceIndex, path string, v, parent *android.View, ancestorHides bool) {
	if v == nil {
		return
	}
	hidesDescendants := v.Attributes["android:importantForAccessibility"] == "noHideDescendants"
	if v.ContentDescription != "" && !ancestorHides && !hiddenFromAccessibility(v) {
		cd := resolveStringValue(idx, v.ContentDescription)
		if cd != "" && !hidesDescendants {
			for _, child := range v.Children {
				childText := resolveStringValue(idx, child.Text)
				if childText != "" && !hiddenFromAccessibility(child) && strings.EqualFold(cd, childText) {
					ctx.Emit(baseFindingAtAttr(path, v, "android:contentDescription", r.BaseRule,
						fmt.Sprintf("`contentDescription` on `%s` duplicates visible text of child `%s`. "+
							"TalkBack reads both, causing stutter.", v.Type, child.Type)))
				}
			}
		}
		if parent != nil && cd != "" {
			for _, sibling := range parent.Children {
				if sibling == v || hiddenFromAccessibility(sibling) {
					continue
				}
				sibText := resolveStringValue(idx, sibling.Text)
				if sibText != "" && strings.EqualFold(cd, sibText) {
					ctx.Emit(baseFindingAtAttr(path, v, "android:contentDescription", r.BaseRule,
						fmt.Sprintf("`contentDescription` on `%s` duplicates visible text of sibling `%s`. "+
							"TalkBack reads both, causing stutter.", v.Type, sibling.Type)))
				}
			}
		}
	}
	for _, child := range v.Children {
		r.visit(ctx, idx, path, child, v, ancestorHides || hidesDescendants)
	}
}

// resolveStringValue returns the text for a literal or @string/ reference. Other
// references (@null, ?attr/..., unknown strings) have no text and resolve to "".
func resolveStringValue(idx *android.ResourceIndex, raw string) string {
	if strings.HasPrefix(raw, "@string/") {
		if s, ok := idx.Strings[strings.TrimPrefix(raw, "@string/")]; ok {
			return s
		}
		return ""
	}
	if strings.HasPrefix(raw, "@") || strings.HasPrefix(raw, "?") {
		return ""
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
		if idx.StringsWithMarkup[name] || htmlTagInStringRe.MatchString(value) {
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
	for _, layout := range idx.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			cd := v.ContentDescription
			if strings.HasPrefix(cd, "@string/") {
				names[strings.TrimPrefix(cd, "@string/")] = true
			}
		})
	}
	return names
}
