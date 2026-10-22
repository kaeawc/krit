package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// CollectInOnCreateWithoutLifecycleRule detects Flow.collect calls in lifecycle
// callbacks that are not wrapped by repeatOnLifecycle.
type CollectInOnCreateWithoutLifecycleRule struct {
	FlatDispatchBase
	BaseRule
}

var lifecycleCollectCallbacks = map[string]bool{
	"onCreate":      true,
	"onStart":       true,
	"onViewCreated": true,
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *CollectInOnCreateWithoutLifecycleRule) Confidence() float64 { return api.ConfidenceMedium }

func hasAncestorCallNamedFlat(file *scanner.File, idx uint32, name string) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" {
			return false
		}
		if file.FlatType(p) != "call_expression" {
			continue
		}
		if flatCallNameAny(file, p) == name {
			return true
		}
	}
	return false
}

// GlobalCoroutineUsageRule detects GlobalScope.launch/async and direct GlobalScope references.
type GlobalCoroutineUsageRule struct {
	FlatDispatchBase
	BaseRule
}

// Description implements DescriptionProvider.
func (*GlobalCoroutineUsageRule) Description() string {
	return "Flags use of GlobalScope for launching coroutines. Coroutines in GlobalScope are not tied to any lifecycle and can leak if not cancelled. Prefer a structured CoroutineScope."
}

// Confidence reports a tier-2 (medium) base confidence. Coroutines rule. Detection matches kotlinx.coroutines call shapes via
// name lists and structural patterns; project wrappers can escape or
// collide. Classified per roadmap/17.
func (r *GlobalCoroutineUsageRule) Confidence() float64 { return api.ConfidenceMedium }

// InjectDispatcherRule detects hardcoded Dispatchers.IO/Default/Unconfined in call expressions.
type InjectDispatcherRule struct {
	FlatDispatchBase
	BaseRule
	DispatcherNames []string
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// flags any call that passes Dispatchers.IO/Default/Unconfined as an
// argument, with exclusions for idiomatic dispatcher hosts (e.g.
// withContext, flowOn), `Main` (which can't be injected), and
// object/@JvmStatic enclosing functions. It does NOT understand DI
// annotations — a class that already accepts `@Inject` would still
// trip the rule if the test-injected default happens to be a
// Dispatchers constant. Medium confidence reflects the DI-awareness
// gap noted in roadmap/17.
func (r *InjectDispatcherRule) Confidence() float64 { return api.ConfidenceMedium }

func directCallArgumentsFlat(file *scanner.File, idx uint32) uint32 {
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 || file.FlatType(child) != "call_suffix" {
			continue
		}
		for j := 0; j < file.FlatNamedChildCount(child); j++ {
			gc := file.FlatNamedChild(child, j)
			if gc != 0 && file.FlatType(gc) == "value_arguments" {
				return gc
			}
		}
	}
	return 0
}

func injectDispatcherNames(names []string) map[string]bool {
	if len(names) == 0 {
		return map[string]bool{"IO": true, "Default": true, "Unconfined": true, "Main": true}
	}
	out := make(map[string]bool, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = true
		}
	}
	return out
}

// injectDispatcherOperand is one hardcoded `Dispatchers.<name>` reference
// passed as (part of) a call argument.
type injectDispatcherOperand struct {
	node uint32
	name string
}

// findDirectDispatcherArgumentsFlat returns every hardcoded dispatcher the
// call's arguments pass, in source order.
func findDirectDispatcherArgumentsFlat(file *scanner.File, args uint32, names map[string]bool) []injectDispatcherOperand {
	var out []injectDispatcherOperand
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		arg := file.FlatNamedChild(args, i)
		if arg == 0 || file.FlatType(arg) != "value_argument" || file.FlatNamedChildCount(arg) == 0 {
			continue
		}
		out = appendInjectDispatcherOperandsFlat(file, file.FlatNamedChild(arg, 0), names, out)
	}
	return out
}

// appendInjectDispatcherOperandsFlat appends the `Dispatchers.<name>`
// references an argument value passes: the value itself, or every operand of
// a `+` chain that combines it with other context elements
// (`SupervisorJob() + Dispatchers.IO`, `Dispatchers.Main + Dispatchers.IO`).
// The combined context goes to the same call, so each dispatcher is as
// hardcoded there as a lone argument; the FIR checker reports each of them.
// A chain with any other operator is not a context combination and is left
// alone.
func appendInjectDispatcherOperandsFlat(file *scanner.File, value uint32, names map[string]bool, out []injectDispatcherOperand) []injectDispatcherOperand {
	if value == 0 {
		return out
	}
	switch file.FlatType(value) {
	case "navigation_expression":
		receiver := ""
		if file.FlatNamedChildCount(value) > 0 {
			first := file.FlatNamedChild(value, 0)
			if file.FlatType(first) == "simple_identifier" {
				receiver = file.FlatNodeText(first)
			}
		}
		member := flatNavigationExpressionLastIdentifier(file, value)
		if receiver == "Dispatchers" && names[member] {
			out = append(out, injectDispatcherOperand{node: value, name: member})
		}
	case "parenthesized_expression":
		if file.FlatNamedChildCount(value) == 1 {
			return appendInjectDispatcherOperandsFlat(file, file.FlatNamedChild(value, 0), names, out)
		}
	case "additive_expression":
		for child := file.FlatFirstChild(value); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "-" {
				return out
			}
		}
		for i := 0; i < file.FlatNamedChildCount(value); i++ {
			out = appendInjectDispatcherOperandsFlat(file, file.FlatNamedChild(value, i), names, out)
		}
	}
	return out
}

// injectDispatcherIdiomaticHost reports whether a call_expression's
// callee is one of the API hosts where a hardcoded dispatcher argument
// is conventional and dispatcher injection wouldn't help. Mirrors the
// `isIdiomaticDispatcherHost` filter in the krit-fir InjectDispatcher
// checker so the two implementations stay in parity.
//
// Recognized:
//
//   - `flowOn(...)`, `shareIn(...)`, `CoroutineScope(...)` — always.
//   - `viewModelScope.launch(...)` / `viewModelScope.async(...)`.
//   - `lifecycleScope.launch(...)` / `lifecycleScope.launchWhen{Created,Started,Resumed}(...)`.
func injectDispatcherIdiomaticHost(file *scanner.File, callIdx uint32) bool {
	callee := injectDispatcherCalleeName(file, callIdx)
	switch callee {
	case "flowOn", "shareIn", "CoroutineScope":
		return true
	case "async":
		return injectDispatcherReceiverName(file, callIdx) == "viewModelScope"
	case "launch":
		recv := injectDispatcherReceiverName(file, callIdx)
		return recv == "viewModelScope" || recv == "lifecycleScope"
	case "launchWhenCreated", "launchWhenStarted", "launchWhenResumed":
		return injectDispatcherReceiverName(file, callIdx) == "lifecycleScope"
	}
	return false
}

// injectDispatcherCalleeName returns the simple method name of a
// call_expression. For `launch(...)` returns "launch"; for
// `vm.launch(...)` returns "launch". Empty for shapes we don't model.
func injectDispatcherCalleeName(file *scanner.File, callIdx uint32) string {
	callee := file.FlatNamedChild(callIdx, 0)
	if callee == 0 {
		return ""
	}
	switch file.FlatType(callee) {
	case "simple_identifier":
		return file.FlatNodeText(callee)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, callee)
	}
	return ""
}

// injectDispatcherReceiverName returns the immediate receiver
// identifier of a member call. For `vm.launch(...)` returns "vm".
// For chained navigation `a.b.launch(...)` returns "b". Empty when
// there's no explicit receiver.
func injectDispatcherReceiverName(file *scanner.File, callIdx uint32) string {
	callee := file.FlatNamedChild(callIdx, 0)
	if callee == 0 || file.FlatType(callee) != "navigation_expression" {
		return ""
	}
	first := file.FlatNamedChild(callee, 0)
	if first == 0 {
		return ""
	}
	switch file.FlatType(first) {
	case "simple_identifier":
		return file.FlatNodeText(first)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, first)
	}
	return ""
}

func injectDispatcherReferenceConfirmed(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) != 2 || segments[0] != "Dispatchers" {
		return false
	}
	if fileDeclaresIdentifierNamed(file, idx, "Dispatchers") {
		return false
	}
	return fileImportsFQN(file, "kotlinx.coroutines.Dispatchers")
}

// injectDispatcherHasInjectableHost reports whether the dispatcher
// usage at idx sits inside a function where constructor injection of a
// CoroutineDispatcher is actually possible: a member function of a
// class, object, or companion object.
//
// It returns false for the shapes the rule must not flag because there
// is no injectable host:
//
//   - top-level functions (no enclosing class/object/companion), and
//   - top-level extension functions (receiver fixed by the call site,
//     and likewise no enclosing type to inject into).
//
// A *member* extension function (e.g. `fun String.foo()` declared inside
// a class) still has an injectable host — the enclosing class — and so
// continues to flag. That falls out of the ancestor test below without
// any extension-specific special-casing.
//
// Detection is purely structural via the flat AST: an injectable host
// exists exactly when some ancestor of the usage is a class_declaration,
// object_declaration, or companion_object. Top-level plain and top-level
// extension functions both lack such an ancestor and are suppressed,
// while members (plain or extension) keep flagging. This mirrors the
// top-level/extension exemption added to the krit-fir InjectDispatcher
// checker, which uses FIR symbol facts to reach the same decision.
//
// The receiver-type AST shape (a `.` token child of the
// function_declaration, modelled by tree-sitter as
// user_type/nullable_type, `.`, simple_identifier) is reported by
// flatFunctionDeclarationIsExtension for callers that need to
// distinguish extensions explicitly.
func injectDispatcherHasInjectableHost(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	_, hasType := flatEnclosingAncestor(file, idx,
		"class_declaration", "object_declaration", "companion_object")
	return hasType
}

// flatFunctionDeclarationIsExtension reports whether a
// function_declaration node declares an extension function. Extension
// functions carry a receiver type before the name, which tree-sitter
// models as a type node followed by a `.` token child of the
// function_declaration (e.g. `fun String.foo()` →
// user_type "String", `.`, simple_identifier "foo"; `fun <T> T.foo()`
// inserts type_parameters first but still yields a `.` child).
// Non-extension functions have no `.` direct child, so a single direct
// `.` token is a reliable structural signal — no name/substring match.
func flatFunctionDeclarationIsExtension(file *scanner.File, fn uint32) bool {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "." {
			return true
		}
	}
	return false
}

func fileDeclaresIdentifierNamed(file *scanner.File, except uint32, name string) bool {
	if file == nil || name == "" {
		return false
	}
	targetStart := file.FlatStartByte(except)
	for idx := file.FlatFirstChild(0); idx != 0; idx = file.FlatNextSib(idx) {
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "property_declaration":
			if idx != except && extractIdentifierFlat(file, idx) == name {
				return true
			}
		}
	}
	for p, ok := file.FlatParent(except); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration":
			if declarationSubtreeHasNameBefore(file, p, targetStart, name, map[string]bool{
				"parameter":             true,
				"property_declaration":  true,
				"variable_declaration":  true,
				"class_declaration":     true,
				"object_declaration":    true,
				"function_declaration":  false,
				"anonymous_initializer": false,
			}) {
				return true
			}
		case "class_body":
			if declarationSubtreeHasNameBefore(file, p, targetStart, name, map[string]bool{
				"property_declaration": true,
				"class_declaration":    true,
				"object_declaration":   true,
			}) {
				return true
			}
		}
	}
	return false
}

func declarationSubtreeHasNameBefore(file *scanner.File, root uint32, targetStart uint32, name string, types map[string]bool) bool {
	found := false
	file.FlatWalkAllNodes(root, func(idx uint32) {
		if found || idx == root || file.FlatStartByte(idx) > targetStart {
			return
		}
		if !types[file.FlatType(idx)] {
			return
		}
		if extractIdentifierFlat(file, idx) == name {
			found = true
		}
	})
	return found
}

var flowTypeNames = map[string]bool{
	"Flow": true, "StateFlow": true, "SharedFlow": true,
	"MutableStateFlow": true, "MutableSharedFlow": true,
}
