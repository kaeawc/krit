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
			"A nullable name is skipped when the enclosing function null-checks it anywhere (`!= null`, `?:`, " +
				"`!!`, requireNotNull, ...), where Lint follows the exact smart cast.",
			"Lint's quick fixes are not ported.",
		}}
}

func (r *UnsafeLifecycleWhenUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnsafeLifecycleWhenUsage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Suspend calls are calls of same-file suspend functions and of the kotlinx.coroutines suspend functions; " +
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
			found, arg := "", false
			file.FlatWalkNodes(class, "delegation_specifier", func(spec uint32) {
				if arg {
					return
				}
				text := strings.Join(strings.Fields(file.FlatNodeText(spec)), "")
				if base, _, _ := strings.Cut(text, "<"); base == "MutableLiveData" || base == "LiveData" ||
					base == "MediatorLiveData" {
					found, arg = axlSingleTypeArgument(file, strings.TrimSuffix(text, "()"))
				}
			})
			return found, arg
		}
		return "", false
	}
	init := acrInitializer(file, decl)
	if init == 0 || file.FlatType(init) != "call_expression" {
		return "", false
	}
	name, _, hasReceiver := acrCallCallee(file, init)
	if hasReceiver || (name != "MutableLiveData" && name != "MediatorLiveData") ||
		!acrNameResolvesTo(file, axlLivedataPackage, name) {
		return "", false
	}
	return axlSingleTypeArgument(file, file.FlatNodeText(acrCalleeExpression(file, init))+recyclerCallTypeArgumentText(file, init))
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
// whose condition tests it against null or with `is`, or an earlier statement of an enclosing block that proves it
// non-null (`name!!`, `name ?: return`, requireNotNull/checkNotNull(name), `if (name == null) return`).
func axlNameNullChecked(file *scanner.File, node uint32, name string) bool {
	guard := []string{name + "!=null", "null!=" + name, name + "is", "requireNotNull(" + name, "checkNotNull(" + name}
	proof := []string{name + "!!", name + "?:return", name + "?:throw", "requireNotNull(" + name, "checkNotNull(" + name,
		name + "==null)return", name + "==null)throw", name + "==null){return", name + "==null){throw"}
	compact := func(n uint32) string { return strings.Join(strings.Fields(file.FlatNodeText(n)), "") }
	child := node
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression", "when_entry":
			// The condition is the first named child; a guard counts for the branches after it.
			if cond := flatFirstNamedChild(file, p); cond != 0 && cond != child {
				text := compact(cond)
				for _, g := range guard {
					if strings.Contains(text, g) {
						return true
					}
				}
			}
		case "statements":
			for st := file.FlatFirstChild(p); st != 0 && st != child; st = file.FlatNextSib(st) {
				text := compact(st)
				for _, pr := range proof {
					if strings.Contains(text, pr) {
						return true
					}
				}
			}
		case "function_declaration", "class_body", "source_file":
			return false
		}
		child = p
	}
	return false
}

// axlLastChild returns the last child of a node, named or not (a `null` literal is an unnamed keyword).
func axlLastChild(file *scanner.File, node uint32) uint32 {
	var last uint32
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		last = c
	}
	return last
}

// axlLifecycleWhenMethods are the Lifecycle.whenX extensions of PausingDispatcher.
var axlLifecycleWhenMethods = map[string]bool{"whenCreated": true, "whenStarted": true, "whenResumed": true}

// axlKnownSuspendFunctions are kotlinx.coroutines suspend functions (top-level or members) by name.
var axlKnownSuspendFunctions = map[string]bool{
	"delay": true, "withContext": true, "yield": true, "await": true, "awaitAll": true, "join": true,
	"joinAll": true, "withTimeout": true, "withTimeoutOrNull": true, "coroutineScope": true,
	"supervisorScope": true, "suspendCoroutine": true, "suspendCancellableCoroutine": true, "collect": true,
	"first": true, "single": true, "toList": true, "emit": true, "send": true, "receive": true,
}

// check mirrors LifecycleWhenChecks on `lifecycle.whenStarted { }` (one lambda argument, an androidx.lifecycle
// import): a try whose block makes a suspend call has its finally block checked for View access, at the call.
func (r *UnsafeLifecycleWhenUsageRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) || acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, _, hasReceiver := acrCallCallee(file, node)
	if !axlLifecycleWhenMethods[name] || !hasReceiver || !acrNameResolvesTo(file, axlLivedataPackage, name) {
		return
	}
	lambda := axlCallTrailingLambda(file, node)
	if lambda == 0 || acrCallValueArgument(file, node, 0, "") != 0 {
		return
	}
	msg := "Unsafe View access from finally/catch block inside of `Lifecycle." + name + "` scope"
	file.FlatWalkNodes(lambda, "try_expression", func(try uint32) {
		if axlInNestedDeclaration(file, try, lambda) {
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
		if finally == 0 || !axlBlockSuspends(ctx, try, finally) {
			return
		}
		file.FlatWalkNodes(finally, "call_expression", func(call uint32) {
			if axlCallAccessesView(ctx, call, 0, map[uint32]bool{}) {
				ctx.EmitAtNode(call, msg)
			}
		})
		file.FlatWalkNodes(finally, "assignment", func(a uint32) {
			if axlAssignmentAccessesView(ctx, a) {
				ctx.EmitAtNode(a, msg)
			}
		})
	})
}

// axlInNestedDeclaration reports whether node sits in a function or class declared inside root.
func axlInNestedDeclaration(file *scanner.File, node, root uint32) bool {
	for p, ok := file.FlatParent(node); ok && p != root; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_literal", "anonymous_function":
			return true
		}
	}
	return false
}

// axlBlockSuspends reports whether the try block (everything in try before finally) calls a suspend function.
func axlBlockSuspends(ctx *api.Context, try, finally uint32) bool {
	file := ctx.File
	found := false
	file.FlatWalkNodes(try, "call_expression", func(call uint32) {
		if found || file.FlatStartByte(call) >= file.FlatStartByte(finally) {
			return
		}
		name, _, _ := acrCallCallee(file, call)
		if axlKnownSuspendFunctions[name] && len(acrFunctionDecls(file, name)) == 0 {
			found = true
			return
		}
		for _, fn := range acrFunctionDecls(file, name) {
			if acrFunctionHasModifier(file, fn, "suspend") {
				found = true
			}
		}
	})
	return found
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
