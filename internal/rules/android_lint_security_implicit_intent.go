package rules

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnsafeImplicitIntentLaunchRule mirrors Lint's UnsafeImplicitIntentLaunch (UnsafeImplicitIntentDetector): an
// implicit intent started as an activity or sent as a broadcast whose action matches an intent filter of a
// non-exported component, or of a receiver registered with RECEIVER_NOT_EXPORTED.
type UnsafeImplicitIntentLaunchRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the action and the launch are followed through one function,
// and actions must be compile-time constants.
func (r *UnsafeImplicitIntentLaunchRule) Confidence() float64 { return api.ConfidenceMedium }

const (
	implicitIntentActivitySuffix  = " (used to start an activity)"
	implicitIntentBroadcastSuffix = " (used to send a broadcast)"
)

func (r *UnsafeImplicitIntentLaunchRule) check(ctx *api.Context) {
	classes := lintSourceClasses(ctx.ParsedFiles)
	nonExported := implicitIntentNonExportedComponents(ctx.Manifests, classes)
	actions := newImplicitIntentActionEval(ctx.ParsedFiles)
	dynamic := map[string]bool{}
	var sent []implicitIntentSend
	for _, file := range ctx.ParsedFiles {
		if file == nil || scanner.IsTestFile(file.Path) {
			continue
		}
		file.FlatWalkAllNodes(0, func(n uint32) {
			switch file.FlatType(n) {
			case "call_expression", "method_invocation", "object_creation_expression":
			default:
				return
			}
			call, ok := lintCallAt(file, n)
			if !ok {
				return
			}
			switch call.Name {
			case "Intent":
				sent = append(sent, implicitIntentSends(file, call, actions)...)
			case "registerReceiver":
				for _, action := range notExportedReceiverActions(file, call, actions) {
					dynamic[action+implicitIntentBroadcastSuffix] = true
				}
			}
		})
	}
	for _, s := range sent {
		if comps := nonExported[s.key]; len(comps) > 0 {
			ctx.Emit(lintFindingAtNodeIn(s.file, s.at, "The intent action `"+s.key+"` matches the intent filter of a "+
				"non-exported component `"+comps[0]+"` from a manifest. If you are trying to invoke this specific "+
				"component via the action then you should make the intent explicit by calling "+
				"`Intent.set{Component,Class,ClassName}`."))
		} else if dynamic[s.key] {
			ctx.Emit(lintFindingAtNodeIn(s.file, s.at, "The intent action `"+s.key+"` matches the intent filter of a "+
				"non-exported receiver, registered via a call to `Context.registerReceiver`, or similar. If you are "+
				"trying to invoke this specific receiver via the action then you should use "+
				"`Intent.setPackage(<APPLICATION_ID>)`."))
		}
	}
}

// lintFindingAtNodeIn builds a finding at the start of a node of a given source file.
func lintFindingAtNodeIn(file *scanner.File, idx uint32, msg string) scanner.Finding {
	return scanner.Finding{File: file.Path, Line: file.FlatRow(idx) + 1, Col: file.FlatCol(idx) + 1,
		StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Message: msg}
}

// implicitIntentNonExportedComponents maps "<action> (used to ...)" to the sorted names of the non-exported
// activities, activity aliases and receivers whose intent filters declare the action. A component without
// android:exported="true" counts as non-exported, as in Lint.
func implicitIntentNonExportedComponents(manifests []*manifest.Manifest, classes *lintSourceIndex) map[string][]string {
	sets := map[string]map[string]bool{}
	for _, m := range manifests {
		if m == nil || m.Root == nil || scanner.IsTestFile(m.Path) {
			continue
		}
		for _, app := range m.Root.ChildrenByTag("application") {
			for _, comp := range app.Children {
				suffix := ""
				switch comp.Tag {
				case "activity", "activity-alias":
					suffix = implicitIntentActivitySuffix
				case "receiver":
					suffix = implicitIntentBroadcastSuffix
				default:
					continue
				}
				if comp.AttrValue("android:exported") == "true" {
					continue
				}
				name := comp.AttrValue("android:name")
				fqn := manifestClassFQN(m.Package, name)
				if m.Package == "" {
					if resolved, cls := lintManifestClass(m, name, classes); cls != nil {
						fqn = resolved
					}
				}
				for _, filter := range comp.ChildrenByTag("intent-filter") {
					for _, action := range filter.ChildrenByTag("action") {
						key := action.AttrValue("android:name") + suffix
						if sets[key] == nil {
							sets[key] = map[string]bool{}
						}
						sets[key][fqn] = true
					}
				}
			}
		}
	}
	out := make(map[string][]string, len(sets))
	for key, set := range sets {
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		// Lint keeps the components in a TreeSet and reports the first.
		sort.Strings(names)
		out[key] = names
	}
	return out
}

// implicitIntentSend is one place an action was given to an implicit intent that is then launched.
type implicitIntentSend struct {
	file *scanner.File
	at   uint32
	key  string
}

// implicitIntentSends follows one Intent constructor through its function, as Lint's visitConstructor does: the
// actions set on it, whether it becomes explicit, and whether it is passed to startActivity or sendBroadcast.
func implicitIntentSends(file *scanner.File, ctor lintCall, actions *implicitIntentActionEval) []implicitIntentSend {
	switch file.FlatType(ctor.Idx) {
	case "object_creation_expression":
		if _, anon := file.FlatFindChild(ctor.Idx, "class_body"); anon {
			return nil
		}
	case "call_expression":
		if ctor.ReceiverNode != 0 && ctor.TypeText != intentClass {
			return nil
		}
	default:
		return nil
	}
	if !lintTypeRefMatches(file, ctor.TypeText, intentClass) {
		return nil
	}
	// Constructors taking a Class are explicit; Intent(Intent) copies are not followed.
	for _, arg := range ctor.Args {
		if arg != 0 && implicitIntentClassArgument(file, arg) {
			return nil
		}
	}
	fn := filenameFlowScope(file, ctor.Idx)
	if fn == 0 {
		return nil
	}
	flow := &implicitIntentFlow{file: file, start: ctor.Idx, tracked: map[string]bool{}, implicit: true,
		actions: map[string][]uint32{}, eval: actions}
	if len(ctor.Args) > 0 {
		if action, ok := actions.eval(file, ctor.Args[0]); ok {
			flow.addAction(action, ctor.Idx)
		} else if len(ctor.Args) == 1 {
			return nil
		}
	}
	flow.visit(fn)
	if !flow.implicit || (!flow.activity && !flow.broadcast) {
		return nil
	}
	var out []implicitIntentSend
	for _, action := range flow.order {
		for _, at := range flow.actions[action] {
			if flow.activity {
				out = append(out, implicitIntentSend{file: file, at: at, key: action + implicitIntentActivitySuffix})
			}
			if flow.broadcast {
				out = append(out, implicitIntentSend{file: file, at: at, key: action + implicitIntentBroadcastSuffix})
			}
		}
	}
	return out
}

// implicitIntentFlow is the data flow Lint's UnsafeImplicitIntentDetector runs over the constructor's method.
type implicitIntentFlow struct {
	file                *scanner.File
	start               uint32
	tracked             map[string]bool
	trackedNodes        []uint32
	implicit            bool
	activity, broadcast bool
	actions             map[string][]uint32
	order               []string
	eval                *implicitIntentActionEval
}

func (f *implicitIntentFlow) addAction(action string, at uint32) {
	if _, seen := f.actions[action]; !seen {
		f.order = append(f.order, action)
	}
	f.actions[action] = append(f.actions[action], at)
}

func (f *implicitIntentFlow) isTracked(n uint32) bool {
	n = lintSkipParensJS(f.file, n)
	if n == f.start {
		return true
	}
	for _, t := range f.trackedNodes {
		if t == n {
			return true
		}
	}
	switch f.file.FlatType(n) {
	case "simple_identifier", "identifier":
		return f.tracked[f.file.FlatNodeText(n)]
	}
	return false
}

// visit walks the function in source order, children first, and stops once the intent is explicit.
func (f *implicitIntentFlow) visit(n uint32) {
	file := f.file
	for child := file.FlatFirstChild(n); child != 0 && f.implicit; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration", "local_class_declaration":
			continue
		}
		f.visit(child)
	}
	if !f.implicit {
		return
	}
	switch file.FlatType(n) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok {
			f.tracked[lintDeclaredName(file, vd)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "variable_declarator":
		if id, ok := file.FlatFindChild(n, "identifier"); ok {
			f.tracked[file.FlatNodeText(id)] = f.isTracked(lintInitializerAfterEquals(file, n))
		}
	case "assignment", "assignment_expression":
		f.assign(n)
	case "call_expression", "method_invocation":
		f.call(n)
	}
}

func (f *implicitIntentFlow) assign(n uint32) {
	file := f.file
	lhs := file.FlatNamedChild(n, 0)
	rhs := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1)
	target := strings.ReplaceAll(strings.Join(strings.Fields(file.FlatNodeText(lhs)), ""), "`", "")
	if lintIdentOnly(target) {
		f.tracked[target] = f.isTracked(rhs)
		return
	}
	// A Kotlin property setter on the intent: `intent.action = ...`.
	owner, prop, ok := strings.Cut(target, ".")
	if !ok || !f.tracked[owner] || strings.Contains(prop, ".") {
		return
	}
	switch prop {
	case "action":
		if action, ok := f.eval.eval(file, rhs); ok {
			f.addAction(action, lhs)
		}
	case "package", "component":
		f.implicit = false
	}
}

// intentBuilderMethods are Intent members that return the same intent, so a chain keeps tracking it.
var intentBuilderMethods = lintStringSet("setAction", "setData", "setDataAndType", "setType", "addCategory",
	"addFlags", "setFlags", "putExtra", "putExtras", "setIdentifier", "setSelector", "setClipData",
	"setDataAndNormalize", "setTypeAndNormalize", "removeExtra", "removeCategory", "removeFlags")

func (f *implicitIntentFlow) call(n uint32) {
	file := f.file
	call, ok := lintCallAt(file, n)
	if !ok {
		return
	}
	if call.ReceiverNode != 0 && f.isTracked(call.ReceiverNode) {
		switch call.Name {
		case "setPackage", "setComponent", "setClass", "setClassName":
			f.implicit = false
			return
		case "setAction":
			if len(call.Args) > 0 {
				if action, ok := f.eval.eval(file, call.Args[0]); ok {
					f.addAction(action, n)
				}
			}
		}
		if intentBuilderMethods[call.Name] {
			f.trackedNodes = append(f.trackedNodes, n)
		}
	}
	for _, arg := range call.Args {
		if arg == 0 || !f.isTracked(arg) {
			continue
		}
		switch call.Name {
		case "startActivity":
			f.activity = true
		case "sendBroadcast", "sendBroadcastAsUser":
			f.broadcast = true
		}
	}
}

// notExportedReceiverActions returns the actions of a registerReceiver call made with RECEIVER_NOT_EXPORTED: the
// actions of its IntentFilter argument, built in the same function.
func notExportedReceiverActions(file *scanner.File, call lintCall, actions *implicitIntentActionEval) []string {
	notExported := false
	var filter uint32
	for _, arg := range call.Args {
		if arg == 0 {
			continue
		}
		if v, ok := lintEvalInt(file, arg, receiverFlagConsts); ok && v&4 != 0 {
			notExported = true
		}
		if filter == 0 && intentFilterExpression(file, arg) {
			filter = arg
		}
	}
	if !notExported || filter == 0 {
		return nil
	}
	return intentFilterActions(file, filter, call.Idx, actions)
}

// intentFilterExpression reports whether the expression is an android.content.IntentFilter: a constructor call or
// a value declared with that type.
func intentFilterExpression(file *scanner.File, expr uint32) bool {
	expr = lintSkipParensJS(file, expr)
	if c, ok := lintCallAt(file, expr); ok && c.Name == "IntentFilter" {
		return lintTypeRefMatches(file, c.TypeText, "android.content.IntentFilter")
	}
	if c, ok := lintCallAt(file, expr); ok && (c.Name == "apply" || c.Name == "also") && c.ReceiverNode != 0 {
		return intentFilterExpression(file, c.ReceiverNode)
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		typ := strings.TrimSuffix(lintDeclaredTypeText(file, expr, file.FlatNodeText(expr)), "?")
		return typ != "" && lintTypeRefMatches(file, typ, "android.content.IntentFilter")
	}
	return false
}

// intentFilterActions collects the constant actions of an IntentFilter expression: its constructor argument, addAction
// calls in an apply block, and, for a local variable, its initializer and the addAction calls made on it before use.
func intentFilterActions(file *scanner.File, expr, use uint32, actions *implicitIntentActionEval) []string {
	expr = lintSkipParensJS(file, expr)
	var out []string
	addFrom := func(n uint32) {
		file.FlatWalkAllNodes(n, func(c uint32) {
			call, ok := lintCallAt(file, c)
			if !ok {
				return
			}
			if (call.Name == "IntentFilter" || call.Name == "addAction") && len(call.Args) > 0 {
				if action, ok := actions.eval(file, call.Args[0]); ok {
					out = append(out, action)
				}
			}
		})
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		fn := filenameFlowScope(file, use)
		if fn == 0 {
			return nil
		}
		if init := lintInitializerOf(file, use, name, true); init != 0 {
			addFrom(init)
		}
		file.FlatWalkAllNodes(fn, func(c uint32) {
			if file.FlatStartByte(c) >= file.FlatStartByte(use) {
				return
			}
			call, ok := lintCallAt(file, c)
			if ok && call.Name == "addAction" && strings.TrimSpace(call.ReceiverText) == name && len(call.Args) > 0 {
				if action, ok := actions.eval(file, call.Args[0]); ok {
					out = append(out, action)
				}
			}
		})
	default:
		addFrom(expr)
	}
	return out
}

// implicitIntentClassArgument reports whether the expression is a class literal: Foo::class.java, Foo.class.
func implicitIntentClassArgument(file *scanner.File, expr uint32) bool {
	expr = lintSkipParensJS(file, expr)
	text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
	return strings.HasSuffix(text, "::class.java") || file.FlatType(expr) == "class_literal"
}
