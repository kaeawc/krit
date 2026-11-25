package rules

// Ports of the resource checks bundled in androidx.appcompat:appcompat 1.8.0 (appcompat-lint): ColorStateListAlphaDetector,
// ImageViewTintDetector, SwitchUsageXmlDetector, TextViewCompoundDrawablesXmlDetector and OnClickXmlDetector.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UseAndroidAlphaRule mirrors UseAndroidAlpha: a <selector> <item> with android:color and app:alpha but no android:alpha.
type UseAndroidAlphaRule struct {
	LayoutResourceBase
	AndroidRule
}

// UseAppTintRule mirrors UseAppTint: android:tint on an ImageView or ImageButton.
type UseAppTintRule struct {
	LayoutResourceBase
	AndroidRule
}

// UseSwitchCompatOrMaterialXMLRule mirrors UseSwitchCompatOrMaterialXml: a <Switch> layout element.
type UseSwitchCompatOrMaterialXMLRule struct {
	LayoutResourceBase
	AndroidRule
}

// UseCompatTextViewDrawableXMLRule mirrors UseCompatTextViewDrawableXml: android: compound drawable attributes on a TextView.
type UseCompatTextViewDrawableXMLRule struct {
	LayoutResourceBase
	AndroidRule
}

// UsingOnClickInXMLRule mirrors UsingOnClickInXml: an onClick layout attribute when minSdk is below 23.
type UsingOnClickInXMLRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal resource XML.
func (r *UseAndroidAlphaRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal resource XML.
func (r *UseAppTintRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal resource XML.
func (r *UseSwitchCompatOrMaterialXMLRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal resource XML.
func (r *UseCompatTextViewDrawableXMLRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal resource XML.
func (r *UsingOnClickInXMLRule) Confidence() float64 { return api.ConfidenceHigh }

func registerAndroidXAppCompatXMLChecks() {
	alpha := &UseAndroidAlphaRule{AndroidRule: lintAndroidRule("UseAndroidAlpha",
		"`android:alpha` attribute missing on `ColorStateList`", ALCCorrectness, ALSError, 1)}
	registerAppCompatResourceRule(alpha.AndroidRule, alpha, alpha.Confidence(), alpha.check)
	tint := &UseAppTintRule{AndroidRule: lintAndroidRule("UseAppTint",
		"`app:tint` attribute should be used on `ImageView` and `ImageButton`", ALCCorrectness, ALSError, 1)}
	registerAppCompatResourceRule(tint.AndroidRule, tint, tint.Confidence(), tint.check)
	sw := &UseSwitchCompatOrMaterialXMLRule{AndroidRule: lintAndroidRule("UseSwitchCompatOrMaterialXml",
		"Replace usage of `Switch` widget", ALCCorrectness, ALSWarning, 5)}
	registerAppCompatResourceRule(sw.AndroidRule, sw, sw.Confidence(), sw.check)
	drawables := &UseCompatTextViewDrawableXMLRule{AndroidRule: lintAndroidRule("UseCompatTextViewDrawableXml",
		"Compat compound drawable attributes should be used on `TextView`", ALCCorrectness, ALSWarning, 1)}
	registerAppCompatResourceRule(drawables.AndroidRule, drawables, drawables.Confidence(), drawables.check)
	onClick := &UsingOnClickInXMLRule{AndroidRule: lintAndroidRule("UsingOnClickInXml",
		"Using `android:onClick` on older version of the platform is broken", ALCCorrectness, ALSWarning, 1)}
	registerAppCompatResourceRule(onClick.AndroidRule, onClick, onClick.Confidence(), onClick.check)
}

func registerAppCompatResourceRule(rule AndroidRule, impl interface{ AndroidDependencies() AndroidDataDependency },
	confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(impl.AndroidDependencies()), Confidence: confidence,
		Implementation: impl, NeedsLibraryFacts: true, Check: check,
	})
}

// appCompatOnClasspath reports whether the project declares a dependency that brings appcompat, whose AAR ships these
// checks. Lint only runs them when that AAR is on the classpath; an undeclared or transitive-only appcompat is missed.
func appCompatOnClasspath(ctx *api.Context) bool {
	if ctx.ResourceIndex == nil {
		return false
	}
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(appCompatCoordinates...)
}

// appCompatNSPrefixes returns the prefixes the layout root binds to uri, as Lint matches attributes by namespace URI.
func appCompatNSPrefixes(root *android.View, uri string) []string {
	var out []string
	for name, value := range root.Attributes {
		if prefix, ok := strings.CutPrefix(name, "xmlns:"); ok && value == uri {
			out = append(out, prefix)
		}
	}
	sort.Strings(out)
	return out
}

// appCompatNSAttr returns the first attribute of v named local in one of the namespace prefixes.
func appCompatNSAttr(v *android.View, prefixes []string, local string) (string, bool) {
	for _, prefix := range prefixes {
		if _, ok := v.Attributes[prefix+":"+local]; ok {
			return prefix + ":" + local, true
		}
	}
	return "", false
}

// appCompatLayoutWalk visits every view of every layout file with the root's android and res-auto prefixes.
func appCompatLayoutWalk(ctx *api.Context, issueID string, fn func(layout *android.Layout, v *android.View, androidNS []string)) {
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		if layout.RootView == nil {
			continue
		}
		androidNS := appCompatNSPrefixes(layout.RootView, lintAndroidURI)
		walkViewsUnlessIgnored(layout.RootView, []string{issueID}, func(v *android.View) {
			fn(layout, v, androidNS)
		})
	}
}

func (r *UseAppTintRule) check(ctx *api.Context) {
	if !appCompatOnClasspath(ctx) {
		return
	}
	appCompatLayoutWalk(ctx, r.RuleName, func(layout *android.Layout, v *android.View, androidNS []string) {
		if v.Type != "ImageView" && v.Type != "ImageButton" {
			return
		}
		if attr, ok := appCompatNSAttr(v, androidNS, "tint"); ok {
			ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule, "Must use `app:tint` instead of `android:tint`"))
		}
	})
}

func (r *UseSwitchCompatOrMaterialXMLRule) check(ctx *api.Context) {
	if !appCompatOnClasspath(ctx) {
		return
	}
	appCompatLayoutWalk(ctx, r.RuleName, func(layout *android.Layout, v *android.View, _ []string) {
		if v.Type == "Switch" {
			ctx.Emit(baseFindingAt(layout.FilePath, v.Line, v.Col, r.BaseRule,
				"Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library"))
		}
	})
}

// appCompatCompoundDrawableAttrs maps each android: TextView attribute to its app: replacement, in Lint's order.
var appCompatCompoundDrawableAttrs = [][2]string{
	{"drawableLeft", "drawableLeftCompat"},
	{"drawableRight", "drawableRightCompat"},
	{"drawableTop", "drawableTopCompat"},
	{"drawableBottom", "drawableBottomCompat"},
	{"drawableStart", "drawableStartCompat"},
	{"drawableEnd", "drawableEndCompat"},
	{"drawableTint", "drawableTint"},
	{"drawableTintMode", "drawableTintMode"},
}

func (r *UseCompatTextViewDrawableXMLRule) check(ctx *api.Context) {
	if !appCompatOnClasspath(ctx) {
		return
	}
	appCompatLayoutWalk(ctx, r.RuleName, func(layout *android.Layout, v *android.View, androidNS []string) {
		if v.Type != "TextView" {
			return
		}
		for _, pair := range appCompatCompoundDrawableAttrs {
			if attr, ok := appCompatNSAttr(v, androidNS, pair[0]); ok {
				ctx.Emit(baseFindingAtAttr(layout.FilePath, v, attr, r.BaseRule,
					"Use `app:"+pair[1]+"` instead of `android:"+pair[0]+"`"))
			}
		}
	})
}

// usingOnClickMinSdk returns the minSdk that applies to a layout file, or 0 when the build does not show it.
func usingOnClickMinSdk(ctx *api.Context, path string) int {
	if levels := android.ResolveSDKLevels(path); levels.MinSdk > 0 {
		return levels.MinSdk
	}
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.MinSdkVersion
}

func (r *UsingOnClickInXMLRule) check(ctx *api.Context) {
	if !appCompatOnClasspath(ctx) {
		return
	}
	minSdks := map[string]int{}
	appCompatLayoutWalk(ctx, r.RuleName, func(layout *android.Layout, v *android.View, _ []string) {
		minSdk, ok := minSdks[layout.FilePath]
		if !ok {
			minSdk = usingOnClickMinSdk(ctx, layout.FilePath)
			minSdks[layout.FilePath] = minSdk
		}
		// Lint reports only below API 23. Lint assumes API 1 when minSdk is unknown; krit abstains instead, since a
		// minSdk set by a convention plugin is invisible to it.
		if minSdk <= 0 || minSdk >= 23 {
			return
		}
		// Lint matches the local name onClick in any namespace.
		names := make([]string, 0, 1)
		for name := range v.Attributes {
			if _, local, ok := strings.Cut(name, ":"); ok && local == "onClick" && !strings.HasPrefix(name, "xmlns:") {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			if strings.HasPrefix(v.Attributes[name], "@{") {
				// Data binding resolves the handler itself on every supported platform version.
				continue
			}
			ctx.Emit(baseFindingAtAttr(layout.FilePath, v, name, r.BaseRule,
				"Use databinding or explicit wiring of click listener in code"))
		}
	})
}

// appCompatResourceFolders are the resource folder types Lint visits with an XML scanner; values and raw hold no
// <selector> documents.
var appCompatResourceFolders = map[string]bool{
	"anim": true, "animator": true, "color": true, "drawable": true, "font": true, "interpolator": true,
	"layout": true, "menu": true, "mipmap": true, "navigation": true, "transition": true, "xml": true,
}

func (r *UseAndroidAlphaRule) check(ctx *api.Context) {
	if !appCompatOnClasspath(ctx) {
		return
	}
	dirs := append([]string(nil), ctx.ResourceIndex.ResDirs...)
	sort.Strings(dirs)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			base, _, _ := strings.Cut(e.Name(), "-")
			if !e.IsDir() || !appCompatResourceFolders[base] {
				continue
			}
			r.checkFolder(ctx, filepath.Join(dir, e.Name()))
		}
	}
}

func (r *UseAndroidAlphaRule) checkFolder(ctx *api.Context, folder string) {
	files, err := os.ReadDir(folder)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".xml") {
			continue
		}
		path := filepath.Join(folder, f.Name())
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "selector") || !strings.Contains(string(data), "alpha") {
			continue
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil || root == nil {
			continue
		}
		r.checkDocument(ctx, path, root)
	}
}

func (r *UseAndroidAlphaRule) checkDocument(ctx *api.Context, path string, root *android.XMLNode) {
	androidNS, autoNS := map[string]bool{}, map[string]bool{}
	for _, a := range root.Attrs {
		if prefix, ok := strings.CutPrefix(a.Name, "xmlns:"); ok {
			androidNS[prefix] = androidNS[prefix] || a.Value == lintAndroidURI
			autoNS[prefix] = autoNS[prefix] || a.Value == lintResAutoURI
		}
	}
	nsAttr := func(n *android.XMLNode, ns map[string]bool, local string) *android.XMLAttribute {
		for i := range n.Attrs {
			if prefix, name, ok := strings.Cut(n.Attrs[i].Name, ":"); ok && name == local && ns[prefix] {
				return &n.Attrs[i]
			}
		}
		return nil
	}
	// Lint visits each <selector> and every <item> below it, so an item of a nested selector is reported once per
	// enclosing selector at the same position; krit reports it once.
	reported := map[*android.XMLNode]bool{}
	var items func(n *android.XMLNode, tokens toolsIgnoreSet)
	items = func(n *android.XMLNode, tokens toolsIgnoreSet) {
		for _, child := range n.Children {
			childTokens := tokens.extend(child.Attr("tools:ignore"))
			if child.Tag == "item" && !reported[child] && !childTokens.silences(r.RuleName) {
				reported[child] = true
				if nsAttr(child, androidNS, "color") != nil && nsAttr(child, androidNS, "alpha") == nil {
					if alpha := nsAttr(child, autoNS, "alpha"); alpha != nil {
						ctx.Emit(scanner.Finding{File: path, Line: alpha.Line, Col: alpha.Col,
							Message: "Must use `android:alpha` if `app:alpha` is used"})
					}
				}
			}
			items(child, childTokens)
		}
	}
	walkXMLToolsIgnore(root, func(n *android.XMLNode, tokens toolsIgnoreSet) {
		if n.Tag == "selector" && !tokens.silences(r.RuleName) {
			items(n, tokens)
		}
	})
}
