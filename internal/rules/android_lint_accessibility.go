package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const (
	androidServiceClass       = "android.app.Service"
	androidViewClass          = "android.view.View"
	accessibilityEventClass   = "android.view.accessibility.AccessibilityEvent"
	windowStateChangedName    = "TYPE_WINDOW_STATE_CHANGED"
	windowStateChangedValue   = 32
	accessibilityFocusName    = "ACTION_ACCESSIBILITY_FOCUS"
	accessibilityFocusValue   = 64
	windowStateChangedMessage = "Manually populating or sending TYPE_WINDOW_STATE_CHANGED events should be avoided. " +
		"They may be ignored on certain versions of Android. Prefer setting UI metadata using " +
		"`View.onInitializeAccessibilityNodeInfo`, `Activity.setTitle`, `ViewCompat.setAccessibilityPaneTitle`, etc. " +
		"to inform users of crucial changes to the UI."
)

// androidKnownServiceSimpleNames are framework and library classes that extend android.app.Service.
var androidKnownServiceSimpleNames = map[string]struct{}{
	"Service": {}, "IntentService": {}, "LifecycleService": {}, "JobIntentService": {}, "JobService": {},
	"MediaBrowserService": {}, "MediaBrowserServiceCompat": {}, "MediaSessionService": {}, "MediaLibraryService": {},
	"VpnService": {}, "AccessibilityService": {}, "NotificationListenerService": {}, "InputMethodService": {},
	"WallpaperService": {}, "RemoteViewsService": {}, "DreamService": {}, "HostApduService": {}, "TileService": {},
	"ConnectionService": {}, "CallScreeningService": {}, "FirebaseMessagingService": {}, "AbstractInputMethodService": {},
}

// lintKnownViewSimpleNames are View subclasses by simple name: the shared list plus the remaining platform widgets and
// common support-library views, for hierarchies the resolver cannot follow.
var lintKnownViewSimpleNames = lintMergeNames(androidKnownViewSimpleNames, []string{
	"AbsListView", "AbsSeekBar", "AbsSpinner", "AdapterView", "AutoCompleteTextView", "CalendarView", "CheckBox",
	"CheckedTextView", "Chronometer", "CompoundButton", "DatePicker", "ExpandableListView", "GridLayout", "ImageButton",
	"ImageSwitcher", "MultiAutoCompleteTextView", "NumberPicker", "ProgressBar", "RadioButton", "RadioGroup", "RatingBar",
	"SearchView", "SeekBar", "Space", "Spinner", "Switch", "TabHost", "TableLayout", "TableRow", "TextClock", "TextSwitcher",
	"TimePicker", "Toolbar", "ToggleButton", "VideoView", "ViewAnimator", "ViewFlipper", "ViewStub", "ViewSwitcher",
	"AppCompatImageButton", "AppCompatSpinner", "AppCompatSeekBar", "AppCompatToggleButton", "AppCompatAutoCompleteTextView",
	"MaterialTextView", "Chip", "ChipGroup", "TabLayout", "TextInputLayout", "BottomNavigationView", "NavigationView",
	"AppBarLayout", "CollapsingToolbarLayout", "ViewPager", "SwipeRefreshLayout", "DrawerLayout", "Slider", "SwitchMaterial",
	"MaterialToolbar", "ExtendedFloatingActionButton", "ConstraintHelper", "Guideline", "Barrier", "Group",
})

func lintMergeNames(base map[string]struct{}, extra []string) map[string]struct{} {
	out := make(map[string]struct{}, len(base)+len(extra))
	for name := range base {
		out[name] = struct{}{}
	}
	for _, name := range extra {
		out[name] = struct{}{}
	}
	return out
}

var windowStateChangedConsts = map[string]int64{
	accessibilityEventClass + "." + windowStateChangedName: windowStateChangedValue,
}

var accessibilityFocusConsts = map[string]int64{
	"android.view.accessibility.AccessibilityNodeInfo." + accessibilityFocusName:             accessibilityFocusValue,
	"androidx.core.view.accessibility.AccessibilityNodeInfoCompat." + accessibilityFocusName: accessibilityFocusValue,
}

// ---- NotificationId0 ----

// NotificationID0Rule mirrors Lint's NotificationId0: Service.startForeground with a constant notification id of 0.
type NotificationID0Rule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *NotificationID0Rule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NotificationID0Rule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "startForeground" || len(call.Args) == 0 || call.Args[0] == 0 {
		return
	}
	// Lint reads the first value argument whatever its Kotlin label.
	id := call.Args[0]
	if v, known := lintEvalIntLocals(file, id, nil); !known || v != 0 || lintIsLongLiteral(file, id) {
		return
	}
	if !lintCallIsMemberOf(ctx, call, androidServiceClass, androidKnownServiceSimpleNames) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, id, "", "The notification id **cannot** be 0"))
}

// lintIsLongLiteral reports whether the expression is a bare Long literal such as `0L`, which Lint does not fold to an Int.
func lintIsLongLiteral(file *scanner.File, idx uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	return len(text) > 1 && (text[len(text)-1] == 'L' || text[len(text)-1] == 'l') && text[0] >= '0' && text[0] <= '9'
}

// ---- GetContentDescriptionOverride ----

// GetContentDescriptionOverrideRule mirrors Lint's GetContentDescriptionOverride: a View subclass that declares
// getContentDescription().
type GetContentDescriptionOverrideRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: View subclasses are recognized by name and resolver hierarchy.
func (r *GetContentDescriptionOverrideRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *GetContentDescriptionOverrideRule) check(ctx *api.Context) {
	file := ctx.File
	name := lintDeclaredMethodName(file, ctx.Idx)
	if name == 0 || file.FlatNodeText(name) != "getContentDescription" || lintDeclaredParamCount(file, ctx.Idx) != 0 {
		return
	}
	cls, ok := lintDirectClassOf(file, ctx.Idx)
	if !ok || !lintClassExtends(ctx, cls, androidViewClass, lintKnownViewSimpleNames) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, name, "", "Overriding `getContentDescription()` on a View is not recommended"))
}

// lintDeclaredMethodName returns the name identifier of a Kotlin function or Java method declaration, or 0.
func lintDeclaredMethodName(file *scanner.File, fn uint32) uint32 {
	kind := "simple_identifier"
	if file.FlatType(fn) == "method_declaration" {
		kind = "identifier"
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == kind {
			return child
		}
	}
	return 0
}

// lintDeclaredParamCount counts the parameters of a Kotlin function or Java method declaration.
func lintDeclaredParamCount(file *scanner.File, fn uint32) int {
	kind, param := "function_value_parameters", "parameter"
	if file.FlatType(fn) == "method_declaration" {
		kind, param = "formal_parameters", "formal_parameter"
	}
	params, ok := file.FlatFindChild(fn, kind)
	if !ok {
		return 0
	}
	n := 0
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case param, "spread_parameter":
			n++
		}
	}
	return n
}

// lintDirectClassOf returns the class-like declaration whose body directly contains the member.
func lintDirectClassOf(file *scanner.File, member uint32) (uint32, bool) {
	body, ok := file.FlatParent(member)
	if !ok || file.FlatType(body) != "class_body" {
		return 0, false
	}
	cls, ok := file.FlatParent(body)
	if !ok {
		return 0, false
	}
	switch file.FlatType(cls) {
	case "class_declaration", "object_declaration", "object_literal", "object_creation_expression":
		return cls, true
	}
	return 0, false
}

// ---- AccessibilityWindowStateChangedEvent ----

// AccessibilityWindowStateChangedEventRule mirrors Lint's AccessibilityWindowStateChangedEvent: sending
// TYPE_WINDOW_STATE_CHANGED through View.sendAccessibilityEvent, or naming it inside the populate-event overrides.
type AccessibilityWindowStateChangedEventRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *AccessibilityWindowStateChangedEventRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AccessibilityWindowStateChangedEventRule) check(ctx *api.Context) {
	file := ctx.File
	switch file.FlatType(ctx.Idx) {
	case "function_declaration", "method_declaration":
		r.checkPopulateOverride(ctx, ctx.Idx)
	default:
		r.checkSend(ctx)
	}
}

func (r *AccessibilityWindowStateChangedEventRule) checkSend(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "sendAccessibilityEvent" || len(call.Args) != 1 || call.Args[0] == 0 {
		return
	}
	if v, known := lintEvalIntLenient(file, call.Args[0], windowStateChangedConsts); !known || v != windowStateChangedValue {
		return
	}
	if !lintCallIsMemberOf(ctx, call, androidViewClass, lintKnownViewSimpleNames) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", windowStateChangedMessage))
}

// checkPopulateOverride reports each TYPE_WINDOW_STATE_CHANGED reference in an overridden populate-event method of a View.
func (r *AccessibilityWindowStateChangedEventRule) checkPopulateOverride(ctx *api.Context, fn uint32) {
	file := ctx.File
	name := lintDeclaredMethodName(file, fn)
	if name == 0 {
		return
	}
	switch file.FlatNodeText(name) {
	case "dispatchPopulateAccessibilityEvent", "onPopulateAccessibilityEvent":
	default:
		return
	}
	if file.FlatType(fn) == "function_declaration" && !file.FlatHasModifier(fn, "override") {
		return
	}
	cls, ok := lintDirectClassOf(file, fn)
	if !ok || !lintClassExtends(ctx, cls, androidViewClass, lintKnownViewSimpleNames) {
		return
	}
	ident := "simple_identifier"
	if file.Language == scanner.LangJava {
		ident = "identifier"
	}
	file.FlatWalkNodes(fn, ident, func(ref uint32) {
		if file.FlatNodeText(ref) == windowStateChangedName && lintRefIsAccessibilityEventField(file, ref) {
			ctx.Emit(lintFindingAtNode(ctx, ref, "", windowStateChangedMessage))
		}
	})
}

// lintRefIsAccessibilityEventField reports whether the identifier names AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED,
// qualified by the class or imported statically.
func lintRefIsAccessibilityEventField(file *scanner.File, ref uint32) bool {
	if qualifier := lintFieldQualifier(file, ref); qualifier != "" {
		return lintTypeRefMatches(file, qualifier, accessibilityEventClass)
	}
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[windowStateChangedName]; ok {
		return full == accessibilityEventClass+"."+windowStateChangedName
	}
	for _, star := range imp.stars {
		if star == accessibilityEventClass {
			return true
		}
	}
	return false
}

// lintFieldQualifier returns the text before the dot of a qualified field reference whose selector is ref, or "".
func lintFieldQualifier(file *scanner.File, ref uint32) string {
	parent, ok := file.FlatParent(ref)
	if !ok {
		return ""
	}
	switch file.FlatType(parent) {
	case "navigation_suffix":
		nav, ok := file.FlatParent(parent)
		if !ok {
			return ""
		}
		if recv := file.FlatNamedChild(nav, 0); recv != 0 && recv != parent {
			return strings.TrimSpace(file.FlatNodeText(recv))
		}
	case "field_access":
		if recv := file.FlatNamedChild(parent, 0); recv != 0 && recv != ref {
			return strings.TrimSpace(file.FlatNodeText(recv))
		}
	}
	return ""
}

// ---- AccessibilityFocus ----

// AccessibilityFocusRule mirrors Lint's AccessibilityFocus: View.performAccessibilityAction with ACTION_ACCESSIBILITY_FOCUS.
type AccessibilityFocusRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the receiver is proved from declarations when no resolved target exists.
func (r *AccessibilityFocusRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AccessibilityFocusRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "performAccessibilityAction" || len(call.Args) != 2 || call.Args[0] == 0 {
		return
	}
	if !lintForcesAccessibilityFocus(file, call.Args[0]) || !lintCallIsMemberOf(ctx, call, androidViewClass, lintKnownViewSimpleNames) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "",
		"Do not force accessibility focus, as this interferes with screen readers and gives an inconsistent user experience, especially across apps"))
}

// lintForcesAccessibilityFocus reports whether the action argument folds to 64 or mentions ACTION_ACCESSIBILITY_FOCUS.
func lintForcesAccessibilityFocus(file *scanner.File, action uint32) bool {
	if v, known := lintEvalIntLenient(file, action, accessibilityFocusConsts); known && v == accessibilityFocusValue {
		return true
	}
	found := false
	file.FlatWalkAllNodes(action, func(n uint32) {
		switch file.FlatType(n) {
		case "simple_identifier", "identifier":
			found = found || file.FlatNodeText(n) == accessibilityFocusName
		}
	})
	return found
}

// ---- shared helpers ----

// lintEvalIntLenient folds an integer expression like Lint's UExpression.evaluate: constants, read-only locals and
// properties, same-file `Class.NAME` constants, and Int conversions.
func lintEvalIntLenient(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	return lintEvalIntWith(file, idx, consts, false)
}

// lintEvalIntLocals is lintEvalIntLenient that also follows a mutable local that nothing reassigns, as Lint's
// ConstantEvaluator does.
func lintEvalIntLocals(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	return lintEvalIntWith(file, idx, consts, true)
}

func lintEvalIntWith(file *scanner.File, idx uint32, consts map[string]int64, mutableLocals bool) (int64, bool) {
	if idx == 0 {
		return 0, false
	}
	p := &lintEvalParser{file: file, consts: consts, scope: idx, toks: lintEvalTokens(file.FlatNodeText(idx)), lenient: true, mutableLocals: mutableLocals}
	v, ok := p.expr(0)
	if !ok || p.pos != len(p.toks) {
		return 0, false
	}
	return v, true
}

// lintCallIsMemberOf reports whether the called method is a member of fqn. A resolved call target decides when there
// is one. Otherwise an unqualified or `this` call needs an enclosing class that extends fqn, and a qualified call
// needs a receiver whose declared type does.
func lintCallIsMemberOf(ctx *api.Context, call lintCall, fqn string, known map[string]struct{}) bool {
	if verdict, resolved := lintResolvedOwnerExtends(ctx, call, fqn); resolved {
		return verdict
	}
	file := ctx.File
	if call.ReceiverNode == 0 || lintIsSelfReceiver(call.ReceiverText) {
		for cls, ok := lintEnclosingClassLike(file, call.Idx); ok; cls, ok = lintEnclosingClassLike(file, cls) {
			// A class that declares the method itself, or that cannot see its outer instance, ends the search.
			if lintClassExtends(ctx, cls, fqn, known) {
				return true
			}
			if lintClassDeclaresMethod(file, cls, call.Name) || !lintOuterInstanceVisible(file, cls) {
				return false
			}
		}
		return false
	}
	typ := lintReceiverClassText(file, call)
	return typ != "" && lintTypeNameReaches(ctx, typ, fqn, known, 0)
}

// lintClassDeclaresMethod reports whether the class body directly declares a function or method called name.
func lintClassDeclaresMethod(file *scanner.File, cls uint32, name string) bool {
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return false
	}
	for member := file.FlatFirstChild(body); member != 0; member = file.FlatNextSib(member) {
		switch file.FlatType(member) {
		case "function_declaration", "method_declaration":
			if n := lintDeclaredMethodName(file, member); n != 0 && file.FlatNodeText(n) == name {
				return true
			}
		}
	}
	return false
}

// lintOuterInstanceVisible reports whether code in the class can call members of the enclosing class: anonymous and
// local classes, Kotlin `inner` classes, and non-static Java member classes.
func lintOuterInstanceVisible(file *scanner.File, cls uint32) bool {
	switch file.FlatType(cls) {
	case "object_literal", "object_creation_expression":
		return true
	case "class_declaration":
		if _, local := flatEnclosingCallable(file, cls); local {
			return true
		}
		if file.Language == scanner.LangJava {
			return !file.FlatHasModifier(cls, "static")
		}
		return file.FlatHasModifier(cls, "inner")
	}
	return false
}

// lintIsSelfReceiver reports whether a receiver names the current instance: `this`, `super`, `this@Outer`, `Outer.this`.
func lintIsSelfReceiver(text string) bool {
	return text == "this" || text == "super" || strings.HasPrefix(text, "this@") || strings.HasPrefix(text, "super@") ||
		strings.HasSuffix(text, ".this") || strings.HasSuffix(text, ".super")
}

// lintEnclosingClassLike returns the nearest class-like declaration around idx. An anonymous Java class counts only
// when it has a body.
func lintEnclosingClassLike(file *scanner.File, idx uint32) (uint32, bool) {
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

// lintReceiverClassText names the receiver's class from its declaration, constructor call, or cast.
func lintReceiverClassText(file *scanner.File, call lintCall) string {
	recv := flatUnwrapParenExpr(file, call.ReceiverNode)
	switch file.FlatType(recv) {
	case "as_expression":
		if typ := flatLastNamedChild(file, recv); typ != 0 {
			return strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(typ)), "?")
		}
	case "cast_expression":
		if typ := file.FlatNamedChild(recv, 0); typ != 0 {
			return strings.TrimSpace(file.FlatNodeText(typ))
		}
	}
	return lintReceiverTypeText(file, call)
}

// lintResolvedOwnerExtends asks the compiler facts for the declaring class of the call. The second result is false when
// no facts resolve the call.
func lintResolvedOwnerExtends(ctx *api.Context, call lintCall, fqn string) (verdict, resolved bool) {
	if fact, ok := javaSemanticCallFact(ctx, call.Idx); ok && fact.MethodOwner != "" {
		return javaProfileTypeMatches(ctx, fact.MethodOwner, fqn), true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call.Idx)
	if !ok || !target.Resolved {
		return false, false
	}
	owner := strings.ReplaceAll(target.QualifiedName, "#", ".")
	if dot := strings.LastIndex(owner, "."); dot >= 0 {
		owner = owner[:dot]
	}
	if owner == fqn {
		return true, true
	}
	return lintHierarchyReaches(ctx.Resolver, owner, fqn, nil, map[string]bool{}), true
}

// lintClassExtends reports whether the class-like declaration extends fqn through its declared supertypes.
func lintClassExtends(ctx *api.Context, cls uint32, fqn string, known map[string]struct{}) bool {
	return lintClassExtendsDepth(ctx, cls, fqn, known, 0)
}

func lintClassExtendsDepth(ctx *api.Context, cls uint32, fqn string, known map[string]struct{}, depth int) bool {
	if depth > 6 {
		return false
	}
	for _, ref := range lintDirectSupertypeRefs(ctx.File, cls) {
		if lintTypeNameReaches(ctx, ref.name, fqn, known, depth+1) {
			return true
		}
	}
	return false
}

// lintDirectSupertypeRefs lists the supertype names written on a Kotlin or Java class-like declaration.
func lintDirectSupertypeRefs(file *scanner.File, cls uint32) []androidSupertypeRef {
	if file.Language != scanner.LangJava {
		return androidDirectSupertypesFlat(file, cls)
	}
	var names []string
	if file.FlatType(cls) == "object_creation_expression" {
		for child := file.FlatFirstChild(cls); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				names = append(names, file.FlatNodeText(child))
			}
		}
	} else if sc, ok := file.FlatFindChild(cls, "superclass"); ok {
		for child := file.FlatFirstChild(sc); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "scoped_type_identifier", "generic_type":
				names = append(names, file.FlatNodeText(child))
			}
		}
	}
	refs := make([]androidSupertypeRef, 0, len(names))
	for _, name := range names {
		if i := strings.Index(name, "<"); i >= 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		refs = append(refs, androidSupertypeRef{name: name, simple: androidSimpleName(name), qualified: strings.Contains(name, ".")})
	}
	return refs
}

// lintTypeNameReaches reports whether the type written as name is fqn or a subtype of it. It follows classes declared
// in the file, the resolver hierarchy, and the known framework simple names.
func lintTypeNameReaches(ctx *api.Context, name, fqn string, known map[string]struct{}, depth int) bool {
	file := ctx.File
	name = strings.TrimSpace(name)
	if depth > 6 || name == "" {
		return false
	}
	if i := strings.Index(name, "<"); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSuffix(name, "?")
	if name == fqn || lintTypeRefMatches(file, name, fqn) {
		return true
	}
	simple := androidSimpleName(name)
	qualified := strings.Contains(name, ".")
	if !qualified {
		if local := lintSameFileClass(file, simple); local != 0 {
			return lintClassExtendsDepth(ctx, local, fqn, known, depth+1)
		}
	}
	full := name
	if !qualified && ctx.Resolver != nil {
		if resolved := ctx.Resolver.ResolveImport(simple, file); resolved != "" {
			full = resolved
		}
	}
	if full == fqn || lintHierarchyReaches(ctx.Resolver, full, fqn, known, map[string]bool{}) {
		return true
	}
	_, ok := known[simple]
	return ok && lintAndroidTypeName(full)
}

// lintAndroidTypeName reports whether a type name is unqualified or lives in an Android package.
func lintAndroidTypeName(name string) bool {
	return !strings.Contains(name, ".") || strings.HasPrefix(name, "android.") || strings.HasPrefix(name, "androidx.") ||
		strings.HasPrefix(name, "com.google.android.")
}

// lintHierarchyReaches reports whether the resolver's class hierarchy for name reaches fqn.
func lintHierarchyReaches(resolver typeinfer.TypeResolver, name, fqn string, known map[string]struct{}, seen map[string]bool) bool {
	if resolver == nil || name == "" || seen[name] {
		return false
	}
	seen[name] = true
	if name == fqn {
		return true
	}
	if _, ok := known[androidSimpleName(name)]; ok && lintAndroidTypeName(name) {
		return true
	}
	info := resolver.ClassHierarchy(name)
	if info == nil {
		return false
	}
	if info.FQN == fqn {
		return true
	}
	for _, super := range info.Supertypes {
		if lintHierarchyReaches(resolver, super, fqn, known, seen) {
			return true
		}
	}
	return false
}
