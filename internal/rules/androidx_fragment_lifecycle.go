package rules

// Ports of the lifecycle-owner checks bundled in androidx.fragment 1.9.1 (fragment-lint):
// UnsafeFragmentLifecycleObserverDetector (FragmentLiveDataObserve, FragmentBackPressedCallback,
// FragmentAddMenuProvider) and UnsafeRepeatOnLifecycleDetector.

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// FragmentLiveDataObserveRule mirrors FragmentLiveDataObserve: LiveData.observe with the Fragment as LifecycleOwner
// from a view lifecycle method.
type FragmentLiveDataObserveRule struct {
	FlatDispatchBase
	AndroidRule
}

// FragmentBackPressedCallbackRule mirrors FragmentBackPressedCallback: OnBackPressedDispatcher.addCallback with the
// Fragment as LifecycleOwner from a view lifecycle method.
type FragmentBackPressedCallbackRule struct {
	FlatDispatchBase
	AndroidRule
}

// FragmentAddMenuProviderRule mirrors FragmentAddMenuProvider: MenuHost.addMenuProvider with the Fragment as
// LifecycleOwner from a view lifecycle method.
type FragmentAddMenuProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// UnsafeRepeatOnLifecycleDetectorRule mirrors UnsafeRepeatOnLifecycleDetector: repeatOnLifecycle in a Fragment's view
// lifecycle method whose launch scope does not mention viewLifecycleOwner.
type UnsafeRepeatOnLifecycleDetectorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receivers are proven from same-file declarations or resolved
// call targets, and helpers are followed only within the file.
func (r *FragmentLiveDataObserveRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see FragmentLiveDataObserveRule.
func (r *FragmentBackPressedCallbackRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see FragmentLiveDataObserveRule.
func (r *FragmentAddMenuProviderRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a high base confidence: like Lint, the check reads names and source text.
func (r *UnsafeRepeatOnLifecycleDetectorRule) Confidence() float64 { return api.ConfidenceHigh }

func registerAndroidXFragmentLifecycleChecks() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsResolver
	classNodes := []string{"class_declaration"}
	facts := &api.JavaFactProfile{ReceiverTypesForCallees: []string{"observe", "addCallback", "addMenuProvider"}}
	const brief = "Use getViewLifecycleOwner() as the LifecycleOwner instead of a Fragment instance."

	live := &FragmentLiveDataObserveRule{AndroidRule: lintAndroidRule("FragmentLiveDataObserve",
		"Use getViewLifecycleOwner() as the LifecycleOwner instead of a Fragment instance when observing a LiveData object.",
		ALCCorrectness, ALSError, 5)}
	registerLintRule(live.AndroidRule, live, live.Confidence(), func(ctx *api.Context) {
		fragmentOwnerCheck(ctx, fragmentOwnerObserve)
	}, classNodes, needs, facts)

	back := &FragmentBackPressedCallbackRule{AndroidRule: lintAndroidRule("FragmentBackPressedCallback", brief,
		ALCCorrectness, ALSError, 5)}
	registerLintRule(back.AndroidRule, back, back.Confidence(), func(ctx *api.Context) {
		fragmentOwnerCheck(ctx, fragmentOwnerAddCallback)
	}, classNodes, needs, facts)

	menu := &FragmentAddMenuProviderRule{AndroidRule: lintAndroidRule("FragmentAddMenuProvider", brief,
		ALCCorrectness, ALSError, 5)}
	registerLintRule(menu.AndroidRule, menu, menu.Confidence(), func(ctx *api.Context) {
		fragmentOwnerCheck(ctx, fragmentOwnerAddMenuProvider)
	}, classNodes, needs, facts)

	repeat := &UnsafeRepeatOnLifecycleDetectorRule{AndroidRule: lintAndroidRule("UnsafeRepeatOnLifecycleDetector",
		"RepeatOnLifecycle should be used with viewLifecycleOwner in Fragments.", ALCCorrectness, ALSError, 5)}
	registerLintRule(repeat.AndroidRule, repeat, repeat.Confidence(), repeat.check,
		[]string{"call_expression", "method_invocation"}, 0, nil)
}

const (
	fragmentClass       = "androidx.fragment.app.Fragment"
	dialogFragmentClass = "androidx.fragment.app.DialogFragment"
	// fragmentMaxHelperDepth bounds how deep helper calls are followed, as Lint does outside the IDE.
	fragmentMaxHelperDepth = 20
)

// fragmentViewLifecycleMethods are the Fragment callbacks from which the view lifecycle is available.
var fragmentViewLifecycleMethods = map[string]bool{
	"onCreateView": true, "onViewCreated": true, "onActivityCreated": true, "onViewStateRestored": true,
}

// fragmentKnown and dialogFragmentKnown are library classes known to extend Fragment and DialogFragment, for when the
// resolver has no hierarchy for them.
var (
	fragmentKnown = map[string]struct{}{
		"Fragment": {}, "ListFragment": {}, "PreferenceFragmentCompat": {}, "NavHostFragment": {}, "DialogFragment": {},
		"AppCompatDialogFragment": {}, "BottomSheetDialogFragment": {}, "SupportMapFragment": {},
		"BrowseSupportFragment": {}, "VerticalGridSupportFragment": {}, "RowsSupportFragment": {},
	}
	dialogFragmentKnown = map[string]struct{}{
		"DialogFragment": {}, "AppCompatDialogFragment": {}, "BottomSheetDialogFragment": {},
	}
)

// fragmentOwnerKind is one of the unsafe LifecycleOwner APIs of UnsafeFragmentLifecycleObserverDetector.
type fragmentOwnerKind int

const (
	fragmentOwnerNone fragmentOwnerKind = iota
	fragmentOwnerObserve
	fragmentOwnerAddCallback
	fragmentOwnerAddMenuProvider
)

// fragmentClassIsViewFragment reports whether the class-like declaration extends Fragment without being a DialogFragment.
func fragmentClassIsViewFragment(ctx *api.Context, cls uint32) bool {
	return lintClassExtends(ctx, cls, fragmentClass, fragmentKnown) && !lintClassExtends(ctx, cls, dialogFragmentClass, dialogFragmentKnown)
}

// fragmentOwnerVisit walks the view lifecycle methods of one Fragment class and the same-file helpers they call.
type fragmentOwnerVisit struct {
	ctx      *api.Context
	kind     fragmentOwnerKind
	fragment string
	method   string
	// active holds the helper calls being followed, as Lint's visitedMethods does.
	active   map[uint32]bool
	reported map[uint32]bool
}

// fragmentOwnerCheck runs UnsafeFragmentLifecycleObserverDetector over the dispatched class for one API.
func fragmentOwnerCheck(ctx *api.Context, kind fragmentOwnerKind) {
	file := ctx.File
	cls := ctx.Idx
	if !lintClassExtends(ctx, cls, fragmentClass, fragmentKnown) {
		return
	}
	v := &fragmentOwnerVisit{ctx: ctx, kind: kind, fragment: fragmentClassName(file, cls), active: map[uint32]bool{},
		reported: map[uint32]bool{}}
	for _, fn := range fragmentClassMethods(file, cls) {
		name := fragmentFunctionName(file, fn)
		if !fragmentViewLifecycleMethods[name] {
			continue
		}
		v.method = name
		v.walk(fragmentFunctionBody(file, fn))
	}
}

// fragmentClassMethods lists the functions declared directly in the class body.
func fragmentClassMethods(file *scanner.File, cls uint32) []uint32 {
	var out []uint32
	for child := file.FlatFirstChild(cls); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "class_body", "enum_class_body":
			for member := file.FlatFirstChild(child); member != 0; member = file.FlatNextSib(member) {
				switch file.FlatType(member) {
				case "function_declaration", "method_declaration":
					out = append(out, member)
				}
			}
		}
	}
	return out
}

func fragmentFunctionName(file *scanner.File, fn uint32) string {
	kind := "simple_identifier"
	if file.FlatType(fn) == "method_declaration" {
		kind = "identifier"
	}
	if id, ok := file.FlatFindChild(fn, kind); ok {
		return file.FlatNodeText(id)
	}
	return ""
}

func fragmentFunctionBody(file *scanner.File, fn uint32) uint32 {
	for _, kind := range []string{"function_body", "block"} {
		if body, ok := file.FlatFindChild(fn, kind); ok {
			return body
		}
	}
	return 0
}

// walk visits every call below node in source order, checking unsafe owner calls and following helpers.
func (v *fragmentOwnerVisit) walk(node uint32) {
	if node == 0 {
		return
	}
	file := v.ctx.File
	file.FlatWalkAllNodes(node, func(n uint32) {
		switch file.FlatType(n) {
		case "call_expression", "method_invocation":
			if !fragmentIsLambdaWrapper(file, n) {
				v.visitCall(n)
			}
		}
	})
}

func (v *fragmentOwnerVisit) visitCall(idx uint32) {
	if v.active[idx] || len(v.active) > fragmentMaxHelperDepth {
		return
	}
	call, ok := lintCallAt(v.ctx.File, idx)
	// Lint stops at Compose boundaries, which often hold large UI trees.
	if !ok || call.Name == "setContent" {
		return
	}
	if v.checkCall(call) {
		return
	}
	helper := v.helperDeclaration(call)
	if helper == 0 || fragmentHasComposableAnnotation(v.ctx.File, helper) {
		return
	}
	v.active[idx] = true
	v.walk(fragmentFunctionBody(v.ctx.File, helper))
	delete(v.active, idx)
}

// fragmentIsLambdaWrapper reports whether a Kotlin call node only adds a trailing lambda to the call that is its first
// child (`f(x) { }` parses as a call of the call `f(x)`); the inner call carries the name and arguments.
func fragmentIsLambdaWrapper(file *scanner.File, idx uint32) bool {
	inner := file.FlatFirstChild(idx)
	return file.FlatType(idx) == "call_expression" && inner != 0 && file.FlatType(inner) == "call_expression"
}

// fragmentHasTrailingLambda reports whether the call takes a trailing lambda, directly or through its wrapper.
func fragmentHasTrailingLambda(file *scanner.File, idx uint32) bool {
	if flatCallTrailingLambda(file, idx) != 0 {
		return true
	}
	parent, ok := file.FlatParent(idx)
	return ok && fragmentIsLambdaWrapper(file, parent) && file.FlatFirstChild(parent) == idx
}

// fragmentOwnerArgument returns the LifecycleOwner argument of an unsafe call, or 0.
func fragmentOwnerArgument(file *scanner.File, call lintCall, kind fragmentOwnerKind) uint32 {
	trailing := fragmentHasTrailingLambda(file, call.Idx)
	switch kind {
	case fragmentOwnerObserve:
		// observe(owner, observer), with the observer possibly a trailing lambda.
		if len(call.Args) == 2 || (len(call.Args) == 1 && trailing) {
			return call.Arg(0, "owner")
		}
	case fragmentOwnerAddCallback:
		// The member addCallback(owner, callback). The Kotlin addCallback(owner) { } form is an extension in
		// OnBackPressedDispatcherKt, which Lint's owner check does not match.
		if len(call.Args) == 2 && !trailing {
			return call.Arg(0, "owner")
		}
	case fragmentOwnerAddMenuProvider:
		// addMenuProvider(provider, owner[, state]).
		if len(call.Args) >= 2 {
			return call.Arg(1, "owner")
		}
	}
	return 0
}

// fragmentOwnerKindOf names the unsafe API the call is, using its resolved target when the oracle or Java facts have
// one and receiver evidence otherwise.
func fragmentOwnerKindOf(ctx *api.Context, call lintCall) fragmentOwnerKind {
	var kind fragmentOwnerKind
	var owners []string
	switch call.Name {
	case "observe":
		kind, owners = fragmentOwnerObserve, []string{"androidx.lifecycle.LiveData", "androidx.lifecycle.LiveDataKt"}
	case "addCallback":
		kind, owners = fragmentOwnerAddCallback, []string{"androidx.activity.OnBackPressedDispatcher", "androidx.activity.OnBackPressedDispatcherKt"}
	case "addMenuProvider":
		kind, owners = fragmentOwnerAddMenuProvider, []string{"androidx.core.view.MenuHost", "androidx.core.view.MenuHostHelper"}
	default:
		return fragmentOwnerNone
	}
	if owner, resolved := fragmentResolvedOwner(ctx, call); resolved {
		for _, want := range owners {
			if owner == want || lintHierarchyReaches(ctx.Resolver, owner, want, nil, map[string]bool{}) ||
				(kind == fragmentOwnerAddMenuProvider && fragmentMenuHostClasses[owner[strings.LastIndex(owner, ".")+1:]]) {
				return kind
			}
		}
		return fragmentOwnerNone
	}
	if call.ReceiverNode == 0 {
		return fragmentOwnerNone
	}
	if kind == fragmentOwnerObserve {
		if fragmentReceiverIsLiveData(ctx, call) {
			return kind
		}
		return fragmentOwnerNone
	}
	// The remaining proofs read receiver names, so a same-file function of that name may be what the call resolves to.
	if fragmentFileDeclaresFunction(ctx.File, call.Name) {
		return fragmentOwnerNone
	}
	switch kind {
	case fragmentOwnerAddCallback:
		if last := fragmentReceiverLastSegment(call); last == "onBackPressedDispatcher" || last == "getOnBackPressedDispatcher()" {
			return kind
		}
		if lintTypeRefMatches(ctx.File, lintReceiverClassText(ctx.File, call), "androidx.activity.OnBackPressedDispatcher") {
			return kind
		}
	case fragmentOwnerAddMenuProvider:
		if fragmentReceiverIsMenuHost(ctx, call) {
			return kind
		}
	}
	return fragmentOwnerNone
}

// fragmentMenuHostClasses are framework classes that implement MenuHost by declaring addMenuProvider.
var fragmentMenuHostClasses = map[string]bool{
	"ComponentActivity": true, "FragmentActivity": true, "AppCompatActivity": true, "MenuHost": true, "MenuHostHelper": true,
}

func fragmentResolvedOwner(ctx *api.Context, call lintCall) (string, bool) {
	if fact, ok := javaSemanticCallFact(ctx, call.Idx); ok && fact.MethodOwner != "" {
		return fact.MethodOwner, true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call.Idx)
	if !ok || !target.Resolved {
		return "", false
	}
	owner := strings.ReplaceAll(target.QualifiedName, "#", ".")
	if dot := strings.LastIndex(owner, "."); dot >= 0 {
		owner = owner[:dot]
	}
	return owner, true
}

// fragmentFileDeclaresFunction reports whether the file declares a function of that name, which a call may resolve to
// instead of the library API.
func fragmentFileDeclaresFunction(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if !found && fragmentFunctionName(file, fn) == name {
			found = true
		}
	})
	if !found {
		file.FlatWalkNodes(0, "method_declaration", func(fn uint32) {
			if !found && fragmentFunctionName(file, fn) == name {
				found = true
			}
		})
	}
	return found
}

func fragmentReceiverLastSegment(call lintCall) string {
	text := strings.TrimSpace(call.ReceiverText)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "!!"), "?")
	return text[strings.LastIndex(text, ".")+1:]
}

// fragmentLiveDataKnown are the LiveData classes of androidx.lifecycle.
var fragmentLiveDataKnown = map[string]struct{}{"LiveData": {}, "MutableLiveData": {}, "MediatorLiveData": {}}

// fragmentReceiverIsLiveData proves a LiveData receiver from this file: a declared or constructed variable, or a
// property of that name declared in the file with a LiveData type. A receiver typed only in another file is not
// proven (the oracle resolves it when it runs).
func fragmentReceiverIsLiveData(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if javaSemanticCallReceiverTypeMatches(ctx, call.Idx, "androidx.lifecycle.LiveData") {
		return true
	}
	isLiveData := func(typ string) bool {
		typ = lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(typ), "?"))
		return typ != "" && lintTypeNameReaches(ctx, typ, "androidx.lifecycle.LiveData", fragmentLiveDataKnown, 0) &&
			(lintTypeRefMatches(file, typ, "androidx.lifecycle."+typ[strings.LastIndex(typ, ".")+1:]) || lintSameFileClass(file, typ) != 0)
	}
	if isLiveData(lintReceiverClassText(file, call)) {
		return true
	}
	last := fragmentReceiverLastSegment(call)
	if !lintIdentOnly(last) || last == call.ReceiverText {
		return false
	}
	// `model.items` with `val items: LiveData<T>` declared in this file.
	return isLiveData(fragmentPropertyTypeText(file, last))
}

// fragmentPropertyTypeText returns the type of the properties called name in this file, from their declared type or
// constructor initializer, or "" when there are none or they disagree.
func fragmentPropertyTypeText(file *scanner.File, name string) string {
	typ, consistent := "", true
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		vd, ok := file.FlatFindChild(decl, "variable_declaration")
		if !ok || lintDeclaredName(file, vd) != name {
			return
		}
		t := lintChildTypeText(file, vd)
		if t == "" {
			t = lintConstructorCallType(file, lintInitializerAfterEquals(file, decl))
		}
		t = lintTrimGenerics(t)
		switch {
		case t == "":
			consistent = false
		case typ == "":
			typ = t
		case typ != t:
			consistent = false
		}
	})
	if !consistent {
		return ""
	}
	return typ
}

// fragmentReceiverIsMenuHost proves a MenuHost receiver: the hosting activity, a MenuHost cast, or a MenuHost or
// activity typed variable.
func fragmentReceiverIsMenuHost(ctx *api.Context, call lintCall) bool {
	switch fragmentReceiverLastSegment(call) {
	case "requireActivity()", "activity", "getActivity()", "requireActivity":
		return true
	}
	typ := lintReceiverClassText(ctx.File, call)
	if typ == "" {
		return false
	}
	simple := typ[strings.LastIndex(typ, ".")+1:]
	if !fragmentMenuHostClasses[simple] {
		return false
	}
	for _, pkg := range []string{"androidx.core.view.", "androidx.activity.", "androidx.fragment.app.", "androidx.appcompat.app."} {
		if lintTypeRefMatches(ctx.File, typ, pkg+simple) {
			return true
		}
	}
	return false
}

// checkCall reports an unsafe owner call whose LifecycleOwner argument is a Fragment instance. It returns true when
// the call is such a call, reported now or already.
func (v *fragmentOwnerVisit) checkCall(call lintCall) bool {
	ctx, file := v.ctx, v.ctx.File
	kind := fragmentOwnerKindOf(ctx, call)
	if kind == fragmentOwnerNone {
		return false
	}
	arg := fragmentOwnerArgument(file, call, kind)
	if arg == 0 {
		return false
	}
	ownerClass, isFragment := fragmentArgumentClass(ctx, call.Idx, arg)
	if !isFragment {
		return false
	}
	if kind != v.kind {
		return true
	}
	container, _ := fragmentEnclosingClass(file, call.Idx)
	if ownerClass != 0 && ownerClass == container {
		if v.reported[arg] {
			return false
		}
		v.reported[arg] = true
		fix := "viewLifecycleOwner"
		if file.Language == scanner.LangJava {
			fix = "getViewLifecycleOwner()"
		}
		ctx.Emit(lintFindingAtNode(ctx, arg, "", "Use "+fix+" as the LifecycleOwner."))
		return true
	}
	if v.reported[call.Idx] {
		return false
	}
	v.reported[call.Idx] = true
	ctx.Emit(lintFindingAtNode(ctx, call.Idx, "", "Unsafe call to "+call.Name+
		" with Fragment instance as LifecycleOwner from "+v.fragment+"."+v.method+"."))
	return true
}

// fragmentEnclosingClass returns the nearest class-like declaration around idx.
func fragmentEnclosingClass(file *scanner.File, idx uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "object_literal":
			return cur, true
		case "object_creation_expression":
			if _, hasBody := file.FlatFindChild(cur, "class_body"); hasBody {
				return cur, true
			}
		}
	}
	return 0, false
}

// fragmentArgumentClass reports whether the argument is a Fragment instance that is not a DialogFragment, and the
// same-file class declaration it is an instance of (0 for a library or other-file class).
func fragmentArgumentClass(ctx *api.Context, call, arg uint32) (uint32, bool) {
	file := ctx.File
	arg = flatUnwrapParenExpr(file, arg)
	text := strings.TrimSpace(file.FlatNodeText(arg))
	switch {
	case text == "this":
		cls, ok := fragmentThisClass(file, arg)
		if !ok {
			return 0, false
		}
		return cls, fragmentClassIsViewFragment(ctx, cls)
	case strings.HasPrefix(text, "this@"):
		cls := fragmentEnclosingClassNamed(file, arg, strings.TrimPrefix(text, "this@"))
		return cls, cls != 0 && fragmentClassIsViewFragment(ctx, cls)
	case strings.HasSuffix(text, ".this") && file.Language == scanner.LangJava:
		cls := fragmentEnclosingClassNamed(file, arg, strings.TrimSuffix(text, ".this"))
		return cls, cls != 0 && fragmentClassIsViewFragment(ctx, cls)
	case text == "requireParentFragment()" || text == "getParentFragment()" || text == "parentFragment" ||
		text == "parentFragment!!" || text == "requireParentFragment":
		return 0, true
	}
	if !lintIdentOnly(text) {
		return 0, false
	}
	typ := lintTrimGenerics(strings.TrimSuffix(lintDeclaredTypeText(file, call, text), "?"))
	if typ == "" {
		return 0, false
	}
	if local := lintSameFileClass(file, typ); local != 0 {
		return local, fragmentClassIsViewFragment(ctx, local)
	}
	return 0, lintTypeNameReaches(ctx, typ, fragmentClass, fragmentKnown, 0) &&
		!lintTypeNameReaches(ctx, typ, dialogFragmentClass, dialogFragmentKnown, 0)
}

// fragmentThisClass returns the class `this` refers to at idx: the nearest enclosing class-like declaration, unless an
// apply/run/with block rebinds it first.
func fragmentThisClass(file *scanner.File, idx uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if fragmentLambdaRebindsThis(file, cur) {
				return 0, false
			}
		case "function_declaration":
			if functionReceiverTypeText(file, cur) != "" {
				return 0, false
			}
		case "class_declaration", "object_declaration", "object_literal":
			return cur, true
		case "object_creation_expression":
			if _, hasBody := file.FlatFindChild(cur, "class_body"); hasBody {
				return cur, true
			}
		}
	}
	return 0, false
}

// fragmentLambdaRebindsThis reports whether the lambda is the block of a scope function with a receiver.
func fragmentLambdaRebindsThis(file *scanner.File, lambda uint32) bool {
	call := lambda
	for i := 0; i < 3; i++ {
		parent, ok := file.FlatParent(call)
		if !ok {
			return false
		}
		call = parent
		if file.FlatType(call) == "call_expression" {
			break
		}
	}
	if file.FlatType(call) != "call_expression" {
		return false
	}
	c, ok := lintCallAt(file, call)
	if inner := file.FlatFirstChild(call); (!ok || c.Name == "") && inner != 0 && file.FlatType(inner) == "call_expression" {
		c, ok = lintCallAt(file, inner)
	}
	if !ok {
		return false
	}
	switch c.Name {
	case "apply", "with", "buildString", "buildList", "buildSet", "buildMap":
		return true
	case "run":
		return c.ReceiverNode != 0
	}
	return false
}

// fragmentClassName returns the simple name of a Kotlin or Java class or object declaration.
func fragmentClassName(file *scanner.File, cls uint32) string {
	for _, kind := range []string{"type_identifier", "simple_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(cls, kind); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// fragmentEnclosingClassNamed returns the enclosing class declaration with that name.
func fragmentEnclosingClassNamed(file *scanner.File, idx uint32, name string) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if t := file.FlatType(cur); (t == "class_declaration" || t == "object_declaration") && fragmentClassName(file, cur) == name {
			return cur
		}
	}
	return 0
}

// helperDeclaration returns the same-file function a call should be followed into, as Lint's isInteresting does: a
// call on the enclosing class itself, or one that passes a Fragment instance. 0 when there is none or it is ambiguous.
func (v *fragmentOwnerVisit) helperDeclaration(call lintCall) uint32 {
	ctx, file := v.ctx, v.ctx.File
	recv := strings.TrimSpace(call.ReceiverText)
	container, hasContainer := fragmentEnclosingClass(file, call.Idx)
	onSelf := (recv == "" || recv == "this") && hasContainer
	passesFragment := false
	for _, arg := range call.Args {
		if _, ok := fragmentArgumentClass(ctx, call.Idx, arg); ok {
			passesFragment = true
			break
		}
	}
	if !onSelf && !passesFragment {
		return 0
	}
	var candidates []uint32
	collect := func(fn uint32) {
		if fragmentFunctionName(file, fn) == call.Name && fragmentFunctionArity(file, fn) == len(call.Args)+fragmentTrailingLambdaCount(file, call.Idx) {
			candidates = append(candidates, fn)
		}
	}
	if onSelf {
		for _, fn := range fragmentClassMethods(file, container) {
			collect(fn)
		}
	} else {
		file.FlatWalkNodes(0, "function_declaration", collect)
		file.FlatWalkNodes(0, "method_declaration", collect)
	}
	if len(candidates) != 1 {
		return 0
	}
	return candidates[0]
}

func fragmentTrailingLambdaCount(file *scanner.File, call uint32) int {
	if fragmentHasTrailingLambda(file, call) {
		return 1
	}
	return 0
}

// fragmentFunctionArity counts the declared parameters of a Kotlin or Java function.
func fragmentFunctionArity(file *scanner.File, fn uint32) int {
	n := 0
	for _, kind := range []string{"function_value_parameters", "formal_parameters"} {
		params, ok := file.FlatFindChild(fn, kind)
		if !ok {
			continue
		}
		for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
			switch file.FlatType(p) {
			case "parameter", "formal_parameter", "spread_parameter":
				n++
			}
		}
	}
	return n
}

// fragmentHasComposableAnnotation reports whether the function is annotated @Composable; Lint does not follow those.
func fragmentHasComposableAnnotation(file *scanner.File, fn uint32) bool {
	mods, ok := file.FlatFindChild(fn, "modifiers")
	return ok && strings.Contains(file.FlatNodeText(mods), "Composable")
}

func (r *UnsafeRepeatOnLifecycleDetectorRule) check(ctx *api.Context) {
	file := ctx.File
	if fragmentIsLambdaWrapper(file, ctx.Idx) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "repeatOnLifecycle" {
		return
	}
	fn, ok := flatEnclosingAncestor(file, ctx.Idx, "function_declaration", "method_declaration")
	if !ok || !fragmentViewLifecycleMethods[fragmentFunctionName(file, fn)] {
		return
	}
	cls, ok := fragmentEnclosingClass(file, ctx.Idx)
	if !ok || !fragmentClassIsViewFragment(ctx, cls) {
		return
	}
	if fragmentLaunchScopeMentionsViewLifecycleOwner(file, ctx.Idx, fn) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "The repeatOnLifecycle API should be used with viewLifecycleOwner"))
}

// fragmentLaunchScopeMentionsViewLifecycleOwner mirrors Lint's launch scope test: starting at the receiver of the call
// that encloses repeatOnLifecycle (typically `scope.launch { }`), it looks for viewLifecycleOwner, case-insensitively,
// in the source text of that receiver and of each enclosing expression up to the method body. A call without such a
// receiver fails the test. Like Lint, a mention anywhere in the method body, comments included, passes it.
func fragmentLaunchScopeMentionsViewLifecycleOwner(file *scanner.File, idx, fn uint32) bool {
	var enclosing uint32
	for prev, cur, ok := idx, uint32(0), true; ok && prev != fn; prev = cur {
		cur, ok = file.FlatParent(prev)
		if !ok || cur == fn {
			break
		}
		if t := file.FlatType(cur); t == "call_expression" || t == "method_invocation" {
			// The wrapper that only adds repeatOnLifecycle's own trailing lambda is not an enclosing call.
			if fragmentIsLambdaWrapper(file, cur) && file.FlatFirstChild(cur) == prev {
				continue
			}
			enclosing = cur
			break
		}
	}
	if enclosing == 0 {
		return false
	}
	outer, ok := lintCallAt(file, enclosing)
	if inner := file.FlatFirstChild(enclosing); (!ok || outer.Name == "") && inner != 0 && file.FlatType(inner) == "call_expression" {
		outer, ok = lintCallAt(file, inner)
	}
	if !ok || outer.ReceiverNode == 0 {
		return false
	}
	body := fragmentFunctionBody(file, fn)
	if body == 0 {
		body = fn
	}
	return strings.Contains(strings.ToLower(file.FlatNodeText(body)), "viewlifecycleowner")
}
