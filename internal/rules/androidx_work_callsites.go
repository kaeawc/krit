package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of the call-site checks bundled in androidx.work:work-runtime 2.12.0 (androidx.work.lint).

const (
	workConstraintsBuilder   = "androidx.work.Constraints.Builder"
	workPeriodicRequest      = "androidx.work.PeriodicWorkRequest"
	workPeriodicBuilder      = "androidx.work.PeriodicWorkRequest.Builder"
	workPeriodicBuilderFn    = "androidx.work.PeriodicWorkRequestBuilder"
	workManagerClass         = "androidx.work.WorkManager"
	workRxWorkerClass        = "androidx.work.RxWorker"
	workTimeUnitClass        = "java.util.concurrent.TimeUnit"
	workDurationClass        = "java.time.Duration"
	workIdleBatteryMessage   = "Constraints may not be met for some devices"
	workIntervalMessage      = "Interval duration for `PeriodicWorkRequest`s must be at least 15 minutes."
	workPeriodicEnqueueMsg   = "Use `enqueueUniquePeriodicWork()` instead of `enqueue()`"
	workRxSetProgressMessage = "`setProgress` is deprecated. Use `setCompletableProgress` instead."
)

// workCoordinates lists the androidx.work artifacts; their AARs bring work-runtime and its lint.jar.
var workCoordinates = []librarymodel.Coordinate{
	{Group: "androidx.work", Name: "work-runtime"},
	{Group: "androidx.work", Name: "work-runtime-ktx"},
	{Group: "androidx.work", Name: "work-rxjava2"},
	{Group: "androidx.work", Name: "work-rxjava3"},
	{Group: "androidx.work", Name: "work-gcm"},
	{Group: "androidx.work", Name: "work-multiprocess"},
	{Group: "androidx.work", Name: "work-testing"},
}

// workMayBePresent is false only when the project's dependencies prove WorkManager absent: Lint runs these checks
// only when the work-runtime AAR, and with it the lint.jar, is on the classpath.
func workMayBePresent(ctx *api.Context) bool {
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.MayUseAnyDependency(workCoordinates...)
}

// IdleBatteryChargingConstraintsRule mirrors IdleBatteryChargingConstraintsDetector.
type IdleBatteryChargingConstraintsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence is high: both calls must have a receiver proven to be a Constraints.Builder.
func (r *IdleBatteryChargingConstraintsRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *IdleBatteryChargingConstraintsRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "setRequiresDeviceIdle" || call.ReceiverNode == 0 || !workArgIsTrue(file, call) {
		return
	}
	if !workExprIsType(file, call.ReceiverNode, call.Idx, workConstraintsBuilder, 0) || !workMayBePresent(ctx) {
		return
	}
	block := workEnclosingBlock(file, call.ReceiverNode)
	if block == 0 {
		return
	}
	name, named := workRootIdentifier(file, call.ReceiverNode)
	if charging := workFindChargingCall(file, block, name, named); charging != 0 {
		ctx.Emit(lintFindingAtNode(ctx, charging, "", workIdleBatteryMessage))
	}
}

// workArgIsTrue reports whether the call's first argument folds to the constant true.
func workArgIsTrue(file *scanner.File, call lintCall) bool {
	if len(call.Args) == 0 {
		return false
	}
	v, ok := lintEvalBool(file, call.Args[0])
	return ok && v
}

// workEnclosingBlock returns the nearest block around idx, as UAST's getParentOfType<UBlockExpression>: a braced
// Kotlin body (function, lambda, branch) or a Java block. An expression body has none of its own.
func workEnclosingBlock(file *scanner.File, idx uint32) uint32 {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "statements", "block", "constructor_body":
			return cur
		}
	}
	return 0
}

// workRootIdentifier walks a receiver through qualified expressions to its leftmost simple name, as Lint's
// identifierName does. named is false when the walk ends at anything else (a constructor or other bare call,
// `this`), which Lint represents as null.
func workRootIdentifier(file *scanner.File, idx uint32) (string, bool) {
	for depth := 0; idx != 0 && depth < 32; depth++ {
		switch file.FlatType(idx) {
		case "simple_identifier", "identifier":
			return file.FlatNodeText(idx), true
		case "parenthesized_expression":
			idx = flatUnwrapParenExpr(file, idx)
		case "navigation_expression":
			idx = file.FlatFirstChild(idx)
		case "field_access":
			obj, ok := file.FlatFindChild(idx, "identifier")
			first := file.FlatFirstChild(idx)
			if ok && obj == first {
				return file.FlatNodeText(obj), true
			}
			idx = first
		case "call_expression", "method_invocation":
			call, ok := lintCallAt(file, idx)
			if !ok || call.ReceiverNode == 0 {
				return "", false
			}
			idx = call.ReceiverNode
		default:
			return "", false
		}
	}
	return "", false
}

// workFindChargingCall walks block the way Lint's AbstractUastVisitor does when every visitCallExpression returns
// true: a call is checked, then only its receiver is walked; arguments and lambdas passed to a call are skipped. It
// returns the first setRequiresCharging(true) call on a Constraints.Builder with the same root identifier.
func workFindChargingCall(file *scanner.File, block uint32, name string, named bool) uint32 {
	var found uint32
	var walk func(idx uint32)
	walk = func(idx uint32) {
		if found != 0 || idx == 0 {
			return
		}
		switch file.FlatType(idx) {
		case "call_expression", "method_invocation", "object_creation_expression":
			call, ok := lintCallAt(file, idx)
			if ok && call.ReceiverNode != 0 {
				walk(call.ReceiverNode)
			}
			if found != 0 || !ok || call.Name != "setRequiresCharging" || call.ReceiverNode == 0 || !workArgIsTrue(file, call) {
				return
			}
			if other, otherNamed := workRootIdentifier(file, call.ReceiverNode); other != name || otherNamed != named {
				return
			}
			if workExprIsType(file, call.ReceiverNode, call.Idx, workConstraintsBuilder, 0) {
				found = idx
			}
			return
		}
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			walk(child)
		}
	}
	walk(block)
	return found
}

// workBuilderSetters are the Constraints.Builder and WorkRequest.Builder methods that return the builder itself.
func workReturnsBuilder(name string) bool {
	return strings.HasPrefix(name, "set") || strings.HasPrefix(name, "add") || name == "keepResultsForAtLeast"
}

// workExprIsType reports whether the expression is proven to be an instance of fqn: a constructor call, a chain of
// builder setters on one, or a name whose declaration or initializer is.
func workExprIsType(file *scanner.File, expr, scope uint32, fqn string, depth int) bool {
	if expr == 0 || depth > 6 {
		return false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return workExprIsType(file, flatUnwrapParenExpr(file, expr), scope, fqn, depth+1)
	case "call_expression", "method_invocation", "object_creation_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return false
		}
		if call.Name != "" && call.Name[0] >= 'A' && call.Name[0] <= 'Z' {
			return lintTypeRefMatches(file, call.TypeText, fqn)
		}
		if strings.HasSuffix(fqn, ".Builder") && call.ReceiverNode != 0 && workReturnsBuilder(call.Name) {
			return workExprIsType(file, call.ReceiverNode, scope, fqn, depth+1)
		}
		return false
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if typ := lintDeclaredTypeText(file, scope, name); typ != "" {
			return lintTypeRefMatches(file, typ, fqn)
		}
		if init := lintInitializerOf(file, scope, name, false); init != 0 {
			return workExprIsType(file, init, init, fqn, depth+1)
		}
	}
	return false
}

// InvalidPeriodicWorkRequestIntervalRule mirrors InvalidPeriodicWorkRequestIntervalDetector.
type InvalidPeriodicWorkRequestIntervalRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence is high: the constructor, the interval and its unit are all source constants.
func (r *InvalidPeriodicWorkRequestIntervalRule) Confidence() float64 { return api.ConfidenceHigh }

// workMinutesPer converts a TimeUnit constant or Duration factory to minutes as TimeUnit.MINUTES.convert does:
// numerator/denominator, truncated toward zero.
var workTimeUnitMinutes = map[string][2]int64{
	"NANOSECONDS": {1, 60_000_000_000}, "MICROSECONDS": {1, 60_000_000}, "MILLISECONDS": {1, 60_000},
	"SECONDS": {1, 60}, "MINUTES": {1, 1}, "HOURS": {60, 1}, "DAYS": {1440, 1},
}

var workDurationMinutes = map[string][2]int64{
	"ofNanos": {1, 60_000_000_000}, "ofMillis": {1, 60_000}, "ofSeconds": {1, 60}, "ofMinutes": {1, 1},
	"ofHours": {60, 1}, "ofDays": {1440, 1},
}

func (r *InvalidPeriodicWorkRequestIntervalRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "Builder" || len(call.Args) < 2 || !lintTypeRefMatches(file, call.TypeText, workPeriodicBuilder) {
		return
	}
	interval := flatUnwrapParenExpr(file, call.Arg(1, "repeatInterval"))
	if interval == 0 || !workMayBePresent(ctx) {
		return
	}
	minutes, ok := workIntervalMinutes(file, call, interval)
	if ok && minutes < 15 {
		ctx.Emit(lintFindingAtNode(ctx, call.Idx, "", workIntervalMessage))
	}
}

// workIntervalMinutes evaluates the repeat interval the way the detector does: a long constant with a TimeUnit
// constant, or a Duration.ofX(long) call.
func workIntervalMinutes(file *scanner.File, call lintCall, interval uint32) (int64, bool) {
	if inner, ok := lintCallAt(file, interval); ok {
		factor, known := workDurationMinutes[inner.Name]
		if !known || len(inner.Args) == 0 || !workIsDurationFactory(file, inner) {
			return 0, false
		}
		value, ok := workEvalLong(file, inner.Args[0])
		if !ok {
			return 0, false
		}
		return workConvert(value, factor), true
	}
	value, ok := workEvalLong(file, interval)
	if !ok {
		return 0, false
	}
	unit := workTimeUnitName(file, flatUnwrapParenExpr(file, call.Arg(2, "repeatIntervalTimeUnit")))
	factor, known := workTimeUnitMinutes[unit]
	if !known {
		return 0, false
	}
	return workConvert(value, factor), true
}

func workConvert(value int64, factor [2]int64) int64 {
	if factor[0] != 1 {
		return value * factor[0]
	}
	return value / factor[1]
}

// workEvalLong folds a constant the detector accepts as a long. Java needs a long literal or constant: an int
// argument has expression type int, which the detector skips (`as? Long`). Kotlin literals take the parameter's
// Long type.
func workEvalLong(file *scanner.File, idx uint32) (int64, bool) {
	if idx == 0 {
		return 0, false
	}
	if file.Language == scanner.LangJava && !workJavaLongExpr(file, idx) {
		return 0, false
	}
	return lintEvalInt(file, idx, nil)
}

// workJavaLongExpr reports whether a Java constant expression is typed long: a literal with an L suffix, or a name
// declared long.
func workJavaLongExpr(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal":
		text := file.FlatNodeText(idx)
		return strings.HasSuffix(text, "L") || strings.HasSuffix(text, "l")
	case "identifier":
		typ := lintDeclaredTypeText(file, idx, file.FlatNodeText(idx))
		if typ == "" {
			typ = workJavaPrimitiveFieldType(file, file.FlatNodeText(idx))
		}
		return typ == "long" || typ == "Long"
	}
	return false
}

// workJavaPrimitiveFieldType finds the primitive type of a Java field or local called name anywhere in the file, which
// lintDeclaredTypeText does not report because primitives are not type identifiers.
func workJavaPrimitiveFieldType(file *scanner.File, name string) string {
	typ := ""
	for _, kind := range []string{"field_declaration", "local_variable_declaration"} {
		file.FlatWalkNodes(0, kind, func(decl uint32) {
			if typ != "" {
				return
			}
			var prim string
			for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "integral_type", "floating_point_type", "boolean_type":
					prim = strings.TrimSpace(file.FlatNodeText(child))
				case "variable_declarator":
					if id, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeText(id) == name {
						typ = prim
					}
				}
			}
		})
	}
	return typ
}

// workIsDurationFactory reports whether the call is java.time.Duration.ofX: qualified by a Duration that resolves
// to java.time.Duration, or statically imported.
func workIsDurationFactory(file *scanner.File, call lintCall) bool {
	if call.ReceiverNode == 0 {
		imported, ok := lintImportsOf(file).bySimple[call.Name]
		return ok && imported == workDurationClass+"."+call.Name
	}
	return lintTypeRefMatches(file, call.ReceiverText, workDurationClass)
}

// workTimeUnitName returns the constant name of a java.util.concurrent.TimeUnit reference, or "".
func workTimeUnitName(file *scanner.File, idx uint32) string {
	if idx == 0 {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(idx))
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		if lintTypeRefMatches(file, text[:dot], workTimeUnitClass) {
			return text[dot+1:]
		}
		return ""
	}
	if imported, ok := lintImportsOf(file).bySimple[text]; ok && imported == workTimeUnitClass+"."+text {
		return text
	}
	return ""
}

// BadPeriodicWorkRequestEnqueueRule mirrors PeriodicEnqueueIssueDetector.
type BadPeriodicWorkRequestEnqueueRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence is high: the receiver must be a WorkManager and an argument a PeriodicWorkRequest, both proven.
func (r *BadPeriodicWorkRequestEnqueueRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *BadPeriodicWorkRequestEnqueueRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "enqueue" || call.ReceiverNode == 0 || len(call.Args) == 0 {
		return
	}
	if !workIsWorkManager(file, call.ReceiverNode, call.Idx, 0) || !workMayBePresent(ctx) {
		return
	}
	for _, arg := range call.Args {
		if workIsPeriodicRequest(file, arg, call.Idx, 0) || workIsPeriodicList(file, arg, call.Idx) {
			ctx.Emit(lintFindingAtNode(ctx, call.Idx, "", workPeriodicEnqueueMsg))
			return
		}
	}
}

// workIsWorkManager reports whether the receiver is a WorkManager: WorkManager.getInstance(...), or a name declared
// or initialized as one.
func workIsWorkManager(file *scanner.File, expr, scope uint32, depth int) bool {
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return workIsWorkManager(file, flatUnwrapParenExpr(file, expr), scope, depth+1)
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		return ok && call.Name == "getInstance" && lintTypeRefMatches(file, call.ReceiverText, workManagerClass)
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if typ := lintDeclaredTypeText(file, scope, name); typ != "" {
			return lintTypeRefMatches(file, typ, workManagerClass)
		}
		if init := lintInitializerOf(file, scope, name, false); init != 0 {
			return workIsWorkManager(file, init, init, depth+1)
		}
	}
	return false
}

// workIsPeriodicRequest reports whether the expression is proven to be a PeriodicWorkRequest: build() on a periodic
// builder chain, or a name declared or initialized as one.
func workIsPeriodicRequest(file *scanner.File, expr, scope uint32, depth int) bool {
	if expr == 0 || depth > 6 {
		return false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return workIsPeriodicRequest(file, flatUnwrapParenExpr(file, expr), scope, depth+1)
	case "call_expression", "method_invocation":
		call, ok := lintCallAt(file, expr)
		return ok && call.Name == "build" && len(call.Args) == 0 && workIsPeriodicBuilder(file, call.ReceiverNode, scope, depth+1)
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if typ := lintDeclaredTypeText(file, scope, name); typ != "" {
			return lintTypeRefMatches(file, typ, workPeriodicRequest)
		}
		if init := lintInitializerOf(file, scope, name, false); init != 0 {
			return workIsPeriodicRequest(file, init, init, depth+1)
		}
	}
	return false
}

// workIsPeriodicBuilder reports whether the expression is a PeriodicWorkRequest.Builder: its constructor, the
// PeriodicWorkRequestBuilder<W>() factory, or a builder method chained on either.
func workIsPeriodicBuilder(file *scanner.File, expr, scope uint32, depth int) bool {
	if expr == 0 || depth > 12 {
		return false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return workIsPeriodicBuilder(file, flatUnwrapParenExpr(file, expr), scope, depth+1)
	case "call_expression", "method_invocation", "object_creation_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return false
		}
		if call.Name == "Builder" || file.FlatType(expr) == "object_creation_expression" {
			return lintTypeRefMatches(file, call.TypeText, workPeriodicBuilder)
		}
		if call.Name == "PeriodicWorkRequestBuilder" && call.ReceiverNode == 0 {
			imported, ok := lintImportsOf(file).bySimple[call.Name]
			return ok && imported == workPeriodicBuilderFn
		}
		return call.ReceiverNode != 0 && call.Name != "build" && workIsPeriodicBuilder(file, call.ReceiverNode, scope, depth+1)
	case "simple_identifier", "identifier":
		if init := lintInitializerOf(file, scope, file.FlatNodeText(expr), false); init != 0 {
			return workIsPeriodicBuilder(file, init, init, depth+1)
		}
	}
	return false
}

// workIsPeriodicList reports whether a Kotlin argument has the type List<out PeriodicWorkRequest>: listOf(...) of
// periodic requests, or a name declared List<PeriodicWorkRequest>. Lint also matches List<out WorkRequest>, which
// includes lists of one-time requests; krit does not report those.
func workIsPeriodicList(file *scanner.File, expr, scope uint32) bool {
	if file.Language != scanner.LangKotlin || expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "call_expression":
		call, ok := lintCallAt(file, expr)
		if !ok || call.Name != "listOf" || call.ReceiverNode != 0 || len(call.Args) == 0 || flatCallTrailingLambda(file, expr) != 0 {
			return false
		}
		if _, shadowed := lintImportsOf(file).bySimple["listOf"]; shadowed || workSameFileFunction(file, "listOf") {
			return false
		}
		if typeArg := workCallTypeArgument(file, expr); typeArg != "" {
			return lintTypeRefMatches(file, typeArg, workPeriodicRequest)
		}
		for _, arg := range call.Args {
			if !workIsPeriodicRequest(file, arg, scope, 0) {
				return false
			}
		}
		return true
	case "simple_identifier":
		decl := workDeclaredFullTypeText(file, scope, file.FlatNodeText(expr))
		elem, ok := strings.CutPrefix(strings.ReplaceAll(decl, " ", ""), "List<")
		elem = strings.TrimPrefix(strings.TrimSuffix(elem, ">"), "out")
		return ok && lintTypeRefMatches(file, elem, workPeriodicRequest)
	}
	return false
}

// workCallTypeArgument returns the single explicit type argument of a Kotlin call such as listOf<T>(...), or "".
func workCallTypeArgument(file *scanner.File, call uint32) string {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return ""
	}
	args, ok := file.FlatFindChild(suffix, "type_arguments")
	if !ok {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(args))
	text = strings.TrimSuffix(strings.TrimPrefix(text, "<"), ">")
	if strings.Contains(text, ",") {
		return ""
	}
	return strings.TrimSpace(text)
}

// workSameFileFunction reports whether the file declares a top-level or member function called name.
func workSameFileFunction(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(decl uint32) {
		if id, ok := file.FlatFindChild(decl, "simple_identifier"); ok && file.FlatNodeText(id) == name {
			found = true
		}
	})
	return found
}

// workDeclaredFullTypeText returns the declared Kotlin type of a parameter or property called name, keeping type
// arguments, or "".
func workDeclaredFullTypeText(file *scanner.File, scope uint32, name string) string {
	if lintDeclaredTypeText(file, scope, name) == "" {
		return ""
	}
	typ := ""
	check := func(node uint32) {
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "user_type" {
				typ = strings.Join(strings.Fields(file.FlatNodeText(child)), " ")
				return
			}
		}
	}
	file.FlatWalkNodes(0, "parameter", func(decl uint32) {
		if typ == "" && lintDeclaredName(file, decl) == name {
			check(decl)
		}
	})
	file.FlatWalkNodes(0, "property_declaration", func(decl uint32) {
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok && typ == "" && lintDeclaredName(file, vd) == name {
			check(vd)
		}
	})
	return typ
}

// UseRxSetProgress2Rule mirrors RxWorkerSetProgressDetector.
type UseRxSetProgress2Rule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence is high: the call must resolve to a member of androidx.work.RxWorker.
func (r *UseRxSetProgress2Rule) Confidence() float64 { return api.ConfidenceHigh }

func (r *UseRxSetProgress2Rule) check(ctx *api.Context) {
	file := ctx.File
	// Lint does not analyze test sources unless checkTestSources is on.
	if scanner.IsTestFile(file.Path) {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != "setProgress" || len(call.Args) != 1 {
		return
	}
	if call.ReceiverNode == 0 || lintIsSelfReceiver(call.ReceiverText) {
		// A class of its own setProgress shadows RxWorker's.
		if cls, ok := lintEnclosingClassLike(file, call.Idx); ok && lintClassDeclaresMethod(file, cls, call.Name) {
			return
		}
	}
	if !lintCallIsMemberOf(ctx, call, workRxWorkerClass, nil) || !workMayBePresent(ctx) {
		return
	}
	f := lintFindingAtNode(ctx, call.Idx, "", workRxSetProgressMessage)
	if name := workCallNameNode(file, call.Idx); name != 0 && workResultUnused(file, call.Idx) {
		f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(name)), EndByte: int(file.FlatEndByte(name)),
			Replacement: "setCompletableProgress"}
	}
	ctx.Emit(f)
}

// workCallNameNode returns the method-name identifier of a call.
func workCallNameNode(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "call_expression":
		first := file.FlatFirstChild(idx)
		switch file.FlatType(first) {
		case "simple_identifier":
			return first
		case "navigation_expression":
			suffix := flatLastNamedChild(file, first)
			if id, ok := file.FlatFindChild(suffix, "simple_identifier"); ok {
				return id
			}
		}
	case "method_invocation":
		var name uint32
		for child := file.FlatFirstChild(idx); child != 0 && file.FlatType(child) != "argument_list"; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "identifier" {
				name = child
			}
		}
		return name
	}
	return 0
}

// workResultUnused reports whether the call is a statement of its own, so its Single result is discarded and the
// Completable that setCompletableProgress returns changes nothing.
func workResultUnused(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "statements":
		// The last statement of a lambda is its value.
		if lambda, ok := file.FlatParent(parent); ok && file.FlatType(lambda) == "lambda_literal" && flatLastNamedChild(file, parent) == idx {
			return false
		}
		return true
	case "expression_statement":
		return true
	}
	return false
}
