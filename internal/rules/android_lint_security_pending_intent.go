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
		if call, ok := lintCallAt(file, expr); ok {
			switch {
			case call.ReceiverNode == 0 && intentCollectionFactories[call.Name]:
				for _, arg := range call.Args {
					if intentExpressionImplicit(file, arg, pendingCall, depth+1) {
						return true
					}
				}
				return false
			}
		}
	case "simple_identifier", "identifier":
		if implicit, ok := intentLocalArrayImplicit(file, file.FlatNodeText(expr), expr, pendingCall, depth); ok {
			return implicit
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

// intentWithBlockState classifies `with(intent) { setAction(...) }` (#1622), which parses as a call wrapping
// `with(intent)` with the trailing lambda. isWith is false for any other expression.
func intentWithBlockState(file *scanner.File, expr, pendingCall uint32, depth int) (state intentState, ok, isWith bool) {
	inner := file.FlatFirstChild(expr)
	if inner == 0 || file.FlatType(inner) != "call_expression" || intentTrailingLambda(file, expr) == 0 {
		return intentState{}, false, false
	}
	call, ok := lintCallAt(file, inner)
	if !ok || call.ReceiverNode != 0 || call.Name != "with" || len(call.Args) != 1 {
		return intentState{}, false, false
	}
	state, ok = intentStateOf(file, call.Args[0], pendingCall, depth+1)
	if !ok || !intentScopeBlockIntentCalls(file, lintCall{File: file, Idx: expr}, &state, false) {
		return intentState{}, false, true
	}
	return state, true, true
}

// intentCollectionFactories build an array or list of intents from their arguments: Lint's isKotlinCollection
// accepts the vararg arrayOf and listOf.
var intentCollectionFactories = lintStringSet("arrayOf", "listOf")

// intentLocalArrayImplicit classifies a local variable holding intents for getActivities, as Lint's
// IntentArrayType does: initialized with arrayOf/listOf or a Java array initializer, or with an empty array of
// constant size (arrayOfNulls(n), `new Intent[n]`) whose elements are the last write to each constant index before
// the call. ok is false when the local is not such an array; any other use of it before the call, or a write in a
// nested lambda or function, is an escape, which abstains.
func intentLocalArrayImplicit(file *scanner.File, name string, ref, pendingCall uint32, depth int) (implicit, ok bool) {
	assignNode, value, fn, found := intentLocalAssignment(file, name, pendingCall)
	if !found {
		return false, false
	}
	value = lintSkipParensPI(file, value)
	size, isArray := intentLocalArrayShape(file, value)
	if !isArray {
		return false, false
	}
	callStart := file.FlatStartByte(pendingCall)
	writes := map[int64]uint32{}
	escaped := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if escaped || n == ref {
			return
		}
		switch file.FlatType(n) {
		case "simple_identifier", "identifier":
		default:
			return
		}
		if file.FlatNodeText(n) != name || file.FlatStartByte(n) <= file.FlatEndByte(value) || file.FlatStartByte(n) >= callStart ||
			piContains(file, assignNode, n) {
			return
		}
		for p, ok := file.FlatParent(n); ok && p != fn; p, ok = file.FlatParent(p) {
			switch file.FlatType(p) {
			case "lambda_literal", "anonymous_function", "function_declaration", "lambda_expression", "method_declaration":
				escaped = true
				return
			}
		}
		index, rhs, isWrite := intentIndexedWrite(file, n)
		if !isWrite {
			escaped = true
			return
		}
		if i, ok := lintEvalInt(file, index, nil); ok {
			writes[i] = rhs
		}
	})
	if escaped {
		return false, true
	}
	if size >= 0 {
		for i, v := range writes {
			if i < size && intentExpressionImplicit(file, v, pendingCall, depth+1) {
				return true, true
			}
		}
		return false, true
	}
	if len(writes) > 0 {
		return false, true
	}
	return intentExpressionImplicit(file, value, pendingCall, depth+1), true
}

// intentLocalArrayShape reports whether value creates an intent array: size is the constant length of an empty
// array (arrayOfNulls(n), `new Intent[n]`), or -1 for an array or list built from its elements. A non-constant
// size counts as 0, as in Lint.
func intentLocalArrayShape(file *scanner.File, value uint32) (size int64, ok bool) {
	switch file.FlatType(value) {
	case "call_expression":
		call, ok := lintCallAt(file, value)
		if !ok || call.ReceiverNode != 0 {
			return 0, false
		}
		switch {
		case call.Name == "arrayOfNulls" && len(call.Args) == 1:
			if n, ok := lintEvalInt(file, call.Args[0], nil); ok {
				return n, true
			}
			return 0, true
		case intentCollectionFactories[call.Name]:
			return -1, true
		}
	case "array_creation_expression":
		if _, ok := file.FlatFindChild(value, "array_initializer"); ok {
			return -1, true
		}
		if dims, ok := file.FlatFindChild(value, "dimensions_expr"); ok {
			if n, ok := lintEvalInt(file, file.FlatNamedChild(dims, 0), nil); ok {
				return n, true
			}
			return 0, true
		}
	}
	return 0, false
}

// intentIndexedWrite reports whether the identifier n is the array of an indexed write: Kotlin `n[i] = v` or Java
// `n[i] = v`.
func intentIndexedWrite(file *scanner.File, n uint32) (index, value uint32, ok bool) {
	parent, ok := file.FlatParent(n)
	if !ok || file.FlatNamedChild(parent, 0) != n {
		return 0, 0, false
	}
	switch file.FlatType(parent) {
	case "directly_assignable_expression":
		suffix, ok := file.FlatFindChild(parent, "indexing_suffix")
		if !ok {
			return 0, 0, false
		}
		assign, ok := file.FlatParent(parent)
		if !ok || file.FlatType(assign) != "assignment" {
			return 0, 0, false
		}
		return file.FlatNamedChild(suffix, 0), file.FlatNamedChild(assign, file.FlatNamedChildCount(assign)-1), true
	case "array_access":
		assign, ok := file.FlatParent(parent)
		if !ok || file.FlatType(assign) != "assignment_expression" || file.FlatNamedChild(assign, 0) != parent {
			return 0, 0, false
		}
		return file.FlatNamedChild(parent, 1), file.FlatNamedChild(assign, file.FlatNamedChildCount(assign)-1), true
	}
	return 0, 0, false
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
		if state, ok, isWith := intentWithBlockState(file, expr, pendingCall, depth); isWith {
			return state, ok
		}
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
		if len(call.Args) != 0 || !intentScopeBlockApply(file, call, &state, name == "also") {
			state.escaped = true
		}
	case "let", "run":
		// Followed as Lint does: every statement is an Intent method call on the scope object (#1622).
		if len(call.Args) != 0 || !intentScopeBlockIntentCalls(file, call, &state, name == "let") {
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

// intentScopeBlockApply applies the statements of the trailing apply or also block on the intent: viaIt for also,
// which names it as a parameter, the receiver for apply. It reports false when a statement is anything but an
// Intent setter call or property assignment on the scope object.
func intentScopeBlockApply(file *scanner.File, call lintCall, state *intentState, viaIt bool) bool {
	lambda := intentTrailingLambda(file, call.Idx)
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
	for _, stmt := range intentLambdaStatements(file, lambda) {
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
			switch inner.Name {
			case "apply", "also", "let", "run", "with":
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

// intentScopeBlockIntentCalls applies a let (viaIt), run or with block the way Lint's
// areAllLambdaExpressionsIntentMethodCalls does: every statement, the last one included, must be a call of an Intent
// method on `it` (let) or the implicit receiver (run, with), so the block's value is the intent. A named lambda
// parameter, a property assignment or a bare `it`/`this` does not qualify.
func intentScopeBlockIntentCalls(file *scanner.File, call lintCall, state *intentState, viaIt bool) bool {
	lambda := intentTrailingLambda(file, call.Idx)
	if lambda == 0 {
		return false
	}
	if _, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
		return false
	}
	stmts := intentLambdaStatements(file, lambda)
	if len(stmts) == 0 {
		return false
	}
	for _, stmt := range stmts {
		if file.FlatType(stmt) != "call_expression" {
			return false
		}
		inner, ok := lintCallAt(file, stmt)
		if !ok {
			return false
		}
		recv := strings.TrimSpace(inner.ReceiverText)
		if (viaIt && recv != "it") || (!viaIt && recv != "" && recv != "this") {
			return false
		}
		if !intentMethods[inner.Name] && !intentTargetMethods[inner.Name] {
			return false
		}
		*state = intentAfterCall(file, *state, inner)
		if state.escaped {
			return false
		}
	}
	return true
}

// intentTargetMethods are the Intent members that name a target.
var intentTargetMethods = lintStringSet("setPackage", "setComponent", "setClass", "setClassName")

// intentLambdaStatements returns the named statements of a lambda body.
func intentLambdaStatements(file *scanner.File, lambda uint32) []uint32 {
	var stmts []uint32
	if block, ok := file.FlatFindChild(lambda, "statements"); ok {
		for stmt := file.FlatFirstChild(block); stmt != 0; stmt = file.FlatNextSib(stmt) {
			if file.FlatIsNamed(stmt) {
				stmts = append(stmts, stmt)
			}
		}
	}
	return stmts
}

// intentTrailingLambda returns the trailing lambda of a call, or 0.
func intentTrailingLambda(file *scanner.File, call uint32) uint32 {
	var lambda uint32
	for suffix := file.FlatFirstChild(call); suffix != 0; suffix = file.FlatNextSib(suffix) {
		if file.FlatType(suffix) != "call_suffix" {
			continue
		}
		if annotated, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
			if l, ok := file.FlatFindChild(annotated, "lambda_literal"); ok {
				lambda = l
			}
		}
	}
	return lambda
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
	assignNode, value, fn, ok := intentLocalAssignment(file, name, pendingCall)
	if !ok {
		return intentState{}, false
	}
	callStart := file.FlatStartByte(pendingCall)
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

// intentLocalAssignment finds a local variable's last assignment before the PendingIntent call, as a declaration
// initializer or a plain assignment, in a block that encloses the call. It also returns the enclosing function.
func intentLocalAssignment(file *scanner.File, name string, pendingCall uint32) (assignNode, value, fn uint32, found bool) {
	fn, ok := flatEnclosingCallable(file, pendingCall)
	if !ok || lintNameIsParameterAt(file, pendingCall, name) {
		return 0, 0, 0, false
	}
	callStart := file.FlatStartByte(pendingCall)
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
		return 0, 0, 0, false
	}
	// The assignment must reach the call on every path: its block has to enclose the call.
	block, ok := file.FlatParent(assignNode)
	if file.FlatType(block) == "local_variable_declaration" || file.FlatType(block) == "expression_statement" {
		block, ok = file.FlatParent(block)
	}
	if !ok || !piContains(file, block, pendingCall) {
		return 0, 0, 0, false
	}
	return assignNode, value, fn, true
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
