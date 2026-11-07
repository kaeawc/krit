package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
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
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"MissingPrefix"}, func(v *android.View) {
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
					ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attrName, r.BaseRule,
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
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"NamespaceTypo"}, func(v *android.View) {
			for attrName, attrVal := range v.Attributes {
				if !strings.HasPrefix(attrName, "xmlns:") || attrVal == correctAndroidNS {
					continue
				}
				// A package namespace such as apk/res/com.example.app is a different, valid namespace (see ResAuto).
				if pkg, ok := strings.CutPrefix(attrVal, resPackagePrefix); ok && strings.Contains(pkg, ".") {
					continue
				}
				// Check if the namespace looks like a typo of the Android NS
				if strings.Contains(attrVal, "schemas.android.com") ||
					strings.Contains(attrVal, "schema.android.com") ||
					strings.Contains(attrVal, "shemas.android.com") {
					dist := levenshtein(attrVal, correctAndroidNS)
					if dist > 0 && dist <= 5 {
						ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attrName, r.BaseRule,
							fmt.Sprintf("Possible typo in namespace URI `%s`. Did you mean `%s`?",
								attrVal, correctAndroidNS)))
					}
				}
			}
		})
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
	for _, layout := range idx.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{"ResAuto"}, func(v *android.View) {
			for attr, val := range v.Attributes {
				// Check xmlns declarations that use a hardcoded package
				if strings.HasPrefix(attr, "xmlns:") && strings.HasPrefix(val, resPackagePrefix) {
					suffix := val[len(resPackagePrefix):]
					if suffix != "" && suffix != "android" {
						ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
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
	for _, layout := range idx.AllLayouts() {
		root := layout.RootView
		if root == nil || resourceToolsIgnoreMatches(root, "UnusedNamespace") {
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
			// Lint only tracks custom-attribute namespaces (apk/res/<package>), not app, tools or other URIs.
			uri := root.Attributes[attr]
			if !strings.HasPrefix(uri, "http://schemas.android.com/apk/res/") || uri == "http://schemas.android.com/apk/res/android" {
				continue
			}
			if !used[prefix] {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, root, attr, r.BaseRule,
					fmt.Sprintf("Unused namespace `%s`", prefix)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AppCompatResource
// ---------------------------------------------------------------------------

// AppCompatResourceRule mirrors Lint's AppCompatResourceDetector: menu
// showAsAction, actionViewClass and actionProviderClass belong in the app:
// namespace when the module uses appcompat, and in the android: namespace
// when it does not. It stays silent when the dependency graph is unknown.
type AppCompatResourceRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android resource-id rule. Detection flags R.id/R.layout usage patterns
// and naming conventions via structural checks on resources. Classified
// per roadmap/17.
func (r *AppCompatResourceRule) Confidence() float64 { return api.ConfidenceMedium }

var appCompatMenuAttrs = []string{"showAsAction", "actionViewClass", "actionProviderClass"}

// appCompatCoordinates lists artifacts that bring appcompat onto the classpath.
var appCompatCoordinates = []librarymodel.Coordinate{
	{Group: "androidx.appcompat", Name: "appcompat"},
	{Group: "com.android.support", Name: "appcompat-v7"},
	{Group: "com.google.android.material", Name: "material"},
}

func (r *AppCompatResourceRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Profile
	usesAppCompat := profile.HasAnyDependency(appCompatCoordinates...)
	// Lint treats a project without Gradle information as not using appcompat.
	knownAbsent := !profile.HasGradle || profile.ProvesDependencyAbsent(appCompatCoordinates...)
	if !usesAppCompat && !knownAbsent {
		return
	}
	for _, menu := range idx.AllMenus() {
		folderVersion := iconFolderVersion(menu.FilePath)
		walkViewsUnlessIgnored(menu.RootView, []string{"AppCompatResource"}, func(v *android.View) {
			for _, name := range appCompatMenuAttrs {
				if usesAppCompat && folderVersion < 14 && v.Attributes["android:"+name] != "" {
					ctx.Emit(baseFindingAtAttr(menu.FilePath, v, "android:"+name, r.BaseRule,
						fmt.Sprintf("Should use `app:%s` with the appcompat library with `xmlns:app=\"http://schemas.android.com/apk/res-auto\"`", name)))
				}
				if !usesAppCompat {
					for attr := range v.Attributes {
						prefix, local, ok := strings.Cut(attr, ":")
						if ok && local == name && prefix != "android" && prefix != "tools" && prefix != "xmlns" {
							ctx.Emit(baseFindingAtAttr(menu.FilePath, v, attr, r.BaseRule,
								fmt.Sprintf("Should use `android:%s` when not using the appcompat library", name)))
						}
					}
				}
			}
		})
	}
}
