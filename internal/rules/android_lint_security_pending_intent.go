package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// MutableImplicitPendingIntentRule mirrors Lint's MutableImplicitPendingIntent (PendingIntentMutableImplicitDetector):
// a PendingIntent created with FLAG_MUTABLE around an implicit Intent.
type MutableImplicitPendingIntentRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the intent is classified from its construction and the
// calls made on it in the same function, and anything else abstains.
func (r *MutableImplicitPendingIntentRule) Confidence() float64 { return api.ConfidenceMedium }

const (
	pendingIntentFlagMutable           = 1 << 25
	pendingIntentFlagNoCreate          = 1 << 29
	pendingIntentFlagAllowUnsafeImpl   = 1 << 24
	mutableImplicitPendingIntentBase   = "Mutable implicit `PendingIntent` will throw an exception"
	mutableImplicitPendingIntentAdvice = ", follow either of these recommendations: for an existing `PendingIntent` use " +
		"`FLAG_NO_CREATE` and for a new `PendingIntent` either make it immutable or make the `Intent` within explicit"
)

func (r *MutableImplicitPendingIntentRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	switch call.Name {
	case "getActivity", "getActivities", "getBroadcast", "getService":
	default:
		return
	}
	if !lintTypeRefMatches(file, call.ReceiverText, pendingIntentClass) {
		return
	}
	flagsArg := call.Arg(3, "flags")
	intentArg := call.Arg(2, "intent")
	if call.Name == "getActivities" {
		intentArg = call.Arg(2, "intents")
	}
	if flagsArg == 0 || intentArg == 0 {
		return
	}
	flags, ok := lintEvalInt(file, flagsArg, pendingIntentConsts)
	if !ok || flags&pendingIntentFlagMutable == 0 || flags&pendingIntentFlagNoCreate != 0 ||
		flags&pendingIntentFlagAllowUnsafeImpl != 0 {
		return
	}
	if !intentExpressionImplicit(file, intentArg, ctx.Idx, 0) {
		return
	}
	// Lint drops the finding below API 23, and below API 34 reports a warning about the future behavior.
	msg, severity := mutableImplicitPendingIntentBase, ""
	if sdk := lintFileTargetSdk(ctx); sdk > 0 {
		if sdk < 23 {
			return
		}
		if sdk < 34 {
			severity = "warning"
			msg += " once this app starts targeting Android 14 or above"
		}
	}
	at := ctx.Idx
	// Lint's location spans the qualified call, starting at the PendingIntent receiver.
	if parent, ok := file.FlatParent(at); ok && file.FlatType(at) == "call_expression" && file.FlatType(parent) == "navigation_expression" {
		at = parent
	}
	ctx.Emit(lintFindingAtNode(ctx, at, severity, msg+mutableImplicitPendingIntentAdvice))
}

// intentState is Lint's IntentType: whether the intent names a component or a package, and whether it escaped into
// code the rule cannot follow, in which case it is not reported.
type intentState struct {
	component, pkg, escaped bool
}

func (s intentState) implicit() bool { return !s.escaped && !s.component && !s.pkg }

// intentExpressionImplicit reports whether the intent expression (or, for getActivities, the array of intents) is
// proven implicit.
func intentExpressionImplicit(file *scanner.File, expr, pendingCall uint32, depth int) bool {
	expr = lintSkipParensPI(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		if call, ok := lintCallAt(file, expr); ok && call.ReceiverNode == 0 && (call.Name == "arrayOf") {
			for _, arg := range call.Args {
				if intentExpressionImplicit(file, arg, pendingCall, depth+1) {
					return true
				}
			}
			return false
		}
	case "array_creation_expression":
		init, ok := file.FlatFindChild(expr, "array_initializer")
		if !ok {
			return false
		}
		for child := file.FlatFirstChild(init); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && intentExpressionImplicit(file, child, pendingCall, depth+1) {
				return true
			}
		}
		return false
	}
	state, ok := intentStateOf(file, expr, pendingCall, depth)
	return ok && state.implicit()
}

// intentStateOf classifies an intent-valued expression: a constructor call, a chain of calls on one, or a local
// variable assigned in the same function.
func intentStateOf(file *scanner.File, expr, pendingCall uint32, depth int) (intentState, bool) {
	expr = lintSkipParensPI(file, expr)
	if expr == 0 || depth > 4 {
		return intentState{}, false
	}
	if state, ok := intentConstruction(file, expr); ok {
		return state, true
	}
	switch file.FlatType(expr) {
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		if !ok || call.ReceiverNode == 0 {
			return intentState{}, false
		}
		state, ok := intentStateOf(file, call.ReceiverNode, pendingCall, depth+1)
		if !ok {
			return intentState{}, false
		}
		return intentAfterCall(file, state, call), true
	case "simple_identifier", "identifier":
		return intentLocalState(file, file.FlatNodeText(expr), expr, pendingCall, depth)
	}
	return intentState{}, false
}

// intentConstruction classifies an Intent constructor call. Intent(Context, Class) and Intent(String, Uri, Context,
// Class) are explicit; the others (no argument, an action, an action and a Uri, or a copy) are implicit. It reports
// false when expr is not an Intent constructor or the two-argument overload cannot be told apart.
func intentConstruction(file *scanner.File, expr uint32) (intentState, bool) {
	call, ok := lintCallAt(file, expr)
	if !ok || call.Name != "Intent" {
		return intentState{}, false
	}
	switch file.FlatType(expr) {
	case "object_creation_expression":
		if _, anon := file.FlatFindChild(expr, "class_body"); anon {
			return intentState{}, false
		}
	case "call_expression":
		if call.ReceiverNode != 0 && call.TypeText != intentClass {
			return intentState{}, false
		}
	default:
		return intentState{}, false
	}
	if !lintTypeRefMatches(file, call.TypeText, intentClass) {
		return intentState{}, false
	}
	for _, label := range call.Labels {
		if label != "" {
			return intentState{}, false
		}
	}
	switch len(call.Args) {
	case 0, 1:
		return intentState{}, true
	case 2:
		switch {
		case intentClassArgument(file, call.Args[1]):
			return intentState{component: true}, true
		case intentURIArgument(file, call.Args[1]):
			return intentState{}, true
		}
	case 4:
		if intentClassArgument(file, call.Args[3]) {
			return intentState{component: true}, true
		}
	}
	return intentState{}, false
}

// intentClassArgument reports whether the expression is a class literal: Foo::class.java, Foo.class.
func intentClassArgument(file *scanner.File, expr uint32) bool {
	text := strings.Join(strings.Fields(file.FlatNodeText(lintSkipParensPI(file, expr))), "")
	return strings.HasSuffix(text, "::class.java") || file.FlatType(lintSkipParensPI(file, expr)) == "class_literal"
}

// intentURIArgument reports whether the expression is an android.net.Uri: Uri.parse(...), Uri.EMPTY, or a value
// declared with that type.
func intentURIArgument(file *scanner.File, expr uint32) bool {
	expr = lintSkipParensPI(file, expr)
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	for _, prefix := range []string{"Uri.parse(", "Uri.fromFile(", "Uri.fromParts(", "Uri.EMPTY"} {
		if strings.HasPrefix(text, prefix) && lintTypeRefMatches(file, "Uri", "android.net.Uri") {
			return true
		}
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		typ := strings.TrimSuffix(lintDeclaredTypeText(file, expr, file.FlatNodeText(expr)), "?")
		return typ != "" && lintTypeRefMatches(file, typ, "android.net.Uri")
	}
	return false
}

// intentAfterCall applies Lint's setIntentTypeAfterCallInPlace for a call made on the intent.
func intentAfterCall(file *scanner.File, state intentState, call lintCall) intentState {
	name := call.Name
	// lintCall keeps a 0 slot for a keyword argument with no named node, such as Kotlin's `null`.
	firstNull := len(call.Args) > 0 && (call.Args[0] == 0 || strings.TrimSpace(file.FlatNodeText(call.Args[0])) == "null")
	switch name {
	case "setPackage":
		state.pkg = !firstNull
	case "setComponent":
		state.component = !firstNull
	case "setClass", "setClassName":
		state.component = true
	case "apply", "also":
		if !intentScopeBlockKnown(file, call, &state, name == "also") {
			state.escaped = true
		}
	default:
		if !intentMethods[name] {
			state.escaped = true
		}
	}
	return state
}

// intentMethods are Intent members that neither name a target nor let the intent escape.
var intentMethods = lintStringSet(
	"putExtra", "putExtras", "putCharSequenceArrayListExtra", "putIntegerArrayListExtra",
	"putParcelableArrayListExtra", "putStringArrayListExtra", "removeExtra", "replaceExtras", "addFlags", "setFlags",
	"removeFlags", "setAction", "setData", "setDataAndNormalize", "setDataAndType", "setDataAndTypeAndNormalize",
	"setType", "setTypeAndNormalize", "addCategory", "removeCategory", "setSelector", "setClipData", "setIdentifier",
	"setSourceBounds", "setExtrasClassLoader")

// intentProperties are the Kotlin property names of Intent setters that do not name a target.
var intentProperties = lintStringSet("action", "data", "type", "flags", "selector", "clipData", "identifier",
	"sourceBounds")

// intentScopeBlockKnown applies the statements of an apply or also block on the intent. It reports false when a
// statement is anything but an Intent setter call or property assignment on the scope object.
func intentScopeBlockKnown(file *scanner.File, call lintCall, state *intentState, viaIt bool) bool {
	if len(call.Args) != 0 {
		return false
	}
	lambda := uint32(0)
	file.FlatWalkNodes(call.Idx, "lambda_literal", func(n uint32) {
		if lambda == 0 {
			lambda = n
		}
	})
	if lambda == 0 {
		return false
	}
	self := ""
	if viaIt {
		self = "it"
		if params, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
			self = strings.TrimSpace(file.FlatNodeText(params))
			if !lintIdentOnly(self) {
				return false
			}
		}
	} else if _, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
		return false
	}
	stmts, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return true
	}
	for stmt := file.FlatFirstChild(stmts); stmt != 0; stmt = file.FlatNextSib(stmt) {
		if !file.FlatIsNamed(stmt) {
			continue
		}
		switch file.FlatType(stmt) {
		case "call_expression":
			inner, ok := lintCallAt(file, stmt)
			if !ok {
				return false
			}
			recv := strings.TrimSpace(inner.ReceiverText)
			if (viaIt && recv != self) || (!viaIt && recv != "" && recv != "this") {
				return false
			}
			if inner.Name == "apply" || inner.Name == "also" {
				return false
			}
			*state = intentAfterCall(file, *state, inner)
			if state.escaped {
				return false
			}
		case "assignment":
			if !intentPropertyAssignment(file, stmt, self, state) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// intentPropertyAssignment applies `owner.prop = value` (or `prop = value` inside apply when owner is "") to the
// intent state. It reports false for a property it does not know.
func intentPropertyAssignment(file *scanner.File, assign uint32, owner string, state *intentState) bool {
	lhs := file.FlatNamedChild(assign, 0)
	target := strings.Join(strings.Fields(file.FlatNodeText(lhs)), "")
	target = strings.ReplaceAll(target, "`", "")
	switch {
	case owner != "" && strings.HasPrefix(target, owner+"."):
		target = strings.TrimPrefix(target, owner+".")
	case owner == "" && strings.HasPrefix(target, "this."):
		target = strings.TrimPrefix(target, "this.")
	}
	if !lintIdentOnly(target) {
		return false
	}
	value := strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(assign, file.FlatNamedChildCount(assign)-1)))
	switch target {
	case "package":
		state.pkg = value != "null"
	case "component":
		state.component = value != "null"
	default:
		if !intentProperties[target] {
			return false
		}
	}
	return true
}

// intentLocalState classifies a local variable at the PendingIntent call: its last assignment before the call, then
// every use between that assignment and the call.
func intentLocalState(file *scanner.File, name string, ref, pendingCall uint32, depth int) (intentState, bool) {
	fn, ok := flatEnclosingCallable(file, pendingCall)
	if !ok || lintNameIsParameterAt(file, pendingCall, name) {
		return intentState{}, false
	}
	callStart := file.FlatStartByte(pendingCall)
	// The last assignment before the call, as a declaration initializer or a plain assignment.
	var assignNode, value uint32
	consider := func(node, v uint32) {
		if v == 0 || file.FlatStartByte(node) >= callStart {
			return
		}
		if assignNode == 0 || file.FlatStartByte(node) > file.FlatStartByte(assignNode) {
			assignNode, value = node, v
		}
	}
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
			consider(decl, lintInitializerAfterEquals(file, decl))
		}
	})
	file.FlatWalkNodes(fn, "variable_declarator", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
			consider(decl, lintInitializerAfterEquals(file, decl))
		}
	})
	for _, kind := range []string{"assignment", "assignment_expression"} {
		file.FlatWalkNodes(fn, kind, func(assign uint32) {
			lhs := file.FlatNamedChild(assign, 0)
			if strings.TrimSpace(file.FlatNodeText(lhs)) == name {
				consider(assign, file.FlatNamedChild(assign, file.FlatNamedChildCount(assign)-1))
			}
		})
	}
	if assignNode == 0 {
		return intentState{}, false
	}
	// The assignment must reach the call on every path: its block has to enclose the call.
	block, ok := file.FlatParent(assignNode)
	if file.FlatType(block) == "local_variable_declaration" || file.FlatType(block) == "expression_statement" {
		block, ok = file.FlatParent(block)
	}
	if !ok || !piContains(file, block, pendingCall) {
		return intentState{}, false
	}
	state, ok := intentStateOf(file, value, pendingCall, depth+1)
	if !ok {
		return intentState{}, false
	}
	// Uses between the assignment and the call: setters on it update the state; anything else escapes.
	known := true
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if !known || n == ref {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "identifier":
		default:
			return
		}
		if file.FlatNodeText(n) != name || file.FlatStartByte(n) <= file.FlatEndByte(value) || file.FlatStartByte(n) >= callStart {
			return
		}
		if piContains(file, assignNode, n) {
			return
		}
		if !intentUse(file, n, name, &state) {
			known = false
		}
	})
	if !known {
		return intentState{}, false
	}
	return state, true
}

// intentUse applies one use of the intent variable: a call on it, or an assignment to one of its properties. It
// reports false for any other use, which Lint treats as an escape.
func intentUse(file *scanner.File, n uint32, name string, state *intentState) bool {
	parent, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	if _, inLambda := flatEnclosingAncestor(file, n, "lambda_literal", "lambda_expression"); inLambda {
		if fnOfUse, _ := flatEnclosingCallable(file, n); fnOfUse != 0 {
			return false
		}
	}
	switch file.FlatType(parent) {
	case "directly_assignable_expression":
		// `intent.prop = value` in Kotlin.
		assign, ok := file.FlatParent(parent)
		if !ok || file.FlatType(assign) != "assignment" || file.FlatNamedChild(parent, 0) != n {
			return false
		}
		return intentPropertyAssignment(file, assign, name, state)
	case "method_invocation":
		call, ok := lintCallAt(file, parent)
		if !ok || call.ReceiverNode != n {
			return false
		}
		*state = intentAfterCall(file, *state, call)
		return !state.escaped
	case "navigation_expression":
		grand, ok := file.FlatParent(parent)
		if !ok {
			return false
		}
		switch file.FlatType(grand) {
		case "call_expression":
			call, ok := lintCallAt(file, grand)
			if !ok || call.ReceiverNode != n {
				return false
			}
			*state = intentAfterCall(file, *state, call)
			return !state.escaped
		}
	}
	return false
}

// lintSkipParensPI unwraps parenthesized expressions.
func lintSkipParensPI(file *scanner.File, idx uint32) uint32 {
	for idx != 0 && file.FlatType(idx) == "parenthesized_expression" {
		inner := uint32(0)
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				inner = child
				break
			}
		}
		idx = inner
	}
	return idx
}

// piContains reports whether outer spans inner.
func piContains(file *scanner.File, outer, inner uint32) bool {
	return file.FlatStartByte(outer) <= file.FlatStartByte(inner) && file.FlatEndByte(inner) <= file.FlatEndByte(outer)
}
