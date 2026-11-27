package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnsafeIntentLaunchRule mirrors Lint's UnsafeIntentLaunch (UnsafeIntentLaunchDetector): an intent that may come
// from another app (an extra of the incoming intent, the incoming intent itself, or a component callback's intent
// parameter) launched by an exported, unprotected component without being sanitized.
type UnsafeIntentLaunchRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receivers are proven from declarations and supertypes in
// project sources, and the data flow is followed through same-file helpers.
func (r *UnsafeIntentLaunchRule) Confidence() float64 { return api.ConfidenceMedium }

// unsafeIntentCallbacks are the component callbacks whose Intent parameter Lint treats as untrusted.
var unsafeIntentCallbacks = map[string][]string{
	"activity": {"onNewIntent", "onActivityResult", "onActivityReenter"},
	"service":  {"onBind", "onUnbind", "onRebind", "onTaskRemoved", "onStartCommand", "onStart"},
	"receiver": {"onReceive"},
}

// unsafeIntentActivityLaunchMethods are the Activity members Lint counts as launching an intent.
var unsafeIntentActivityLaunchMethods = lintStringSet("createPendingResult", "navigateUpTo", "navigateUpToFromChild",
	"startActivityIfNeeded", "startActivityForResult", "startActivityFromChild", "startActivityFromFragment",
	"startIntentSender", "startIntentSenderFromChild", "startIntentSenderForResult", "startNextMatchingActivity",
	"setResult")

// unsafeIntentContextLaunchMethods are the Context members that take an Intent to start, bind or broadcast.
var unsafeIntentContextLaunchMethods = lintStringSet("startActivity", "startActivities", "startService",
	"startForegroundService", "stopService", "bindService", "bindIsolatedService", "bindServiceAsUser",
	"sendBroadcast", "sendBroadcastAsUser", "sendBroadcastWithMultiplePermissions", "sendOrderedBroadcast",
	"sendOrderedBroadcastAsUser", "sendStickyBroadcast", "sendStickyBroadcastAsUser", "sendStickyOrderedBroadcast",
	"sendStickyOrderedBroadcastAsUser", "removeStickyBroadcast", "removeStickyBroadcastAsUser", "startIntentSender")

// unsafeIntentPendingIntentMethods are the PendingIntent factories Lint counts as launching an intent.
var unsafeIntentPendingIntentMethods = lintStringSet("getActivity", "getBroadcast", "getService", "getForegroundService")

// intentReturningNotSelf are Intent methods that return another intent, which is not the tracked one.
var intentReturningNotSelf = lintStringSet("cloneFilter", "getOriginalIntent", "getSelector", "getParcelableExtra")

var (
	activityBaseClasses = lintStringSet("android.app.Activity", "androidx.appcompat.app.AppCompatActivity",
		"androidx.fragment.app.FragmentActivity", "androidx.activity.ComponentActivity",
		"androidx.core.app.ComponentActivity", "android.app.ListActivity", "android.preference.PreferenceActivity",
		"android.app.NativeActivity", "android.app.AliasActivity", "android.app.ExpandableListActivity",
		"android.app.LauncherActivity", "android.accounts.AccountAuthenticatorActivity")
	serviceBaseClasses = lintStringSet("android.app.Service", "android.app.IntentService",
		"androidx.core.app.JobIntentService", "androidx.lifecycle.LifecycleService", "android.app.job.JobService",
		"android.accessibilityservice.AccessibilityService", "android.service.notification.NotificationListenerService",
		"android.inputmethodservice.InputMethodService", "android.net.VpnService", "android.service.wallpaper.WallpaperService",
		"androidx.media.MediaBrowserServiceCompat", "android.service.media.MediaBrowserService",
		"androidx.media3.session.MediaSessionService", "androidx.media3.session.MediaLibraryService",
		"android.nfc.cardemulation.HostApduService", "com.google.firebase.messaging.FirebaseMessagingService")
	receiverBaseClasses = lintStringSet("android.content.BroadcastReceiver", "android.appwidget.AppWidgetProvider",
		"android.app.admin.DeviceAdminReceiver", "androidx.media.session.MediaButtonReceiver")
)

// unsafeIntentContextClasses are classes that are, or extend, android.content.Context.
var unsafeIntentContextClasses = func() map[string]bool {
	out := lintStringSet("android.content.Context", "android.content.ContextWrapper", "android.view.ContextThemeWrapper",
		"androidx.appcompat.view.ContextThemeWrapper", "android.app.Application")
	for _, set := range []map[string]bool{activityBaseClasses, serviceBaseClasses} {
		for fqn := range set {
			out[fqn] = true
		}
	}
	return out
}()

const unsafeIntentMaxCallDepth = 3

// unsafeIntentScan holds the project-wide indexes one run of the rule needs.
type unsafeIntentScan struct {
	ctx      *api.Context
	classes  *lintSourceIndex
	exported map[string]bool
	reported map[string]bool
	// functions indexes the project's top-level functions and class or object members that take an Intent, by
	// owner and name, built on first use (#1623).
	functions map[unsafeIntentFunctionKey][]unsafeIntentFunction
}

// unsafeIntentFunctionKey names a function by the simple name of its class or object ("" for a top-level function)
// and its own name.
type unsafeIntentFunctionKey struct {
	owner, name string
}

// unsafeIntentFunction is one function or method declaration in the project, with the facts a cross-file lookup
// compares, computed once when the index is built.
type unsafeIntentFunction struct {
	file *scanner.File
	fn   uint32
	pkg  string
	// ownerFQN is the qualified name of the declaring class or object ("pkg.Outer.Launcher"), or "" for a top-level
	// function.
	ownerFQN string
	params   int
}

func (r *UnsafeIntentLaunchRule) check(ctx *api.Context) {
	s := &unsafeIntentScan{ctx: ctx, classes: lintSourceClasses(ctx.ParsedFiles), reported: map[string]bool{}}
	s.exported = unsafeIntentExportedComponents(ctx.Manifests, s.classes)
	s.addRuntimeRegisteredReceivers()
	if len(s.exported) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || scanner.IsTestFile(file.Path) {
			continue
		}
		s.scanFile(file)
	}
}

// unsafeIntentExportedComponents returns the activities, receivers and services that Lint's getProtectedComponent
// calls unprotected: exported (explicitly, or implicitly through an intent filter) and not guarded by a permission
// that is probably signature-level.
func unsafeIntentExportedComponents(manifests []*manifest.Manifest, classes *lintSourceIndex) map[string]bool {
	out := map[string]bool{}
	for _, m := range manifests {
		if m == nil || m.Root == nil {
			continue
		}
		for _, app := range m.Root.ChildrenByTag("application") {
			for _, comp := range app.Children {
				switch comp.Tag {
				case "activity", "receiver", "service":
				default:
					continue
				}
				exported := comp.AttrValue("android:exported")
				if exported != "true" && (exported != "" || len(comp.ChildrenByTag("intent-filter")) == 0) {
					continue
				}
				if perm := strings.TrimSpace(comp.AttrValue("android:permission")); perm != "" && !lintKnownNormalPermissions[perm] {
					continue
				}
				name := comp.AttrValue("android:name")
				fqn := name
				if strings.HasPrefix(name, ".") {
					fqn = m.Package + name
					if m.Package == "" {
						if resolved, cls := lintManifestClass(m, name, classes); cls != nil {
							fqn = resolved
						}
					}
				}
				out[fqn] = true
			}
		}
	}
	return out
}

func (s *unsafeIntentScan) scanFile(file *scanner.File) {
	// Component callbacks: the intent parameter of onNewIntent, onStartCommand, onReceive, ...
	file.FlatWalkNodes(0, "class_declaration", func(cls uint32) {
		s.scanComponentClass(file, cls)
	})
	// Calls that read an intent from outside: getIntent(), getParcelableExtra, getParcelable, parseUri.
	file.FlatWalkAllNodes(0, func(n uint32) {
		switch file.FlatType(n) {
		case "call_expression", "method_invocation", "simple_identifier":
		default:
			return
		}
		at, ok := s.untrustedSource(file, n)
		if !ok {
			return
		}
		fn := filenameFlowScope(file, n)
		if fn == 0 {
			return
		}
		flow := s.newFlow(file, fn, 0)
		flow.startNodes = []uint32{n}
		flow.run()
		if flow.launched {
			s.report(file, at, flow.incidentClass)
		}
	})
}

// scanComponentClass starts a flow from the Intent parameter of each callback the component declares.
func (s *unsafeIntentScan) scanComponentClass(file *scanner.File, cls uint32) {
	c := s.classes.classAt(file, cls)
	if c == nil {
		return
	}
	kind := ""
	switch {
	case s.classes.extendsTarget(c, activityBaseClasses):
		kind = "activity"
	case s.classes.extendsTarget(c, serviceBaseClasses):
		kind = "service"
	case s.classes.extendsTarget(c, receiverBaseClasses):
		kind = "receiver"
	default:
		return
	}
	for _, callback := range unsafeIntentCallbacks[kind] {
		stop := false
		javascriptInterfaceEachMember(file, cls, func(member uint32) {
			if stop || javascriptInterfaceMemberName(file, member) != callback {
				return
			}
			param, paramName := unsafeIntentIntentParameter(file, member)
			if param == 0 {
				// Lint stops checking the class at a callback without an Intent parameter.
				stop = true
				return
			}
			flow := s.newFlow(file, member, 0)
			flow.names[paramName] = true
			flow.checkProtectedBroadcast = kind == "receiver"
			flow.run()
			if flow.launched {
				s.report(file, param, flow.incidentClass)
			}
		})
		if stop {
			return
		}
	}
}

// unsafeIntentIntentParameter returns the first parameter of the function typed android.content.Intent.
func unsafeIntentIntentParameter(file *scanner.File, fn uint32) (uint32, string) {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		params, ok = file.FlatFindChild(fn, "formal_parameters")
	}
	if !ok {
		return 0, ""
	}
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "parameter", "formal_parameter":
		default:
			continue
		}
		typ := strings.TrimSuffix(lintChildTypeText(file, p), "?")
		if typ == "" || !lintTypeRefMatches(file, typ, intentClass) {
			continue
		}
		name := lintDeclaredName(file, p)
		if name == "" {
			if id, ok := file.FlatFindChild(p, "identifier"); ok {
				name = file.FlatNodeText(id)
			}
		}
		if name != "" {
			return p, name
		}
	}
	return 0, ""
}

// untrustedSource reports whether node reads an intent Lint treats as untrusted, and the node its finding points at.
func (s *unsafeIntentScan) untrustedSource(file *scanner.File, n uint32) (uint32, bool) {
	if file.FlatType(n) == "simple_identifier" {
		// Kotlin's `intent` property is Activity.getIntent().
		if file.FlatNodeText(n) == "intent" && s.isActivityIntentProperty(file, n) {
			return n, true
		}
		return 0, false
	}
	call, ok := lintCallAt(file, n)
	if !ok {
		return 0, false
	}
	at := n
	switch call.Name {
	case "getIntent":
		if len(call.Args) == 0 && s.receiverIsContext(file, call) {
			return at, true
		}
	case "getParcelableExtra":
		if call.ReceiverNode != 0 && (s.isIntentValue(file, call.ReceiverNode, n) ||
			lintTypeRefMatches(file, call.ReceiverText, "androidx.core.content.IntentCompat")) {
			return at, true
		}
	case "getParcelable":
		if call.ReceiverNode != 0 && (s.isBundleValue(file, call.ReceiverNode, n) ||
			lintTypeRefMatches(file, call.ReceiverText, "androidx.core.os.BundleCompat")) {
			return at, true
		}
	case "parseUri":
		if lintTypeRefMatches(file, call.ReceiverText, intentClass) && len(call.Args) > 0 && s.isStringExtra(file, call.Args[0], n) {
			return at, true
		}
	}
	return 0, false
}

// isActivityIntentProperty reports whether a Kotlin `intent` reference reads Activity.getIntent(): unqualified in an
// activity (and not shadowed), or `<activity>.intent`.
func (s *unsafeIntentScan) isActivityIntentProperty(file *scanner.File, n uint32) bool {
	if file.Language == scanner.LangJava {
		return false
	}
	parent, ok := file.FlatParent(n)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "navigation_suffix":
		nav, ok := file.FlatParent(parent)
		if !ok || file.FlatType(nav) != "navigation_expression" {
			return false
		}
		recv := file.FlatNamedChild(nav, 0)
		if strings.TrimSpace(file.FlatNodeText(recv)) == "this" {
			return s.enclosingClassExtends(file, n, activityBaseClasses)
		}
		return s.valueIsType(file, recv, n, activityBaseClasses, []string{"requireActivity()", "activity", "getActivity()"})
	case "directly_assignable_expression", "value_argument_label", "variable_declaration", "parameter":
		return false
	}
	if grand, ok := file.FlatParent(parent); ok && file.FlatType(parent) == "navigation_expression" &&
		file.FlatNamedChild(parent, 0) == n && file.FlatType(grand) == "directly_assignable_expression" {
		return false
	}
	if lintNameIsParameterAt(file, n, "intent") || lintLocalDeclarationBeforeII(file, n, "intent") {
		return false
	}
	return s.enclosingClassExtends(file, n, activityBaseClasses)
}

// lintLocalDeclarationBeforeII reports whether a local variable named name is declared before use in its function.
func lintLocalDeclarationBeforeII(file *scanner.File, use uint32, name string) bool {
	fn, ok := flatEnclosingCallable(file, use)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && lintDeclaredName(file, vd) == name &&
			file.FlatStartByte(decl) < file.FlatStartByte(use) {
			found = true
		}
	})
	return found
}

// enclosingClassExtends reports whether the innermost named class around n extends one of the bases.
func (s *unsafeIntentScan) enclosingClassExtends(file *scanner.File, n uint32, bases map[string]bool) bool {
	for cur, ok := file.FlatParent(n); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "object_literal", "lambda_literal":
			if file.FlatType(cur) == "object_literal" {
				return false
			}
		case "class_declaration":
			c := s.classes.classAt(file, cur)
			return c != nil && s.classes.extendsTarget(c, bases)
		}
	}
	return false
}

// valueIsType reports whether an expression is declared with one of the types (or a project subclass), or is one of
// the known accessor expressions.
func (s *unsafeIntentScan) valueIsType(file *scanner.File, expr, scope uint32, bases map[string]bool, accessors []string) bool {
	expr = lintSkipParensJS(file, expr)
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	text = strings.TrimSuffix(text, "!!")
	for _, a := range accessors {
		if text == a {
			return true
		}
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		typ := strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, scope, text)), "?")
		if typ == "" {
			return false
		}
		return s.typeExtends(file, typ, bases)
	}
	return false
}

// typeExtends reports whether a written type names one of the bases or a project class that extends one.
func (s *unsafeIntentScan) typeExtends(file *scanner.File, typ string, bases map[string]bool) bool {
	for fqn := range bases {
		if lintTypeRefMatches(file, typ, fqn) {
			return true
		}
	}
	scope := s.classes.scope(file)
	probe := &preferenceClassDecl{fqn: scope.pkg + ".__probe", supers: []string{typ}, file: scope}
	if c := s.classes.classes[resolvePreferenceSuper(probe, typ, s.classes.byFQN)]; c != nil {
		return s.classes.extendsTarget(c, bases)
	}
	return false
}

// receiverIsContext reports whether a call is made on a Context: unqualified (or on `this`) inside a class that
// extends Context, on a value declared as a Context type, or on a known Context accessor.
func (s *unsafeIntentScan) receiverIsContext(file *scanner.File, call lintCall) bool {
	if call.ReceiverNode == 0 || strings.TrimSpace(call.ReceiverText) == "this" {
		return s.enclosingClassExtends(file, call.Idx, unsafeIntentContextClasses)
	}
	return s.valueIsType(file, call.ReceiverNode, call.Idx, unsafeIntentContextClasses, []string{"context",
		"getContext()", "requireContext()", "applicationContext", "getApplicationContext()", "baseContext",
		"getBaseContext()", "activity", "getActivity()", "requireActivity()", "itemView.context",
		"view.context", "requireView().context"})
}

// isIntentValue reports whether expr is an android.content.Intent: declared with that type, the activity's intent,
// or a getIntent() call.
func (s *unsafeIntentScan) isIntentValue(file *scanner.File, expr, scope uint32) bool {
	expr = lintSkipParensJS(file, expr)
	text := strings.TrimSuffix(strings.Join(strings.Fields(file.FlatNodeText(expr)), ""), "!!")
	if text == "intent" && s.isActivityIntentProperty(file, expr) {
		return true
	}
	if c, ok := lintCallAt(file, expr); ok && c.Name == "getIntent" && len(c.Args) == 0 && s.receiverIsContext(file, c) {
		return true
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		typ := strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, scope, text)), "?")
		return typ != "" && lintTypeRefMatches(file, typ, intentClass)
	}
	return false
}

// isBundleValue reports whether expr is an android.os.Bundle: declared with that type, or the extras of an intent.
func (s *unsafeIntentScan) isBundleValue(file *scanner.File, expr, scope uint32) bool {
	expr = lintSkipParensJS(file, expr)
	text := strings.TrimSuffix(strings.Join(strings.Fields(file.FlatNodeText(expr)), ""), "!!")
	for _, suffix := range []string{".extras", ".getExtras()", "?.extras"} {
		if strings.HasSuffix(text, suffix) {
			return true
		}
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		typ := strings.TrimSuffix(strings.TrimSpace(lintDeclaredTypeText(file, scope, text)), "?")
		return typ != "" && lintTypeRefMatches(file, typ, "android.os.Bundle")
	}
	return false
}

// isStringExtra reports whether the argument of parseUri is Intent.getStringExtra or Bundle.getString, directly or
// through the last assignment of a local variable.
func (s *unsafeIntentScan) isStringExtra(file *scanner.File, arg, scope uint32) bool {
	arg = lintSkipParensJS(file, arg)
	switch file.FlatType(arg) {
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, scope, file.FlatNodeText(arg), false); init != 0 {
			arg = lintSkipParensJS(file, init)
		}
	}
	call, ok := lintCallAt(file, arg)
	if !ok || call.ReceiverNode == 0 {
		return false
	}
	switch call.Name {
	case "getStringExtra":
		return s.isIntentValue(file, call.ReceiverNode, arg)
	case "getString":
		return s.isBundleValue(file, call.ReceiverNode, arg)
	}
	return false
}

func (s *unsafeIntentScan) report(file *scanner.File, at uint32, component string) {
	if component == "" || !s.exported[component] {
		return
	}
	key := file.Path + ":" + strconv.Itoa(int(file.FlatStartByte(at)))
	if s.reported[key] {
		return
	}
	s.reported[key] = true
	f := lintFindingAtNodeIn(file, at, "This intent could be coming from an untrusted source. It is later launched by "+
		"an unprotected component "+component+". You could either make the component "+component+" protected; or "+
		"sanitize this intent using androidx.core.content.IntentSanitizer.")
	s.ctx.Emit(f)
}

// unsafeIntentFlow is Lint's IntentLaunchChecker: a data flow over one function from the untrusted intent.
type unsafeIntentFlow struct {
	scan                    *unsafeIntentScan
	file                    *scanner.File
	fn                      uint32
	depth                   int
	startNodes              []uint32
	names                   map[string]bool
	checkProtectedBroadcast bool
	incidentClass           string
	incidentKnown           bool
	launched, returned      bool
}

func (s *unsafeIntentScan) newFlow(file *scanner.File, fn uint32, depth int) *unsafeIntentFlow {
	return &unsafeIntentFlow{scan: s, file: file, fn: fn, depth: depth, names: map[string]bool{}}
}

func (f *unsafeIntentFlow) run() { f.visit(f.fn) }

func (f *unsafeIntentFlow) isTracked(n uint32) bool {
	file := f.file
	n = lintSkipParensJS(file, n)
	for {
		// `x!!` passes the tracked value through.
		if file.FlatType(n) == "postfix_expression" && strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(n)), "!!") {
			n = file.FlatNamedChild(n, 0)
			continue
		}
		break
	}
	for _, t := range f.startNodes {
		if t == n {
			return true
		}
	}
	switch file.FlatType(n) {
	case "simple_identifier", "identifier":
		return f.names[file.FlatNodeText(n)]
	}
	return false
}

func (f *unsafeIntentFlow) visit(n uint32) {
	file := f.file
	// The lambda of a let or also on the intent sees it as its parameter; mark it before visiting the body.
	if t := file.FlatType(n); t == "call_expression" {
		if call, ok := lintCallAt(file, n); ok && (call.Name == "let" || call.Name == "also") && call.ReceiverNode != 0 {
			f.visit(call.ReceiverNode)
			if f.isTracked(call.ReceiverNode) {
				if name := scopeLambdaParam(file, n); name != "" {
					f.names[name] = true
				}
			}
		}
	}
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration", "local_class_declaration", "object_literal":
			continue
		}
		f.visit(child)
	}
	switch file.FlatType(n) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok {
			f.names[lintDeclaredName(file, vd)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "variable_declarator":
		if id, ok := file.FlatFindChild(n, "identifier"); ok {
			f.names[file.FlatNodeText(id)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "assignment", "assignment_expression":
		lhs := file.FlatNamedChild(n, 0)
		rhs := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1)
		if name := strings.TrimSpace(file.FlatNodeText(lhs)); lintIdentOnly(name) {
			f.names[name] = f.isTracked(rhs)
		}
	case "jump_expression", "return_statement":
		text := strings.TrimSpace(file.FlatNodeText(n))
		if strings.HasPrefix(text, "return") {
			if value := file.FlatNamedChild(n, 0); value != 0 && f.isTracked(value) {
				f.returned = true
			}
		}
	case "call_expression", "method_invocation", "object_creation_expression":
		f.call(n)
	}
}

// scopeLambdaParam returns the name a let/also lambda gives its argument: its declared parameter, or `it`.
func scopeLambdaParam(file *scanner.File, call uint32) string {
	lambda := uint32(0)
	file.FlatWalkNodes(call, "lambda_literal", func(n uint32) {
		if lambda == 0 {
			lambda = n
		}
	})
	if lambda == 0 {
		return ""
	}
	if params, ok := file.FlatFindChild(lambda, "lambda_parameters"); ok {
		if name := strings.TrimSpace(file.FlatNodeText(params)); lintIdentOnly(name) {
			return name
		}
		return ""
	}
	return "it"
}

func (f *unsafeIntentFlow) call(n uint32) {
	file := f.file
	call, ok := lintCallAt(file, n)
	if !ok {
		return
	}
	if call.ReceiverNode != 0 && f.isTracked(call.ReceiverNode) {
		switch {
		case call.Name == "apply" || call.Name == "also":
			// apply and also return their receiver.
			f.startNodes = append(f.startNodes, n)
		case !intentReturningNotSelf[call.Name] && intentBuilderMethods[call.Name]:
			f.startNodes = append(f.startNodes, n)
		}
	}
	for i, arg := range call.Args {
		if arg == 0 || !f.isTracked(arg) {
			continue
		}
		f.argument(call, i)
	}
}

func (f *unsafeIntentFlow) argument(call lintCall, index int) {
	file := f.file
	if !f.incidentKnown {
		f.incidentKnown = true
		f.incidentClass = f.scan.enclosingClassFQN(file, call.Idx)
	}
	if f.scan.isLaunch(file, call) {
		if !f.checkProtectedBroadcast || !f.inProtectedBroadcastBranch(call.Idx) {
			f.launched = true
		}
		return
	}
	if f.depth >= unsafeIntentMaxCallDepth {
		return
	}
	calleeFile, callee, param := f.scan.resolveProjectCallee(file, call, index)
	if callee == 0 || param == "" {
		return
	}
	inner := f.scan.newFlow(calleeFile, callee, f.depth+1)
	inner.names[param] = true
	inner.incidentClass, inner.incidentKnown = f.incidentClass, f.incidentKnown
	inner.run()
	// A helper called in a branch that tests for a protected broadcast only sees that broadcast, so its launch is
	// exempt as a direct launch there is; a returned intent is still tracked.
	protected := inner.launched && f.checkProtectedBroadcast && f.inProtectedBroadcastBranch(call.Idx)
	switch {
	case inner.launched && !protected:
		f.launched = true
		if !f.incidentKnown || f.incidentClass == "" {
			f.incidentClass, f.incidentKnown = inner.incidentClass, true
		}
	case inner.returned:
		f.startNodes = append(f.startNodes, call.Idx)
	}
}

// enclosingClassFQN returns the qualified name of the named class around n, or "" inside an anonymous object.
func (s *unsafeIntentScan) enclosingClassFQN(file *scanner.File, n uint32) string {
	for cur, ok := file.FlatParent(n); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "object_literal":
			return ""
		case "object_creation_expression":
			if _, anon := file.FlatFindChild(cur, "class_body"); anon {
				return ""
			}
		case "class_declaration", "object_declaration":
			name := extractIdentifierFlat(file, cur)
			if name == "" {
				if id, ok := file.FlatFindChild(cur, "type_identifier"); ok {
					name = file.FlatNodeText(id)
				}
			}
			return preferenceClassFQN(file, cur, name, s.classes.scope(file).pkg)
		}
	}
	return ""
}

// isLaunch reports whether a call hands the intent to the system: a Context start/send/bind method, an Activity
// launch method, BroadcastReceiver.peekService, or a PendingIntent factory.
func (s *unsafeIntentScan) isLaunch(file *scanner.File, call lintCall) bool {
	switch {
	case unsafeIntentPendingIntentMethods[call.Name]:
		return lintTypeRefMatches(file, call.ReceiverText, pendingIntentClass)
	case call.Name == "peekService":
		return (call.ReceiverNode == 0 || strings.TrimSpace(call.ReceiverText) == "this") &&
			s.enclosingClassExtends(file, call.Idx, receiverBaseClasses)
	case unsafeIntentActivityLaunchMethods[call.Name]:
		if lintTypeRefMatches(file, call.ReceiverText, "androidx.core.app.ActivityCompat") {
			return true
		}
		if call.ReceiverNode == 0 || strings.TrimSpace(call.ReceiverText) == "this" {
			return s.enclosingClassExtends(file, call.Idx, activityBaseClasses)
		}
		return s.valueIsType(file, call.ReceiverNode, call.Idx, activityBaseClasses,
			[]string{"activity", "getActivity()", "requireActivity()"})
	case unsafeIntentContextLaunchMethods[call.Name]:
		if lintTypeRefMatches(file, call.ReceiverText, "androidx.core.content.ContextCompat") {
			return true
		}
		return s.receiverIsContext(file, call)
	}
	return false
}

// resolveProjectCallee finds the project function a call reaches and the name of the parameter that receives
// argument index: a same-file function for an unqualified (or `this`) call, and otherwise (#1623) a top-level
// function of another file that the call's file imports or shares a package with, or, for `Owner.fn(...)`, a member
// of the project class or object Owner.
func (s *unsafeIntentScan) resolveProjectCallee(file *scanner.File, call lintCall, index int) (*scanner.File, uint32, string) {
	recv := strings.TrimSpace(call.ReceiverText)
	if call.ReceiverNode == 0 || recv == "this" {
		var found uint32
		count := 0
		for _, kind := range []string{"function_declaration", "method_declaration"} {
			file.FlatWalkNodes(0, kind, func(fn uint32) {
				if javascriptInterfaceMemberName(file, fn) == call.Name && providerParameterCount(file, fn) == len(call.Args) {
					found = fn
					count++
				}
			})
		}
		if count == 1 {
			return file, found, unsafeIntentParamFor(file, found, call, index)
		}
		if count > 0 || recv == "this" {
			return nil, 0, ""
		}
	}
	calleeFile, callee := s.resolveCrossFileCallee(file, call)
	if callee == 0 {
		return nil, 0, ""
	}
	return calleeFile, callee, unsafeIntentParamFor(calleeFile, callee, call, index)
}

// resolveCrossFileCallee resolves a call to exactly one function declared in another project file:
//   - an unqualified call to a top-level function imported explicitly (possibly under an alias), through a wildcard
//     import, or from the caller's package, or to a member brought in by a member import (`import a.Launcher.launch`,
//     Java's `import static`, or a static wildcard import);
//   - `Owner.fn(...)` to a member (or companion member) of the project class or object Owner, named by an import
//     (possibly aliased or of a nested class), from the caller's package, or through a wildcard import.
//
// This approximates Kotlin's resolution: an unqualified call reaches the top-level function even when an inherited
// member with the same name, declared in another file, would win. That is rare.
func (s *unsafeIntentScan) resolveCrossFileCallee(file *scanner.File, call lintCall) (*scanner.File, uint32) {
	recv := strings.TrimSpace(call.ReceiverText)
	if call.ReceiverNode != 0 && (!lintIdentOnly(recv) || recv == "super" || recv == "this") {
		// super.onNewIntent(intent), this@Outer.fn(...) and a.b.fn(...) do not name a helper by its owner.
		return nil, 0
	}
	functions := s.functionIndex()
	imports := lintImportsOf(file)
	pkg := s.classes.scope(file).pkg
	visible := func(declPkg string) bool {
		if declPkg == pkg {
			return true
		}
		for _, star := range imports.stars {
			if star == declPkg {
				return true
			}
		}
		return false
	}
	var match unsafeIntentFunction
	count := 0
	consider := func(cand unsafeIntentFunction, ok bool) {
		if ok && cand.file != file && cand.params == len(call.Args) {
			match = cand
			count++
		}
	}
	// members considers the functions name declared by the class or object ownerFQN.
	members := func(ownerFQN, name string) {
		for _, cand := range functions[unsafeIntentFunctionKey{owner: lintLastSegment(ownerFQN), name: name}] {
			consider(cand, cand.ownerFQN == ownerFQN)
		}
	}
	switch full, imported := imports.bySimple[call.Name]; {
	case call.ReceiverNode == 0 && imported:
		// `import a.b.forward as relay` makes relay(...) a call to a.b.forward; `import a.Launcher.launch` (or Java's
		// `import static`) makes launch(...) a call to a member of a.Launcher.
		prefix, name := "", full
		if cut := strings.LastIndexByte(full, '.'); cut >= 0 {
			prefix, name = full[:cut], full[cut+1:]
		}
		for _, cand := range functions[unsafeIntentFunctionKey{name: name}] {
			consider(cand, cand.pkg == prefix)
		}
		if prefix != "" {
			members(prefix, name)
		}
	case call.ReceiverNode == 0:
		for _, cand := range functions[unsafeIntentFunctionKey{name: call.Name}] {
			consider(cand, visible(cand.pkg))
		}
		// A wildcard import of a class or object (Java's `import static a.Launcher.*`) brings in its members.
		for _, star := range imports.stars {
			members(star, call.Name)
		}
	default:
		if owner, ok := imports.bySimple[recv]; ok {
			// `import a.Launcher as L` or `import a.Outer.Launcher` names the owner's qualified name.
			members(owner, call.Name)
			break
		}
		if lintSameFileClass(file, recv) != 0 {
			break
		}
		members(implicitJoinPkg(pkg, recv), call.Name)
		for _, star := range imports.stars {
			members(star+"."+recv, call.Name)
		}
	}
	if count != 1 {
		return nil, 0
	}
	return match.file, match.fn
}

// functionIndex returns the project's functions that take an Intent, keyed by owner and name, built on first use.
// Only a function with an Intent parameter can receive the tracked intent.
func (s *unsafeIntentScan) functionIndex() map[unsafeIntentFunctionKey][]unsafeIntentFunction {
	if s.functions != nil {
		return s.functions
	}
	s.functions = map[unsafeIntentFunctionKey][]unsafeIntentFunction{}
	for _, f := range s.ctx.ParsedFiles {
		if f == nil || f.FlatTree == nil || scanner.IsTestFile(f.Path) || !fileFactsCache().Mentions(f, "Intent") {
			continue
		}
		pkg := s.classes.scope(f).pkg
		for _, kind := range []string{"function_declaration", "method_declaration"} {
			f.FlatWalkNodes(0, kind, func(fn uint32) {
				if param, _ := unsafeIntentIntentParameter(f, fn); param == 0 {
					return
				}
				name := javascriptInterfaceMemberName(f, fn)
				owners, named := unsafeIntentFunctionOwners(f, fn)
				if name == "" || !named {
					return
				}
				entry := unsafeIntentFunction{file: f, fn: fn, pkg: pkg, params: providerParameterCount(f, fn)}
				key := unsafeIntentFunctionKey{name: name}
				if len(owners) > 0 {
					key.owner = owners[len(owners)-1]
					entry.ownerFQN = implicitJoinPkg(pkg, strings.Join(owners, "."))
				}
				s.functions[key] = append(s.functions[key], entry)
			})
		}
	}
	return s.functions
}

// unsafeIntentFunctionOwners returns the names of the classes, objects, interfaces, enums and records that enclose fn,
// outermost first, or none for a top-level function. A companion object's members are members of its class, which
// is how `Owner.fn(...)` calls them. It reports false for a function another file cannot call by name: a local
// function (declared inside a function, lambda, initializer or accessor), a member of a local class or of an
// anonymous object, or a member of an unnamed declaration.
func unsafeIntentFunctionOwners(file *scanner.File, fn uint32) ([]string, bool) {
	var owners []string
	for cur, ok := file.FlatParent(fn); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "companion_object":
			if len(owners) == 0 {
				continue
			}
			name := unsafeIntentDeclName(file, cur)
			if name == "" {
				name = "Companion"
			}
			owners = append([]string{name}, owners...)
		case "class_declaration", "object_declaration", "interface_declaration", "enum_declaration",
			"record_declaration", "annotation_type_declaration":
			name := unsafeIntentDeclName(file, cur)
			if name == "" {
				return nil, false
			}
			owners = append([]string{name}, owners...)
		case "function_declaration", "method_declaration", "constructor_declaration", "secondary_constructor",
			"anonymous_initializer", "static_initializer", "lambda_literal", "lambda_expression", "anonymous_function",
			"getter", "setter", "object_literal", "object_creation_expression":
			return nil, false
		}
	}
	return owners, true
}

// lintLastSegment is the part of a qualified name after its last dot.
func lintLastSegment(qualified string) string {
	return qualified[strings.LastIndexByte(qualified, '.')+1:]
}

func implicitJoinPkg(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// unsafeIntentDeclName is the name a class, object or interface declaration declares.
func unsafeIntentDeclName(file *scanner.File, decl uint32) string {
	for _, kind := range []string{"type_identifier", "simple_identifier", "identifier"} {
		if id, ok := file.FlatFindChild(decl, kind); ok {
			return file.FlatNodeText(id)
		}
	}
	return ""
}

// unsafeIntentParamFor returns the name of fn's parameter that receives argument index of call.
func unsafeIntentParamFor(file *scanner.File, fn uint32, call lintCall, index int) string {
	label := ""
	if index < len(call.Labels) {
		label = call.Labels[index]
	}
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		params, ok = file.FlatFindChild(fn, "formal_parameters")
	}
	if !ok {
		return ""
	}
	pos := 0
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "parameter", "formal_parameter":
		default:
			continue
		}
		name := lintDeclaredName(file, p)
		if name == "" {
			if id, ok := file.FlatFindChild(p, "identifier"); ok {
				name = file.FlatNodeText(id)
			}
		}
		if (label != "" && name == label) || (label == "" && pos == unsafeIntentPositional(call, index)) {
			return name
		}
		pos++
	}
	return ""
}

// addRuntimeRegisteredReceivers adds the receiver classes registered at runtime without protection (#1623):
// Context.registerReceiver or ContextCompat.registerReceiver without RECEIVER_NOT_EXPORTED, a protecting broadcast
// permission, or a filter of only protected broadcasts, which leaves the receiver exported to other apps.
func (s *unsafeIntentScan) addRuntimeRegisteredReceivers() {
	for _, file := range s.ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || scanner.IsTestFile(file.Path) ||
			!fileFactsCache().Mentions(file, "registerReceiver") {
			continue
		}
		for _, kind := range []string{"call_expression", "method_invocation"} {
			file.FlatWalkNodes(0, kind, func(n uint32) {
				call, ok := lintCallAt(file, n)
				if !ok || call.Name != "registerReceiver" {
					return
				}
				// reg is the registration seen as Context.registerReceiver, without ContextCompat's context argument.
				reg := call
				if lintTypeRefMatches(file, call.ReceiverText, "androidx.core.content.ContextCompat") {
					if len(call.Args) == 0 {
						return
					}
					reg.Args, reg.Labels = call.Args[1:], call.Labels[1:]
				} else if !s.receiverIsContext(file, call) {
					return
				}
				receiver := reg.Arg(0, "receiver")
				if len(reg.Args) < 2 || receiver == 0 || unsafeIntentRegistrationProtected(file, reg) {
					return
				}
				if fqn := s.receiverClassOf(file, receiver, n); fqn != "" {
					s.exported[fqn] = true
				}
			})
		}
	}
}

// unsafeIntentRegistrationProtected ports Lint's isRuntimeReceiverProtected for a registerReceiver call, seen as
// Context.registerReceiver, reading its arguments by position:
//   - registerReceiver(receiver, filter, broadcastPermission, scheduler[, flags]) is protected by a permission, as
//     unsafeIntentPermissionProtects decides.
//   - registerReceiver(receiver, filter[, broadcastPermission, scheduler], flags) is protected when the flags
//     evaluate, as constants through locals and properties, to a value that includes RECEIVER_NOT_EXPORTED. As in
//     Lint, flags that cannot be evaluated (`RECEIVER_NOT_EXPORTED or extra`) do not protect.
//   - A null filter, or one whose actions all evaluate to protected broadcasts, only receives broadcasts other apps
//     cannot send. A filter that cannot be traced, or has an action that cannot be evaluated, does not protect.
func unsafeIntentRegistrationProtected(file *scanner.File, reg lintCall) bool {
	var permission, flags uint32
	switch len(reg.Args) {
	case 3:
		flags = reg.Arg(2, "flags")
	case 4:
		permission = reg.Arg(2, "broadcastPermission")
	case 5:
		permission, flags = reg.Arg(2, "broadcastPermission"), reg.Arg(4, "flags")
	}
	if unsafeIntentPermissionProtects(file, permission, 0) {
		return true
	}
	if flags != 0 {
		if v, ok := lintEvalInt(file, flags, receiverFlagConsts); ok && v&receiverFlagConsts[contextClass+".RECEIVER_NOT_EXPORTED"] != 0 {
			return true
		}
	}
	// A null argument is a keyword without a named node, so its slot holds 0.
	filter := reg.Arg(1, "filter")
	if filter == 0 || strings.TrimSpace(file.FlatNodeText(filter)) == "null" {
		return true
	}
	// An action Krit cannot evaluate (a parameter, a constant declared in another file) may be any broadcast, so the
	// filter is not known to hold only protected ones.
	protected, _, inspected, unevaluated := receiverFilterActions(file, filter, reg.Idx)
	return protected && inspected && !unevaluated
}

// unsafeIntentPermissionProtects reports whether a broadcastPermission argument protects a runtime receiver. Lint
// evaluates it with ConstantEvaluator, following field initializers, and treats a null or blank result as
// unprotected, as is a known normal permission (KNOWN_NORMAL_PERMISSIONS) any app can hold. So a call such as
// getString(R.string.perm), a parameter, "" or null does not protect. A field or property reference Krit cannot read,
// such as BuildConfig.PERM or a constant declared in another file, is one Lint evaluates through its initializer, so
// it counts as protecting.
func unsafeIntentPermissionProtects(file *scanner.File, arg uint32, depth int) bool {
	if arg == 0 || depth > 4 {
		return false
	}
	arg = lintSkipParensJS(file, arg)
	text := strings.Join(strings.Fields(file.FlatNodeText(arg)), "")
	if text == "null" {
		return false
	}
	if v, ok := lintEvalString(file, arg); ok {
		return strings.TrimSpace(v) != "" && !lintKnownNormalPermissions[v]
	}
	switch file.FlatType(arg) {
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, arg, text, true); init != 0 {
			return unsafeIntentPermissionProtects(file, init, depth+1)
		}
		// A parameter, including a lambda's implicit `it`, has no value to evaluate; any other name is a field or
		// property declared elsewhere.
		return text != "it" && !lintNameIsParameterAt(file, arg, text)
	case "navigation_expression", "field_access":
		cut := strings.LastIndexByte(text, '.')
		if cut <= 0 || !lintIdentOnly(strings.ReplaceAll(text, ".", "")) {
			return false
		}
		if lintTypeRefMatches(file, text[:cut], "android.Manifest.permission") {
			return !lintKnownNormalPermissions["android.permission."+text[cut+1:]]
		}
		if init := lintQualifiedConstantInitializer(file, text[:cut], text[cut+1:]); init != 0 {
			// A constant of a class or object declared in this file (Perms.EMPTY).
			return unsafeIntentPermissionProtects(file, init, depth+1)
		}
		return true
	}
	return false
}

// receiverClassOf returns the project class a registered receiver argument instantiates: a constructor call, or a
// local or property initialized with one or declared with that type.
func (s *unsafeIntentScan) receiverClassOf(file *scanner.File, arg, scope uint32) string {
	arg = lintSkipParensJS(file, arg)
	typ := ""
	if call, ok := lintCallAt(file, arg); ok && call.Name != "" && call.Name[0] >= 'A' && call.Name[0] <= 'Z' {
		typ = call.TypeText
		if typ == "" {
			typ = call.Name
		}
	} else if name := strings.TrimSpace(file.FlatNodeText(arg)); lintIdentOnly(name) {
		typ = strings.TrimSuffix(lintDeclaredTypeText(file, scope, name), "?")
	}
	if typ == "" {
		return ""
	}
	fqn, cls := s.classes.resolve(file, typ)
	if cls == nil || !s.classes.extendsTarget(cls, receiverBaseClasses) {
		return ""
	}
	return fqn
}

// unsafeIntentPositional is the positional index of argument index among the unlabeled arguments.
func unsafeIntentPositional(call lintCall, index int) int {
	pos := 0
	for i := 0; i < index && i < len(call.Labels); i++ {
		if call.Labels[i] == "" {
			pos++
		}
	}
	return pos
}

// inProtectedBroadcastBranch ports Lint's exemption for an onReceive that launches the intent only in a branch
// testing its action against a protected broadcast: `if (intent.action == ACTION)` or a `when`/`switch` case.
func (f *unsafeIntentFlow) inProtectedBroadcastBranch(call uint32) bool {
	file := f.file
	for cur, ok := file.FlatParent(call); ok && cur != f.fn; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "if_expression", "if_statement":
			cond, then := lintIfParts(file, cur)
			if cond == 0 || then == 0 || !piContainsII(file, then, call) {
				continue
			}
			if f.conditionTestsProtectedAction(cond) {
				return true
			}
		case "when_entry", "switch_block_statement_group", "switch_rule":
			if f.caseIsProtectedAction(cur) {
				return true
			}
		}
	}
	return false
}

func piContainsII(file *scanner.File, outer, inner uint32) bool {
	return file.FlatStartByte(outer) <= file.FlatStartByte(inner) && file.FlatEndByte(inner) <= file.FlatEndByte(outer)
}

// lintIfParts returns the condition and the then-branch of a Kotlin or Java if.
func lintIfParts(file *scanner.File, n uint32) (cond, then uint32) {
	var named []uint32
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
		}
		if file.FlatType(child) == "else" {
			break
		}
	}
	if len(named) < 2 {
		return 0, 0
	}
	cond = named[0]
	if file.FlatType(cond) == "parenthesized_expression" || file.FlatType(cond) == "condition" {
		cond = lintSkipParensJS(file, cond)
		if inner := file.FlatNamedChild(cond, 0); file.FlatType(cond) == "condition" && inner != 0 {
			cond = inner
		}
	}
	return cond, named[1]
}

// conditionTestsProtectedAction reports whether a condition compares the intent's action with a protected broadcast
// action, with == or equals.
func (f *unsafeIntentFlow) conditionTestsProtectedAction(cond uint32) bool {
	file := f.file
	cond = lintSkipParensJS(file, cond)
	var left, right uint32
	text := strings.TrimSpace(file.FlatNodeText(cond))
	switch file.FlatType(cond) {
	case "equality_expression", "binary_expression":
		if !strings.Contains(text, "==") || strings.Contains(text, "!=") {
			return false
		}
		left, right = file.FlatNamedChild(cond, 0), file.FlatNamedChild(cond, file.FlatNamedChildCount(cond)-1)
	default:
		call, ok := lintCallAt(file, cond)
		if !ok || call.Name != "equals" || len(call.Args) != 1 || call.ReceiverNode == 0 {
			return false
		}
		left, right = call.ReceiverNode, call.Args[0]
	}
	return (f.isIntentAction(left) && f.isProtectedAction(right)) || (f.isIntentAction(right) && f.isProtectedAction(left))
}

// caseIsProtectedAction reports whether a when entry (or switch case) on the intent's action matches a protected
// broadcast action.
func (f *unsafeIntentFlow) caseIsProtectedAction(entry uint32) bool {
	file := f.file
	owner, ok := file.FlatParent(entry)
	for ok && file.FlatType(owner) != "when_expression" && file.FlatType(owner) != "switch_expression" &&
		file.FlatType(owner) != "switch_statement" {
		owner, ok = file.FlatParent(owner)
	}
	if !ok {
		return false
	}
	var subject uint32
	if s, found := file.FlatFindChild(owner, "when_subject"); found {
		subject = file.FlatNamedChild(s, 0)
	} else if p, found := file.FlatFindChild(owner, "parenthesized_expression"); found {
		subject = lintSkipParensJS(file, p)
	}
	if subject == 0 || !f.isIntentAction(subject) {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(entry))
	if strings.HasPrefix(text, "else") || strings.HasPrefix(text, "default") {
		return false
	}
	first := file.FlatNamedChild(entry, 0)
	if file.FlatType(first) == "when_condition" || file.FlatType(first) == "switch_label" {
		first = file.FlatNamedChild(first, 0)
	}
	return first != 0 && f.isProtectedAction(first)
}

// isIntentAction reports whether expr reads the tracked intent's action: intent.action, intent.getAction(), or a
// local assigned from one.
func (f *unsafeIntentFlow) isIntentAction(expr uint32) bool {
	file := f.file
	expr = lintSkipParensJS(file, expr)
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, expr, file.FlatNodeText(expr), false); init != 0 {
			return f.isIntentAction(init)
		}
		return false
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	for _, suffix := range []string{".action", "?.action", ".getAction()"} {
		if strings.HasSuffix(text, suffix) {
			owner := strings.TrimSuffix(strings.TrimSuffix(text, suffix), "!!")
			return f.names[owner]
		}
	}
	return false
}

// isProtectedAction reports whether expr is a protected broadcast action: a string or a framework constant.
func (f *unsafeIntentFlow) isProtectedAction(expr uint32) bool {
	file := f.file
	if v, ok := lintEvalString(file, expr); ok {
		return lintProtectedBroadcastActions[v]
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(lintSkipParensJS(file, expr))), "")
	text = strings.TrimPrefix(text, "android.content.")
	if action, ok := lintFrameworkActionConstants[text]; ok {
		return lintProtectedBroadcastActions[action]
	}
	return false
}
