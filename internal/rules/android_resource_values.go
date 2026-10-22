package rules

// Android Resource XML rules: Value/string rules.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// WebViewInScrollViewResource
// ---------------------------------------------------------------------------

// WebViewInScrollViewResourceRule detects a WebView placed inside a
// ScrollView. WebView has its own scrolling and nesting it inside a
// ScrollView causes broken scroll behavior.
type WebViewInScrollViewResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *WebViewInScrollViewResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WebViewInScrollViewResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		findWebViewInScroll(layout.RootView, false, layout.FilePath, r.BaseRule, ctx)
	}
}

func findWebViewInScroll(v *android.View, insideScroll bool, path string, rule BaseRule, ctx *api.Context) {
	if v == nil {
		return
	}
	isScroll := android.IsScrollableView(v.Type)
	if insideScroll && v.Type == "WebView" {
		ctx.Emit(baseFinding(path, v.Line, rule,
			"WebView inside a ScrollView causes broken scrolling. "+
				"Remove the ScrollView or use a different container."))
	}
	for _, child := range v.Children {
		findWebViewInScroll(child, insideScroll || isScroll, path, rule, ctx)
	}
}

// ---------------------------------------------------------------------------
// OnClickResource
// ---------------------------------------------------------------------------

// OnClickResourceRule detects android:onClick attributes in layout XML.
// Using onClick in XML is discouraged in favor of View.setOnClickListener in
// code, which is type-safe and does not rely on reflection.
type OnClickResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *OnClickResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OnClickResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if handler := v.Attributes["android:onClick"]; handler != "" {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`android:onClick=\"%s\"` in `%s`. Use `View.setOnClickListener` in code instead of XML onClick handlers.",
						handler, v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TextFieldsResource
// ---------------------------------------------------------------------------

// TextFieldsResourceRule detects EditText views that have neither inputType
// nor hint specified. Without inputType the keyboard may be inappropriate.
type TextFieldsResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *TextFieldsResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TextFieldsResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "EditText" && v.Type != "AppCompatEditText" {
				return
			}
			hasInputType := v.Attributes["android:inputType"] != ""
			hasHint := v.Attributes["android:hint"] != ""
			if !hasInputType && !hasHint {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` is missing both `android:inputType` and `android:hint`. "+
						"Specify `inputType` so the correct keyboard is shown.", v.Type)))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UnusedAttributeResource
// ---------------------------------------------------------------------------

// UnusedAttributeResourceRule detects layout attributes that require a
// minimum API level. These attributes are silently ignored on older platforms.
type UnusedAttributeResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// apiLevelAttrs maps attribute names to the minimum API level that supports them.
var apiLevelAttrs = map[string]int{
	"android:elevation":         21,
	"android:translationZ":      21,
	"android:stateListAnimator": 21,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedAttributeResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedAttributeResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, minAPI := range apiLevelAttrs {
				if v.Attributes[attr] != "" {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Attribute `%s` is only used in API level %d and higher "+
							"(current min is unspecified). It will be ignored on older platforms.",
							attr, minAPI)))
				}
			}
		})
	}
}

func resourceRootsFromIndex(idx *android.ResourceIndex) []string {
	seen := make(map[string]struct{})
	add := func(path string) {
		root, ok := resourceRootFromPath(path)
		if !ok {
			return
		}
		seen[root] = struct{}{}
	}

	for _, loc := range idx.StringsLocation {
		add(loc.FilePath)
	}
	for _, entry := range idx.ExtraTexts {
		add(entry.FilePath)
	}
	for _, layout := range idx.Layouts {
		if layout != nil {
			add(layout.FilePath)
		}
	}

	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func localeTagFromValuesDir(dir string) (string, bool) {
	if !strings.HasPrefix(dir, "values-") {
		return "", false
	}

	qualifier := strings.TrimPrefix(dir, "values-")
	if strings.HasPrefix(qualifier, "b+") {
		return normalizeLocaleTag(qualifier), true
	}

	parts := strings.Split(qualifier, "-")
	if len(parts) == 0 || !isLocaleLanguageQualifier(parts[0]) {
		return "", false
	}

	locale := parts[0]
	if len(parts) > 1 && strings.HasPrefix(parts[1], "r") && len(parts[1]) == 3 {
		locale += "-" + parts[1]
	}
	return normalizeLocaleTag(locale), true
}

// ---------------------------------------------------------------------------
// MissingQuantityResource
// ---------------------------------------------------------------------------

// MissingQuantityResourceRule detects plural resources that are missing the
// required "other" quantity. In English (and all locales), "other" is always
// required.
type MissingQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *MissingQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if _, ok := quantities["other"]; !ok {
			ctx.Emit(baseFinding("res/values/plurals.xml", 0, r.BaseRule,
				fmt.Sprintf("Plural `%s` is missing the required `other` quantity.", name)))
		}
	}
}

// ---------------------------------------------------------------------------
// UnusedQuantityResource
// ---------------------------------------------------------------------------

// UnusedQuantityResourceRule detects plural quantities that are unnecessary
// for the default language (English). For English, "zero", "two", "few", and
// "many" are not used by the plural rules and will never be selected at runtime.
type UnusedQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// unusedEnglishQuantities are quantities that English plural rules never select.
var unusedEnglishQuantities = map[string]bool{
	"zero": true,
	"two":  true,
	"few":  true,
	"many": true,
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *UnusedQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		for qty := range quantities {
			if unusedEnglishQuantities[qty] {
				ctx.Emit(baseFinding("res/values/plurals.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s` defines unused quantity `%s` for English. "+
						"Only `one` and `other` are used.", name, qty)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ImpliedQuantityResource
// ---------------------------------------------------------------------------

// ImpliedQuantityResourceRule detects plural "one" quantities whose value does
// not contain a %d format specifier. When "one" is selected, the actual number
// may not be 1 (e.g., 21, 31 in some locales), so the value should include %d
// to display the correct number.
type ImpliedQuantityResourceRule struct {
	ValuesPluralsResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence.
func (r *ImpliedQuantityResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ImpliedQuantityResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for name, quantities := range idx.Plurals {
		if oneVal, ok := quantities["one"]; ok {
			if !strings.Contains(oneVal, "%d") {
				ctx.Emit(baseFinding("res/values/plurals.xml", 0, r.BaseRule,
					fmt.Sprintf("Plural `%s` quantity `one` value `%s` does not contain `%%d`. "+
						"The actual number may not be 1; use `%%d` to display it correctly.",
						name, truncate(oneVal, 40))))
			}
		}
	}
}

// validConversions is the set of valid format conversion characters.
var validConversions = map[byte]bool{
	's': true, 'S': true, 'd': true, 'D': true, 'f': true,
	'x': true, 'X': true, 'o': true, 'e': true, 'E': true,
	'g': true, 'G': true, 'b': true, 'B': true, 'h': true,
	'H': true, 'c': true, 'C': true, 'a': true, 'A': true,
	'n': true, 't': true, 'T': true,
}
