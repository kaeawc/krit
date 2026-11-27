package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of ComposableLifecycleCurrentStateDetector (lifecycle-runtime-compose), ViewModelConstructorInComposableDetector
// (lifecycle-viewmodel-compose), TestLifecycleOwnerInCoroutineDetector (lifecycle-runtime-testing) and
// RepeatOnLifecycleDetector (lifecycle-runtime) from androidx.lifecycle 2.11.1.

const axlLifecyclePackage = "androidx.lifecycle"

// LifecycleCurrentStateInCompositionRule ports LifecycleCurrentStateInComposition: Lifecycle.currentState read in
// composition, which does not observe changes.
type LifecycleCurrentStateInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// ViewModelConstructorInComposableRule ports ViewModelConstructorInComposable: a ViewModel constructed directly in
// composition instead of through viewModel().
type ViewModelConstructorInComposableRule struct {
	FlatDispatchBase
	BaseRule
}

// TestLifecycleOwnerInCoroutineRule ports TestLifecycleOwnerInCoroutine: setting currentState inside runTest, whose
// blocking setter hangs inside a coroutine.
type TestLifecycleOwnerInCoroutineRule struct {
	FlatDispatchBase
	BaseRule
}

// RepeatOnLifecycleWrongUsageRule ports RepeatOnLifecycleWrongUsage: repeatOnLifecycle reached from onStart or
// onResume of a Fragment or Activity, which starts a new repeating block every time.
type RepeatOnLifecycleWrongUsageRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *LifecycleCurrentStateInCompositionRule) fileMentions() []string {
	return []string{"currentState"}
}
func (r *ViewModelConstructorInComposableRule) fileMentions() []string {
	return []string{"androidx.compose", "ViewModel"}
}
func (r *TestLifecycleOwnerInCoroutineRule) fileMentions() []string {
	return []string{"kotlinx.coroutines.test", "currentState"}
}
func (r *RepeatOnLifecycleWrongUsageRule) fileMentions() []string {
	return []string{"repeatOnLifecycle"}
}

// Confidence is medium: the receiver and composition proofs are source-level.
func (r *LifecycleCurrentStateInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence is medium: the ViewModel hierarchy and composition proofs are source-level.
func (r *ViewModelConstructorInComposableRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LifecycleCurrentStateInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "LifecycleCurrentStateInComposition", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The receiver must be `<owner>.lifecycle` or a value declared or initialized as a Lifecycle or " +
				"LifecycleRegistry. Lint's quick fix (currentStateAsState().value plus an import) is not ported.",
		}, acrCompositionLimitations...)}
}

func (r *ViewModelConstructorInComposableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ViewModelConstructorInComposable", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"The constructed class must be declared in the same file and extend androidx.lifecycle.ViewModel through " +
				"same-file classes and imports; a ViewModel declared in another file is not checked until the per-file " +
				"cache tracks cross-file inputs (#1778).",
		}, acrCompositionLimitations...)}
}

func (r *TestLifecycleOwnerInCoroutineRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TestLifecycleOwnerInCoroutine", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Follows the detector source and its tests: the Lint CLI 9.4 reported nothing for this ID against the " +
				"released lifecycle-runtime-testing 2.11.1 and kotlinx-coroutines-test 1.10.2 classes.",
			"As in the detector, the last assignment in the runTest body decides: `x.currentState = " +
				"Lifecycle.State.X` reports, and a later assignment of another constant clears it.",
		}}
}

func (r *RepeatOnLifecycleWrongUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RepeatOnLifecycleWrongUsage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Calls are followed into functions declared in the same file; Lint also follows them into other files.",
			"The Fragment or Activity ancestry is proven from the file alone (a supertype imported from androidx, or " +
				"a same-file base class); a base class declared in another file is not followed until the per-file cache " +
				"tracks cross-file inputs (#1778).",
		}}
}

// check mirrors ComposableLifecycleCurrentStateDetector: `<lifecycle>.currentState` within composition, at the
// currentState name.
func (r *LifecycleCurrentStateInCompositionRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	suffix := flatLastNamedChild(file, node)
	if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
		return
	}
	name, _ := file.FlatFindChild(suffix, "simple_identifier")
	if name == 0 || !file.FlatNodeTextEquals(name, "currentState") {
		return
	}
	if p, ok := file.FlatParent(node); ok && file.FlatType(p) == "directly_assignable_expression" {
		return
	}
	if !axlExprIsLifecycle(file, file.FlatFirstChild(node), 0) || !acrInComposition(file, node) {
		return
	}
	ctx.EmitAtNode(name, "Lifecycle.currentState should not be called within composition")
}

// axlExprIsLifecycle proves an expression is an androidx.lifecycle.Lifecycle: a `.lifecycle` property read (every
// LifecycleOwner's), or a value declared or initialized as a Lifecycle or LifecycleRegistry.
func axlExprIsLifecycle(file *scanner.File, expr uint32, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "navigation_expression":
		if suffix := flatLastNamedChild(file, expr); suffix != 0 && file.FlatType(suffix) == "navigation_suffix" &&
			file.FlatNodeTextEquals(file.FlatNamedChild(suffix, 0), "lifecycle") {
			return axlLifecyclePropertyIsLifecycle(file, file.FlatFirstChild(expr))
		}
		fallthrough
	case "simple_identifier":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 {
			return false
		}
		if t := acrDeclaredType(file, decl); t != 0 {
			return acrTypeIs(file, t, axlLifecyclePackage, "Lifecycle", "LifecycleRegistry")
		}
		return axlExprIsLifecycle(file, acrInitializer(file, decl), depth+1)
	case "call_expression":
		return acrCallTargets(file, expr, axlLifecyclePackage, "LifecycleRegistry")
	}
	return false
}

// axlLifecycleOwnerTypes are the library LifecycleOwner types whose `lifecycle` is a Lifecycle.
var axlLifecycleOwnerTypes = map[string][]string{
	axlLifecyclePackage:          {"LifecycleOwner", "LifecycleService", "ProcessLifecycleOwner"},
	"androidx.activity":          {"ComponentActivity"},
	"androidx.core.app":          {"ComponentActivity"},
	"androidx.fragment.app":      {"Fragment", "FragmentActivity", "DialogFragment"},
	"androidx.appcompat.app":     {"AppCompatActivity"},
	"androidx.lifecycle.testing": {"TestLifecycleOwner"},
	"androidx.navigation":        {"NavBackStackEntry"},
}

// axlLifecyclePropertyIsLifecycle reports whether `<owner>.lifecycle` is a Lifecycle: the owner's same-file type
// declares a `lifecycle` property typed or initialized as one (or none, while extending a known owner), or the owner
// is a known LifecycleOwner. An owner whose declaration or type is not in the file falls back to the property name,
// which LifecycleOwner defines.
func axlLifecyclePropertyIsLifecycle(file *scanner.File, owner uint32) bool {
	owner = acrUnwrapParens(file, owner)
	if owner == 0 {
		return false
	}
	if nav := strings.Join(strings.Fields(file.FlatNodeText(owner)), ""); strings.HasSuffix(nav, "LocalLifecycleOwner.current") {
		return true
	}
	var typeNode uint32
	switch file.FlatType(owner) {
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, owner)
		if decl == 0 {
			return true
		}
		if typeNode = acrDeclaredType(file, decl); typeNode == 0 {
			init := acrInitializer(file, decl)
			if init == 0 || file.FlatType(init) != "call_expression" {
				return true
			}
			name, _, hasReceiver := acrCallCallee(file, init)
			if hasReceiver {
				return true
			}
			if class := lintSameFileClass(file, name); class != 0 {
				return axlClassLifecycleMember(file, class)
			}
			return true
		}
	default:
		return true
	}
	for pkg, names := range axlLifecycleOwnerTypes {
		if acrTypeIs(file, typeNode, pkg, names...) {
			return true
		}
	}
	if class := acrSameFileClass(file, typeNode); class != 0 {
		return axlClassLifecycleMember(file, class)
	}
	return true
}

// axlClassLifecycleMember reports whether a same-file class's `lifecycle` member is a Lifecycle: a property typed
// or initialized as one, or, with no such member, a class extending a known LifecycleOwner.
func axlClassLifecycleMember(file *scanner.File, class uint32) bool {
	if member := acrClassMember(file, class, "lifecycle"); member != 0 {
		if t := acrDeclaredType(file, member); t != 0 {
			return acrTypeIs(file, t, axlLifecyclePackage, "Lifecycle", "LifecycleRegistry")
		}
		return axlExprIsLifecycle(file, acrInitializer(file, member), 1)
	}
	for pkg, names := range axlLifecycleOwnerTypes {
		if acrClassSupertypeIs(file, class, 0, pkg, names...) {
			return true
		}
	}
	return false
}

// check mirrors ViewModelConstructorInComposableDetector: a constructor call of a ViewModel subclass within
// composition, at the class name.
func (r *ViewModelConstructorInComposableRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	callee := acrCalleeExpression(file, node)
	var nameNode uint32
	var name string
	switch file.FlatType(callee) {
	case "simple_identifier":
		nameNode, name = callee, file.FlatNodeText(callee)
	case "navigation_expression":
		nameNode = flatLastIdentifierNode(file, callee)
		name = strings.Join(strings.Fields(file.FlatNodeText(callee)), "")
	default:
		return
	}
	simple := name[strings.LastIndex(name, ".")+1:]
	if !acrFirstRuneIsUpper(simple) || len(acrFunctionDecls(file, simple)) > 0 ||
		strings.HasPrefix(name, "androidx.compose.") ||
		strings.HasPrefix(fileFactsCache().Imports(file).Aliases[simple], "androidx.compose.") {
		// Composables such as Text and Box are imported from androidx.compose; skip them before the hierarchy walk.
		return
	}
	if name == "ViewModel" || name == "AndroidViewModel" || !axlClassReachesViewModel(ctx, name, simple) {
		return
	}
	if !acrInComposition(file, node) {
		return
	}
	ctx.EmitAtNode(nameNode, "Constructing a view model in a composable")
}

// axlClassReachesViewModel reports whether the class a constructor call names extends androidx.lifecycle.ViewModel,
// proven from this file alone: a same-file class whose supertype chain reaches ViewModel or AndroidViewModel through
// same-file classes and imports. The per-file findings cache is keyed by the file's own content (#1778), so a
// hierarchy declared in another file is not used.
func axlClassReachesViewModel(ctx *api.Context, name, simple string) bool {
	if name != simple {
		return false
	}
	local := lintSameFileClass(ctx.File, simple)
	return local != 0 && axlFileOnlyExtends(ctx.File, local, axlViewModelFQNs, 0)
}

// axlViewModelFQNs are the library ViewModel classes.
var axlViewModelFQNs = map[string]bool{
	axlLifecyclePackage + ".ViewModel": true, axlLifecyclePackage + ".AndroidViewModel": true,
}

// axlLifecycleHostFQNs are the Fragment and ComponentActivity classes RepeatOnLifecycleDetector applies to (the
// classes and their library subclasses).
var axlLifecycleHostFQNs = map[string]bool{
	"androidx.fragment.app.Fragment": true, "androidx.fragment.app.DialogFragment": true,
	"androidx.appcompat.app.AppCompatDialogFragment":                    true,
	"com.google.android.material.bottomsheet.BottomSheetDialogFragment": true,
	"androidx.preference.PreferenceFragmentCompat":                      true, "androidx.core.app.ComponentActivity": true,
	"androidx.activity.ComponentActivity": true, "androidx.fragment.app.FragmentActivity": true,
	"androidx.appcompat.app.AppCompatActivity": true,
}

// axlFileOnlyExtends reports whether a class reaches one of targets through its written supertypes, using only the
// file: a qualified name, an explicit import (aliases included) or a star import naming a target, or a same-file
// class that does. A simple name the file imports from another package (a lookalike such as
// com.mycorp.mvvm.ViewModel), or does not import at all, does not count.
func axlFileOnlyExtends(file *scanner.File, cls uint32, targets map[string]bool, depth int) bool {
	if depth > 6 {
		return false
	}
	facts := fileFactsCache().Imports(file)
	for _, ref := range lintDirectSupertypeRefs(file, cls) {
		if ref.qualified {
			if targets[ref.name] {
				return true
			}
			continue
		}
		if local := lintSameFileClass(file, ref.simple); local != 0 && local != cls {
			if axlFileOnlyExtends(file, local, targets, depth+1) {
				return true
			}
			continue
		}
		if target, ok := facts.Aliases[ref.simple]; ok {
			if targets[target] {
				return true
			}
			continue
		}
		for w := range facts.Wildcards {
			if targets[strings.TrimSuffix(w, ".*")+"."+ref.simple] {
				return true
			}
		}
	}
	return false
}

// axlLifecycleStates are the Lifecycle.State constants.
var axlLifecycleStates = map[string]bool{
	"INITIALIZED": true, "DESTROYED": true, "CREATED": true, "STARTED": true, "RESUMED": true,
}

// check mirrors TestLifecycleOwnerInCoroutineDetector on a kotlinx.coroutines.test runTest call: when the last
// assignment of a constant in the testBody lambda sets currentState to a Lifecycle.State, the call is reported at
// its name.
func (r *TestLifecycleOwnerInCoroutineRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	call, ok := axlContentLambda(file, lambda, "testBody", true)
	if !ok || !acrCallTargets(file, call, "kotlinx.coroutines.test", "runTest") {
		return
	}
	sets := false
	file.FlatWalkNodes(lambda, "assignment", func(a uint32) {
		target := file.FlatFirstChild(a)
		value := flatLastNamedChild(file, a)
		// The detector only updates its verdict when the left side resolves to a field or method (not a local) and
		// the right side to a field.
		if target == 0 || value == 0 || axlAssignsLocal(file, target) || !axlFieldLike(file, value) {
			return
		}
		sets = axlRightmostName(file, target) == "currentState" && axlLifecycleStates[axlRightmostName(file, value)]
	})
	if !sets {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "Incorrect use of currentState property inside of Coroutine, please use the "+
		"suspending setCurrentState() function.")
}

// axlAssignsLocal reports whether an assignment target is a bare name declared as a local or parameter.
func axlAssignsLocal(file *scanner.File, target uint32) bool {
	first := file.FlatFirstChild(target)
	if first == 0 || file.FlatType(first) != "simple_identifier" || file.FlatNextSib(first) != 0 {
		return false
	}
	decl := acrValueDeclaration(file, first, file.FlatNodeText(first))
	return decl != 0 && (file.FlatType(decl) != "property_declaration" || acrIsLocalDeclaration(file, decl))
}

// axlFieldLike reports whether an expression can resolve to a field: a qualified name (`Lifecycle.State.STARTED`),
// or a simple name that is not a local or parameter.
func axlFieldLike(file *scanner.File, expr uint32) bool {
	if !axlNameLike(file, expr) {
		return false
	}
	if file.FlatType(expr) != "simple_identifier" {
		return true
	}
	decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr))
	return decl == 0 || (file.FlatType(decl) == "property_declaration" && !acrIsLocalDeclaration(file, decl))
}

// axlNameLike reports whether expr is a simple or qualified name (`X`, `a.b.X`), the shape that can resolve to a
// field.
func axlNameLike(file *scanner.File, expr uint32) bool {
	switch file.FlatType(expr) {
	case "simple_identifier":
		return true
	case "navigation_expression":
		for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "simple_identifier", "navigation_suffix":
			case "navigation_expression":
				if !axlNameLike(file, child) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	return false
}

// axlRightmostName returns the last identifier of a name, assignable or navigation expression.
func axlRightmostName(file *scanner.File, expr uint32) string {
	var last string
	file.FlatWalkAllNodes(expr, func(n uint32) {
		if file.FlatType(n) == "simple_identifier" {
			last = file.FlatNodeText(n)
		}
	})
	return last
}

// check mirrors RepeatOnLifecycleDetector on a Kotlin class extending Fragment or androidx.core ComponentActivity:
// repeatOnLifecycle calls reached from onStart or onResume, following calls into same-file functions once per
// class.
func (r *RepeatOnLifecycleWrongUsageRule) check(ctx *api.Context) {
	file, cls := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) || !axlClassIsLifecycleHost(ctx, cls) {
		return
	}
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return
	}
	className := acrDeclarationName(file, cls)
	visited := map[uint32]bool{}
	for fn := file.FlatFirstChild(body); fn != 0; fn = file.FlatNextSib(fn) {
		if file.FlatType(fn) != "function_declaration" {
			continue
		}
		method := acrDeclarationName(file, fn)
		if method != "onStart" && method != "onResume" {
			continue
		}
		msg := "Wrong usage of repeatOnLifecycle from " + className + "." + method + "."
		axlVisitRepeatCalls(ctx, cls, fn, msg, visited, 0)
	}
}

// axlVisitRepeatCalls walks the calls of fn's body in source order: repeatOnLifecycle from androidx.lifecycle is
// reported, and an unqualified (or `this.`) call of a same-file function (a member of cls or a top-level function)
// not visited yet is followed into its body.
func axlVisitRepeatCalls(ctx *api.Context, cls, fn uint32, msg string, visited map[uint32]bool, depth int) {
	file := ctx.File
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok || depth > 8 {
		return
	}
	file.FlatWalkNodes(body, "call_expression", func(call uint32) {
		if acrIsInnerCallOfTrailingLambda(file, call) {
			return
		}
		name, _, _ := acrCallCallee(file, call)
		if name == "repeatOnLifecycle" {
			if len(acrFunctionDecls(file, name)) == 0 && acrNameResolvesTo(file, axlLifecyclePackage, name) {
				ctx.EmitAtNode(call, msg)
			}
			return
		}
		if _, _, hasReceiver := acrCallCallee(file, call); hasReceiver && !axlReceiverIsThis(file, call) {
			return
		}
		target := axlSameFileCallee(file, cls, name)
		if target == 0 || visited[target] {
			return
		}
		visited[target] = true
		axlVisitRepeatCalls(ctx, cls, target, msg, visited, depth+1)
	})
}

// axlReceiverIsThis reports whether a call's explicit receiver is an unlabeled `this` (a `super` call reaches the
// library method, not a same-file one).
func axlReceiverIsThis(file *scanner.File, call uint32) bool {
	receiver := file.FlatFirstChild(acrCalleeExpression(file, call))
	return receiver != 0 && file.FlatType(receiver) == "this_expression" && file.FlatNodeTextEquals(receiver, "this")
}

// axlSameFileCallee returns the single function named name that a call can reach in the file: a member of cls, or
// else a top-level function.
func axlSameFileCallee(file *scanner.File, cls uint32, name string) uint32 {
	if name == "" {
		return 0
	}
	if body, ok := file.FlatFindChild(cls, "class_body"); ok {
		for fn := file.FlatFirstChild(body); fn != 0; fn = file.FlatNextSib(fn) {
			if file.FlatType(fn) == "function_declaration" && acrDeclarationName(file, fn) == name {
				return fn
			}
		}
	}
	return acrSingleTopLevelFunction(file, name)
}

// axlClassIsLifecycleHost reports whether a class extends androidx.fragment.app.Fragment or
// androidx.core.app.ComponentActivity (which androidx.activity.ComponentActivity, FragmentActivity and
// AppCompatActivity extend), proven from the file alone (see axlClassReachesViewModel).
func axlClassIsLifecycleHost(ctx *api.Context, cls uint32) bool {
	return ctx.File.FlatType(cls) == "class_declaration" && axlFileOnlyExtends(ctx.File, cls, axlLifecycleHostFQNs, 0)
}
