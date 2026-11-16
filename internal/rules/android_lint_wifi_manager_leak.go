package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	androidContextClass     = "android.content.Context"
	androidApplicationClass = "android.app.Application"
	// wifiLeakFixedInAPI is the first API level whose WifiManager no longer keeps the looked-up Context alive.
	wifiLeakFixedInAPI = 24
)

// wifiVerdict says how sure Lint is that a WIFI_SERVICE lookup leaks.
type wifiVerdict int

const (
	wifiNoLeak wifiVerdict = iota
	wifiPotentialLeak
	wifiLeak
)

// WifiManagerLeakRule mirrors the error half of Lint's WifiManagerLeak: a WIFI_SERVICE lookup on a Context that is
// certainly not the application context, below API 24.
type WifiManagerLeakRule struct {
	FlatDispatchBase
	AndroidRule
}

// WifiManagerPotentialLeakRule mirrors the warning half: the lookup receiver is a plain Context of unknown origin.
type WifiManagerPotentialLeakRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receivers are classified from declarations.
func (r *WifiManagerLeakRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence: receivers are classified from declarations.
func (r *WifiManagerPotentialLeakRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *WifiManagerLeakRule) check(ctx *api.Context)          { wifiLeakCheck(ctx, wifiLeak) }
func (r *WifiManagerPotentialLeakRule) check(ctx *api.Context) { wifiLeakCheck(ctx, wifiPotentialLeak) }

func wifiLeakCheck(ctx *api.Context, want wifiVerdict) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "getSystemService" || len(call.Args) != 1 || !wifiServiceArgument(file, call.Args[0]) {
		return
	}
	if lintFileMinSdk(ctx) >= wifiLeakFixedInAPI {
		return
	}
	verdict, shown := wifiReceiverVerdict(ctx, call)
	if verdict != want {
		return
	}
	var fix string
	if call.ReceiverNode == 0 {
		shown, fix = "getSystemService", "getApplicationContext().getSystemService"
	} else {
		fix = shown + ".getApplicationContext()"
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "The WIFI_SERVICE must be looked up on the Application context or memory will leak on devices < Android N. "+
		"Try changing `"+shown+"` to `"+fix+"`"))
}

// lintFileMinSdk is the module's minimum SDK from its build file or manifest, else the project-wide one, or 0 when unknown.
func lintFileMinSdk(ctx *api.Context) int {
	if ctx.File != nil {
		if levels := android.ResolveSDKLevels(ctx.File.Path); levels.MinSdk > 0 {
			return levels.MinSdk
		}
	}
	if facts := librarymodel.EnsureFacts(ctx.LibraryFacts); facts != nil {
		return facts.Profile.MinSdkVersion
	}
	return 0
}

// wifiServiceArgument reports whether the argument names Context.WIFI_SERVICE, not the string it holds.
func wifiServiceArgument(file *scanner.File, arg uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	owner, found := strings.CutSuffix(text, ".WIFI_SERVICE")
	if !found {
		return text == "WIFI_SERVICE"
	}
	return lintTypeRefMatches(file, owner, androidContextClass)
}

// wifiReceiverVerdict classifies the receiver of the lookup and returns the text Lint quotes for it.
func wifiReceiverVerdict(ctx *api.Context, call lintCall) (wifiVerdict, string) {
	file := ctx.File
	if call.ReceiverNode == 0 || lintIsSelfReceiver(call.ReceiverText) {
		return wifiImplicitReceiverVerdict(ctx, call), call.ReceiverText
	}
	return wifiExpressionVerdict(ctx, call.ReceiverNode, 0), strings.TrimSpace(file.FlatNodeText(call.ReceiverNode))
}

// wifiImplicitReceiverVerdict handles an unqualified or `this` lookup: it leaks inside any Context class except an
// Application. A Kotlin lambda that rebinds `this` to its receiver decides by that receiver instead.
func wifiImplicitReceiverVerdict(ctx *api.Context, call lintCall) wifiVerdict {
	file := ctx.File
	for cur, ok := file.FlatParent(call.Idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		owner, isCall := wifiLambdaOwner(file, cur)
		switch {
		case !isCall:
			return wifiNoLeak
		case wifiThisPreservingCalls[owner.Name]:
		case wifiThisRebindingCalls[owner.Name]:
			recv := owner.ReceiverNode
			if owner.Name == "with" && len(owner.Args) > 0 {
				recv = owner.Args[0]
			}
			if recv != 0 && file.FlatType(recv) == "simple_identifier" && wifiNameVerdict(ctx, recv, 0) == wifiLeak {
				return wifiLeak
			}
			return wifiNoLeak
		default:
			return wifiNoLeak
		}
	}
	for cls, ok := lintEnclosingClassLike(file, call.Idx); ok; cls, ok = lintEnclosingClassLike(file, cls) {
		if lintClassExtends(ctx, cls, androidApplicationClass, wifiApplicationNames) {
			return wifiNoLeak
		}
		if lintClassExtends(ctx, cls, androidContextClass, wifiContextNames) {
			return wifiLeak
		}
	}
	return wifiNoLeak
}

// wifiLambdaOwner returns the call a Kotlin lambda literal is passed to.
func wifiLambdaOwner(file *scanner.File, lambda uint32) (lintCall, bool) {
	cur, ok := file.FlatParent(lambda)
	for ok && file.FlatType(cur) != "call_expression" {
		switch file.FlatType(cur) {
		case "annotated_lambda", "call_suffix", "value_argument", "value_arguments":
			cur, ok = file.FlatParent(cur)
		default:
			return lintCall{}, false
		}
	}
	if !ok {
		return lintCall{}, false
	}
	// `with(x) { }` parses as a call of the call `with(x)`; the inner one names the function.
	if inner := file.FlatFirstChild(cur); file.FlatType(inner) == "call_expression" {
		cur = inner
	}
	return lintCallAt(file, cur)
}

var (
	// wifiThisPreservingCalls take lambdas that leave `this` bound to the enclosing class.
	wifiThisPreservingCalls = map[string]bool{
		"let": true, "also": true, "lazy": true, "forEach": true, "map": true, "runOnUiThread": true, "post": true,
		"postDelayed": true, "launch": true, "repeat": true, "synchronized": true, "use": true, "takeIf": true,
	}
	// wifiThisRebindingCalls take lambdas whose receiver becomes `this`.
	wifiThisRebindingCalls = map[string]bool{"run": true, "apply": true, "with": true}
)

var (
	wifiApplicationNames = map[string]struct{}{"Application": {}, "MultiDexApplication": {}}
	wifiContextNames     = lintContextNames()
	// wifiContextProperties are Kotlin synthetic properties whose getter returns a Context that may be an Activity.
	wifiContextProperties = map[string]bool{"context": true, "baseContext": true, "application": true, "activity": true}
)

func lintContextNames() map[string]struct{} {
	names := make(map[string]struct{}, len(contextSubclassNames))
	for name, ok := range contextSubclassNames {
		if ok && name != "Context" {
			names[name] = struct{}{}
		}
	}
	return names
}

// wifiExpressionVerdict classifies a receiver expression. Calls and Context-returning properties leak unless they
// return the application context. A name follows its initializer when it is a local and its declared type otherwise.
func wifiExpressionVerdict(ctx *api.Context, expr uint32, depth int) wifiVerdict {
	file := ctx.File
	switch file.FlatType(expr) {
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		if !ok || call.Name == "getApplicationContext" || file.FlatType(expr) == "call_expression" && call.Name == "applicationContext" {
			return wifiNoLeak
		}
		return wifiLeak
	case "navigation_expression":
		name := flatNavigationExpressionLastIdentifier(file, expr)
		if wifiContextProperties[name] {
			return wifiLeak
		}
		return wifiNoLeak
	case "simple_identifier", "identifier":
		return wifiNameVerdict(ctx, expr, depth)
	}
	return wifiNoLeak
}

func wifiNameVerdict(ctx *api.Context, ref uint32, depth int) wifiVerdict {
	file := ctx.File
	name := file.FlatNodeText(ref)
	if name == "it" && file.Language != scanner.LangJava {
		return wifiItVerdict(ctx, ref)
	}
	if init := lintInitializerOf(file, ref, name, false); init != 0 {
		if depth >= 3 {
			return wifiNoLeak
		}
		return wifiExpressionVerdict(ctx, init, depth+1)
	}
	if lintIsKotlinVar(file, name) {
		return wifiNoLeak
	}
	return wifiTypeVerdict(file, lintDeclaredTypeText(file, ref, name))
}

// wifiTypeVerdict classifies a declared receiver type: a plain Context may or may not be the application context,
// an Application is, and any other Context subclass is not.
func wifiTypeVerdict(file *scanner.File, typ string) wifiVerdict {
	if typ == "" {
		return wifiNoLeak
	}
	simple := typ[strings.LastIndex(typ, ".")+1:]
	_, isApplication := wifiApplicationNames[simple]
	_, isContext := wifiContextNames[simple]
	switch {
	case lintTypeRefMatches(file, typ, androidContextClass):
		return wifiPotentialLeak
	case isApplication || lintTypeRefMatches(file, typ, androidApplicationClass):
		return wifiNoLeak
	case isContext && lintAndroidTypeName(typ):
		return wifiLeak
	}
	return wifiNoLeak
}

// wifiItVerdict classifies the implicit `it` of a let or also lambda by the type of the call's receiver.
func wifiItVerdict(ctx *api.Context, ref uint32) wifiVerdict {
	file := ctx.File
	lambda, ok := flatEnclosingAncestor(file, ref, "lambda_literal")
	if !ok {
		return wifiNoLeak
	}
	owner, isCall := wifiLambdaOwner(file, lambda)
	if !isCall || owner.ReceiverNode == 0 || (owner.Name != "let" && owner.Name != "also") {
		return wifiNoLeak
	}
	recv := owner.ReceiverNode
	switch file.FlatType(recv) {
	case "simple_identifier":
		return wifiTypeVerdict(file, lintDeclaredTypeText(file, recv, file.FlatNodeText(recv)))
	case "navigation_expression":
		switch flatNavigationExpressionLastIdentifier(file, recv) {
		case "applicationContext", "baseContext", "context":
			return wifiPotentialLeak
		}
	}
	return wifiNoLeak
}

// lintIsKotlinVar reports whether the file declares a mutable Kotlin property or local called name.
func lintIsKotlinVar(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != name {
			return
		}
		if kind, ok := file.FlatFindChild(decl, "binding_pattern_kind"); ok && strings.TrimSpace(file.FlatNodeText(kind)) == "var" {
			found = true
		}
	})
	return found
}
