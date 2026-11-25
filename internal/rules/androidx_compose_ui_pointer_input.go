package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of MultipleAwaitPointerEventScopesDetector and ReturnFromAwaitPointerEventScopeDetector from
// androidx.compose.ui:ui-android 1.12.1.

const acuiPointerPackage = "androidx.compose.ui.input.pointer"

// MultipleAwaitPointerEventScopesRule ports MultipleAwaitPointerEventScopes: several awaitPointerEventScope blocks
// in one pointerInput (or one declaration) can drop events between them.
type MultipleAwaitPointerEventScopesRule struct {
	FlatDispatchBase
	BaseRule
}

// ReturnFromAwaitPointerEventScopeRule ports ReturnFromAwaitPointerEventScope: using the result of
// awaitPointerEventScope outside it can drop the events queued after it returns.
type ReturnFromAwaitPointerEventScopeRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MultipleAwaitPointerEventScopesRule) fileMentions() []string {
	return []string{acuiPointerPackage}
}
func (r *ReturnFromAwaitPointerEventScopeRule) fileMentions() []string {
	return []string{acuiPointerPackage}
}

func (r *MultipleAwaitPointerEventScopesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "MultipleAwaitPointerEventScopes", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A pointerInput call bounds the count, as Lint's Modifier-typed expression does; other Modifier-typed " +
				"expressions between the call and its declaration are not recognized.",
		}}
}

func (r *ReturnFromAwaitPointerEventScopeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ReturnFromAwaitPointerEventScope", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A lambda counts as a PointerInputScope handler only through a declared suspend PointerInputScope " +
				"receiver type on its variable or parameter, or an enclosing pointerInput/coroutineScope call.",
		}}
}

func registerAndroidxComposeUIPointerInputRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	multiple := &MultipleAwaitPointerEventScopesRule{BaseRule: BaseRule{RuleName: "MultipleAwaitPointerEventScopes",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects several awaitPointerEventScope blocks in one pointerInput or declaration, which can drop input events."}}
	registerAcuiRule(multiple.BaseRule, multiple, []string{"call_expression"}, kotlin, api.FixNone, multiple.check)

	ret := &ReturnFromAwaitPointerEventScopeRule{BaseRule: BaseRule{RuleName: "ReturnFromAwaitPointerEventScope",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects results of awaitPointerEventScope that are returned or stored, which can drop input events."}}
	registerAcuiRule(ret.BaseRule, ret, []string{"call_expression"}, kotlin, api.FixNone, ret.check)
}

// acuiAwaitPointerEventScopeCall reports whether call is a PointerInputScope.awaitPointerEventScope call: the file
// imports from androidx.compose.ui.input.pointer (the member needs no import of its own) and declares no function
// of that name. It returns the callee name node and whether the call has an explicit receiver.
func acuiAwaitPointerEventScopeCall(file *scanner.File, call uint32) (name uint32, qualified, ok bool) {
	c, found := lintCallAt(file, call)
	if !found || c.Name != "awaitPointerEventScope" || len(acrFunctionDecls(file, c.Name)) > 0 ||
		!acuiImportsPackage(file, acuiPointerPackage) {
		return 0, false, false
	}
	name = lintCallNameNode(file, call)
	return name, c.ReceiverNode != 0, name != 0
}

// acuiImportsPackage reports whether the file imports anything from pkg or lives in it.
func acuiImportsPackage(file *scanner.File, pkg string) bool {
	if filePackageName(file) == pkg {
		return true
	}
	facts := fileFactsCache().Imports(file)
	if facts == nil {
		return false
	}
	if _, ok := facts.Wildcards[pkg+".*"]; ok {
		return true
	}
	for fqn := range facts.FQNs {
		if strings.HasPrefix(fqn, pkg+".") && !strings.Contains(fqn[len(pkg)+1:], ".") {
			return true
		}
	}
	return false
}

// check mirrors MultipleAwaitPointerEventScopesDetector: the nearest enclosing pointerInput call (Lint's first
// Modifier-typed ancestor expression) or containing declaration holds more than one awaitPointerEventScope call.
func (r *MultipleAwaitPointerEventScopesRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, _, ok := acuiAwaitPointerEventScopeCall(file, call)
	if !ok {
		return
	}
	scope := acuiAwaitScopeContainer(file, call)
	if scope == 0 {
		return
	}
	count := 0
	for _, root := range acuiAwaitCountRoots(file, scope) {
		file.FlatWalkNodes(root, "call_expression", func(n uint32) {
			if _, _, ok := acuiAwaitPointerEventScopeCall(file, n); ok {
				count++
			}
		})
	}
	if count > 1 {
		ctx.EmitAtNode(name, "Suspicious use of multiple awaitPointerEventScope blocks. Using multiple "+
			"awaitPointerEventScope blocks may cause some input events to be dropped.")
	}
}

// acuiAwaitCountRoots returns the subtrees Lint's count covers. For a pointerInput call that is the call's own
// arguments and trailing lambda: tree-sitter nests the receiver of `a.pointerInput { }` inside the call node, but
// Lint's UCallExpression is only the selector.
func acuiAwaitCountRoots(file *scanner.File, scope uint32) []uint32 {
	if file.FlatType(scope) != "call_expression" {
		return []uint32{scope}
	}
	var roots []uint32
	for call := scope; call != 0; {
		var inner uint32
		for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
			switch {
			case file.FlatType(child) == "call_suffix":
				roots = append(roots, child)
			case file.FlatType(child) == "call_expression" && acrIsInnerCallOfTrailingLambda(file, child):
				inner = child
			}
		}
		call = inner
	}
	return roots
}

// acuiAwaitScopeContainer returns the nearest ancestor that is a pointerInput call or a declaration (function,
// property, local variable, class or object), which is where Lint counts the calls.
func acuiAwaitScopeContainer(file *scanner.File, call uint32) uint32 {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			if name, _, _ := acrCallCallee(file, p); name == "pointerInput" {
				return p
			}
		case "function_declaration", "property_declaration", "class_declaration", "object_declaration",
			"secondary_constructor", "getter", "setter", "source_file":
			return p
		}
	}
	return 0
}

// check mirrors ReturnFromAwaitPointerEventScopeDetector: an unqualified awaitPointerEventScope call (a qualified
// one's UAST parent is the qualified expression) that initializes a local variable, or that is returned (`return`,
// an expression body, or a lambda's last expression) outside a pointerInput/PointerInputEventHandler/coroutineScope
// call or a lambda declared as `suspend PointerInputScope.() -> …`.
func (r *ReturnFromAwaitPointerEventScopeRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	name, qualified, ok := acuiAwaitPointerEventScopeCall(file, call)
	if !ok || qualified {
		return
	}
	child := call
	parent, ok := file.FlatParent(call)
	for ok && file.FlatType(parent) == "parenthesized_expression" {
		child = parent
		parent, ok = file.FlatParent(parent)
	}
	if !ok {
		return
	}
	returned := false
	switch file.FlatType(parent) {
	case "property_declaration":
		if acrIsLocalDeclaration(file, parent) && lintInitializerAfterEquals(file, parent) == child {
			ctx.EmitAtNode(name, acuiReturnFromAwaitMessage)
		}
		return
	case "jump_expression":
		returned = strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return")
	case "function_body":
		first := file.FlatFirstChild(parent)
		returned = first != 0 && file.FlatType(first) == "="
	case "statements":
		if lambda, ok := file.FlatParent(parent); ok && file.FlatType(lambda) == "lambda_literal" {
			returned = flatLastNamedChild(file, parent) == child
		}
	}
	if returned && !acuiValidAwaitReturn(file, call) {
		ctx.EmitAtNode(name, acuiReturnFromAwaitMessage)
	}
}

const acuiReturnFromAwaitMessage = "Returning from awaitPointerEventScope may cause some input events to be dropped"

// acuiValidAwaitReturn mirrors validUseOfAwaitPointerEventScopeWithReturn: an ancestor call named pointerInput,
// PointerInputEventHandler or coroutineScope, or an ancestor variable or parameter whose lambda initializer is
// declared `suspend PointerInputScope.() -> …`.
func acuiValidAwaitReturn(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "call_expression":
			switch name, _, _ := acrCallCallee(file, p); name {
			case "pointerInput", "PointerInputEventHandler", "coroutineScope":
				return true
			}
		case "lambda_literal":
			if acuiLambdaDeclaredAsPointerHandler(file, p) {
				return true
			}
		}
	}
	return false
}

// acuiLambdaDeclaredAsPointerHandler reports whether a lambda literal is the initializer of a property or the default
// of a parameter whose declared type is a suspend function type with a PointerInputScope receiver.
func acuiLambdaDeclaredAsPointerHandler(file *scanner.File, lambda uint32) bool {
	parent, ok := file.FlatParent(lambda)
	if !ok {
		return false
	}
	var typeOwner uint32
	switch file.FlatType(parent) {
	case "property_declaration":
		if lintInitializerAfterEquals(file, parent) != lambda {
			return false
		}
		typeOwner, _ = file.FlatFindChild(parent, "variable_declaration")
	case "function_value_parameters":
		// A parameter's `= default` follows the parameter node.
		eq, ok := file.FlatPrevSibling(lambda)
		if !ok || file.FlatType(eq) != "=" {
			return false
		}
		if typeOwner, ok = file.FlatPrevSibling(eq); !ok || file.FlatType(typeOwner) != "parameter" {
			return false
		}
	default:
		return false
	}
	suspend := false
	for child := file.FlatFirstChild(typeOwner); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_modifiers":
			suspend = strings.Contains(file.FlatNodeText(child), "suspend")
		case "function_type":
			if !suspend {
				return false
			}
			receiver := file.FlatFirstChild(child)
			if receiver == 0 {
				return false
			}
			switch file.FlatType(receiver) {
			case "type_identifier", "user_type":
				text := strings.Join(strings.Fields(file.FlatNodeText(receiver)), "")
				return text == acuiPointerPackage+".PointerInputScope" ||
					acrSimpleNameRefersTo(file, text, acuiPointerPackage, "PointerInputScope")
			}
			return false
		}
	}
	return false
}
