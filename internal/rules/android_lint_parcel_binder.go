package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	parcelClass = "android.os.Parcel"
	binderClass = "android.os.Binder"
)

// parcelClassLoaderMethods are the Parcel readers that take an optional class loader. The value says whether the
// reader also exists without any argument.
var parcelClassLoaderMethods = map[string]bool{
	"readParcelable": false, "readParcelableArray": false, "readBundle": true, "readArray": false,
	"readSparseArray": false, "readValue": false, "readPersistableBundle": true,
}

var parcelKnownNames = map[string]struct{}{"Parcel": {}}

// ---- ParcelClassLoader ----

// ParcelClassLoaderRule mirrors Lint's ParcelClassLoader: a Parcel reader called without a class loader or with a
// literal null one.
type ParcelClassLoaderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *ParcelClassLoaderRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ParcelClassLoaderRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	zeroArgs, known := parcelClassLoaderMethods[call.Name]
	if !ok || !known {
		return
	}
	var msg string
	switch n := lintValueArgumentCount(file, call); {
	case n == 0 && zeroArgs:
		msg = fmt.Sprintf("Using the default class loader will not work if you are restoring your own classes. "+
			"Consider using for example `%s(getClass().getClassLoader())` instead.", call.Name)
	case n == 1 && lintArgIsNullLiteral(file, call, 0):
		msg = "Passing null here (to use the default class loader) will not work if you are restoring your own classes. " +
			"Consider using for example `getClass().getClassLoader()` instead."
	default:
		return
	}
	if !parcelCallOnParcel(ctx, call) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, lintCallNameNode(file, ctx.Idx), "", msg))
}

// lintValueArgumentCount counts the value arguments of a call; a Kotlin trailing lambda is one of them.
func lintValueArgumentCount(file *scanner.File, call lintCall) int {
	n := len(call.Args)
	if file.FlatType(call.Idx) == "call_expression" && flatCallTrailingLambda(file, call.Idx) != 0 {
		n++
	}
	return n
}

// lintArgIsNullLiteral reports whether argument i is a bare `null`, with an optional Kotlin label. A parenthesized or
// cast null is not a literal for Lint.
func lintArgIsNullLiteral(file *scanner.File, call lintCall, i int) bool {
	if file.Language == scanner.LangJava {
		return i < len(call.Args) && file.FlatType(call.Args[i]) == "null_literal"
	}
	args := flatCallKeyArguments(file, call.Idx)
	n := 0
	for arg := file.FlatFirstChild(args); args != 0 && arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if n == i {
			last := uint32(0)
			for c := file.FlatFirstChild(arg); c != 0; c = file.FlatNextSib(c) {
				last = c
			}
			return last != 0 && file.FlatType(last) == "null"
		}
		n++
	}
	return false
}

// parcelCallOnParcel reports whether the call's receiver, or its implicit lambda receiver, is an android.os.Parcel.
func parcelCallOnParcel(ctx *api.Context, call lintCall) bool {
	if verdict, resolved := lintResolvedOwnerExtends(ctx, call, parcelClass); resolved {
		return verdict
	}
	file := ctx.File
	if call.ReceiverNode == 0 {
		return lintImplicitReceiverIsParcel(ctx, call)
	}
	recv := lintUnwrapReceiver(file, call.ReceiverNode)
	if lintIsParcelObtain(ctx, recv) {
		return true
	}
	return lintCallIsMemberOf(ctx, lintWithReceiver(file, call, recv), parcelClass, parcelKnownNames)
}

// lintUnwrapReceiver strips parentheses and Kotlin `!!` from a receiver expression.
func lintUnwrapReceiver(file *scanner.File, idx uint32) uint32 {
	for idx != 0 {
		switch file.FlatType(idx) {
		case "parenthesized_expression":
			inner := flatUnwrapParenExpr(file, idx)
			if inner == idx {
				return idx
			}
			idx = inner
		case "postfix_expression":
			first := file.FlatNamedChild(idx, 0)
			if first == 0 || !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(idx)), "!!") {
				return idx
			}
			idx = first
		default:
			return idx
		}
	}
	return idx
}

// lintWithReceiver returns the call described with another receiver expression.
func lintWithReceiver(file *scanner.File, call lintCall, recv uint32) lintCall {
	call.ReceiverNode = recv
	call.ReceiverText = strings.TrimSpace(file.FlatNodeText(recv))
	return call
}

// lintIsParcelObtain reports whether the expression is the `Parcel.obtain()` factory call.
func lintIsParcelObtain(ctx *api.Context, idx uint32) bool {
	inner, ok := lintCallAt(ctx.File, idx)
	return ok && inner.Name == "obtain" && lintTypeRefMatches(ctx.File, inner.ReceiverText, parcelClass)
}

// lintImplicitReceiverIsParcel handles `parcel.apply { readBundle() }`, `run`, and `with(parcel) { ... }`, whose lambda
// receiver is the Parcel.
func lintImplicitReceiverIsParcel(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	recv, ok := lintThisBinding(file, call.Idx)
	if !ok {
		return false
	}
	recv = lintUnwrapReceiver(file, recv)
	if lintIsParcelObtain(ctx, recv) {
		return true
	}
	probe := lintWithReceiver(file, lintCall{File: file, Idx: call.Idx, Name: call.Name}, recv)
	return lintCallIsMemberOf(ctx, probe, parcelClass, parcelKnownNames)
}

// ---- BinderGetCallingInMainThread ----

// binderMainThreadMethods lists, per framework class, the lifecycle methods that run on the main thread.
var binderMainThreadMethods = []struct {
	classes []string
	known   map[string]struct{}
	methods map[string]bool
}{
	{
		classes: []string{"android.app.Activity"},
		known: lintMergeNames(nil, []string{
			"Activity", "AppCompatActivity", "FragmentActivity", "ComponentActivity", "ListActivity", "TabActivity",
			"PreferenceActivity", "ExpandableListActivity", "LauncherActivity", "NativeActivity", "AliasActivity",
		}),
		methods: map[string]bool{"onCreate": true, "onRestart": true, "onStart": true},
	},
	{
		classes: []string{"android.app.Service"},
		known:   androidKnownServiceSimpleNames,
		methods: map[string]bool{"onCreate": true, "onBind": true, "onRebind": true},
	},
	{
		classes: []string{"android.app.Fragment", "androidx.fragment.app.Fragment"},
		known: lintMergeNames(nil, []string{
			"Fragment", "DialogFragment", "ListFragment", "PreferenceFragment", "PreferenceFragmentCompat",
			"BottomSheetDialogFragment", "AppCompatDialogFragment", "MapFragment", "SupportMapFragment",
		}),
		methods: map[string]bool{"onAttach": true, "onCreate": true, "onCreateView": true, "onStart": true, "onViewCreated": true},
	},
}

var binderKnownNames = map[string]struct{}{"Binder": {}}

// BinderGetCallingInMainThreadRule mirrors Lint's BinderGetCallingInMainThread: Binder.getCallingUid or getCallingPid
// inside an Activity, Service or Fragment lifecycle method that runs on the main thread.
type BinderGetCallingInMainThreadRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *BinderGetCallingInMainThreadRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *BinderGetCallingInMainThreadRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || (call.Name != "getCallingUid" && call.Name != "getCallingPid") {
		return
	}
	method, cls, ok := lintEnclosingMethod(file, ctx.Idx)
	if !ok {
		return
	}
	name := lintDeclaredMethodName(file, method)
	if name == 0 || !binderLifecycleMethod(ctx, cls, file.FlatNodeText(name)) || !binderCallOnBinder(ctx, call) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		fmt.Sprintf("Binder.%s() should not be used inside %s()", call.Name, file.FlatNodeText(name))))
}

// lintEnclosingMethod returns the method that directly contains idx and its class. Lambdas and Kotlin local functions
// belong to the outer method; constructors, initializers and accessors have no method here.
func lintEnclosingMethod(file *scanner.File, idx uint32) (method, cls uint32, ok bool) {
	for cur, has := file.FlatParent(idx); has; cur, has = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "method_declaration":
			cls, ok = lintDirectClassOf(file, cur)
			return cur, cls, ok
		case "function_declaration":
			if parent, hasParent := file.FlatParent(cur); hasParent && file.FlatType(parent) == "class_body" {
				cls, ok = lintDirectClassOf(file, cur)
				return cur, cls, ok
			}
		case "class_body", "source_file", "program":
			return 0, 0, false
		}
	}
	return 0, 0, false
}

// binderLifecycleMethod reports whether the class strictly extends a framework class for which name is a main-thread
// lifecycle method.
func binderLifecycleMethod(ctx *api.Context, cls uint32, name string) bool {
	for _, entry := range binderMainThreadMethods {
		if !entry.methods[name] {
			continue
		}
		for _, fqn := range entry.classes {
			if lintClassExtends(ctx, cls, fqn, entry.known) {
				return true
			}
		}
	}
	return false
}

// binderCallOnBinder reports whether the call resolves to a member of android.os.Binder.
func binderCallOnBinder(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if verdict, resolved := lintResolvedOwnerExtends(ctx, call, binderClass); resolved {
		return verdict
	}
	if call.ReceiverNode == 0 {
		imp := lintImportsOf(file)
		if imp.bySimple[call.Name] == binderClass+"."+call.Name {
			return true
		}
		for _, star := range imp.stars {
			if star == binderClass {
				return true
			}
		}
	} else if lintTypeRefMatches(file, call.ReceiverText, binderClass) {
		return true
	}
	return lintCallIsMemberOf(ctx, call, binderClass, binderKnownNames)
}

// parcelMethodNames lists the Parcel readers checked by ParcelClassLoader.
func parcelMethodNames() map[string]bool {
	names := make(map[string]bool, len(parcelClassLoaderMethods))
	for name := range parcelClassLoaderMethods {
		names[name] = true
	}
	return names
}
