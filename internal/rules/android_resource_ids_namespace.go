package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// MissingPrefixResource
// ---------------------------------------------------------------------------

// MissingPrefixResourceRule detects layout attributes that are missing the
// android: namespace prefix. Attributes like "text" or "id" without "android:"
// are silently ignored at runtime.
type MissingPrefixResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// knownAndroidAttrs maps attribute names that require the android: prefix.
var knownAndroidAttrs = map[string]bool{
	"id": true, "text": true, "hint": true,
	"layout_width": true, "layout_height": true,
	"layout_weight": true, "layout_gravity": true, "layout_margin": true,
	"layout_marginLeft": true, "layout_marginRight": true,
	"layout_marginTop": true, "layout_marginBottom": true,
	"layout_marginStart": true, "layout_marginEnd": true,
	"padding": true, "paddingLeft": true, "paddingRight": true,
	"paddingTop": true, "paddingBottom": true,
	"paddingStart": true, "paddingEnd": true,
	"orientation": true, "gravity": true, "visibility": true,
	"background": true, "textColor": true, "textSize": true,
	"src": true, "contentDescription": true, "inputType": true,
	"clickable": true, "focusable": true, "enabled": true,
	"minWidth": true, "minHeight": true, "maxWidth": true, "maxHeight": true,
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *MissingPrefixResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingPrefixResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attrName := range v.Attributes {
				// Skip attributes that are legitimately unprefixed
				if attrName == "style" || strings.HasPrefix(attrName, "xmlns") ||
					strings.HasPrefix(attrName, "class") {
					continue
				}
				// Skip any attribute that already has a prefix (contains :)
				if strings.Contains(attrName, ":") {
					continue
				}
				// Check if this is a known android attribute missing its prefix
				if knownAndroidAttrs[attrName] {
					ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
						fmt.Sprintf("Attribute `%s` missing `android:` prefix. Use `android:%s` instead.",
							attrName, attrName)))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NamespaceTypoResource
// ---------------------------------------------------------------------------

// NamespaceTypoResourceRule detects misspelled Android namespace URIs.
// The correct namespace is "http://schemas.android.com/apk/res/android".
type NamespaceTypoResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

const correctAndroidNS = "http://schemas.android.com/apk/res/android"

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *NamespaceTypoResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NamespaceTypoResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		if layout.RootView == nil {
			continue
		}
		for attrName, attrVal := range layout.RootView.Attributes {
			if !strings.HasPrefix(attrName, "xmlns:") {
				continue
			}
			if attrVal == correctAndroidNS {
				continue
			}
			// Check if the namespace looks like a typo of the Android NS
			if strings.Contains(attrVal, "schemas.android.com") ||
				strings.Contains(attrVal, "schema.android.com") ||
				strings.Contains(attrVal, "shemas.android.com") {
				dist := levenshtein(attrVal, correctAndroidNS)
				if dist > 0 && dist <= 5 {
					ctx.Emit(baseFinding(layout.FilePath, layout.RootView.Line, r.BaseRule,
						fmt.Sprintf("Possible typo in namespace URI `%s`. Did you mean `%s`?",
							attrVal, correctAndroidNS)))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ResAutoResource
// ---------------------------------------------------------------------------

// ResAutoResourceRule detects hardcoded package namespaces in resource XML.
// Attributes using http://schemas.android.com/apk/res/<package> should use
// http://schemas.android.com/apk/res-auto instead.
type ResAutoResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

const resPackagePrefix = "http://schemas.android.com/apk/res/"

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *ResAutoResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ResAutoResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			for attr, val := range v.Attributes {
				// Check xmlns declarations that use a hardcoded package
				if strings.HasPrefix(attr, "xmlns:") && strings.HasPrefix(val, resPackagePrefix) {
					suffix := val[len(resPackagePrefix):]
					if suffix != "" && suffix != "android" {
						ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
							fmt.Sprintf("Namespace declaration `%s=\"%s\"` uses a hardcoded package name. "+
								"Use `http://schemas.android.com/apk/res-auto` instead.",
								attr, val)))
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UnusedNamespaceResource
// ---------------------------------------------------------------------------

// UnusedNamespaceResourceRule detects xmlns namespace declarations on the root
// view that are never used by any attribute in the layout tree.
type UnusedNamespaceResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *UnusedNamespaceResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedNamespaceResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		root := layout.RootView
		if root == nil {
			continue
		}
		// Collect xmlns declarations from root
		prefixes := make(map[string]bool)
		for attr := range root.Attributes {
			if strings.HasPrefix(attr, "xmlns:") {
				prefix := strings.TrimPrefix(attr, "xmlns:")
				prefixes[prefix] = true
			}
		}
		if len(prefixes) == 0 {
			continue
		}
		// Walk entire tree and mark prefixes as used
		used := make(map[string]bool)
		walkViews(root, func(v *android.View) {
			for attr := range v.Attributes {
				if strings.HasPrefix(attr, "xmlns:") {
					continue
				}
				if colonIdx := strings.Index(attr, ":"); colonIdx > 0 {
					prefix := attr[:colonIdx]
					if prefixes[prefix] {
						used[prefix] = true
					}
				}
			}
		})
		// Report unused
		for attr := range root.Attributes {
			if !strings.HasPrefix(attr, "xmlns:") {
				continue
			}
			prefix := strings.TrimPrefix(attr, "xmlns:")
			if !used[prefix] {
				ctx.Emit(baseFinding(layout.FilePath, root.Line, r.BaseRule,
					fmt.Sprintf("Unused namespace declaration `xmlns:%s`. "+
						"No attributes in the layout use the `%s:` prefix.",
						prefix, prefix)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AppCompatResource
// ---------------------------------------------------------------------------

// AppCompatResourceRule detects menu items using android:showAsAction instead
// of app:showAsAction. When using AppCompat, the app: namespace is required.
type AppCompatResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *AppCompatResourceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AppCompatResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	for _, layout := range idx.Layouts {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Attributes["android:showAsAction"] != "" {
				ctx.Emit(baseFinding(layout.FilePath, v.Line, r.BaseRule,
					fmt.Sprintf("`%s` uses `android:showAsAction` instead of `app:showAsAction`. Use `app:showAsAction` for AppCompat compatibility.", v.Type)))
			}
		})
	}
}
