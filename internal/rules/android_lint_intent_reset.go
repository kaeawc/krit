package rules

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const intentClass = "android.content.Intent"

// intentSelfReturning lists the Intent methods declared to return an Intent. Lint's data flow follows a tracked
// Intent through a call to any of them.
var intentSelfReturning = map[string]bool{
	"addCategory": true, "addFlags": true, "cloneFilter": true, "getSelector": true,
	"putCharSequenceArrayListExtra": true, "putExtra": true, "putExtras": true, "putIntegerArrayListExtra": true,
	"putParcelableArrayListExtra": true, "putStringArrayListExtra": true, "replaceExtras": true, "setAction": true,
	"setClass": true, "setClassName": true, "setComponent": true, "setData": true, "setDataAndNormalize": true,
	"setDataAndType": true, "setDataAndTypeAndNormalize": true, "setFlags": true, "setIdentifier": true,
	"setPackage": true, "setType": true, "setTypeAndNormalize": true,
}

// intentPropertySetters maps the Kotlin synthetic properties Lint sees as calls to the setter they invoke.
var intentPropertySetters = map[string]string{"data": "setData", "type": "setType", "action": "setAction", "component": "setComponent"}

// IntentResetRule mirrors Lint's IntentReset: `setType` after `setData` (or the reverse) on one Intent clears the
// earlier value. The Intent is followed from its constructor through local variables and builder-style calls.
type IntentResetRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the Intent is followed by name inside one method.
func (r *IntentResetRule) Confidence() float64 { return api.ConfidenceMedium }

// intentEvent is one step of the data flow, ordered by source position.
type intentEvent struct {
	pos  int
	node uint32
	// kind is "call" for setData/setType, "bind" for a declaration or assignment of a local.
	kind string
	// name is the setter for a call, or the local being bound.
	name     string
	nameNode uint32
	// decl is the declaration a bind event writes to.
	decl uint32
	recv uint32
	init uint32
	args []uint32
}

// intentFlow is the state Lint's analyzer keeps while it walks one method.
type intentFlow struct {
	file   *scanner.File
	ctor   uint32
	scope  uint32
	locals map[string]bool
	// tracked holds the declarations of the locals that hold the Intent, so a shadowing local is a different one.
	tracked map[uint32]bool
	// allCalls makes collect report every call, with its arguments, instead of only the type and data setters.
	allCalls bool
}

func (r *IntentResetRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "Intent" || !lintTypeRefMatches(file, call.TypeText, intentClass) {
		return
	}
	scope := intentScope(file, ctx.Idx)
	if scope == 0 || !intentScopeMayReset(file, scope) {
		return
	}
	flow := &intentFlow{file: file, ctor: ctx.Idx, scope: scope, locals: intentLocals(file, scope), tracked: map[uint32]bool{}}
	events := flow.collect(scope)
	inCtor := intentCtorSetsURI(file, call, ctx.Idx)
	var seenData, seenType *intentEvent
	if inCtor {
		seenData = &intentEvent{}
	}
	for i := range events {
		e := &events[i]
		if e.kind == "bind" {
			flow.bind(e)
			continue
		}
		if !flow.isTracked(e.recv, 0) {
			continue
		}
		if e.name == "setData" {
			seenData = e
		} else {
			seenType = e
		}
		if seenData == nil || seenType == nil {
			continue
		}
		if !inCtor && !intentSameBlock(file, seenData.node, seenType.node) {
			continue
		}
		ctx.Emit(lintFindingAtNode(ctx, e.nameNode, "", intentResetMessage(e, seenData, seenType, inCtor)))
		seenData, seenType = nil, nil
	}
}

func intentResetMessage(e, seenData, seenType *intentEvent, inCtor bool) string {
	cleared, other := "data", seenData
	if e.name == "setData" {
		cleared, other = "type", seenType
	}
	after := "setting URI in `Intent` constructor"
	if !inCtor {
		after = "calling `" + other.name + "`"
	}
	return "Calling `" + e.name + "` after " + after + " will clear the " + cleared + ": Call `setDataAndType` instead?"
}

// intentScope returns the method, constructor, initializer or accessor that holds the node, or 0 for a field initializer.
func intentScope(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "method_declaration", "constructor_declaration", "secondary_constructor", "anonymous_initializer", "getter", "setter":
			return cur
		case "function_declaration":
			// A local function belongs to the method around it.
			if outer := intentScope(file, cur); outer != 0 {
				return outer
			}
			return cur
		case "class_body", "source_file", "program", "enum_class_body":
			return 0
		}
	}
	return 0
}

// intentScopeMayReset is a cheap text gate: some type setter has to appear in the scope.
func intentScopeMayReset(file *scanner.File, scope uint32) bool {
	text := file.FlatNodeText(scope)
	return strings.Contains(text, "setType") || strings.Contains(text, ".type")
}

// intentLocals collects the names declared as parameters or local variables under scope. Fields are not followed.
func intentLocals(file *scanner.File, scope uint32) map[string]bool {
	locals := map[string]bool{}
	for _, kind := range []string{"parameter", "formal_parameter", "variable_declaration", "variable_declarator", "catch_formal_parameter"} {
		file.FlatWalkNodes(scope, kind, func(decl uint32) {
			for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
				if t := file.FlatType(c); t == "simple_identifier" || t == "identifier" {
					locals[file.FlatNodeText(c)] = true
					break
				}
			}
		})
	}
	return locals
}

// collect gathers the setter calls and the local bindings under scope in source order.
func (f *intentFlow) collect(scope uint32) []intentEvent {
	var events []intentEvent
	file := f.file
	for _, kind := range []string{"call_expression", "method_invocation", "object_creation_expression"} {
		if kind == "object_creation_expression" && !f.allCalls {
			continue
		}
		file.FlatWalkNodes(scope, kind, func(n uint32) {
			if e, ok := f.callEvent(n); ok {
				events = append(events, e)
			}
		})
	}
	file.FlatWalkNodes(scope, "assignment", func(n uint32) {
		if e, ok := f.kotlinAssignmentEvent(n); ok {
			events = append(events, e)
		}
	})
	file.FlatWalkNodes(scope, "assignment_expression", func(n uint32) {
		if e, ok := f.javaAssignmentEvent(n); ok {
			events = append(events, e)
		}
	})
	file.FlatWalkNodes(scope, "property_declaration", func(n uint32) {
		if vd, ok := file.FlatFindChild(n, "variable_declaration"); ok {
			f.addBind(&events, n, lintDeclaredName(file, vd), lintInitializerAfterEquals(file, n))
		}
	})
	file.FlatWalkNodes(scope, "variable_declarator", func(n uint32) {
		if id, ok := file.FlatFindChild(n, "identifier"); ok {
			f.addBind(&events, n, file.FlatNodeText(id), lintInitializerAfterEquals(file, n))
		}
	})
	sort.SliceStable(events, func(i, j int) bool { return events[i].pos < events[j].pos })
	return events
}

func (f *intentFlow) addBind(events *[]intentEvent, node uint32, name string, init uint32) {
	if name == "" || init == 0 || !f.locals[name] {
		return
	}
	*events = append(*events, intentEvent{pos: int(f.file.FlatStartByte(node)), node: node, kind: "bind", name: name, decl: node, init: init})
}

// callEvent describes a one-argument `setData`/`setType` call. A null literal argument is ignored, as in Lint.
func (f *intentFlow) callEvent(n uint32) (intentEvent, bool) {
	file := f.file
	call, ok := lintCallAt(file, n)
	if ok && f.allCalls {
		pos := int(file.FlatStartByte(n))
		if name := lintCallNameNode(file, n); name != 0 {
			pos = int(file.FlatStartByte(name))
		}
		return intentEvent{pos: pos, node: n, kind: "call", name: call.Name, recv: call.ReceiverNode, args: call.Args}, true
	}
	if !ok || (call.Name != "setData" && call.Name != "setType") || call.ReceiverNode == 0 || lintValueArgumentCount(file, call) != 1 {
		return intentEvent{}, false
	}
	if len(call.Args) == 1 && intentIsNullLiteral(file, call.Args[0]) {
		return intentEvent{}, false
	}
	name := lintCallNameNode(file, n)
	if name == 0 {
		return intentEvent{}, false
	}
	return intentEvent{pos: int(file.FlatStartByte(name)), node: n, kind: "call", name: call.Name, nameNode: name, recv: call.ReceiverNode}, true
}

// kotlinAssignmentEvent maps `i.data = x` and `i.type = x` to setData/setType, and `x = y` to a binding.
func (f *intentFlow) kotlinAssignmentEvent(n uint32) (intentEvent, bool) {
	file := f.file
	lhs, ok := file.FlatFindChild(n, "directly_assignable_expression")
	if !ok {
		return intentEvent{}, false
	}
	// A `null` right-hand side is a keyword token without a node.
	rhs := flatLastNamedChild(file, n)
	if rhs == lhs {
		rhs = 0
	}
	if !intentAssignsWith(file, n, lhs, rhs) {
		return intentEvent{}, false
	}
	if file.FlatNamedChildCount(lhs) == 1 {
		if name := file.FlatNamedChild(lhs, 0); file.FlatType(name) == "simple_identifier" {
			ident := file.FlatNodeText(name)
			if !f.locals[ident] {
				return intentEvent{}, false
			}
			decl := intentDeclOf(file, f.scope, name)
			return intentEvent{pos: int(file.FlatStartByte(n)), node: n, kind: "bind", name: ident, decl: decl, init: rhs}, true
		}
		return intentEvent{}, false
	}
	suffix, ok := file.FlatFindChild(lhs, "navigation_suffix")
	if !ok {
		return intentEvent{}, false
	}
	prop, _ := file.FlatFindChild(suffix, "simple_identifier")
	setter := intentPropertySetters[file.FlatNodeText(prop)]
	if prop == 0 || setter == "" || (!f.allCalls && (setter != "setData" && setter != "setType" || intentIsNullLiteral(file, rhs))) {
		return intentEvent{}, false
	}
	return intentEvent{pos: int(file.FlatStartByte(prop)), node: n, kind: "call", name: setter, nameNode: prop, recv: file.FlatNamedChild(lhs, 0)}, true
}

// intentAssignsWith reports whether the assignment is a plain `=`; rhs is 0 when the value is the literal `null`.
func intentAssignsWith(file *scanner.File, n, lhs, rhs uint32) bool {
	if rhs != 0 {
		return strings.TrimSpace(string(file.Content[file.FlatEndByte(lhs):file.FlatStartByte(rhs)])) == "="
	}
	return strings.Join(strings.Fields(string(file.Content[file.FlatEndByte(lhs):file.FlatEndByte(n)])), " ") == "= null"
}

func (f *intentFlow) javaAssignmentEvent(n uint32) (intentEvent, bool) {
	file := f.file
	if file.FlatNamedChildCount(n) != 2 {
		return intentEvent{}, false
	}
	lhs, rhs := file.FlatNamedChild(n, 0), file.FlatNamedChild(n, 1)
	if file.FlatType(lhs) != "identifier" || strings.TrimSpace(string(file.Content[file.FlatEndByte(lhs):file.FlatStartByte(rhs)])) != "=" {
		return intentEvent{}, false
	}
	name := file.FlatNodeText(lhs)
	if !f.locals[name] {
		return intentEvent{}, false
	}
	decl := intentDeclOf(file, f.scope, lhs)
	return intentEvent{pos: int(file.FlatStartByte(n)), node: n, kind: "bind", name: name, decl: decl, init: rhs}, true
}

// bind starts following a local when its value is the tracked Intent and stops when it gets another value.
func (f *intentFlow) bind(e *intentEvent) {
	if e.decl == 0 {
		return
	}
	if f.isTracked(e.init, 0) {
		f.tracked[e.decl] = true
	} else {
		delete(f.tracked, e.decl)
	}
}

// isTracked reports whether the expression is the Intent built by the constructor under analysis.
func (f *intentFlow) isTracked(idx uint32, depth int) bool {
	file := f.file
	if idx == 0 || depth > 12 {
		return false
	}
	if idx == f.ctor {
		return true
	}
	switch file.FlatType(idx) {
	case "parenthesized_expression", "cast_expression", "as_expression", "postfix_expression":
		return f.isTracked(intentOperand(file, idx), depth+1)
	case "simple_identifier", "identifier":
		decl := intentDeclOf(file, f.scope, idx)
		return decl != 0 && f.tracked[decl]
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, idx)
		return ok && call.ReceiverNode != 0 && intentSelfReturning[call.Name] && f.isTracked(call.ReceiverNode, depth+1)
	case "elvis_expression":
		return f.anyChildTracked(idx, 0, depth)
	case "ternary_expression":
		return f.anyChildTracked(idx, 1, depth)
	case "if_expression":
		return f.anyBranchTracked(idx, depth)
	}
	return false
}

// intentDeclOf returns the declaration an identifier names, the way Lint resolves a reference: the nearest
// parameter, lambda parameter, loop or catch variable, or earlier local of that name between it and scope.
// It returns 0 when none of them declares the name.
func intentDeclOf(file *scanner.File, scope, ident uint32) uint32 {
	name := file.FlatNodeText(ident)
	use := file.FlatStartByte(ident)
	for cur, ok := file.FlatParent(ident); ok; cur, ok = file.FlatParent(cur) {
		if decl := intentDeclIn(file, cur, name, use); decl != 0 {
			return decl
		}
		if cur == scope {
			break
		}
	}
	return 0
}

// intentDeclIn returns the declaration of name that node introduces for code inside it at byte offset use.
func intentDeclIn(file *scanner.File, node uint32, name string, use uint32) uint32 {
	switch file.FlatType(node) {
	case "statements", "block", "constructor_body", "switch_block_statement_group":
		return intentLocalBefore(file, node, name, use)
	case "lambda_literal":
		if params, ok := file.FlatFindChild(node, "lambda_parameters"); ok {
			return intentNamedChild(file, params, "variable_declaration", name)
		}
	case "function_declaration", "secondary_constructor", "setter", "anonymous_function":
		if params, ok := file.FlatFindChild(node, "function_value_parameters"); ok {
			return intentNamedChild(file, params, "parameter", name)
		}
	case "method_declaration", "constructor_declaration":
		if params, ok := file.FlatFindChild(node, "formal_parameters"); ok {
			return intentNamedChild(file, params, "formal_parameter", name)
		}
	case "lambda_expression":
		return intentJavaLambdaParam(file, node, name)
	case "for_statement":
		if v := intentNamedChild(file, node, "variable_declaration", name); v != 0 {
			return v
		}
		if v, ok := file.FlatFindChild(node, "local_variable_declaration"); ok {
			return intentNamedChild(file, v, "variable_declarator", name)
		}
	case "enhanced_for_statement":
		if id, ok := file.FlatFindChild(node, "identifier"); ok && file.FlatNodeTextEquals(id, name) {
			return node
		}
	case "catch_block":
		if id, ok := file.FlatFindChild(node, "simple_identifier"); ok && file.FlatNodeTextEquals(id, name) {
			return node
		}
	case "catch_clause":
		if p, ok := file.FlatFindChild(node, "catch_formal_parameter"); ok && intentIdentifierIs(file, p, name) {
			return p
		}
	}
	return 0
}

// intentLocalBefore returns the last local of name that a block declares before use, as the node its bind event uses.
func intentLocalBefore(file *scanner.File, block uint32, name string, use uint32) uint32 {
	var found uint32
	// A declaration is visible after it ends, so `val i = i` in a nested block reads the outer `i`.
	for c := file.FlatFirstChild(block); c != 0 && file.FlatStartByte(c) < use; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "property_declaration":
			if file.FlatEndByte(c) > use {
				continue
			}
			if intentNamedChild(file, c, "variable_declaration", name) != 0 {
				found = c
			} else if multi, ok := file.FlatFindChild(c, "multi_variable_declaration"); ok &&
				intentNamedChild(file, multi, "variable_declaration", name) != 0 {
				found = c
			}
		case "local_variable_declaration":
			if d := intentNamedChild(file, c, "variable_declarator", name); d != 0 && file.FlatEndByte(d) <= use {
				found = d
			}
		}
	}
	return found
}

// intentNamedChild returns the child of the given type that declares name.
func intentNamedChild(file *scanner.File, parent uint32, kind, name string) uint32 {
	for c := file.FlatFirstChild(parent); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == kind && intentIdentifierIs(file, c, name) {
			return c
		}
	}
	return 0
}

// intentIdentifierIs reports whether the first identifier child of decl is name.
func intentIdentifierIs(file *scanner.File, decl uint32, name string) bool {
	for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "simple_identifier" || t == "identifier" {
			return file.FlatNodeTextEquals(c, name)
		}
	}
	return false
}

// intentJavaLambdaParam handles `i -> ...`, `(i, j) -> ...` and `(Intent i) -> ...`.
func intentJavaLambdaParam(file *scanner.File, lambda uint32, name string) uint32 {
	params := file.FlatNamedChild(lambda, 0)
	switch file.FlatType(params) {
	case "identifier":
		if file.FlatNodeTextEquals(params, name) {
			return params
		}
	case "inferred_parameters":
		for c := file.FlatFirstChild(params); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "identifier" && file.FlatNodeTextEquals(c, name) {
				return c
			}
		}
	case "formal_parameters":
		return intentNamedChild(file, params, "formal_parameter", name)
	}
	return 0
}

// intentOperand returns the wrapped expression of a parenthesized, cast or non-null-asserted expression.
func intentOperand(file *scanner.File, idx uint32) uint32 {
	if file.FlatType(idx) == "cast_expression" {
		return flatLastNamedChild(file, idx)
	}
	return file.FlatNamedChild(idx, 0)
}

func (f *intentFlow) anyChildTracked(idx uint32, first, depth int) bool {
	for i := first; i < f.file.FlatNamedChildCount(idx); i++ {
		if f.isTracked(f.file.FlatNamedChild(idx, i), depth+1) {
			return true
		}
	}
	return false
}

func (f *intentFlow) anyBranchTracked(idx uint32, depth int) bool {
	file := f.file
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "control_structure_body" {
			continue
		}
		last := flatLastCodeNamedChild(file, c)
		if last != 0 && file.FlatType(last) == "statements" {
			last = flatLastCodeNamedChild(file, last)
		}
		if f.isTracked(last, depth+1) {
			return true
		}
	}
	return false
}

// intentIsNullLiteral reports whether the argument is `null`, possibly parenthesized. A cast `(Uri) null` is not.
func intentIsNullLiteral(file *scanner.File, arg uint32) bool {
	if arg == 0 {
		return true
	}
	text := strings.TrimSpace(file.FlatNodeText(arg))
	for strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	return text == "null" && file.FlatType(arg) != "call_expression"
}

// ---- Block structure ----

// intentBlockParent returns the nearest enclosing block, conditional or switch clause, the units Lint compares.
func intentBlockParent(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "statements", "if_expression", "when_entry", "block", "if_statement", "ternary_expression",
			"switch_block_statement_group", "switch_rule", "lambda_expression",
			"function_declaration", "method_declaration", "constructor_declaration":
			return cur
		}
	}
	return 0
}

// intentSameBlock applies Lint's check that both setters sit in one block, and in one branch of an if they share.
func intentSameBlock(file *scanner.File, a, b uint32) bool {
	pa, pb := intentBlockParent(file, a), intentBlockParent(file, b)
	if pa != pb || pa == 0 {
		return false
	}
	switch file.FlatType(pa) {
	case "if_expression", "if_statement", "ternary_expression":
		then := intentThenBranch(file, pa)
		if then != 0 && intentWithin(file, a, then) != intentWithin(file, b, then) {
			return false
		}
	}
	return true
}

func intentThenBranch(file *scanner.File, ifNode uint32) uint32 {
	if file.FlatType(ifNode) == "if_expression" {
		then, _ := file.FlatFindChild(ifNode, "control_structure_body")
		return then
	}
	if file.FlatNamedChildCount(ifNode) > 1 {
		return file.FlatNamedChild(ifNode, 1)
	}
	return 0
}

func intentWithin(file *scanner.File, node, ancestor uint32) bool {
	return file.FlatStartByte(node) >= file.FlatStartByte(ancestor) && file.FlatEndByte(node) <= file.FlatEndByte(ancestor)
}

// ---- Constructor URI ----

// intentCtorSetsURI reports whether a constructor argument is the Uri of Intent(String, Uri) or its longer form.
func intentCtorSetsURI(file *scanner.File, call lintCall, ctor uint32) bool {
	n := len(call.Args)
	if n != 2 && n != 4 {
		return false
	}
	arg := call.Args[1]
	if intentIsNullLiteral(file, arg) || intentIsClassValue(file, ctor, arg) {
		return false
	}
	return n == 4 || !intentLooksLikeContextClassPair(file, ctor, call.Args[0], arg)
}

// intentIsClassValue reports whether the expression is a Class: a class literal or a variable declared as one.
func intentIsClassValue(file *scanner.File, scope, arg uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	switch {
	case strings.HasSuffix(text, ".class"), strings.HasSuffix(text, "::class.java"), strings.HasSuffix(text, "::class"),
		strings.HasSuffix(text, "javaClass"), strings.HasSuffix(text, ".getClass()"), strings.HasPrefix(text, "Class."):
		return true
	}
	if !lintIdentOnly(text) {
		return false
	}
	return strings.HasPrefix(lintDeclaredTypeText(file, scope, text), "Class")
}

// intentLooksLikeContextClassPair guards the two-argument case: Intent(Context, Class) takes no Uri. Only a second
// argument that is not clearly a Uri and a first argument that is clearly a context count as that overload.
func intentLooksLikeContextClassPair(file *scanner.File, scope, first, second uint32) bool {
	if intentArgIsURI(file, scope, second) {
		return false
	}
	if first == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(first))
	if strings.HasPrefix(text, "\"") || strings.Contains(text, "ACTION_") {
		return false
	}
	lower := strings.ToLower(text)
	return lower == "this" || strings.HasPrefix(lower, "this@") || strings.Contains(lower, "context") ||
		strings.Contains(lower, "activity") || lower == "ctx" || lower == "applicationcontext"
}

// intentArgIsURI reports whether the expression is recognizably a Uri.
func intentArgIsURI(file *scanner.File, scope, arg uint32) bool {
	text := strings.TrimSpace(file.FlatNodeText(arg))
	if lintIdentOnly(text) {
		typ := lintDeclaredTypeText(file, scope, text)
		return typ == "Uri" || typ == "android.net.Uri"
	}
	return strings.HasPrefix(text, "Uri.") || strings.HasPrefix(text, "android.net.Uri.") || strings.HasSuffix(text, ".toUri()")
}

// ---- IntentWithNullActionLaunch ----

// intentExplicitSetters make an Intent explicit by component or give it an action, whatever their arguments.
var intentExplicitSetters = map[string]bool{"setAction": true, "setClass": true, "setComponent": true, "setClassName": true}

// intentLaunchers are the calls Lint counts as launching the Intent passed to them.
var intentLaunchers = map[string]bool{"sendBroadcast": true, "sendBroadcastAsUser": true, "startActivity": true}

const intentNullActionMessage = "This intent has no action set and is not explicit by component. You should either make this " +
	"intent explicit by component or set an action matching the targeted intent filter."

// IntentWithNullActionLaunchRule mirrors Lint's IntentWithNullActionLaunch: an Intent built with the no-argument
// constructor, never given an action or component, and launched with startActivity or sendBroadcast. Passing the
// Intent to any other call makes Lint stop judging it.
type IntentWithNullActionLaunchRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the Intent is followed by name inside one method.
func (r *IntentWithNullActionLaunchRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *IntentWithNullActionLaunchRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "Intent" || len(call.Args) != 0 || lintValueArgumentCount(file, call) != 0 ||
		!lintTypeRefMatches(file, call.TypeText, intentClass) {
		return
	}
	scope := intentScope(file, ctx.Idx)
	if scope == 0 || (!fileFactsCache().Mentions(file, "startActivity") && !fileFactsCache().Mentions(file, "sendBroadcast")) {
		return
	}
	flow := &intentFlow{file: file, ctor: ctx.Idx, scope: scope, locals: intentLocals(file, scope), tracked: map[uint32]bool{}, allCalls: true}
	var launched, explicit, escaped bool
	for _, e := range flow.collect(scope) {
		if e.kind == "bind" {
			flow.bind(&e)
			continue
		}
		if e.recv != 0 && intentExplicitSetters[e.name] && flow.isTracked(e.recv, 0) {
			explicit = true
		}
		for _, arg := range e.args {
			if intentIgnoresArgument(file, e) || !flow.isTracked(arg, 0) {
				continue
			}
			if intentLaunchers[e.name] {
				launched = true
			} else {
				escaped = true
			}
		}
	}
	if launched && !explicit && !escaped {
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", intentNullActionMessage))
	}
}

// intentIgnoresArgument mirrors DataFlowAnalyzer.ignoreArgument: printing and logging calls neither launch nor
// release the Intent.
func intentIgnoresArgument(file *scanner.File, e intentEvent) bool {
	switch e.name {
	case "print", "println", "log":
		return true
	}
	return len(e.name) == 1 && e.recv != 0 && strings.TrimSpace(file.FlatNodeText(e.recv)) == "Log"
}
