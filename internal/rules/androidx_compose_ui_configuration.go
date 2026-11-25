package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of LocalContextResourcesConfigurationReadDetector (LocalContextConfigurationRead,
// LocalContextGetResourceValueCall, LocalContextResourcesRead), ConfigurationScreenWidthHeightDetector and
// NonObservableLocaleDetector from androidx.compose.ui:ui-android 1.12.1.

// LocalContextConfigurationReadRule ports LocalContextConfigurationRead.
type LocalContextConfigurationReadRule struct {
	FlatDispatchBase
	BaseRule
}

// LocalContextGetResourceValueCallRule ports LocalContextGetResourceValueCall.
type LocalContextGetResourceValueCallRule struct {
	FlatDispatchBase
	BaseRule
}

// LocalContextResourcesReadRule ports LocalContextResourcesRead.
type LocalContextResourcesReadRule struct {
	FlatDispatchBase
	BaseRule
}

// ConfigurationScreenWidthHeightRule ports ConfigurationScreenWidthHeight. It joins source with the manifests so
// that, as in Lint, modules whose manifest declares the watch feature are skipped.
type ConfigurationScreenWidthHeightRule struct {
	BaseRule
}

// NonObservableLocaleRule ports NonObservableLocale.
type NonObservableLocaleRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LocalContextConfigurationReadRule) fileMentions() []string { return []string{"LocalContext"} }
func (r *LocalContextGetResourceValueCallRule) fileMentions() []string {
	return []string{"LocalContext"}
}
func (r *LocalContextResourcesReadRule) fileMentions() []string { return []string{"LocalContext"} }
func (r *NonObservableLocaleRule) fileMentions() []string       { return []string{"Locale"} }

var acuiLocalContextLimitations = []string{
	"A Context or Resources value is followed only through same-file vals and properties initialized to " +
		"LocalContext.current (or its resources).",
}

func (r *LocalContextConfigurationReadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LocalContextConfigurationRead", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acuiLocalContextLimitations}
}

func (r *LocalContextGetResourceValueCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LocalContextGetResourceValueCall", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acuiLocalContextLimitations}
}

func (r *LocalContextResourcesReadRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LocalContextResourcesRead", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acuiLocalContextLimitations}
}

func (r *ConfigurationScreenWidthHeightRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ConfigurationScreenWidthHeight", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The receiver must be shown to be a Configuration (LocalConfiguration.current, resources.configuration, " +
				"or a parameter or val declared or initialized as one).",
			"Runs only in projects with an AndroidManifest.xml, which it needs for Lint's Wear OS exemption.",
		}}
}

func (r *NonObservableLocaleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NonObservableLocale", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Composition is decided by the shared Compose runtime walk, which sees through same-file inline functions " +
				"and let/run/apply/also/with/takeIf/takeUnless/repeat but not other stdlib inline lambdas such as forEach.",
		}}
}

func registerAndroidxComposeUIConfigurationRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	config := &LocalContextConfigurationReadRule{BaseRule: BaseRule{RuleName: "LocalContextConfigurationRead",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects LocalContext.current.resources.configuration reads, which are not updated when the Configuration changes."}}
	registerAcuiRule(config.BaseRule, config, []string{"navigation_expression"}, kotlin, api.FixNone, config.check)

	value := &LocalContextGetResourceValueCallRule{BaseRule: BaseRule{RuleName: "LocalContextGetResourceValueCall",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects LocalContext.current.getString/getColor/getDrawable/... calls, which are not updated when the Configuration changes."}}
	registerAcuiRule(value.BaseRule, value, []string{"call_expression"}, kotlin, api.FixNone, value.check)

	res := &LocalContextResourcesReadRule{BaseRule: BaseRule{RuleName: "LocalContextResourcesRead", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects LocalContext.current.resources reads, which are not updated when the Configuration changes."}}
	registerAcuiRule(res.BaseRule, res, []string{"navigation_expression"}, kotlin, api.FixNone, res.check)

	screen := &ConfigurationScreenWidthHeightRule{BaseRule: BaseRule{RuleName: "ConfigurationScreenWidthHeight",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects Configuration.screenWidthDp/screenHeightDp read in composition instead of LocalWindowInfo.current.containerSize."}}
	api.Register(&api.Rule{
		ID: screen.RuleName, Category: screen.RuleSetName, Description: screen.Desc, Sev: api.Severity(screen.Sev),
		Needs: api.NeedsManifest | api.NeedsParsedFiles, AndroidDeps: uint32(AndroidDepManifest),
		Languages: kotlin, Confidence: api.ConfidenceHigh, Implementation: screen, Check: screen.check, LintIssue: true,
	})

	locale := &NonObservableLocaleRule{BaseRule: BaseRule{RuleName: "NonObservableLocale", RuleSetName: "compose",
		Sev: "error", Desc: "Detects Locale.getDefault()/LocaleList.getAdjustedDefault() in composition, which does not observe locale changes."}}
	registerAcuiRule(locale.BaseRule, locale, []string{"call_expression"}, kotlin, api.FixNone, locale.check)
}

// acuiCompact returns node's source text without whitespace.
func acuiCompact(file *scanner.File, node uint32) string {
	return strings.Join(strings.Fields(file.FlatNodeText(node)), "")
}

// acuiNavigationParts splits a navigation_expression into its receiver expression and suffix identifier name.
func acuiNavigationParts(file *scanner.File, nav uint32) (receiver uint32, name string) {
	if file.FlatType(nav) != "navigation_expression" {
		return 0, ""
	}
	receiver = file.FlatFirstChild(nav)
	suffix, ok := file.FlatFindChild(nav, "navigation_suffix")
	if !ok {
		return receiver, ""
	}
	return receiver, file.FlatChildTextOrEmpty(suffix, "simple_identifier")
}

// acuiIsCompositionLocalCurrent reports whether expr is `<local>.current` for androidx.compose.ui.platform.<local>,
// written with a bound simple name or fully qualified.
func acuiIsCompositionLocalCurrent(file *scanner.File, expr uint32, local string) bool {
	expr = acuiSkipParens(file, expr)
	receiver, name := acuiNavigationParts(file, expr)
	if name != "current" || receiver == 0 {
		return false
	}
	text := acuiCompact(file, receiver)
	return text == acuiPlatformPackage+"."+local || acrSimpleNameRefersTo(file, text, acuiPlatformPackage, local)
}

// acuiResolveInitializer mirrors tryResolveUDeclaration + getUastInitializer for a simple name: the initializer of
// the val or var (local or member) it refers to, or 0.
func acuiResolveInitializer(file *scanner.File, expr uint32) uint32 {
	expr = acuiSkipParens(file, expr)
	if file.FlatType(expr) != "simple_identifier" {
		return 0
	}
	return acuiSkipParens(file, lintInitializerOf(file, expr, file.FlatNodeText(expr), true))
}

// acuiIsLocalContextExpr reports whether expr is LocalContext.current, or a variable initialized to it.
func acuiIsLocalContextExpr(file *scanner.File, expr uint32) bool {
	if acuiIsCompositionLocalCurrent(file, expr, "LocalContext") {
		return true
	}
	init := acuiResolveInitializer(file, expr)
	return init != 0 && acuiIsCompositionLocalCurrent(file, init, "LocalContext")
}

// acuiConfigurationContextExpr mirrors findContextExpressionFromResourcesConfigurationExpression: for the receiver
// of `.configuration`, the Context expression whose resources it reads: `ctx.resources`, or a variable initialized
// to `ctx.resources`. It returns 0 when ctx is not LocalContext.current (or a variable holding it).
func acuiConfigurationContextExpr(file *scanner.File, resources uint32) uint32 {
	resources = acuiSkipParens(file, resources)
	if file.FlatType(resources) == "simple_identifier" {
		resources = acuiResolveInitializer(file, resources)
	}
	receiver, name := acuiNavigationParts(file, resources)
	if name != "resources" || receiver == 0 || !acuiIsLocalContextExpr(file, receiver) {
		return 0
	}
	return receiver
}

// acuiIsLocalContextConfigurationRead reports whether a navigation expression is a Configuration read from
// LocalContext.current's resources, directly or through variables.
func acuiIsLocalContextConfigurationRead(file *scanner.File, nav uint32) bool {
	receiver, name := acuiNavigationParts(file, nav)
	return name == "configuration" && acuiConfigurationContextExpr(file, receiver) != 0
}

func (r *LocalContextConfigurationReadRule) check(ctx *api.Context) {
	if acuiIsLocalContextConfigurationRead(ctx.File, ctx.Idx) {
		ctx.EmitAtNode(ctx.Idx, "Reading Configuration using LocalContext.current.resources.configuration is not "+
			"configuration-aware and may return stale values if the Configuration changes")
	}
}

// check mirrors the resources branch and afterCheckFile: `LocalContext.current.resources` (or `ctx.resources` for a
// variable holding it) is reported unless it is itself part of a Configuration read, directly
// (`….resources.configuration`) or as the initializer of a variable whose `.configuration` is read.
func (r *LocalContextResourcesReadRule) check(ctx *api.Context) {
	file, nav := ctx.File, ctx.Idx
	receiver, name := acuiNavigationParts(file, nav)
	if name != "resources" || receiver == 0 || !acuiIsLocalContextExpr(file, receiver) {
		return
	}
	if parent, ok := file.FlatParent(nav); ok {
		if pr, pname := acuiNavigationParts(file, parent); pr == nav && pname == "configuration" {
			return
		}
	}
	if acuiInitializerReadForConfiguration(file, nav) {
		return
	}
	ctx.EmitAtNode(nav, "Reading Resources using LocalContext.current.resources is not configuration-aware and may "+
		"return stale values if the Configuration changes")
}

// acuiInitializerReadForConfiguration reports whether expr initializes a property whose `.configuration` is read
// in the same file.
func acuiInitializerReadForConfiguration(file *scanner.File, expr uint32) bool {
	decl, ok := file.FlatParent(expr)
	if !ok || file.FlatType(decl) != "property_declaration" || lintInitializerAfterEquals(file, decl) != expr {
		return false
	}
	vd, _ := file.FlatFindChild(decl, "variable_declaration")
	name := file.FlatChildTextOrEmpty(vd, "simple_identifier")
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "navigation_expression", func(n uint32) {
		if found {
			return
		}
		receiver, suffix := acuiNavigationParts(file, n)
		if suffix == "configuration" && file.FlatType(receiver) == "simple_identifier" && file.FlatNodeTextEquals(receiver, name) &&
			acuiResolveInitializer(file, receiver) == expr {
			found = true
		}
	})
	return found
}

// acuiContextResourceValueMethods are the Context methods LocalContextGetResourceValueCall reports.
var acuiContextResourceValueMethods = map[string]bool{
	"getText": true, "getString": true, "getColor": true, "getDrawable": true, "getColorStateList": true,
}

func (r *LocalContextGetResourceValueCallRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, _, qualified, ok := acuiCallee(file, call)
	if !ok || !qualified || !acuiContextResourceValueMethods[name] {
		return
	}
	callee := acrCalleeExpression(file, call)
	receiver, _ := acuiNavigationParts(file, callee)
	if receiver == 0 || !acuiIsLocalContextExpr(file, receiver) {
		return
	}
	ctx.EmitAtNode(call, "Querying resource values using LocalContext.current is not configuration-aware and may "+
		"return stale values if the Configuration changes")
}

// ---------------------------------------------------------------------------
// ConfigurationScreenWidthHeight
// ---------------------------------------------------------------------------

const acuiWatchFeature = "android.hardware.type.watch"

// check mirrors ConfigurationScreenWidthHeightDetector over every parsed Kotlin file. Lint's filterIncident drops
// findings when the module's merged manifest declares the watch feature; krit skips files under the module of a
// manifest that declares it. report() also drops files importing anything from androidx.wear.
func (r *ConfigurationScreenWidthHeightRule) check(ctx *api.Context) {
	var watchRoots []string
	for _, m := range ctx.Manifests {
		if m == nil {
			continue
		}
		for _, f := range m.UsesFeatures {
			if f.Name == acuiWatchFeature {
				watchRoots = append(watchRoots, acuiManifestModuleRoot(m.Path))
				break
			}
		}
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.Language != scanner.LangKotlin || file.FlatTree == nil {
			continue
		}
		if !strings.Contains(string(file.Content), "screenWidthDp") && !strings.Contains(string(file.Content), "screenHeightDp") {
			continue
		}
		if acuiUnderAnyRoot(file.Path, watchRoots) || acuiImportsWear(file) {
			continue
		}
		file.FlatWalkNodes(0, "navigation_expression", func(nav uint32) {
			receiver, name := acuiNavigationParts(file, nav)
			if (name != "screenWidthDp" && name != "screenHeightDp") || receiver == 0 || !acuiIsConfigurationExpr(file, receiver, 0) {
				return
			}
			init := acuiResolveInitializer(file, receiver)
			if !acrInComposition(file, nav) && (init == 0 || !acuiIsCompositionLocalCurrent(file, init, "LocalConfiguration")) {
				return
			}
			ctx.Emit(scanner.Finding{File: file.Path, Line: file.FlatRow(nav) + 1, Col: file.FlatCol(nav) + 1,
				StartByte: int(file.FlatStartByte(nav)), EndByte: int(file.FlatEndByte(nav)),
				Message: fmt.Sprintf("Using Configuration.%s instead of LocalWindowInfo.current.containerSize", name)})
		})
	}
}

// acuiManifestModuleRoot returns the module directory of a manifest: the directory above `src/<set>/`, or the
// manifest's own directory.
func acuiManifestModuleRoot(path string) string {
	dir := filepath.Dir(path)
	if parent := filepath.Dir(dir); filepath.Base(parent) == "src" {
		return filepath.Dir(parent)
	}
	return dir
}

func acuiUnderAnyRoot(path string, roots []string) bool {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	for _, root := range roots {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// acuiImportsWear mirrors report(): any import whose text contains androidx.wear.
func acuiImportsWear(file *scanner.File) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(n uint32) {
		if strings.Contains(file.FlatNodeText(n), "androidx.wear") {
			found = true
		}
	})
	return found
}

// acuiIsConfigurationExpr reports whether expr is an android.content.res.Configuration the source shows:
// LocalConfiguration.current, `….configuration` read from resources, or a name declared or initialized as one.
func acuiIsConfigurationExpr(file *scanner.File, expr uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	expr = acuiSkipParens(file, expr)
	if acuiIsCompositionLocalCurrent(file, expr, "LocalConfiguration") {
		return true
	}
	switch file.FlatType(expr) {
	case "navigation_expression":
		receiver, name := acuiNavigationParts(file, expr)
		if name != "configuration" {
			return false
		}
		_, recvName := acuiNavigationParts(file, acuiSkipParens(file, receiver))
		return recvName == "resources" || (file.FlatType(receiver) == "simple_identifier" && file.FlatNodeTextEquals(receiver, "resources"))
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		if t := acuiDeclaredTypeOf(file, expr, name); t != "" {
			return t == "android.content.res.Configuration" || acrSimpleNameRefersTo(file, t, "android.content.res", "Configuration")
		}
		if init := acuiResolveInitializer(file, expr); init != 0 {
			return acuiIsConfigurationExpr(file, init, depth+1)
		}
	}
	return false
}

// acuiDeclaredTypeOf returns the declared type text (nullability dropped) of the parameter or property a simple
// name refers to: the nearest enclosing function parameter, or a property declared before the use.
func acuiDeclaredTypeOf(file *scanner.File, use uint32, name string) string {
	for p, ok := file.FlatParent(use); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		params, ok := file.FlatFindChild(p, "function_value_parameters")
		if !ok {
			continue
		}
		for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
			if file.FlatType(param) == "parameter" && file.FlatChildTextOrEmpty(param, "simple_identifier") == name {
				return strings.TrimSuffix(acuiCompact(file, acuiTypeChild(file, param)), "?")
			}
		}
	}
	init := lintInitializerOf(file, use, name, true)
	if init == 0 {
		return ""
	}
	decl, ok := file.FlatParent(init)
	if !ok || file.FlatType(decl) != "property_declaration" {
		return ""
	}
	vd, _ := file.FlatFindChild(decl, "variable_declaration")
	if t := acuiTypeChild(file, vd); t != 0 {
		return strings.TrimSuffix(acuiCompact(file, t), "?")
	}
	return ""
}

// ---------------------------------------------------------------------------
// NonObservableLocale
// ---------------------------------------------------------------------------

// acuiNonObservableLocaleCalls maps a method to the classes (package, simple name) whose static method Lint reports.
var acuiNonObservableLocaleCalls = map[string][][2]string{
	"getDefault":         {{"java.util", "Locale"}},
	"getAdjustedDefault": {{"android.os", "LocaleList"}, {"androidx.core.os", "LocaleListCompat"}},
}

func (r *NonObservableLocaleRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, _, qualified, ok := acuiCallee(file, call)
	owners := acuiNonObservableLocaleCalls[name]
	if !ok || !qualified || len(owners) == 0 {
		return
	}
	receiver, _ := acuiNavigationParts(file, acrCalleeExpression(file, call))
	if receiver == 0 {
		return
	}
	text := acuiCompact(file, receiver)
	matched := false
	for _, owner := range owners {
		if text == owner[0]+"."+owner[1] || acrSimpleNameRefersTo(file, text, owner[0], owner[1]) {
			matched = true
		}
	}
	if !matched || !acrInComposition(file, call) {
		return
	}
	ctx.EmitAtNode(call, "Reading locale in a non-observable way in a composable function")
}
