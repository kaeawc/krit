package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of RenderProcessGoneDetector and CrossOriginIsolatedAllowlistDetector from androidx.webkit:webkit 1.18.0,
// and AccessibilityNodeInfoGetTextDetector from androidx.test.uiautomator:uiautomator 2.4.1.

const (
	axlRenderProcessGoneMessage = "WebViewClient should implement `onRenderProcessGone` to handle render process " +
		"crashes or out-of-memory errors. Otherwise, the app will crash when the render process is shut down."
	axlAllowlistMessage = "Calling setCrossOriginIsolatedAllowlist enables potentially dangerous cross-origin " +
		"isolated APIs (such as SharedArrayBuffer). Ensure only trusted origins are added to the allowlist."
	axlNodeTextMessage = "Do not use AccessibilityNodeInfo#getText in a UiAutomatorTestScope."
)

// MissingOnRenderProcessGoneRule ports MissingOnRenderProcessGone: a WebViewClient that does not handle
// onRenderProcessGone, so a render-process crash takes the app down.
type MissingOnRenderProcessGoneRule struct {
	FlatDispatchBase
	BaseRule
}

// CrossOriginIsolatedAllowlistRule ports CrossOriginIsolatedAllowlist: Profile.setCrossOriginIsolatedAllowlist,
// which enables cross-origin isolated APIs for the listed origins.
type CrossOriginIsolatedAllowlistRule struct {
	FlatDispatchBase
	BaseRule
}

// DontUseAccessibilityNodeInfoGetTextRule ports DontUseAccessibilityNodeInfoGetText: AccessibilityNodeInfo.text in a
// UiAutomator element predicate, where the CharSequence (possibly a Spannable) never equals a String.
type DontUseAccessibilityNodeInfoGetTextRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MissingOnRenderProcessGoneRule) fileMentions() []string { return []string{"WebViewClient"} }
func (r *CrossOriginIsolatedAllowlistRule) fileMentions() []string {
	return []string{"rossOriginIsolatedAllowlist"}
}
func (r *DontUseAccessibilityNodeInfoGetTextRule) fileMentions() []string {
	return []string{"androidx.test.uiautomator"}
}

func (r *MissingOnRenderProcessGoneRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MissingOnRenderProcessGone", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, whose check ships in the androidx.webkit:webkit AAR, it runs only when the project declares " +
				"that dependency.",
			"An onRenderProcessGone inherited from a project superclass is found through same-file classes only; a " +
				"class whose superclass is declared in another file is not reported.",
		}}
}

func (r *CrossOriginIsolatedAllowlistRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CrossOriginIsolatedAllowlist", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, whose check ships in the androidx.webkit:webkit AAR, it runs only when the project declares " +
				"that dependency.",
			"The receiver must be proven to be an androidx.webkit.Profile by its declaration, a with(profile) block, " +
				"or a ProfileStore getOrCreateProfile/getProfile call.",
		}}
}

func (r *DontUseAccessibilityNodeInfoGetTextRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DontUseAccessibilityNodeInfoGetText", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, whose check ships in the androidx.test.uiautomator:uiautomator AAR, it runs only when the " +
				"project declares that dependency.",
			"The detector matches onView, onViews and onViewOrNull, which the released uiautomator 2.4 API renamed " +
				"to onElement, onElements and onElementOrNull; krit checks both sets of names, so it also reports what " +
				"the shipped detector no longer can.",
			"As in Lint (JAVA_FILE scope), test sources are not checked.",
		}}
}

// axlWebViewClientClasses are the WebViewClient base classes the detector applies to.
var axlWebViewClientClasses = map[string]string{
	"android.webkit.WebViewClient": "WebViewClient", "androidx.webkit.WebViewClientCompat": "WebViewClientCompat",
}

var axlWebViewClientKnown = map[string]struct{}{"WebViewClient": {}, "WebViewClientCompat": {}}

// check mirrors RenderProcessGoneDetector: a concrete class or object literal extending WebViewClient (or
// WebViewClientCompat) with no two-parameter onRenderProcessGone in itself or a project superclass, at its name;
// and a direct WebViewClient() / WebViewClientCompat() construction, at the call.
func (r *MissingOnRenderProcessGoneRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	switch file.FlatType(node) {
	case "class_declaration", "object_literal":
		r.checkClass(ctx, node)
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, node) {
			return
		}
		for fqn, simple := range axlWebViewClientClasses {
			pkg := fqn[:strings.LastIndex(fqn, ".")]
			if axlWebkitCallTargetsClass(file, node, pkg, simple) {
				ctx.EmitAtNode(node, axlRenderProcessGoneMessage)
				return
			}
		}
	case "object_creation_expression":
		if body, ok := file.FlatFindChild(node, "class_body"); ok {
			r.checkJavaAnonymous(ctx, node, body)
			return
		}
		if axlJavaNewNamesWebViewClient(file, node) {
			ctx.EmitAtNode(node, axlRenderProcessGoneMessage)
		}
	}
}

func (r *MissingOnRenderProcessGoneRule) checkClass(ctx *api.Context, cls uint32) {
	file := ctx.File
	if file.FlatType(cls) == "class_declaration" {
		if axlClassIsAbstractOrNotClass(file, cls) {
			return
		}
		if fqn := axlDeclaredFQN(file, cls); axlWebViewClientClasses[fqn] != "" {
			return
		}
	}
	if !axlExtendsWebViewClient(ctx, cls) || axlHandlesRenderProcessGone(ctx, cls, 0) {
		return
	}
	at := cls
	if file.FlatType(cls) == "class_declaration" {
		if n := lintClassNameNode(file, cls); n != 0 {
			at = n
		}
	}
	ctx.EmitAtNode(at, axlRenderProcessGoneMessage)
}

// checkJavaAnonymous handles `new WebViewClient() { ... }`, reported at the type name like Lint's anonymous class.
func (r *MissingOnRenderProcessGoneRule) checkJavaAnonymous(ctx *api.Context, node, body uint32) {
	file := ctx.File
	if !axlJavaNewNamesWebViewClient(file, node) || axlBodyHandlesRenderProcessGone(file, body) {
		return
	}
	at := node
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			at = child
		}
	}
	ctx.EmitAtNode(at, axlRenderProcessGoneMessage)
}

// axlJavaNewNamesWebViewClient reports whether a Java `new T(...)` constructs WebViewClient or WebViewClientCompat.
func axlJavaNewNamesWebViewClient(file *scanner.File, node uint32) bool {
	c, ok := lintCallAt(file, node)
	if !ok {
		return false
	}
	for fqn, simple := range axlWebViewClientClasses {
		pkg := fqn[:strings.LastIndex(fqn, ".")]
		if c.TypeText == fqn || (c.TypeText == simple && acrJavaImports(file, pkg, simple)) {
			return true
		}
	}
	return false
}

// axlClassIsAbstractOrNotClass reports interfaces, enums, annotation classes and abstract or sealed classes (sealed
// classes are abstract in UAST), which the detector skips.
func axlClassIsAbstractOrNotClass(file *scanner.File, cls uint32) bool {
	if file.Language == scanner.LangJava {
		mods, _ := file.FlatFindChild(cls, "modifiers")
		if mods == 0 {
			return false
		}
		for _, mod := range strings.Fields(file.FlatNodeText(mods)) {
			if mod == "abstract" || mod == "sealed" {
				return true
			}
		}
		return false
	}
	for child := file.FlatFirstChild(cls); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "interface", "enum":
			return true
		case "type_identifier", "simple_identifier":
			return axlClassIsAbstractOrSealed(file, cls) || acrFunctionHasModifier(file, cls, "annotation")
		}
	}
	return axlClassIsAbstractOrSealed(file, cls)
}

func axlClassIsAbstractOrSealed(file *scanner.File, cls uint32) bool {
	return acrFunctionHasModifier(file, cls, "abstract") || acrFunctionHasModifier(file, cls, "sealed")
}

// axlDeclaredFQN returns the package-qualified name of a top-level class declaration.
func axlDeclaredFQN(file *scanner.File, cls uint32) string {
	n := lintClassNameNode(file, cls)
	if n == 0 {
		return ""
	}
	if pkg := filePackageName(file); pkg != "" {
		return pkg + "." + file.FlatNodeText(n)
	}
	return file.FlatNodeText(n)
}

// axlExtendsWebViewClient reports whether a class or object literal extends WebViewClient or WebViewClientCompat.
func axlExtendsWebViewClient(ctx *api.Context, cls uint32) bool {
	for fqn := range axlWebViewClientClasses {
		if lintClassExtends(ctx, cls, fqn, axlWebViewClientKnown) {
			return true
		}
	}
	return false
}

// axlHandlesRenderProcessGone reports whether the class declares a two-parameter onRenderProcessGone, or inherits one
// from a same-file superclass other than WebViewClient/WebViewClientCompat. A superclass declared in another file
// counts as handling it, so the rule abstains: reading it would make the verdict depend on another file's contents,
// which the per-file findings cache does not key on (#1778).
func axlHandlesRenderProcessGone(ctx *api.Context, cls uint32, depth int) bool {
	file := ctx.File
	if body := axlClassBody(file, cls); body != 0 && axlBodyHandlesRenderProcessGone(file, body) {
		return true
	}
	if depth > 6 {
		return true
	}
	refs := lintDirectSupertypeRefs(file, cls)
	for _, ref := range refs {
		if _, known := axlWebViewClientKnown[ref.simple]; known {
			// WebViewClient is the superclass, so the other supertypes are interfaces, which cannot override it.
			return false
		}
	}
	for _, ref := range refs {
		if local := lintSameFileClass(file, ref.simple); local != 0 && !ref.qualified {
			if axlHandlesRenderProcessGone(ctx, local, depth+1) {
				return true
			}
			continue
		}
		return true
	}
	return false
}

// axlClassBody returns the class_body of a Kotlin class or object literal, or of a Java class.
func axlClassBody(file *scanner.File, cls uint32) uint32 {
	if body, ok := file.FlatFindChild(cls, "class_body"); ok {
		return body
	}
	return 0
}

// axlBodyHandlesRenderProcessGone reports whether a class body declares onRenderProcessGone with two parameters.
func axlBodyHandlesRenderProcessGone(file *scanner.File, body uint32) bool {
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			if acrDeclarationName(file, child) == "onRenderProcessGone" && len(acrFunctionParameters(file, child)) == 2 {
				return true
			}
		case "method_declaration":
			if file.FlatChildTextOrEmpty(child, "identifier") != "onRenderProcessGone" {
				continue
			}
			if params, ok := file.FlatFindChild(child, "formal_parameters"); ok &&
				len(flatDirectChildrenOfType(file, params, "formal_parameter")) == 2 {
				return true
			}
		}
	}
	return false
}

// check mirrors CrossOriginIsolatedAllowlistDetector: setCrossOriginIsolatedAllowlist (or the Kotlin property
// assignment that calls it) on an androidx.webkit.Profile, at the call.
func (r *CrossOriginIsolatedAllowlistRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	switch file.FlatType(node) {
	case "assignment":
		target := file.FlatFirstChild(node)
		if target == 0 || axlWebkitRightmostName(file, target) != "crossOriginIsolatedAllowlist" {
			return
		}
		first := file.FlatFirstChild(target)
		if first == 0 || first == target {
			return
		}
		if file.FlatType(first) == "simple_identifier" && file.FlatNextSib(first) != 0 &&
			axlExprIsWebkitProfile(file, first, 0) {
			ctx.EmitAtNode(target, axlAllowlistMessage)
		}
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, node) {
			return
		}
		name, nameNode, hasReceiver := acrCallCallee(file, node)
		if name != "setCrossOriginIsolatedAllowlist" {
			return
		}
		if hasReceiver {
			if axlExprIsWebkitProfile(file, file.FlatFirstChild(acrCalleeExpression(file, node)), 0) {
				ctx.EmitAtNode(node, axlAllowlistMessage)
			}
			return
		}
		if axlImplicitWebkitProfile(file, node) {
			ctx.EmitAtNode(nameNode, axlAllowlistMessage)
		}
	case "method_invocation":
		c, ok := lintCallAt(file, node)
		if !ok || c.Name != "setCrossOriginIsolatedAllowlist" || c.ReceiverNode == 0 {
			return
		}
		if decl := lintDeclaredTypeText(file, node, c.ReceiverText); decl != "" &&
			(decl == "androidx.webkit.Profile" || (decl == "Profile" && acrJavaImports(file, "androidx.webkit", "Profile"))) {
			ctx.EmitAtNode(node, axlAllowlistMessage)
		}
	}
}

// axlExprIsWebkitProfile proves a Kotlin expression is an androidx.webkit.Profile.
func axlExprIsWebkitProfile(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return false
		}
		if t := acrDeclaredType(file, decl); t != 0 {
			return acrTypeIs(file, t, "androidx.webkit", "Profile")
		}
		return axlExprIsWebkitProfile(file, acrInitializer(file, decl), depth+1)
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		return hasReceiver && (name == "getOrCreateProfile" || name == "getProfile") &&
			acuiImportsPackage(file, "androidx.webkit")
	}
	return false
}

// axlImplicitWebkitProfile reports whether the implicit receiver at node is a Profile from with(profile) { } or
// profile.apply/run { }.
func axlImplicitWebkitProfile(file *scanner.File, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "lambda_literal":
			arg, ok := acrLambdaArgumentOf(file, p)
			if !ok {
				continue
			}
			name, _, hasReceiver := acrCallCallee(file, arg.Call)
			switch {
			case name == "with" && !hasReceiver:
				return axlExprIsWebkitProfile(file, acrCallValueArgument(file, arg.Call, 0, "receiver"), 0)
			case (name == "apply" || name == "run") && hasReceiver:
				return axlExprIsWebkitProfile(file, file.FlatFirstChild(acrCalleeExpression(file, arg.Call)), 0)
			}
		case "function_declaration", "class_body", "source_file":
			return false
		}
	}
	return false
}

// axlNodePredicateFunctions are the UiAutomatorTestScope functions whose trailing lambda tests an
// AccessibilityNodeInfo: the detector's names and their released 2.4 names.
var axlNodePredicateFunctions = map[string]bool{
	"onView": true, "onViews": true, "onViewOrNull": true,
	"onElement": true, "onElements": true, "onElementOrNull": true,
}

// check mirrors AccessibilityNodeInfoGetTextDetector on the trailing lambda of onView/onElement and the like: `text`
// read on the lambda's AccessibilityNodeInfo receiver (implicitly or through `this`), or on a value declared as an
// AccessibilityNodeInfo, at `text`.
func (r *DontUseAccessibilityNodeInfoGetTextRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok || !arg.Trailing {
		return
	}
	name, _, _ := acrCallCallee(file, arg.Call)
	if !axlNodePredicateFunctions[name] || len(acrFunctionDecls(file, name)) > 0 {
		return
	}
	file.FlatWalkAllNodes(lambda, func(n uint32) {
		if file.FlatType(n) != "simple_identifier" || !file.FlatNodeTextEquals(n, "text") {
			return
		}
		if axlNodeTextAccess(file, lambda, n) {
			ctx.EmitAtNode(n, axlNodeTextMessage)
		}
	})
}

// axlNodeTextAccess reports whether the `text` name n inside the predicate lambda reads AccessibilityNodeInfo.text.
func axlNodeTextAccess(file *scanner.File, lambda, n uint32) bool {
	parent, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "variable_declaration", "parameter", "class_parameter":
		return false // a declared name, not a read
	case "value_argument":
		if file.FlatFirstChild(parent) == n && file.FlatType(file.FlatNextSib(n)) == "=" {
			return false // the label of a named argument, matches(text = "...")
		}
	case "navigation_suffix":
		nav, ok := file.FlatParent(parent)
		if !ok || axlIsCallee(file, nav) {
			return false
		}
		receiver := file.FlatFirstChild(nav)
		if file.FlatType(receiver) == "this_expression" && file.FlatNodeTextEquals(receiver, "this") {
			return !axlNodeReceiverLambdaBetween(file, n, lambda)
		}
		decl := acrExprDeclaration(file, receiver)
		if decl == 0 {
			return false
		}
		t := acrDeclaredType(file, decl)
		return t != 0 && acrTypeIs(file, t, "android.view.accessibility", "AccessibilityNodeInfo")
	case "call_expression":
		if file.FlatFirstChild(parent) == n {
			return false // text("...") is a call of a function named text
		}
	}
	if acrValueDeclaration(file, n, "text") != 0 || axlNodeReceiverLambdaBetween(file, n, lambda) {
		return false
	}
	return true
}

// axlIsCallee reports whether expr is the callee of a call (`a.text()`).
func axlIsCallee(file *scanner.File, expr uint32) bool {
	p, ok := file.FlatParent(expr)
	return ok && file.FlatType(p) == "call_expression" && file.FlatFirstChild(p) == expr
}

// axlNodeReceiverLambdaBetween reports whether a with/apply/run lambda (which brings its own implicit receiver)
// encloses node inside the predicate lambda root.
func axlNodeReceiverLambdaBetween(file *scanner.File, node, root uint32) bool {
	for p, ok := file.FlatParent(node); ok && p != root; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "lambda_literal" {
			continue
		}
		if arg, ok := acrLambdaArgumentOf(file, p); ok {
			if name, _, _ := acrCallCallee(file, arg.Call); name == "with" || name == "apply" || name == "run" {
				return true
			}
		}
	}
	return false
}

// axlWebkitCallTargetsClass reports whether a Kotlin call's callee names pkg.name: directly, fully qualified, or
// through an import alias.
func axlWebkitCallTargetsClass(file *scanner.File, call uint32, pkg, name string) bool {
	if acrCallTargets(file, call, pkg, name) {
		return true
	}
	callee := acrCalleeExpression(file, call)
	return file.FlatType(callee) == "simple_identifier" &&
		acrSimpleNameRefersTo(file, file.FlatNodeText(callee), pkg, name) && !acrFileDeclaresTopLevel(file, name)
}

// axlWebkitRightmostName returns the last identifier of a name or navigation expression.
func axlWebkitRightmostName(file *scanner.File, expr uint32) string {
	var last string
	file.FlatWalkAllNodes(expr, func(n uint32) {
		if file.FlatType(n) == "simple_identifier" {
			last = file.FlatNodeText(n)
		}
	})
	return last
}
