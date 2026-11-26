package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// Ports of NonNullableMutableLiveDataDetector (androidx.lifecycle:lifecycle-livedata-core) and LifecycleWhenChecks
// (androidx.lifecycle:lifecycle-runtime) from androidx.lifecycle 2.11.1. Both are Kotlin-only, production-source
// checks (Lint's JAVA_FILE_SCOPE).

const axlLivedataPackage = "androidx.lifecycle"

// NullSafeMutableLiveDataRule ports NullSafeMutableLiveData: null, or a nullable value, set on a LiveData whose type
// argument is non-nullable.
type NullSafeMutableLiveDataRule struct {
	FlatDispatchBase
	BaseRule
}

// UnsafeLifecycleWhenUsageRule ports UnsafeLifecycleWhenUsage: View access in the finally block of a try that
// suspends inside Lifecycle.whenStarted/whenCreated/whenResumed, which may run after the Lifecycle left the state.
type UnsafeLifecycleWhenUsageRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NullSafeMutableLiveDataRule) fileMentions() []string { return []string{"LiveData"} }
func (r *UnsafeLifecycleWhenUsageRule) fileMentions() []string {
	return []string{"androidx.lifecycle", "when"}
}

// Confidence is medium: nullability and LiveData types are proven from same-file declarations.
func (r *NullSafeMutableLiveDataRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence is medium: suspend calls and View receivers are proven from same-file declarations and known APIs.
func (r *UnsafeLifecycleWhenUsageRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullSafeMutableLiveDataRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "NullSafeMutableLiveData", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The LiveData's type argument comes from a same-file declaration (declared type, MutableLiveData<T>() " +
				"initializer, or a same-file subclass); a nullable value is a name declared nullable, or a call of a " +
				"same-file function with a nullable return type.",
			"A nullable name is skipped when a smart cast may apply: an enclosing if/when condition testing it " +
				"(`!= null`, `is`), the lambda of a safe call on it, or an earlier statement in an enclosing block " +
				"(`!!`, `?: return`, requireNotNull/require(x != null), `if (x == null || ...) return`, isNullOrEmpty " +
				"exits, a when entry `x == null -> return`). Lint follows the exact smart cast, so a proof outside " +
				"these shapes is reported.",
			"Lint's quick fixes are not ported.",
		}}
}

func (r *UnsafeLifecycleWhenUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeLifecycleWhenUsage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Suspend calls are calls of same-file suspend functions, of the kotlinx.coroutines top-level suspend " +
				"functions, and of suspend members (await, first, collect, send, ...) on a value declared as a " +
				"kotlinx.coroutines Flow, Channel, Deferred or Job; " +
				"View access is a call or property on a value declared as an android.view.View subclass. Calls are " +
				"followed into same-file functions three levels deep, as Lint does.",
			"As in the detector, only finally blocks are checked, and a Lifecycle.State.isAtLeast guard does not " +
				"count (the detector looks for an isAtLeast on Lifecycle itself).",
		}}
}

// check mirrors NonNullableMutableLiveDataDetector for `live.value = x`, `live.setValue(x)` and `live.postValue(x)`.
func (r *NullSafeMutableLiveDataRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) {
		return
	}
	var receiver, arg uint32
	switch file.FlatType(node) {
	case "assignment":
		target := file.FlatFirstChild(node)
		if target == 0 || file.FlatType(target) != "directly_assignable_expression" {
			return
		}
		suffix := flatLastNamedChild(file, target)
		first := file.FlatFirstChild(target)
		if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" || first == suffix ||
			!file.FlatNodeTextEquals(file.FlatNamedChild(suffix, 0), "value") {
			return
		}
		receiver, arg = first, axlLastChild(file, node)
		if file.FlatNextSib(first) != suffix {
			return // a.b.value: only a simple receiver is resolved
		}
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, node) {
			return
		}
		name, _, hasReceiver := acrCallCallee(file, node)
		if (name != "setValue" && name != "postValue") || !hasReceiver {
			return
		}
		args := flatCallKeyArguments(file, node)
		var only uint32
		n := 0
		for a := file.FlatFirstChild(args); args != 0 && a != 0; a = file.FlatNextSib(a) {
			if file.FlatType(a) == "value_argument" {
				only = a
				n++
			}
		}
		if n != 1 {
			return
		}
		receiver, arg = file.FlatFirstChild(acrCalleeExpression(file, node)), axlLastChild(file, only)
	default:
		return
	}
	if receiver == 0 || arg == 0 || file.FlatType(receiver) != "simple_identifier" {
		return
	}
	typeArg, ok := axlLiveDataTypeArgument(file, receiver)
	if !ok || strings.HasSuffix(typeArg, "?") || axlIsTypeParameter(file, node, typeArg) {
		return
	}
	switch {
	case file.FlatNodeTextEquals(arg, "null"):
		ctx.EmitAtNode(arg, "Cannot set non-nullable LiveData value to `null`")
	case axlExprIsNullable(file, arg):
		ctx.EmitAtNode(arg, "Expected non-nullable value")
	}
}

// axlLiveDataTypeArgument returns the written type argument T of the LiveData a receiver name holds: its declared
// type (LiveData<T>, MutableLiveData<T>, MediatorLiveData<T>), a MutableLiveData<T>(...) initializer, or the
// MutableLiveData<T> supertype of a same-file class it is declared as.
func axlLiveDataTypeArgument(file *scanner.File, receiver uint32) (string, bool) {
	decl := acrValueDeclaration(file, receiver, file.FlatNodeText(receiver))
	if decl == 0 {
		return "", false
	}
	if t := acrDeclaredType(file, decl); t != 0 {
		if acrTypeNameIs(file, t, axlLivedataPackage, "LiveData", "MutableLiveData", "MediatorLiveData") {
			return axlSingleTypeArgument(file, file.FlatNodeText(t))
		}
		if class := acrSameFileClass(file, t); class != 0 {
			return axlLiveDataSupertypeArgument(file, class)
		}
		return "", false
	}
	init := acrInitializer(file, decl)
	if init == 0 || file.FlatType(init) != "call_expression" {
		return "", false
	}
	name, _, hasReceiver := acrCallCallee(file, init)
	if hasReceiver || (!acrSimpleNameRefersTo(file, name, axlLivedataPackage, "MutableLiveData") &&
		!acrSimpleNameRefersTo(file, name, axlLivedataPackage, "MediatorLiveData")) {
		return "", false
	}
	return axlSingleTypeArgument(file, file.FlatNodeText(acrCalleeExpression(file, init))+recyclerCallTypeArgumentText(file, init))
}

// axlLiveDataSupertypeArgument returns the type argument of a same-file class's direct androidx.lifecycle LiveData
// supertype (not a nested class's).
func axlLiveDataSupertypeArgument(file *scanner.File, class uint32) (string, bool) {
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if t := acrDelegationSpecifierType(file, child); acrTypeNameIs(file, t, axlLivedataPackage, "LiveData",
			"MutableLiveData", "MediatorLiveData") {
			return axlSingleTypeArgument(file, file.FlatNodeText(t))
		}
	}
	return "", false
}

// recyclerCallTypeArgumentText returns `<T>` for a call with one explicit type argument, or "".
func recyclerCallTypeArgumentText(file *scanner.File, call uint32) string {
	if t := recyclerCallTypeArgument(file, call); t != "" {
		return "<" + t + ">"
	}
	return ""
}

// axlSingleTypeArgument returns the single type argument written in a type text such as `MutableLiveData<String?>`.
func axlSingleTypeArgument(_ *scanner.File, text string) (string, bool) {
	text = strings.Join(strings.Fields(text), "")
	lt, gt := strings.Index(text, "<"), strings.LastIndex(text, ">")
	if lt < 0 || gt <= lt {
		return "", false
	}
	arg := text[lt+1 : gt]
	if arg == "" || strings.Contains(arg, ",") || arg == "*" {
		return "", false
	}
	return arg, true
}

// axlIsTypeParameter reports whether name is a type parameter of an enclosing function or class.
func axlIsTypeParameter(file *scanner.File, node uint32, name string) bool {
	name = strings.TrimSuffix(name, "?")
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration":
			if tps, ok := file.FlatFindChild(p, "type_parameters"); ok {
				for tp := file.FlatFirstChild(tps); tp != 0; tp = file.FlatNextSib(tp) {
					if file.FlatType(tp) == "type_parameter" && strings.Trim(file.FlatChildTextOrEmpty(tp, "type_identifier"), "`") == name {
						return true
					}
				}
			}
		}
	}
	return false
}

// axlExprIsNullable reports whether an argument is provably nullable: a name declared with a nullable type that the
// enclosing function never null-checks, or a call of a same-file function declared to return a nullable type.
func axlExprIsNullable(file *scanner.File, expr uint32) bool {
	expr = acrUnwrapParens(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier":
		name := file.FlatNodeText(expr)
		decl := acrValueDeclaration(file, expr, name)
		if decl == 0 {
			return false
		}
		t := acrDeclaredType(file, decl)
		if t == 0 || file.FlatType(t) != "nullable_type" {
			return false
		}
		return !axlNameNullChecked(file, expr, name)
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, expr)
		if hasReceiver {
			return false
		}
		fn := acrSingleTopLevelFunction(file, name)
		if fn == 0 {
			if decls := acrFunctionDecls(file, name); len(decls) == 1 {
				fn = decls[0]
			}
		}
		if fn == 0 {
			return false
		}
		rt := acrFunctionReturnType(file, fn)
		return rt != 0 && file.FlatType(rt) == "nullable_type"
	}
	return false
}

// axlNameNullChecked reports whether a smart cast may make name non-null at node: an enclosing if (or when branch)
// whose condition tests it against null or with `is`, the lambda of a safe call on it (`name?.let { }`), or an
// earlier statement of an enclosing block that proves it non-null (see axlStatementProvesNonNull).
func axlNameNullChecked(file *scanner.File, node uint32, name string) bool {
	guard := []string{name + "!=null", "null!=" + name, name + "is", "requireNotNull(" + name, "checkNotNull(" + name,
		"!" + name + ".isNullOrEmpty()", "!" + name + ".isNullOrBlank()"}
	child := node
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression", "when_entry":
			// The condition is the first named child; a guard counts for the branches after it.
			if cond := flatFirstNamedChild(file, p); cond != 0 && cond != child {
				text := axlCompactText(file, cond)
				for _, g := range guard {
					if strings.Contains(text, g) {
						return true
					}
				}
				// The else branch of `if (name == null || …)` only runs when name is non-null.
				if file.FlatType(p) == "if_expression" && axlIsElseBranch(file, p, child) && axlNullTestDisjunction(file, cond, name) {
					return true
				}
			}
			// In `when (name) { null -> …; else -> … }` the entries after the null entry see name non-null.
			if file.FlatType(p) == "when_entry" && axlWhenEntryAfterNull(file, p, name) {
				return true
			}
		case "lambda_literal":
			// The lambda of `name?.f { }` only runs when name is non-null.
			if arg, ok := acrLambdaArgumentOf(file, p); ok &&
				strings.HasPrefix(axlCompactText(file, acrCalleeExpression(file, arg.Call)), name+"?.") {
				return true
			}
		case "statements":
			for st := file.FlatFirstChild(p); st != 0 && st != child; st = file.FlatNextSib(st) {
				if axlStatementProvesNonNull(file, st, name) {
					return true
				}
			}
		case "function_declaration", "class_body", "source_file":
			return false
		}
		child = p
	}
	return false
}

// axlStatementProvesNonNull reports whether name is non-null after a statement: `name!!`, `name ?: return|throw`,
// requireNotNull/checkNotNull(name), require/check(name != null), or an early exit taken when name is null:
// `if (name == null || ...) return` (also `name.isNullOrEmpty()`/`isNullOrBlank()`), or a subject-less when whose
// entries up to a `name == null -> return` entry all exit.
func axlStatementProvesNonNull(file *scanner.File, st uint32, name string) bool {
	text := axlCompactText(file, st)
	for _, proof := range []string{name + "!!", name + "?:return", name + "?:throw", "requireNotNull(" + name,
		"checkNotNull(" + name, "require(" + name + "!=null", "require(null!=" + name, "check(" + name + "!=null",
		"check(null!=" + name, name + "?:error(", name + "?:TODO("} {
		if strings.Contains(text, proof) {
			return true
		}
	}
	if axlElvisRunExits(file, st, name) {
		return true
	}
	switch file.FlatType(st) {
	case "if_expression":
		cond := flatFirstNamedChild(file, st)
		body, _ := file.FlatFindChild(st, "control_structure_body")
		return axlNullTestDisjunction(file, cond, name) && axlBodyExits(file, body)
	case "when_expression":
		if _, hasSubject := file.FlatFindChild(st, "when_subject"); hasSubject {
			return false
		}
		for entry := file.FlatFirstChild(st); entry != 0; entry = file.FlatNextSib(entry) {
			if file.FlatType(entry) != "when_entry" {
				continue
			}
			body, _ := file.FlatFindChild(entry, "control_structure_body")
			if !axlBodyExits(file, body) {
				return false
			}
			if cond, ok := file.FlatFindChild(entry, "when_condition"); ok && axlNullTestDisjunction(file, cond, name) {
				return true
			}
		}
	}
	return false
}

// axlNullTestDisjunction reports whether a condition is true whenever name is null: one of its top-level `||` terms
// is `name == null`, `null == name`, `name.isNullOrEmpty()` or `name.isNullOrBlank()`.
func axlNullTestDisjunction(file *scanner.File, cond uint32, name string) bool {
	if cond == 0 {
		return false
	}
	for _, term := range strings.Split(axlCompactText(file, cond), "||") {
		switch term {
		case name + "==null", "null==" + name, name + ".isNullOrEmpty()", name + ".isNullOrBlank()":
			return true
		}
	}
	return false
}

// axlBodyExits reports whether an if/when body ends in return, throw, break or continue, or a call of the
// Nothing-returning error() or TODO() (unless the file declares its own).
func axlBodyExits(file *scanner.File, body uint32) bool {
	if body == 0 {
		return false
	}
	last := flatFirstNamedChild(file, body)
	if file.FlatType(last) == "statements" {
		last = flatLastNamedChild(file, last)
	}
	switch file.FlatType(last) {
	case "jump_expression":
		return true
	case "call_expression":
		name, _, hasReceiver := acrCallCallee(file, last)
		return !hasReceiver && (name == "error" || name == "TODO") && len(acrFunctionDecls(file, name)) == 0
	}
	return false
}

// axlElvisRunExits reports whether a statement contains `name ?: run { …; exit }`: an elvis whose right side is a
// receiverless `run` call (not a same-file lookalike) and whose lambda ends in an exit. `return@run` only leaves
// the lambda, so it is not an exit.
func axlElvisRunExits(file *scanner.File, st uint32, name string) bool {
	found := false
	file.FlatWalkNodes(st, "elvis_expression", func(elvis uint32) {
		if found || file.FlatNamedChildCount(elvis) != 2 || axlCompactText(file, file.FlatNamedChild(elvis, 0)) != name {
			return
		}
		call := file.FlatNamedChild(elvis, 1)
		if file.FlatType(call) != "call_expression" {
			return
		}
		callee, _, hasReceiver := acrCallCallee(file, call)
		if callee != "run" || hasReceiver || len(acrFunctionDecls(file, "run")) != 0 {
			return
		}
		lambda := axlTrailingLambda(file, call)
		body, ok := file.FlatFindChild(lambda, "statements")
		if lambda == 0 || !ok {
			return
		}
		if last := flatLastNamedChild(file, body); strings.HasPrefix(axlCompactText(file, last), "return@run") {
			return
		}
		found = axlBodyExits(file, lambda)
	})
	return found
}

// axlIsElseBranch reports whether child is the else body of an if_expression.
func axlIsElseBranch(file *scanner.File, ifExpr, child uint32) bool {
	afterElse := false
	for c := file.FlatFirstChild(ifExpr); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "else" {
			afterElse = true
		} else if c == child {
			return afterElse
		}
	}
	return false
}

// axlWhenEntryAfterNull reports whether entry follows a `null ->` entry of a `when (name)` expression.
func axlWhenEntryAfterNull(file *scanner.File, entry uint32, name string) bool {
	when, ok := file.FlatParent(entry)
	if !ok || file.FlatType(when) != "when_expression" {
		return false
	}
	subject, ok := file.FlatFindChild(when, "when_subject")
	if !ok || strings.Trim(axlCompactText(file, subject), "()") != name {
		return false
	}
	for e := file.FlatFirstChild(when); e != 0 && e != entry; e = file.FlatNextSib(e) {
		if file.FlatType(e) != "when_entry" {
			continue
		}
		for c := file.FlatFirstChild(e); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "when_condition" && axlCompactText(file, c) == "null" {
				return true
			}
		}
	}
	return false
}

func axlCompactText(file *scanner.File, n uint32) string {
	if n == 0 {
		return ""
	}
	return strings.Join(strings.Fields(file.FlatNodeText(n)), "")
}

// axlLastChild returns the last child of a node, named or not (a `null` literal is an unnamed keyword).
func axlLastChild(file *scanner.File, node uint32) uint32 {
	var last uint32
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		last = c
	}
	return last
}

// axlTopLevelSuspendFunctions are the top-level suspend functions a receiverless call is proven to call, by package.
var axlTopLevelSuspendFunctions = map[string]string{
	"delay": "kotlinx.coroutines", "withContext": "kotlinx.coroutines", "yield": "kotlinx.coroutines",
	"coroutineScope": "kotlinx.coroutines", "supervisorScope": "kotlinx.coroutines", "withTimeout": "kotlinx.coroutines",
	"withTimeoutOrNull": "kotlinx.coroutines", "suspendCancellableCoroutine": "kotlinx.coroutines",
	"awaitAll": "kotlinx.coroutines", "joinAll": "kotlinx.coroutines", "suspendCoroutine": "kotlin.coroutines",
}

// axlSuspendMembers are suspend members and extensions of the kotlinx.coroutines types in axlCoroutineTypes. The
// names are also ordinary List/Sequence/Thread APIs, so a call counts only on a receiver declared as one of those types.
var axlSuspendMembers = map[string]bool{
	"await": true, "join": true, "collect": true, "collectLatest": true, "first": true, "firstOrNull": true,
	"single": true, "singleOrNull": true, "toList": true, "emit": true, "send": true, "receive": true,
}

// axlCoroutineTypes are the kotlinx.coroutines types whose axlSuspendMembers suspend, by package.
var axlCoroutineTypes = map[string][]string{
	"kotlinx.coroutines":          {"Deferred", "Job", "CompletableDeferred", "CompletableJob"},
	"kotlinx.coroutines.flow":     {"Flow", "StateFlow", "SharedFlow", "MutableStateFlow", "MutableSharedFlow", "FlowCollector"},
	"kotlinx.coroutines.channels": {"Channel", "SendChannel", "ReceiveChannel"},
}

// axlSuspendLambdaFunctions take a suspend lambda, which LifecycleWhenVisitor does not enter.
var axlSuspendLambdaFunctions = map[string]bool{
	"launch": true, "async": true, "coroutineScope": true, "supervisorScope": true, "withContext": true,
	"withTimeout": true, "withTimeoutOrNull": true, "flow": true, "channelFlow": true, "callbackFlow": true,
	"produce": true, "runBlocking": true, "whenCreated": true, "whenStarted": true, "whenResumed": true,
}

// check mirrors LifecycleWhenChecks on `lifecycle.whenStarted { }` (one lambda argument, an androidx.lifecycle
// import, possibly aliased): a try whose try clause makes a suspend call has its finally block checked for View
// access, at the call. Like LifecycleWhenVisitor, the search skips suspend lambdas and nested declarations.
func (r *UnsafeLifecycleWhenUsageRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) || acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	written, _, hasReceiver := acrCallCallee(file, node)
	if !hasReceiver {
		return
	}
	name := ""
	for _, method := range []string{"whenCreated", "whenStarted", "whenResumed"} {
		if acrSimpleNameRefersTo(file, written, axlLivedataPackage, method) {
			name = method
		}
	}
	if name == "" {
		return
	}
	lambda := axlCallTrailingLambda(file, node)
	if lambda == 0 || acrCallValueArgument(file, node, 0, "") != 0 {
		return
	}
	msg := "Unsafe View access from finally/catch block inside of `Lifecycle." + name + "` scope"
	file.FlatWalkNodes(lambda, "try_expression", func(try uint32) {
		if axlInSuspendLambdaOrDeclaration(file, try, lambda) {
			return
		}
		block, finally := uint32(0), uint32(0)
		for c := file.FlatFirstChild(try); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "statements":
				if block == 0 {
					block = c
				}
			case "finally_block":
				finally = c
			}
		}
		if block == 0 || finally == 0 || !axlBlockSuspends(ctx, block) {
			return
		}
		file.FlatWalkNodes(finally, "call_expression", func(call uint32) {
			if !axlInSuspendLambdaOrDeclaration(file, call, finally) && axlCallAccessesView(ctx, call, 0, map[uint32]bool{}) {
				ctx.EmitAtNode(call, msg)
			}
		})
		file.FlatWalkNodes(finally, "assignment", func(a uint32) {
			if !axlInSuspendLambdaOrDeclaration(file, a, finally) && axlAssignmentAccessesView(ctx, a) {
				ctx.EmitAtNode(a, msg)
			}
		})
	})
}

// axlInSuspendLambdaOrDeclaration reports whether node sits, below root, in what LifecycleWhenVisitor does not
// enter: a suspend lambda (the lambda of a coroutine builder or a nested whenX), or a declaration (its
// visitDeclaration skips functions, classes and local variables with their initializers).
func axlInSuspendLambdaOrDeclaration(file *scanner.File, node, root uint32) bool {
	for p, ok := file.FlatParent(node); ok && p != root; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_literal", "anonymous_function",
			"property_declaration":
			return true
		case "lambda_literal":
			if arg, ok := acrLambdaArgumentOf(file, p); ok {
				if name, _, _ := acrCallCallee(file, arg.Call); axlSuspendLambdaFunctions[name] {
					return true
				}
			}
		}
	}
	return false
}

// axlBlockSuspends reports whether a try clause calls a suspend function outside suspend lambdas and nested
// declarations: a same-file suspend function, a kotlinx.coroutines top-level suspend function, or a suspend member
// on a value declared as a kotlinx.coroutines type.
func axlBlockSuspends(ctx *api.Context, block uint32) bool {
	file := ctx.File
	found := false
	file.FlatWalkNodes(block, "call_expression", func(call uint32) {
		if found || acrIsInnerCallOfTrailingLambda(file, call) || axlInSuspendLambdaOrDeclaration(file, call, block) {
			return
		}
		name, _, hasReceiver := acrCallCallee(file, call)
		decls := acrFunctionDecls(file, name)
		for _, fn := range decls {
			if acrFunctionHasModifier(file, fn, "suspend") {
				found = true
				return
			}
		}
		if len(decls) > 0 {
			return
		}
		if !hasReceiver {
			if pkg, ok := axlTopLevelSuspendFunctions[name]; ok && acrNameResolvesTo(file, pkg, name) {
				found = true
			}
			return
		}
		found = axlSuspendMembers[name] && axlExprIsCoroutineType(file, file.FlatFirstChild(acrCalleeExpression(file, call)))
	})
	return found
}

// axlExprIsCoroutineType reports whether a name is declared as one of axlCoroutineTypes.
func axlExprIsCoroutineType(file *scanner.File, expr uint32) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || file.FlatType(expr) != "simple_identifier" {
		return false
	}
	decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr))
	if decl == 0 {
		return false
	}
	t := acrDeclaredType(file, decl)
	if t == 0 {
		return false
	}
	for pkg, names := range axlCoroutineTypes {
		if acrTypeNameIs(file, t, pkg, names...) {
			return true
		}
	}
	return false
}

// axlCallAccessesView mirrors CheckAccessUiVisitor on a call: its receiver is a View, or it calls a same-file
// function (three levels deep) whose body accesses one.
func axlCallAccessesView(ctx *api.Context, call uint32, depth int, visited map[uint32]bool) bool {
	file := ctx.File
	name, _, hasReceiver := acrCallCallee(file, call)
	if hasReceiver {
		return axlExprIsView(ctx, file.FlatFirstChild(acrCalleeExpression(file, call)))
	}
	if depth >= 3 {
		return false
	}
	for _, fn := range acrFunctionDecls(file, name) {
		if visited[fn] {
			continue
		}
		visited[fn] = true
		body, ok := file.FlatFindChild(fn, "function_body")
		if !ok {
			continue
		}
		hit := false
		file.FlatWalkNodes(body, "call_expression", func(c uint32) {
			if !hit && !axlInLambda(file, c, body) && axlCallAccessesView(ctx, c, depth+1, visited) {
				hit = true
			}
		})
		file.FlatWalkNodes(body, "assignment", func(a uint32) {
			if !hit && !axlInLambda(file, a, body) && axlAssignmentAccessesView(ctx, a) {
				hit = true
			}
		})
		file.FlatWalkNodes(body, "navigation_expression", func(n uint32) {
			if !hit && !axlInLambda(file, n, body) && axlNavigationReadsView(ctx, n) {
				hit = true
			}
		})
		if hit {
			return true
		}
	}
	return false
}

// axlInLambda reports whether node sits in a lambda under root (the UI visitor does not enter lambdas).
func axlInLambda(file *scanner.File, node, root uint32) bool {
	for p, ok := file.FlatParent(node); ok && p != root; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "lambda_literal" {
			return true
		}
	}
	return false
}

// axlAssignmentAccessesView reports `view.prop = x` (a setter call on a View).
func axlAssignmentAccessesView(ctx *api.Context, a uint32) bool {
	file := ctx.File
	target := file.FlatFirstChild(a)
	if target == 0 || file.FlatType(target) != "directly_assignable_expression" {
		return false
	}
	first := file.FlatFirstChild(target)
	return first != 0 && file.FlatNextSib(first) != 0 && axlExprIsView(ctx, first)
}

// axlNavigationReadsView reports `view.prop` reads (a getter call on a View) outside an assignment target.
func axlNavigationReadsView(ctx *api.Context, n uint32) bool {
	file := ctx.File
	if p, ok := file.FlatParent(n); ok && (file.FlatType(p) == "call_expression" || file.FlatType(p) == "navigation_expression") {
		return false
	}
	return axlExprIsView(ctx, file.FlatFirstChild(n))
}

// axlExprIsView proves an expression is an android.view.View: a name declared with a View subclass type (framework,
// AndroidX widget or same-file class).
func axlExprIsView(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || file.FlatType(expr) != "simple_identifier" {
		return false
	}
	decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr))
	if decl == 0 {
		return false
	}
	t := acrDeclaredType(file, decl)
	if t == 0 {
		return false
	}
	name := acrTypeNodeName(file, t)
	full := name
	if !strings.Contains(name, ".") {
		if target, ok := fileFactsCache().Imports(file).Aliases[name]; ok {
			full = target
		}
	}
	if full == "android.view.View" {
		return true
	}
	for _, s := range typeinfer.KnownClassHierarchy[full] {
		if s == "android.view.View" {
			return true
		}
	}
	if class := lintSameFileClass(file, name); class != 0 {
		return lintClassExtends(&api.Context{File: file}, class, "android.view.View", nil)
	}
	return false
}

// axlCallTrailingLambda returns the trailing lambda literal of a call, or 0.
func axlCallTrailingLambda(file *scanner.File, call uint32) uint32 {
	if suffix, ok := file.FlatFindChild(call, "call_suffix"); ok {
		if al, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
			if l, ok := file.FlatFindChild(al, "lambda_literal"); ok {
				return l
			}
		}
	}
	return 0
}
