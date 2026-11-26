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

// rxKotlinVariableInitializer returns the initializer of the local or property a Kotlin name refers to: the nearest
// declaration before the use in an enclosing block, else a property of an enclosing class or of the file. A
// parameter, which Lint does not follow, ends the search.
func rxKotlinVariableInitializer(file *scanner.File, use uint32) (uint32, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "statements":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if rxKotlinPropertyNamed(file, sib, name) {
					return rxPropertyInitializer(file, sib)
				}
			}
		case "class_body", "source_file":
			for member := file.FlatFirstChild(parent); member != 0; member = file.FlatNextSib(member) {
				if rxKotlinPropertyNamed(file, member, name) {
					return rxPropertyInitializer(file, member)
				}
			}
		case "function_declaration", "lambda_literal", "anonymous_function", "secondary_constructor":
			if rxDeclaresParameter(file, parent, name) {
				return 0, false
			}
		}
		child = parent
	}
	return 0, false
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

func rxKotlinPropertyNamed(file *scanner.File, decl uint32, name string) bool {
	if file.FlatType(decl) != "property_declaration" {
		return false
	}
	varDecl, ok := file.FlatFindChild(decl, "variable_declaration")
	return ok && extractIdentifierFlat(file, varDecl) == name
}

func rxPropertyInitializer(file *scanner.File, decl uint32) (uint32, bool) {
	init := lintInitializerAfterEquals(file, decl)
	return init, init != 0
}

// rxJavaVariableInitializer returns the initializer of the Java local or field a name refers to.
func rxJavaVariableInitializer(file *scanner.File, use uint32) (uint32, bool) {
	name := file.FlatNodeText(use)
	child := use
	for parent, ok := file.FlatParent(use); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "block", "class_body":
			for sib, found := file.FlatPrevSibling(child); found; sib, found = file.FlatPrevSibling(sib) {
				if init, matched := rxJavaDeclarationInitializer(file, sib, name); matched {
					return init, init != 0
				}
			}
			if file.FlatType(parent) == "class_body" {
				for sib, found := file.FlatNextSibling(child); found; sib, found = file.FlatNextSibling(sib) {
					if init, matched := rxJavaDeclarationInitializer(file, sib, name); matched {
						return init, init != 0
					}
				}
			}
		case "method_declaration", "constructor_declaration", "lambda_expression":
			if lintNameIsParameterAt(file, use, name) {
				return 0, false
			}
		}
		child = parent
	}
	return 0, false
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
	if !ok || rxEmitFactories[name] != ctx.Rule.ID || !rxIsEmitFactory(file, name) {
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

// rxIsEmitFactory reports whether an unqualified rxObservable/rxFlowable resolves to kotlinx-coroutines-rx.
func rxIsEmitFactory(file *scanner.File, name string) bool {
	if rxFileDeclaresFunction(file, name) {
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
				return true
			}
		}
	}
	return false
}

// rxFileDeclaresFunction reports whether the file declares a function called name, which shadows an import.
func rxFileDeclaresFunction(file *scanner.File, name string) bool {
	return filefacts.FileFact(fileFactsCache(), file, slotRxLocalFactories, func() map[string]bool {
		names := map[string]bool{}
		file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
			if n := extractIdentifierFlat(file, fn); rxEmitFactories[n] != "" {
				names[n] = true
			}
		})
		return names
	})[name]
}

const slotRxLocalFactories = "rxLocalFactories"

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

// rxLambdaSends reports whether the lambda calls send or trySend on its ProducerScope: without a receiver or on
// `this`. A nested rxObservable/rxFlowable is checked on its own, so its body does not count.
func rxLambdaSends(file *scanner.File, lambda uint32) bool {
	found := false
	var visit func(n uint32)
	visit = func(n uint32) {
		for child := file.FlatFirstChild(n); child != 0 && !found; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "call_expression" {
				if _, _, factory := rxEmitFactoryCall(file, child); factory {
					continue
				}
				c, ok := lintCallAt(file, child)
				if ok && (c.Name == "send" || c.Name == "trySend") &&
					(c.ReceiverNode == 0 || c.ReceiverText == "this" || strings.HasPrefix(c.ReceiverText, "this@")) {
					found = true
					return
				}
			}
			visit(child)
		}
	}
	visit(lambda)
	return found
}
