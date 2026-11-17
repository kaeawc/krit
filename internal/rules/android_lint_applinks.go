package rules

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// AppLinkURLErrorRule mirrors Lint's AppLinkUrlError: an activity intent filter whose <data> attributes cannot form a
// valid deep link, web link or Android App Link.
type AppLinkURLErrorRule struct {
	ManifestBase
	AndroidRule
}

// AppLinkWarningRule mirrors Lint's AppLinkWarning: an http(s) VIEW intent filter shaped like an Android App Link
// that omits android:autoVerify.
type AppLinkWarningRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *AppLinkURLErrorRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal manifest attributes.
func (r *AppLinkWarningRule) Confidence() float64 { return api.ConfidenceHigh }

const (
	appLinkActionView  = "android.intent.action.VIEW"
	appLinkCategoryPre = "android.intent.category."
	appLinkBrowsable   = appLinkCategoryPre + "BROWSABLE"
	appLinkDefault     = appLinkCategoryPre + "DEFAULT"
)

// appLinkPathAttrs are the path attributes of a <data> tag, in Lint's order.
var appLinkPathAttrs = []string{"path", "pathPrefix", "pathPattern", "pathAdvancedPattern", "pathSuffix"}

// appLinkValue is one android: attribute of an intent-filter child with its substituted value.
type appLinkValue struct {
	attr manifest.Attr
	// value is the attribute value with a resource reference replaced, which krit cannot do, so known is false then.
	value string
	known bool
}

// appLinkData is a <data> tag of an intent filter.
type appLinkData struct {
	node *manifest.Node
}

// appLinkFilter is the parsed shape of one intent filter inside an activity.
type appLinkFilter struct {
	node       *manifest.Node
	actions    map[string]bool
	categories map[string]bool
	autoVerify string
	data       []appLinkData
	// incomplete is set when the autoVerify or a scheme is a resource reference krit cannot resolve. The other
	// filter-level checks only ask whether a host, path or MIME type is present, so their references do not count.
	incomplete bool
}

// appLinkDiag is one finding of the AppLink rules.
type appLinkDiag struct {
	warning bool
	line    int
	col     int
	msg     string
}

type appLinkScan struct {
	diags []appLinkDiag
}

func (s *appLinkScan) errorAtNode(n *manifest.Node, msg string) {
	s.diags = append(s.diags, appLinkDiag{line: n.Line, col: n.Col, msg: msg})
}

func (s *appLinkScan) errorAtAttr(a manifest.Attr, msg string) {
	s.diags = append(s.diags, appLinkDiag{line: a.Line, col: a.Col, msg: msg})
}

func (s *appLinkScan) errorAtValue(a manifest.Attr, msg string) {
	s.diags = append(s.diags, appLinkDiag{line: a.Line, col: a.ValueCol, msg: msg})
}

// appLinkSubstituted reports whether Lint skips value checks: data binding expressions and manifest placeholders.
func appLinkSubstituted(v string) bool {
	return strings.HasPrefix(v, "@{") || strings.HasPrefix(v, "@={") || strings.Contains(v, "${")
}

// appLinkResourceReference reports whether Lint would try to resolve the value as a project resource.
func appLinkResourceReference(v string) bool {
	if !strings.HasPrefix(v, "@") || strings.HasPrefix(v, "@{") || strings.HasPrefix(v, "@={") {
		return false
	}
	return !strings.HasPrefix(v, "@android:") && !strings.HasPrefix(v, "@*android:")
}

func appLinkWebScheme(s string) bool { return s == "http" || s == "https" }

func appLinkHasUpper(s string) bool {
	return strings.IndexFunc(s, unicode.IsUpper) >= 0
}

func appLinkBlank(s string) bool { return strings.TrimSpace(s) == "" }

func appLinkAttr(n *manifest.Node, name string) (appLinkValue, bool) {
	a, ok := n.Attr("android:" + name)
	if !ok {
		return appLinkValue{}, false
	}
	return appLinkValue{attr: a, value: a.Value, known: !appLinkResourceReference(a.Value)}, true
}

func newAppLinkFilter(n *manifest.Node) *appLinkFilter {
	f := &appLinkFilter{node: n, actions: map[string]bool{}, categories: map[string]bool{}}
	if v, ok := appLinkAttr(n, "autoVerify"); ok {
		f.autoVerify = v.value
		f.incomplete = !v.known
	}
	for _, c := range n.Children {
		switch c.Tag {
		case "action":
			f.actions[c.AttrValue("android:name")] = true
		case "category":
			f.categories[c.AttrValue("android:name")] = true
		case "data":
			f.data = append(f.data, appLinkData{node: c})
		}
	}
	for _, v := range f.values("scheme") {
		f.incomplete = f.incomplete || !v.known
	}
	return f
}

// values lists one attribute across the data tags, in document order.
func (f *appLinkFilter) values(name string) []appLinkValue {
	var out []appLinkValue
	for _, d := range f.data {
		if v, ok := appLinkAttr(d.node, name); ok {
			out = append(out, v)
		}
	}
	return out
}

func (f *appLinkFilter) valueSet(name string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range f.values(name) {
		if !seen[v.value] {
			seen[v.value] = true
			out = append(out, v.value)
		}
	}
	return out
}

func (f *appLinkFilter) paths() []string {
	var out []string
	for _, name := range appLinkPathAttrs {
		out = append(out, f.valueSet(name)...)
	}
	return out
}

// hostPortCount counts the distinct host and port pairs of data tags that declare a host.
func (f *appLinkFilter) hostPortCount() int {
	seen := map[[2]string]bool{}
	for _, d := range f.data {
		host, ok := appLinkAttr(d.node, "host")
		if !ok {
			continue
		}
		port, _ := appLinkAttr(d.node, "port")
		seen[[2]string{host.value, port.value}] = true
	}
	return len(seen)
}

func (f *appLinkFilter) hasPortWithoutHost() bool {
	for _, d := range f.data {
		_, hasPort := appLinkAttr(d.node, "port")
		_, hasHost := appLinkAttr(d.node, "host")
		if hasPort && !hasHost {
			return true
		}
	}
	return false
}

func (f *appLinkFilter) allSchemes(pred func(string) bool) bool {
	for _, s := range f.valueSet("scheme") {
		if !pred(s) {
			return false
		}
	}
	return true
}

func (f *appLinkFilter) anyScheme(pred func(string) bool) bool {
	for _, s := range f.valueSet("scheme") {
		if pred(s) {
			return true
		}
	}
	return false
}

func (f *appLinkFilter) substitutedOrWeb(s string) bool {
	return appLinkSubstituted(s) || appLinkWebScheme(s)
}

// analyzeAppLinks runs the checks of Lint's AppLinksValidDetector that krit ports on every activity of the manifest.
func analyzeAppLinks(root *manifest.Node) []appLinkDiag {
	scan := &appLinkScan{}
	manifestWalk(root, func(n, _ *manifest.Node) {
		if n.Tag == "activity" || n.Tag == "activity-alias" {
			scan.activity(n)
		}
	})
	return scan.diags
}

func (s *appLinkScan) activity(activity *manifest.Node) {
	hasView := false
	for _, n := range activity.ChildrenByTag("intent-filter") {
		f := newAppLinkFilter(n)
		s.filter(f)
		hasView = hasView || f.actions[appLinkActionView]
	}
	exported, ok := activity.Attr("android:exported")
	if ok && hasView && !appLinkBlank(exported.Value) && exported.Value != "true" {
		s.errorAtNode(activity, "Activity supporting ACTION_VIEW is not exported")
	}
}

func (s *appLinkScan) filter(f *appLinkFilter) {
	if len(f.data) == 0 && f.categories[appLinkBrowsable] && f.autoVerify != "true" {
		s.errorAtNode(f.node, "Missing data element")
	}
	s.mimeTypes(f)
	s.schemes(f)
	s.hosts(f)
	s.ports(f)
	for _, d := range f.data {
		s.portWithoutHost(d)
		for _, name := range appLinkPathAttrs {
			if v, ok := appLinkAttr(d.node, name); ok {
				s.path(f, d.node, v)
			}
		}
	}
	if f.incomplete {
		return
	}
	s.appLinkWarning(f)
	s.invalidAppLink(f)
	s.requiredParts(f)
}

func (s *appLinkScan) mimeTypes(f *appLinkFilter) {
	for _, v := range f.values("mimeType") {
		if v.known && !appLinkSubstituted(v.value) && appLinkHasUpper(v.value) {
			s.errorAtValue(v.attr, "Mime-type matching is case sensitive and should only use lower-case characters")
		}
	}
}

func (s *appLinkScan) schemes(f *appLinkFilter) {
	for _, v := range f.values("scheme") {
		if !v.known || appLinkSubstituted(v.value) {
			continue
		}
		switch {
		case appLinkBlank(v.value):
			s.errorAtAttr(v.attr, "`"+v.attr.Name+"` cannot be empty")
		case strings.HasSuffix(v.value, ":"):
			s.errorAtValue(v.attr, "Don't include trailing colon in the `scheme` declaration")
		case appLinkHasUpper(v.value):
			s.errorAtValue(v.attr, "Scheme matching is case sensitive and should only use lower-case characters")
		}
	}
}

func (s *appLinkScan) hosts(f *appLinkFilter) {
	for _, v := range f.values("host") {
		if !v.known || appLinkSubstituted(v.value) {
			continue
		}
		if appLinkBlank(v.value) {
			s.errorAtAttr(v.attr, "`"+v.attr.Name+"` cannot be empty")
			continue
		}
		switch {
		case strings.LastIndex(v.value, "*") > 0:
			s.errorAtValue(v.attr, "The host wildcard (`*`) can only be the first character")
		case appLinkHasUpper(v.value):
			s.errorAtValue(v.attr, "Host matching is case sensitive and should only use lower-case characters")
		}
		if f.autoVerify == "true" && !strings.Contains(v.value, ".") {
			s.errorAtValue(v.attr, "Android App Links' `host` attributes must be valid web domains")
		}
	}
}

func (s *appLinkScan) ports(f *appLinkFilter) {
	for _, v := range f.values("port") {
		if !v.known || appLinkSubstituted(v.value) {
			continue
		}
		if appLinkBlank(v.value) {
			s.errorAtAttr(v.attr, "`"+v.attr.Name+"` cannot be empty")
			continue
		}
		if port, err := strconv.Atoi(v.value); err != nil || port < 0 || port > 65535 {
			s.errorAtValue(v.attr, "not a valid port number")
		}
	}
}

func (s *appLinkScan) portWithoutHost(d appLinkData) {
	port, hasPort := appLinkAttr(d.node, "port")
	if _, hasHost := appLinkAttr(d.node, "host"); hasPort && !hasHost {
		s.errorAtValue(port.attr, "The port must be specified in the same `<data>` element as the `host`")
	}
}

// path ports handlePath: path shape, query and fragment parts under autoVerify, and the `?` regex character.
func (s *appLinkScan) path(f *appLinkFilter, node *manifest.Node, v appLinkValue) {
	name := strings.TrimPrefix(v.attr.Name, "android:")
	if appLinkBlank(v.attr.Value) {
		s.errorAtAttr(v.attr, "`"+v.attr.Name+"` cannot be empty")
		return
	}
	if !v.known || appLinkSubstituted(v.value) {
		return
	}
	if !strings.HasPrefix(v.value, "/") && (name == "path" || name == "pathPrefix") {
		s.errorAtValue(v.attr, "`"+v.attr.Name+"` attribute should start with `/`, but it is `"+v.value+"`")
	}
	if !strings.HasPrefix(v.value, "/") && !strings.HasPrefix(v.value, ".*") && name == "pathPattern" {
		s.errorAtValue(v.attr, "`"+v.attr.Name+"` attribute should start with `/` or `.*`, but it is `"+v.value+"`")
	}
	if f.autoVerify == "true" && appLinkHasQueryOrFragment(v.value) {
		s.errorAtNode(node, "App link matching does not support query parameters or fragments, unless using "+
			"`<uri-relative-filter-group>` (introduced in Android 15)")
	}
	if strings.Contains(v.value, "?") && (name == "pathPattern" || name == "pathAdvancedPattern") {
		s.errorAtValue(v.attr, name+" does not support `?` as a Regex character")
	}
}

// appLinkHasQueryOrFragment reports whether a path value carries query parameters or a non-blank fragment.
func appLinkHasQueryOrFragment(path string) bool {
	query := ""
	if parts := strings.Split(path, "?"); len(parts) == 2 {
		query = parts[1]
	}
	query, _, _ = strings.Cut(query, "#")
	query = strings.ReplaceAll(query, "&amp;", "&")
	for _, param := range strings.Split(query, "&") {
		if !appLinkBlank(param) {
			return true
		}
	}
	fragment := ""
	if parts := strings.Split(path, "#"); len(parts) == 2 {
		fragment = parts[1]
	}
	fragment, _, _ = strings.Cut(fragment, "?")
	return !appLinkBlank(fragment)
}

func (s *appLinkScan) appLinkWarning(f *appLinkFilter) {
	schemes := f.valueSet("scheme")
	if len(schemes) == 0 || !f.allSchemes(appLinkWebScheme) || f.hostPortCount() == 0 {
		return
	}
	if !f.actions[appLinkActionView] || !f.categories[appLinkBrowsable] || !f.categories[appLinkDefault] {
		return
	}
	if f.autoVerify == "true" || f.autoVerify == "false" {
		return
	}
	s.diags = append(s.diags, appLinkDiag{warning: true, line: f.node.Line, col: f.node.Col + 1, msg: appLinkWarningMessage})
}

const appLinkWarningMessage = "This intent filter has the format of an Android App Link but is missing the `autoVerify` attribute; " +
	"add `android:autoVerify=\"true\"` to ensure your domain will be validated and enable App Link-related Lint warnings. " +
	"If you do not want clicked URLs to bring the user to your app, remove the `android.intent.category.BROWSABLE` category, " +
	"or set `android:autoVerify=\"false\"` to make it clear this is not intended to be an Android App Link."

// hasRequiredElements reports whether the filter has what an Android App Link needs.
func (f *appLinkFilter) hasRequiredElements() bool {
	return f.actions[appLinkActionView] && f.categories[appLinkDefault] && f.categories[appLinkBrowsable] &&
		f.anyScheme(f.substitutedOrWeb) && f.hostPortCount() > 0
}

// hasNoForbiddenElements reports whether the filter has no scheme or MIME type that stops it being an App Link.
func (f *appLinkFilter) hasNoForbiddenElements() bool {
	return f.allSchemes(f.substitutedOrWeb) && len(f.valueSet("mimeType")) == 0
}

// missingAppLinkParts lists what an autoVerify filter lacks to be an Android App Link, in Lint's order.
func (f *appLinkFilter) missingAppLinkParts() []string {
	var missing []string
	if !f.actions[appLinkActionView] {
		missing = append(missing, "VIEW action")
	}
	for _, category := range []string{appLinkBrowsable, appLinkDefault} {
		if !f.categories[category] {
			missing = append(missing, strings.TrimPrefix(category, appLinkCategoryPre)+" category")
		}
	}
	schemes := f.valueSet("scheme")
	if !f.anyScheme(f.substitutedOrWeb) && (len(schemes) > 0 || f.hostPortCount() == 0) {
		missing = append(missing, "`http(s)` scheme")
	}
	if f.hostPortCount() == 0 && !f.hasPortWithoutHost() {
		missing = append(missing, "`host` attribute")
	}
	return missing
}

func appLinkMissingMessage(missing []string) string {
	switch len(missing) {
	case 1:
		return missing[0] + " is missing, but is required for Android App Links"
	case 2:
		return missing[0] + " and " + missing[1] + " are missing, but are required for Android App Links"
	default:
		return "Several elements/attributes (such as " + missing[0] + ") required for Android App Links are missing"
	}
}

func (s *appLinkScan) invalidAppLink(f *appLinkFilter) {
	if f.autoVerify != "true" || (f.hasRequiredElements() && f.hasNoForbiddenElements()) {
		return
	}
	if missing := f.missingAppLinkParts(); len(missing) > 0 {
		s.diags = append(s.diags, appLinkDiag{line: f.node.Line, col: f.node.Col + 1, msg: appLinkMissingMessage(missing)})
	}
}

// requiredParts reports the scheme, host and BROWSABLE requirements of a VIEW filter with data tags.
func (s *appLinkScan) requiredParts(f *appLinkFilter) {
	if len(f.data) == 0 {
		return
	}
	first := f.data[0].node
	view := f.actions[appLinkActionView]
	noScheme := len(f.valueSet("scheme")) == 0
	paths := f.paths()
	hostPorts := f.hostPortCount()
	needsScheme := noScheme && (len(paths) > 0 || hostPorts > 0)
	if needsScheme {
		s.errorAtNode(first, "At least one `scheme` must be specified")
	}
	if !needsScheme && view && noScheme && len(f.valueSet("mimeType")) == 0 {
		s.errorAtNode(first, "VIEW actions require a URI")
	}
	if len(paths) > 0 && hostPorts == 0 {
		s.errorAtNode(first, "At least one `host` must be specified")
	}
	if view && f.anyScheme(appLinkWebScheme) && !f.categories[appLinkBrowsable] {
		s.errorAtNode(f.node, "Activity supporting ACTION_VIEW is not set as BROWSABLE")
	}
}

func (r *AppLinkURLErrorRule) check(ctx *api.Context) {
	for _, d := range analyzeAppLinks(ctx.Manifest.Root) {
		if !d.warning {
			emitAppLinkDiag(ctx, d)
		}
	}
}

func (r *AppLinkWarningRule) check(ctx *api.Context) {
	for _, d := range analyzeAppLinks(ctx.Manifest.Root) {
		if d.warning {
			emitAppLinkDiag(ctx, d)
		}
	}
}

func emitAppLinkDiag(ctx *api.Context, d appLinkDiag) {
	ctx.Emit(scanner.Finding{File: ctx.Manifest.Path, Line: d.line, Col: d.col, Message: d.msg})
}
