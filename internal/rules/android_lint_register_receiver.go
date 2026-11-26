package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	receiverFlagMask = 6
	contextClass     = "android.content.Context"
)

// receiverFlagConsts are the values of the registerReceiver export flags.
var receiverFlagConsts = map[string]int64{
	contextClass + ".RECEIVER_VISIBLE_TO_INSTANT_APPS":          1,
	contextClass + ".RECEIVER_EXPORTED":                         2,
	contextClass + ".RECEIVER_NOT_EXPORTED":                     4,
	"androidx.core.content.ContextCompat.RECEIVER_EXPORTED":     2,
	"androidx.core.content.ContextCompat.RECEIVER_NOT_EXPORTED": 4,
}

// contextSubclassNames are framework and AndroidX classes that extend android.content.Context.
var contextSubclassNames = map[string]bool{
	"Context": true, "ContextWrapper": true, "ContextThemeWrapper": true, "Activity": true, "Service": true, "Application": true,
	"AppCompatActivity": true, "FragmentActivity": true, "ComponentActivity": true, "IntentService": true,
	"JobIntentService": true, "LifecycleService": true, "AccessibilityService": true, "NotificationListenerService": true,
	"MediaBrowserServiceCompat": true, "MediaBrowserService": true, "InputMethodService": true, "VpnService": true,
	"HostApduService": true, "JobService": true, "WallpaperService": true, "ListActivity": true, "PreferenceActivity": true,
	"MultiDexApplication": true, "ActionBarActivity": true, "TvInputService": true, "DreamService": true,
}

// contextAccessors are members that return a Context.
var contextAccessors = map[string]bool{
	"this": true, "context": true, "getContext": true, "requireContext": true, "applicationContext": true,
	"getApplicationContext": true, "baseContext": true, "getBaseContext": true, "activity": true, "getActivity": true,
	"requireActivity": true, "mContext": true, "appContext": true,
}

// UnspecifiedRegisterReceiverFlagRule mirrors Lint's UnspecifiedRegisterReceiverFlag: registerReceiver for non-system
// broadcasts without RECEIVER_EXPORTED or RECEIVER_NOT_EXPORTED.
type UnspecifiedRegisterReceiverFlagRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: filters are traced within one method.
func (r *UnspecifiedRegisterReceiverFlagRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnspecifiedRegisterReceiverFlagRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok {
		return
	}
	filterIndex := 1
	switch call.Name {
	case "registerReceiver", "registerReceiverForAllUsers":
	case "registerReceiverAsUser":
		filterIndex = 2
	default:
		return
	}
	if !receiverIsContext(ctx, call) {
		return
	}
	receiverArg, filterArg := call.Arg(0, "receiver"), call.Arg(filterIndex, "filter")
	if receiverArg == 0 || filterArg == 0 || strings.TrimSpace(file.FlatNodeText(receiverArg)) == "null" {
		return
	}
	if strings.TrimSpace(file.FlatNodeText(filterArg)) == "null" {
		return
	}
	var flagsArg uint32
	switch {
	case call.Name == "registerReceiver" && len(call.Args) == 3:
		flagsArg = call.Arg(2, "flags")
	case call.Name == "registerReceiver" && len(call.Args) == 5:
		flagsArg = call.Arg(4, "flags")
	}
	if flagsArg != 0 {
		flags, known := lintEvalInt(file, flagsArg, receiverFlagConsts)
		if !known || flags&receiverFlagMask != 0 {
			return
		}
	}
	// An action that cannot be evaluated is ignored, as before #1813.
	protected, unprotected, inspected, _ := receiverFilterActions(file, filterArg, ctx.Idx)
	if protected && inspected {
		return
	}
	actions := strings.Join(unprotected, ", ")
	if actions == "" {
		actions = "an IntentFilter that cannot be inspected by lint"
	}
	severity := "warning"
	if sdk := lintTargetSdk(ctx); (sdk == 0 || sdk >= 34) && len(unprotected) > 0 {
		severity = ""
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, severity, strings.TrimSpace(file.FlatNodeText(receiverArg))+
		" is missing RECEIVER_EXPORTED or RECEIVER_NOT_EXPORTED flag for unprotected broadcasts registered for "+actions))
}

// receiverIsContext reports whether the call goes to an android.content.Context: a typed receiver, a known accessor,
// or an unqualified call inside a class that extends a Context subclass.
func receiverIsContext(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if call.ReceiverNode == 0 {
		return lintEnclosingClassIsContext(file, call.Idx, 0)
	}
	text := strings.TrimSpace(call.ReceiverText)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "!!"), "?")
	text = strings.TrimSuffix(strings.TrimPrefix(text, "this."), "()")
	if contextAccessors[text] || contextAccessors[text[strings.LastIndex(text, ".")+1:]] {
		return true
	}
	if lintReceiverIsType(ctx, call, contextClass) {
		return true
	}
	typeText := lintReceiverTypeText(file, call)
	return contextSubclassNames[typeText[strings.LastIndex(typeText, ".")+1:]]
}

func lintEnclosingClassIsContext(file *scanner.File, idx uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	class, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_literal", "object_creation_expression")
	if !ok {
		return false
	}
	for _, super := range lintSupertypeNames(file, class) {
		simple := super[strings.LastIndex(super, ".")+1:]
		if contextSubclassNames[simple] {
			return true
		}
		if decl := lintSameFileClass(file, simple); decl != 0 && decl != class && lintEnclosingClassIsContext(file, file.FlatFirstChild(decl), depth+1) {
			return true
		}
	}
	return false
}

// receiverFilterActions follows the IntentFilter argument to its construction and collects the actions given to the
// constructor, to addAction calls in the scope function that builds it, and, for a filter held in a variable, to
// addAction calls on the variable before the registration. inspected is false when the filter cannot be traced.
// protected and unprotected cover only the actions that evaluate; unevaluated reports that some action did not, or
// that the filter may hold actions krit does not see: it is a property or field, which other members can change, the
// registration has no enclosing function, constructor or initializer block to read, or the variable is used before
// the registration in a way other than a readable addAction call (passed to a function, given to a scope function or
// `with`, aliased).
func receiverFilterActions(file *scanner.File, filter, call uint32) (protected bool, unprotected []string, inspected, unevaluated bool) {
	construction, blockScope, name := uint32(0), uint32(0), ""
	var callable, local uint32
	switch file.FlatType(filter) {
	case "simple_identifier", "identifier":
		name = file.FlatNodeText(filter)
		var init uint32
		callable, local, init = receiverFilterVariable(file, call, name)
		if init == 0 {
			return false, nil, false, false
		}
		// A property or field may get actions from any member, and a registration outside a callable has no body to read.
		unevaluated = local == 0
		construction, blockScope = unwrapFilterConstruction(file, init)
	default:
		construction, blockScope = unwrapFilterConstruction(file, filter)
	}
	if construction == 0 {
		return false, nil, false, false
	}
	var actions []string
	add := func(arg uint32) {
		if v, known := receiverActionValue(file, arg); known {
			actions = append(actions, v)
		} else {
			unevaluated = true
		}
	}
	if inner, ok := lintCallAt(file, construction); ok && len(inner.Args) > 0 {
		add(inner.Args[0])
	}
	// The scope function of an inline filter is inside the registerReceiver call, so its walk takes every addAction
	// on its receiver.
	if blockScope != 0 {
		file.FlatWalkAllNodes(blockScope, func(n uint32) {
			c, ok := lintCallAt(file, n)
			if !ok || c.Name != "addAction" || len(c.Args) == 0 {
				return
			}
			if receiver := strings.TrimSpace(c.ReceiverText); receiver == "" || receiver == "this" || receiver == "it" {
				add(c.Args[0])
			}
		})
	}
	if callable != 0 && receiverFilterVariableUses(file, callable, local, call, name, add) {
		unevaluated = true
	}
	protected = true
	for _, a := range actions {
		if !lintProtectedBroadcastActions[a] {
			protected = false
			unprotected = append(unprotected, a)
		}
	}
	return protected, unprotected, true, unevaluated
}

// receiverFilterVariable resolves a filter variable registered at call: the callable whose body runs the
// registration (0 when there is none, as in a property initializer), the local declaration of the variable in it (0
// for a property or field), and the initializer (0 when the variable cannot be traced).
func receiverFilterVariable(file *scanner.File, call uint32, name string) (callable, local, init uint32) {
	callable = receiverFilterCallable(file, call)
	if callable != 0 {
		decl, localInit, found := receiverFilterLocal(file, callable, call, name)
		if found {
			return callable, decl, localInit
		}
	}
	return callable, 0, lintVariableInitializer(file, call, name)
}

// receiverFilterVariableUses walks what runs in callable before the registration at call (the filter is copied when
// it is registered) and passes the action of each addAction on the variable to add. For a local it starts after the
// declaration and reports any other use that may add actions krit does not see; `this.name` names a property or
// field, never the local.
func receiverFilterVariableUses(file *scanner.File, callable, local, call uint32, name string, add func(uint32)) (otherUse bool) {
	start, stop := file.FlatStartByte(callable), file.FlatStartByte(call)
	if local != 0 {
		start = file.FlatEndByte(local)
	}
	var receiverEnd uint32
	file.FlatWalkAllNodes(callable, func(n uint32) {
		pos := file.FlatStartByte(n)
		if pos >= stop || file.FlatEndByte(n) <= start {
			return
		}
		if c, ok := lintCallAt(file, n); ok && c.Name == "addAction" && len(c.Args) > 0 && c.ReceiverNode != 0 {
			receiver := strings.TrimSpace(c.ReceiverText)
			if local == 0 {
				receiver = strings.TrimPrefix(receiver, "this.")
			}
			if receiver == name {
				add(c.Args[0])
				receiverEnd = file.FlatEndByte(c.ReceiverNode)
				return
			}
		}
		if local == 0 || otherUse || pos < start || pos < receiverEnd || file.FlatNodeText(n) != name {
			return
		}
		if t := file.FlatType(n); (t == "simple_identifier" || t == "identifier") && receiverFilterOtherUse(file, n) {
			otherUse = true
		}
	})
	return otherUse
}

// receiverFilterCallable is the function, constructor, initializer block or accessor whose body runs the
// registration at idx, or 0 when a class body or the file comes first, as in a property initializer.
func receiverFilterCallable(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "method_declaration", "constructor_declaration", "compact_constructor_declaration",
			"secondary_constructor", "anonymous_initializer", "static_initializer", "getter", "setter":
			return cur
		case "block":
			// A Java instance initializer is a block directly in the class body.
			if p, ok := file.FlatParent(cur); ok && (file.FlatType(p) == "class_body" || file.FlatType(p) == "enum_body_declarations") {
				return cur
			}
		case "class_body", "enum_class_body", "enum_body", "interface_body", "source_file", "program":
			return 0
		}
	}
	return 0
}

// receiverFilterLocal finds the last declaration of a local called name in callable before the registration at call.
// found reports such a declaration; init is 0 when it has no initializer or is a var that is reassigned.
func receiverFilterLocal(file *scanner.File, callable, call uint32, name string) (decl, init uint32, found bool) {
	stop := file.FlatStartByte(call)
	file.FlatWalkAllNodes(callable, func(n uint32) {
		if file.FlatStartByte(n) >= stop {
			return
		}
		switch file.FlatType(n) {
		case "property_declaration":
			if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok && lintDeclaredName(file, vd) == name {
				kind, _ := file.FlatFindChild(n, "binding_pattern_kind")
				decl, init, found = n, lintInitializerAfterEquals(file, n), true
				if strings.TrimSpace(file.FlatNodeText(kind)) == "var" {
					init = 0
				}
			}
		case "variable_declarator":
			if id, ok := file.FlatFindChild(n, "identifier"); ok && file.FlatNodeText(id) == name {
				decl, init, found = n, lintInitializerAfterEquals(file, n), true
			}
		}
	})
	if found && init != 0 && lintNameReassigned(file, callable, name) {
		init = 0
	}
	return decl, init, found
}

// intentFilterMembersWithoutActions are IntentFilter members that neither add an action nor hand the filter on.
var intentFilterMembersWithoutActions = map[string]bool{
	"addCategory": true, "addDataScheme": true, "addDataSchemeSpecificPart": true, "addDataAuthority": true,
	"addDataPath": true, "addDataType": true, "setPriority": true, "priority": true, "getPriority": true,
	"hasAction": true, "hasCategory": true, "countActions": true, "countCategories": true, "getAction": true,
}

// receiverFilterOtherUse reports whether the identifier id, spelled like a local filter, is a use of it that may add
// actions krit does not see: anything but another object's member, a named-argument label, a method name, or a call
// of an IntentFilter member that adds no action.
func receiverFilterOtherUse(file *scanner.File, id uint32) bool {
	parent, ok := file.FlatParent(id)
	if !ok {
		return true
	}
	next := file.FlatNextSib(id)
	switch file.FlatType(parent) {
	case "navigation_suffix":
		return false
	case "value_argument":
		if file.FlatType(next) == "=" {
			return false
		}
	case "field_access":
		if file.FlatFirstChild(parent) != id {
			return false
		}
		if field := file.FlatNamedChild(parent, 1); field != 0 {
			return !intentFilterMembersWithoutActions[file.FlatNodeText(field)]
		}
	case "method_invocation":
		if t := file.FlatType(next); t == "argument_list" || t == "type_arguments" {
			return false
		}
		if file.FlatType(next) == "." {
			if member := file.FlatNextSib(next); member != 0 && file.FlatType(member) == "identifier" {
				return !intentFilterMembersWithoutActions[file.FlatNodeText(member)]
			}
		}
	}
	if file.FlatType(next) == "navigation_suffix" {
		if member, ok := file.FlatFindChild(next, "simple_identifier"); ok {
			return !intentFilterMembersWithoutActions[file.FlatNodeText(member)]
		}
	}
	return true
}

// unwrapFilterConstruction finds the IntentFilter construction behind an expression, looking through Kotlin scope
// functions; blockScope is the lambda whose addAction calls belong to the filter.
func unwrapFilterConstruction(file *scanner.File, expr uint32) (construction, blockScope uint32) {
	call, ok := lintCallAt(file, expr)
	if !ok {
		return 0, 0
	}
	if isIntentFilterConstruction(file, call) {
		return expr, 0
	}
	switch call.Name {
	case "apply", "also", "let", "run":
		if call.ReceiverNode != 0 {
			if inner, ok := lintCallAt(file, call.ReceiverNode); ok && isIntentFilterConstruction(file, inner) {
				return call.ReceiverNode, expr
			}
		}
	case "create":
		if lintTypeRefMatches(file, call.ReceiverText, "android.content.IntentFilter") {
			return expr, 0
		}
	}
	return 0, 0
}

func isIntentFilterConstruction(file *scanner.File, call lintCall) bool {
	return lintTypeRefMatches(file, call.TypeText, "android.content.IntentFilter") && call.Name == "IntentFilter"
}

// lintVariableInitializer returns the initializer of the variable called name visible from scope, even when it is reassigned
// later by addAction-style calls.
func lintVariableInitializer(file *scanner.File, scope uint32, name string) uint32 {
	return lintConstantInitializer(file, scope, name)
}

// receiverActionValue resolves an action argument: a string constant or a framework constant such as Intent.ACTION_VIEW.
func receiverActionValue(file *scanner.File, arg uint32) (string, bool) {
	if v, ok := lintEvalString(file, arg); ok && v != "" {
		return v, true
	}
	text := strings.TrimSpace(file.FlatNodeText(arg))
	parts := strings.Split(text, ".")
	if len(parts) >= 2 {
		if v, ok := lintFrameworkActionConstants[parts[len(parts)-2]+"."+parts[len(parts)-1]]; ok {
			return v, true
		}
	}
	return "", false
}
