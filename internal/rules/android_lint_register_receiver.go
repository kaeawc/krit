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
	protected, unprotected, inspected := receiverFilterActions(file, filterArg, ctx.Idx)
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
// constructor and to addAction calls before the registration. inspected is false when the filter cannot be traced.
func receiverFilterActions(file *scanner.File, filter, call uint32) (protected bool, unprotected []string, inspected bool) {
	construction, blockScope, name := uint32(0), uint32(0), ""
	switch file.FlatType(filter) {
	case "simple_identifier", "identifier":
		name = file.FlatNodeText(filter)
		init := lintVariableInitializer(file, call, name)
		if init == 0 {
			return false, nil, false
		}
		construction, blockScope = unwrapFilterConstruction(file, init)
	default:
		construction, blockScope = unwrapFilterConstruction(file, filter)
	}
	if construction == 0 {
		return false, nil, false
	}
	var actions []string
	if inner, ok := lintCallAt(file, construction); ok && len(inner.Args) > 0 {
		if v, known := receiverActionValue(file, inner.Args[0]); known {
			actions = append(actions, v)
		}
	}
	collect := func(root uint32) {
		file.FlatWalkAllNodes(root, func(n uint32) {
			c, ok := lintCallAt(file, n)
			if !ok || c.Name != "addAction" || len(c.Args) == 0 || file.FlatStartByte(n) >= file.FlatStartByte(call) {
				return
			}
			receiver := strings.TrimSpace(c.ReceiverText)
			if (name != "" && receiver == name) || (blockScope != 0 && (receiver == "" || receiver == "this" || receiver == "it")) {
				if v, known := receiverActionValue(file, c.Args[0]); known {
					actions = append(actions, v)
				}
			}
		})
	}
	if blockScope != 0 {
		collect(blockScope)
	}
	if name != "" {
		if fn, ok := flatEnclosingCallable(file, call); ok {
			collect(fn)
		}
	}
	protected = true
	for _, a := range actions {
		if !lintProtectedBroadcastActions[a] {
			protected = false
			unprotected = append(unprotected, a)
		}
	}
	return protected, unprotected, true
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
