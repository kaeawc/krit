package rules

// Ports of slack-lints' Rx checks (slackhq/slack-lints 0.11.1, slack-lint-checks/src/main/java/slack/lint/rx/):
// RxSubscribeOnMainDetector (SubscribeOnMain) and RxObservableEmitDetector (RxObservableDoesNotEmit,
// RxFlowableDoesNotEmit).
//
// RxObservableEmitDetector follows upstream main (c0e92cb74b10) rather than the 0.11.1 release: 0.11.1 only accepts
// a lambda whose ProducerScope is `kotlin.coroutines.ProducerScope`, a class that does not exist, so it never
// reports on kotlinx-coroutines-rx code. Main checks for kotlinx.coroutines.channels.ProducerScope instead.

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	rxSubscribeOnMain          = "SubscribeOnMain"
	rxObservableDoesNotEmit    = "RxObservableDoesNotEmit"
	rxFlowableDoesNotEmit      = "RxFlowableDoesNotEmit"
	rxSubscribeOnMainMessage   = "This will make the code for the initial subscription (above this line) run on the main thread. You probably want `observeOn(AndroidSchedulers.mainThread())`."
	rxDoesNotEmitMessageSuffix = " does not call `send()` or `trySend()`"
)

// rxEmitFactories maps the kotlinx-coroutines-rx factories to the issue they report.
var rxEmitFactories = map[string]string{"rxObservable": rxObservableDoesNotEmit, "rxFlowable": rxFlowableDoesNotEmit}

// rxEmitPackages are the kotlinx-coroutines-rx packages that declare the factories.
var rxEmitPackages = []string{"kotlinx.coroutines.rx3", "kotlinx.coroutines.rx2"}

func rxSubscribeOnMainFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("subscribeOn")) && bytes.Contains(file.Content, []byte("io.reactivex.rxjava3"))
}

func rxEmitFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("kotlinx.coroutines.rx"))
}

// checkSubscribeOnMain reports `subscribeOn(mainThread scheduler)` on an RxJava 3 type.
func checkSubscribeOnMain(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	// The dispatcher's callee filter covers Kotlin calls only; test a Java call's name before reading its arguments.
	if file.FlatType(idx) == "method_invocation" && javaMethodInvocationName(file, idx) != "subscribeOn" {
		return
	}
	if !rxFileUsesOnlyRxJava3(file) {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || call.Name != "subscribeOn" || call.ReceiverNode == 0 || len(call.Args) == 0 {
		return
	}
	if !rxIsMainThreadScheduler(file, call.Args[0]) {
		return
	}
	name := rxCalleeNameNode(file, idx)
	if name == 0 {
		return
	}
	ctx.EmitAtNode(name, rxSubscribeOnMainMessage)
}

// rxFileUsesOnlyRxJava3 is the receiver proof for subscribeOn, which Lint resolves to RxJava 3's Completable,
// Flowable, Maybe, Observable or Single. The source shows that when the file imports from RxJava 3 and from no
// other library that declares subscribeOn (RxJava 1 and 2, Reactor).
func rxFileUsesOnlyRxJava3(file *scanner.File) bool {
	return filefacts.FileFact(fileFactsCache(), file, slotRxJava3Only, func() bool {
		imp := lintImportsOf(file)
		rx3 := false
		check := func(name string) bool {
			switch {
			case strings.HasPrefix(name, "io.reactivex.rxjava3."):
				rx3 = true
			case strings.HasPrefix(name, "io.reactivex."), strings.HasPrefix(name, "rx."), strings.HasPrefix(name, "reactor.core."):
				return false
			}
			return true
		}
		for _, full := range imp.bySimple {
			if !check(full) {
				return false
			}
		}
		for _, star := range imp.stars {
			if !check(star + ".") {
				return false
			}
		}
		return rx3
	})
}

const slotRxJava3Only = "rxJava3Only"

// rxIsMainThreadScheduler mirrors RxSubscribeOnMainDetector's argument check: a call named mainThread or
// immediateMainThread, or a variable whose initializer is one.
func rxIsMainThreadScheduler(file *scanner.File, arg uint32) bool {
	switch file.FlatType(arg) {
	case "call_expression", "method_invocation":
		return rxIsMainThreadCall(file, arg)
	case "simple_identifier":
		init, ok := rxKotlinVariableInitializer(file, arg)
		if !ok {
			return false
		}
		text := strings.TrimSpace(file.FlatNodeText(init))
		return strings.HasSuffix(text, "mainThread()") || strings.HasSuffix(text, "immediateMainThread()")
	case "identifier":
		init, ok := rxJavaVariableInitializer(file, arg)
		// Lint reads only a qualified call (AndroidSchedulers.mainThread()), not a static import.
		if !ok || file.FlatType(init) != "method_invocation" {
			return false
		}
		if c, ok := lintCallAt(file, init); !ok || c.ReceiverNode == 0 {
			return false
		}
		return rxIsMainThreadCall(file, init)
	}
	return false
}

func rxIsMainThreadCall(file *scanner.File, call uint32) bool {
	c, ok := lintCallAt(file, call)
	return ok && (c.Name == "mainThread" || c.Name == "immediateMainThread")
}

// rxKotlinVariableInitializer returns the initializer of the variable a Kotlin name refers to, walking scopes
// outward: a local declared before the use, then a property of an enclosing class or of the file. A binding with no
// initializer (a parameter, a primary-constructor parameter, a for or catch variable, a destructured entry) shadows
// any outer declaration, as it does for Lint, and ends the search with no initializer.
func rxKotlinVariableInitializer(file *scanner.File, use uint32) (uint32, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		if init, bound := rxKotlinScopeBinding(file, parent, child, name); bound {
			return init, init != 0
		}
		child = parent
	}
	return 0, false
}

// rxKotlinScopeBinding reports whether scope binds name for a use under child, and the binding's initializer.
func rxKotlinScopeBinding(file *scanner.File, scope, child uint32, name string) (init uint32, bound bool) {
	switch file.FlatType(scope) {
	case "statements":
		for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
			if init, matched := rxKotlinLocalBinding(file, sib, name); matched {
				return init, true
			}
		}
	case "class_body", "source_file":
		for member := file.FlatFirstChild(scope); member != 0; member = file.FlatNextSib(member) {
			if init, matched := rxKotlinLocalBinding(file, member, name); matched {
				return init, true
			}
		}
	case "for_statement":
		for c := file.FlatFirstChild(scope); c != 0; c = file.FlatNextSib(c) {
			if t := file.FlatType(c); (t == "variable_declaration" || t == "multi_variable_declaration") && rxBindsName(file, c, name) {
				return 0, true
			}
		}
	case "catch_block":
		id, ok := file.FlatFindChild(scope, "simple_identifier")
		return 0, ok && file.FlatNodeText(id) == name
	case "when_expression":
		return rxKotlinWhenSubjectBinding(file, scope, child, name)
	case "lambda_literal":
		params, ok := file.FlatFindChild(scope, "lambda_parameters")
		return 0, ok && rxBindsName(file, params, name)
	case "function_declaration", "anonymous_function", "secondary_constructor":
		return 0, rxDeclaresParameter(file, scope, name)
	case "class_declaration":
		return 0, rxPrimaryConstructorDeclares(file, scope, name)
	}
	return 0, false
}

// rxKotlinWhenSubjectBinding handles `when (val name = init) { ... }`.
func rxKotlinWhenSubjectBinding(file *scanner.File, when, child uint32, name string) (uint32, bool) {
	subject, ok := file.FlatFindChild(when, "when_subject")
	if !ok || subject == child {
		return 0, false
	}
	v, ok := file.FlatFindChild(subject, "variable_declaration")
	if !ok || !rxBindsName(file, v, name) {
		return 0, false
	}
	return lintInitializerAfterEquals(file, subject), true
}

func rxPrimaryConstructorDeclares(file *scanner.File, class uint32, name string) bool {
	ctor, ok := file.FlatFindChild(class, "primary_constructor")
	if !ok {
		return false
	}
	for param := file.FlatFirstChild(ctor); param != 0; param = file.FlatNextSib(param) {
		if file.FlatType(param) == "class_parameter" && extractIdentifierFlat(file, param) == name {
			return true
		}
	}
	return false
}

// rxKotlinLocalBinding reports whether decl declares name: a property (whose initializer it returns) or a
// destructuring declaration entry (which has none).
func rxKotlinLocalBinding(file *scanner.File, decl uint32, name string) (init uint32, matched bool) {
	if file.FlatType(decl) != "property_declaration" {
		return 0, false
	}
	if varDecl, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
		if extractIdentifierFlat(file, varDecl) == name {
			return lintInitializerAfterEquals(file, decl), true
		}
		return 0, false
	}
	if multi, ok := file.FlatFindChild(decl, "multi_variable_declaration"); ok && rxBindsName(file, multi, name) {
		return 0, true
	}
	return 0, false
}

// rxBindsName reports whether a variable_declaration, or one inside a destructuring or parameter list, binds name.
func rxBindsName(file *scanner.File, node uint32, name string) bool {
	found := false
	var visit func(n uint32)
	visit = func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) == "variable_declaration" {
			if id, ok := file.FlatFindChild(n, "simple_identifier"); ok && file.FlatNodeText(id) == name {
				found = true
			}
			return
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			visit(c)
		}
	}
	visit(node)
	return found
}

func rxDeclaresParameter(file *scanner.File, fn uint32, name string) bool {
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters", "lambda_parameters":
			if lintParamsDeclare(file, child, name) {
				return true
			}
		}
	}
	return false
}

// rxJavaVariableInitializer returns the initializer of the Java variable a name refers to, walking scopes outward:
// a local declared before the use (or a try-with-resources resource), then a field. A binding with no initializer (a
// parameter, an enhanced-for or catch variable, a pattern variable) shadows a field, as it does for Lint, and ends
// the search with no initializer.
func rxJavaVariableInitializer(file *scanner.File, use uint32) (uint32, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		if init, bound := rxJavaScopeBinding(file, parent, child, use, name); bound {
			return init, init != 0
		}
		child = parent
	}
	return 0, false
}

// rxJavaScopeBinding reports whether scope binds name for a use under child, and the binding's initializer.
func rxJavaScopeBinding(file *scanner.File, scope, child, use uint32, name string) (init uint32, bound bool) {
	switch file.FlatType(scope) {
	case "block", "class_body":
		return rxJavaMemberBinding(file, scope, child, name)
	case "enhanced_for_statement":
		id, ok := file.FlatFindChild(scope, "identifier")
		return 0, ok && file.FlatNodeText(id) == name
	case "catch_clause":
		param, ok := file.FlatFindChild(scope, "catch_formal_parameter")
		if !ok {
			return 0, false
		}
		id, ok := file.FlatFindChild(param, "identifier")
		return 0, ok && file.FlatNodeText(id) == name
	case "try_with_resources_statement":
		return rxJavaResourceBinding(file, scope, name)
	case "for_statement":
		if decl, ok := file.FlatFindChild(scope, "local_variable_declaration"); ok {
			if value, matched := rxJavaDeclarationInitializer(file, decl, name); matched {
				return value, true
			}
		}
		return 0, rxJavaPatternBinds(file, scope, name)
	case "if_statement", "while_statement", "do_statement", "ternary_expression", "binary_expression":
		return 0, rxJavaPatternBinds(file, scope, name)
	case "method_declaration", "constructor_declaration", "lambda_expression":
		return 0, lintNameIsParameterAt(file, use, name)
	}
	return 0, false
}

// rxJavaMemberBinding finds name among the locals declared before child in a block, or the fields of a class body.
func rxJavaMemberBinding(file *scanner.File, scope, child uint32, name string) (uint32, bool) {
	block := file.FlatType(scope) == "block"
	for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
		if init, matched := rxJavaDeclarationInitializer(file, sib, name); matched {
			return init, true
		}
		// `if (!(o instanceof Scheduler main)) return;` keeps the pattern variable in scope after the if.
		if block && rxJavaPatternBinds(file, sib, name) {
			return 0, true
		}
	}
	if block {
		return 0, false
	}
	for sib, found := file.FlatNextSibling(child); found; sib, found = file.FlatNextSibling(sib) {
		if init, matched := rxJavaDeclarationInitializer(file, sib, name); matched {
			return init, true
		}
	}
	return 0, false
}

func rxJavaResourceBinding(file *scanner.File, try uint32, name string) (uint32, bool) {
	spec, ok := file.FlatFindChild(try, "resource_specification")
	if !ok {
		return 0, false
	}
	for res := file.FlatFirstChild(spec); res != 0; res = file.FlatNextSib(res) {
		if file.FlatType(res) != "resource" {
			continue
		}
		if id, ok := file.FlatFindChild(res, "identifier"); ok && file.FlatNodeText(id) == name {
			return lintInitializerAfterEquals(file, res), true
		}
	}
	return 0, false
}

// rxJavaPatternBinds reports whether an instanceof pattern under node (outside nested lambdas and classes) binds
// name.
func rxJavaPatternBinds(file *scanner.File, node uint32, name string) bool {
	found := false
	var visit func(n uint32)
	visit = func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "lambda_expression", "class_body":
			return
		case "instanceof_expression":
			// The binding follows the type (`o instanceof Scheduler main`); the first child is the tested expression.
			for c := file.FlatNextSib(file.FlatFirstChild(n)); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "identifier" && file.FlatNodeText(c) == name {
					found = true
					return
				}
			}
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			visit(c)
		}
	}
	visit(node)
	return found
}

func rxJavaDeclarationInitializer(file *scanner.File, decl uint32, name string) (uint32, bool) {
	switch file.FlatType(decl) {
	case "local_variable_declaration", "field_declaration":
	default:
		return 0, false
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "variable_declarator" && extractIdentifierFlat(file, child) == name {
			return lintInitializerAfterEquals(file, child), true
		}
	}
	return 0, false
}

// rxCalleeNameNode returns the method-name node of a call, where Lint's call location without the receiver starts.
func rxCalleeNameNode(file *scanner.File, call uint32) uint32 {
	if file.FlatType(call) == "method_invocation" {
		var name uint32
		for child := file.FlatFirstChild(call); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "identifier" {
				name = child
			}
		}
		return name
	}
	callee := file.FlatFirstChild(call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		return callee
	case "navigation_expression":
		var last uint32
		for child := file.FlatFirstChild(callee); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "navigation_suffix" {
				last = child
			}
		}
		if id, ok := file.FlatFindChild(last, "simple_identifier"); ok {
			return id
		}
	}
	return 0
}

// checkRxDoesNotEmit reports an rxObservable/rxFlowable whose ProducerScope lambda never calls send or trySend.
func checkRxDoesNotEmit(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if ctx.Rule == nil {
		return
	}
	name, lambda, ok := rxEmitFactoryCall(file, idx)
	if !ok || rxEmitFactories[name] != ctx.Rule.ID || !rxIsEmitFactory(file, idx, name) {
		return
	}
	if lambda == 0 || rxLambdaSends(file, lambda) {
		return
	}
	ctx.EmitAtNode(idx, name+rxDoesNotEmitMessageSuffix)
}

// rxEmitFactoryCall returns the factory name and last lambda argument of an unqualified rxObservable/rxFlowable
// call. tree-sitter parses `rxObservable<T>(context) { ... }` as a call whose callee is the call with the
// parenthesized arguments, so the outer node is the factory call and the inner one defers to it.
func rxEmitFactoryCall(file *scanner.File, idx uint32) (name string, lambda uint32, ok bool) {
	callee := file.FlatFirstChild(idx)
	if file.FlatType(callee) == "call_expression" {
		inner := file.FlatFirstChild(callee)
		if file.FlatType(inner) != "simple_identifier" || rxEmitFactories[file.FlatNodeText(inner)] == "" {
			return "", 0, false
		}
		return file.FlatNodeText(inner), rxLastLambdaArgument(file, idx), true
	}
	if file.FlatType(callee) != "simple_identifier" || rxEmitFactories[file.FlatNodeText(callee)] == "" {
		return "", 0, false
	}
	if parent, _ := file.FlatParent(idx); file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == idx {
		return "", 0, false
	}
	return file.FlatNodeText(callee), rxLastLambdaArgument(file, idx), true
}

// rxIsEmitFactory reports whether an unqualified rxObservable/rxFlowable resolves to kotlinx-coroutines-rx, with
// Kotlin's priorities: a local function or a member of an enclosing class beats any import, and an explicit import
// beats a top-level function of the file's package, which beats a star import.
func rxIsEmitFactory(file *scanner.File, call uint32, name string) bool {
	if rxScopeDeclaresFunction(file, call, name, false) {
		return false
	}
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[name]; ok {
		for _, pkg := range rxEmitPackages {
			if full == pkg+"."+name {
				return true
			}
		}
		return false
	}
	for _, star := range imp.stars {
		for _, pkg := range rxEmitPackages {
			if star == pkg {
				return !rxTopLevelFunction(file, name)
			}
		}
	}
	return false
}

// rxScopeDeclaresFunction reports whether a function called name is in scope at idx through an enclosing block (a
// local function declared before idx) or, unless localOnly, an enclosing class body.
func rxScopeDeclaresFunction(file *scanner.File, idx uint32, name string, localOnly bool) bool {
	child := idx
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if file.FlatType(sib) == "function_declaration" && extractIdentifierFlat(file, sib) == name {
					return true
				}
			}
		case "class_body":
			if localOnly {
				break
			}
			for member := file.FlatFirstChild(parent); member != 0; member = file.FlatNextSib(member) {
				if file.FlatType(member) == "function_declaration" && extractIdentifierFlat(file, member) == name {
					return true
				}
			}
		}
		child = parent
	}
	return false
}

func rxTopLevelFunction(file *scanner.File, name string) bool {
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "function_declaration" && extractIdentifierFlat(file, child) == name {
			return true
		}
	}
	return false
}

// rxLastLambdaArgument returns the lambda_literal passed as a call's last argument, trailing or in parentheses.
func rxLastLambdaArgument(file *scanner.File, call uint32) uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return 0
	}
	if trailing, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
		lambda, _ := file.FlatFindChild(trailing, "lambda_literal")
		return lambda
	}
	args, ok := file.FlatFindChild(suffix, "value_arguments")
	if !ok {
		return 0
	}
	var last uint32
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			last = flatValueArgumentExpression(file, arg)
		}
	}
	if file.FlatType(last) == "lambda_literal" {
		return last
	}
	return 0
}

// rxLambdaSends reports whether the lambda calls send or trySend on its ProducerScope: on `this`/`this@label`, or
// without a receiver where ProducerScope is the nearest implicit receiver. Inside `x.apply { }`, `x.run { }` or
// `with(x) { }` the nearest receiver is x, and a local `fun send` shadows the member, so those calls do not count. A
// nested rxObservable/rxFlowable is checked on its own, so its body does not count either.
func rxLambdaSends(file *scanner.File, lambda uint32) bool {
	found := false
	var visit func(n uint32, rebound bool)
	visit = func(n uint32, rebound bool) {
		for child := file.FlatFirstChild(n); child != 0 && !found; child = file.FlatNextSib(child) {
			childRebound := rebound
			if file.FlatType(child) == "call_expression" {
				if _, _, factory := rxEmitFactoryCall(file, child); factory {
					continue
				}
				c, ok := lintCallAt(file, child)
				if ok && (c.Name == "send" || c.Name == "trySend") {
					explicit := c.ReceiverText == "this" || strings.HasPrefix(c.ReceiverText, "this@")
					implicit := c.ReceiverNode == 0 && !rebound && !rxScopeDeclaresFunction(file, child, c.Name, true)
					if explicit || implicit {
						found = true
						return
					}
				}
				// `with(x) { }` parses as a call whose callee is `with(x)`, so read the name from the inner call.
				scope, scopeOK := c, ok
				if inner := file.FlatFirstChild(child); file.FlatType(inner) == "call_expression" {
					scope, scopeOK = lintCallAt(file, inner)
				}
				if scopeOK && ((scope.ReceiverNode != 0 && (scope.Name == "apply" || scope.Name == "run")) ||
					(scope.ReceiverNode == 0 && scope.Name == "with")) {
					childRebound = true
				}
			}
			visit(child, childRebound)
		}
	}
	visit(lambda, false)
	return found
}
