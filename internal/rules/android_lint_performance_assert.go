package rules

import (
	"math"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's ArrayInitDetector (UnnecessaryArrayInit) and AssertDetector (ExpensiveAssertion,
// AssertionSideEffect). The walks below follow the detectors' UAST traversals: same node kinds, same depth limits,
// same verdicts. Where Lint resolves a call through the compiler and krit cannot see the target (a function in
// another file, a receiver whose type is not declared in the file), the port abstains.

func registerPerformanceAssertChecks() {
	arrays := &UnnecessaryArrayInitRule{AndroidRule: lintAndroidRule("UnnecessaryArrayInit",
		"Unnecessary array initialization", ALCPerformance, ALSInformational, 6)}
	api.Register(&api.Rule{
		ID: arrays.RuleName, Category: arrays.RuleSetName, Description: arrays.Description(), Sev: api.Severity(arrays.Sev),
		NodeTypes: []string{"call_expression"}, Languages: []scanner.Language{scanner.LangKotlin},
		Confidence: arrays.Confidence(), Fix: api.FixIdiomatic, Implementation: arrays, Check: arrays.check,
	})
	expensive := &ExpensiveAssertionRule{AndroidRule: lintAndroidRule("ExpensiveAssertion", "Expensive Assertions",
		ALCPerformance, ALSWarning, 6)}
	registerPerformanceLintRule(expensive.AndroidRule, expensive, expensive.Confidence(), expensive.check,
		[]string{"call_expression"}, []scanner.Language{scanner.LangKotlin})
	sideEffect := &AssertionSideEffectRule{AndroidRule: lintAndroidRule("AssertionSideEffect",
		"Assertions with Side Effects", ALCPerformance, ALSWarning, 4)}
	registerPerformanceLintRule(sideEffect.AndroidRule, sideEffect, sideEffect.Confidence(), sideEffect.check,
		[]string{"call_expression", "assert_statement"}, []scanner.Language{scanner.LangKotlin, scanner.LangJava})
}

// registerPerformanceLintRule registers a Lint port over the given languages and node types; like every Lint port
// it skips test sources, which Lint does not analyze by default.
func registerPerformanceLintRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context),
	nodeTypes []string, languages []scanner.Language) {
	inner := check
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: languages, Confidence: confidence, Implementation: impl,
		Check: func(ctx *api.Context) {
			if ctx.File != nil && scanner.IsTestFile(ctx.File.Path) {
				return
			}
			inner(ctx)
		},
	})
}

// ---------------------------------------------------------------------------
// UnnecessaryArrayInit
// ---------------------------------------------------------------------------

// UnnecessaryArrayInitRule mirrors Lint's UnnecessaryArrayInit: a Kotlin primitive array constructor whose
// initializer lambda returns a numeric constant equal to zero, the value the array already holds.
type UnnecessaryArrayInitRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports high confidence: the constructor and the constant are read from the source.
func (r *UnnecessaryArrayInitRule) Confidence() float64 { return api.ConfidenceHigh }

// kotlinOptionalInitArrays are the kotlin.* array classes whose constructor takes an optional initializer.
var kotlinOptionalInitArrays = map[string]bool{
	"IntArray": true, "LongArray": true, "ShortArray": true, "ByteArray": true, "FloatArray": true,
	"DoubleArray": true, "CharArray": true, "BooleanArray": true, "UIntArray": true, "ULongArray": true,
	"UShortArray": true, "UByteArray": true,
}

func (r *UnnecessaryArrayInitRule) check(ctx *api.Context) {
	file := ctx.File
	// Lint does not analyze test sources by default.
	if scanner.IsTestFile(file.Path) || perfIsTrailingLambdaInner(file, ctx.Idx) {
		return
	}
	call := perfKotlinCall(file, ctx.Idx)
	if call.kind != perfCall || !kotlinOptionalInitArrays[call.name] || len(call.kids) == 0 {
		return
	}
	if call.recv != nil && call.recv.text(file) != "kotlin" {
		return
	}
	if call.recv == nil && !perfKotlinNameIsBuiltin(file, call.name) {
		return
	}
	_, args := perfKotlinCallParts(file, ctx.Idx)
	lambda := args[len(args)-1]
	if file.FlatType(lambda) == "annotated_lambda" {
		lambda, _ = file.FlatFindChild(lambda, "lambda_literal")
	}
	if lambda == 0 || file.FlatType(lambda) != "lambda_literal" {
		return
	}
	stmts, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return
	}
	body := perfNamedChildren(file, stmts)
	if len(body) != 1 {
		return
	}
	value := perfBuild(file, body[0])
	if value.kind == perfReturn {
		value = value.kids[0]
	}
	n, ok := perfEvalNumber(file, value, 0)
	if !ok || n.float() != 0 {
		return
	}
	text := file.FlatNodeText(lambda)
	start, end := int(file.FlatStartByte(lambda)), int(file.FlatEndByte(lambda))
	f := perfFindingAt(file, start, end, "This initialization lambda (`"+text+"`) is unnecessary and is less efficient")
	// Lint's fix deletes the lambda and the whitespace before it.
	fixStart := start
	for fixStart > 0 && (file.Content[fixStart-1] == ' ' || file.Content[fixStart-1] == '\t') {
		fixStart--
	}
	if fixStart > 0 && file.Content[fixStart-1] == ',' {
		// `IntArray(3, { 0 })`: drop the separating comma too.
		fixStart--
	}
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: fixStart, EndByte: end, Replacement: ""}
	ctx.Emit(f)
}

// perfFindingAt builds a finding of the file spanning the byte range [start,end).
func perfFindingAt(file *scanner.File, start, end int, msg string) scanner.Finding {
	line, col := 1, 1
	for i := 0; i < start && i < len(file.Content); i++ {
		if file.Content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return scanner.Finding{File: file.Path, Line: line, Col: col, StartByte: start, EndByte: end, Message: msg}
}

// perfKotlinNameIsBuiltin reports whether a simple type name refers to the kotlin.* class of that name: no class or
// typealias of the file and no explicit import gives it to another type.
func perfKotlinNameIsBuiltin(file *scanner.File, name string) bool {
	if lintSameFileClass(file, name) != 0 || lintSameFileTypeAlias(file, name) != "" {
		return false
	}
	if imported, ok := lintImportsOf(file).bySimple[name]; ok && imported != "kotlin."+name {
		return false
	}
	return true
}

// perfNumber is a folded numeric constant.
type perfNumber struct {
	isFloat bool
	i       int64
	f       float64
}

func (n perfNumber) float() float64 {
	if n.isFloat {
		return n.f
	}
	return float64(n.i)
}

// perfEvalNumber folds a numeric constant expression the way Lint's evaluate() does for initializer lambdas:
// number literals, unary and binary arithmetic, parentheses, and constants declared in the file. Booleans, chars and
// unsigned literals are not numbers.
func perfEvalNumber(file *scanner.File, e *perfExpr, depth int) (perfNumber, bool) {
	if e == nil || depth > 8 {
		return perfNumber{}, false
	}
	switch e.kind {
	case perfLiteral:
		if e.node == 0 {
			return perfNumber{}, false
		}
		return perfParseNumberLiteral(file.FlatType(e.node), e.text(file))
	case perfParen:
		return perfEvalNumber(file, e.kids[0], depth+1)
	case perfUnary:
		v, ok := perfEvalNumber(file, e.kids[0], depth+1)
		if !ok {
			return v, false
		}
		switch e.op {
		case "-":
			v.i, v.f = -v.i, -v.f
			return v, true
		case "+":
			return v, true
		}
	case perfBinary:
		if len(e.kids) != 2 || e.node != 0 && file.FlatType(e.node) != "additive_expression" &&
			file.FlatType(e.node) != "multiplicative_expression" {
			return perfNumber{}, false
		}
		a, ok1 := perfEvalNumber(file, e.kids[0], depth+1)
		b, ok2 := perfEvalNumber(file, e.kids[1], depth+1)
		if !ok1 || !ok2 {
			return perfNumber{}, false
		}
		return perfArith(e.op, a, b)
	case perfName:
		init := lintConstantInitializer(file, e.node, e.name)
		if init == 0 {
			return perfNumber{}, false
		}
		return perfEvalNumber(file, perfBuild(file, init), depth+1)
	}
	return perfNumber{}, false
}

func perfArith(op string, a, b perfNumber) (perfNumber, bool) {
	if a.isFloat || b.isFloat {
		x, y := a.float(), b.float()
		switch op {
		case "+":
			return perfNumber{isFloat: true, f: x + y}, true
		case "-":
			return perfNumber{isFloat: true, f: x - y}, true
		case "*":
			return perfNumber{isFloat: true, f: x * y}, true
		case "/":
			return perfNumber{isFloat: true, f: x / y}, true
		case "%":
			return perfNumber{isFloat: true, f: math.Mod(x, y)}, true
		}
		return perfNumber{}, false
	}
	switch op {
	case "+":
		return perfNumber{i: a.i + b.i}, true
	case "-":
		return perfNumber{i: a.i - b.i}, true
	case "*":
		return perfNumber{i: a.i * b.i}, true
	case "/":
		if b.i != 0 {
			return perfNumber{i: a.i / b.i}, true
		}
	case "%":
		if b.i != 0 {
			return perfNumber{i: a.i % b.i}, true
		}
	}
	return perfNumber{}, false
}

func perfParseNumberLiteral(kind, text string) (perfNumber, bool) {
	text = strings.ReplaceAll(text, "_", "")
	switch kind {
	case "integer_literal", "long_literal", "hex_literal", "bin_literal":
		v, ok := parseKotlinIntegerLiteral(strings.TrimRight(text, "lL"))
		return perfNumber{i: v}, ok
	case "real_literal":
		t := strings.TrimRight(text, "fFdD")
		v, err := strconv.ParseFloat(t, 64)
		return perfNumber{isFloat: true, f: v}, err == nil
	}
	return perfNumber{}, false
}

// ---------------------------------------------------------------------------
// Assertions
// ---------------------------------------------------------------------------

// perfKotlinAssertCondition returns the condition of a call to kotlin's `assert`, or nil when idx is not one.
// Lint treats the last argument as the condition when the first is a lambda.
func perfKotlinAssertCondition(file *scanner.File, idx uint32) (*perfExpr, *perfExpr) {
	if file.Language != scanner.LangKotlin || perfIsTrailingLambdaInner(file, idx) {
		return nil, nil
	}
	call := perfKotlinCall(file, idx)
	if call.kind != perfCall || call.name != "assert" || len(call.kids) == 0 {
		return nil, nil
	}
	if call.recv != nil && call.recv.text(file) != "kotlin" {
		return nil, nil
	}
	if call.recv == nil {
		// A function of the file visible from the call, or an import of another `assert`, hides kotlin.assert.
		if len(perfEnclosingFunctions(file, idx, "assert", false)) > 0 {
			return nil, nil
		}
		if imported, ok := lintImportsOf(file).bySimple["assert"]; ok && imported != "kotlin.assert" {
			return nil, nil
		}
	}
	// kotlin.assert takes the Boolean condition in parentheses. A call whose first argument is a trailing lambda
	// (`assert { cond }`) is a project's own assert, which may be declared in another file.
	if _, args := perfKotlinCallParts(file, idx); len(args) == 0 || args[0] == 0 {
		return nil, nil
	} else if parent, ok := file.FlatParent(args[0]); !ok || file.FlatType(parent) != "value_argument" {
		return nil, nil
	}
	cond := call.kids[0]
	for cond != nil && cond.kind == perfParen {
		cond = cond.kids[0]
	}
	if cond != nil && cond.kind == perfLambda {
		cond = call.kids[len(call.kids)-1]
	} else {
		cond = call.kids[0]
	}
	return call, cond
}

// ExpensiveAssertionRule mirrors Lint's ExpensiveAssertion: a Kotlin `assert` whose condition does real work, which
// Kotlin evaluates even when assertions are disabled.
type ExpensiveAssertionRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: calls are followed only into functions of the same file.
func (r *ExpensiveAssertionRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ExpensiveAssertionRule) check(ctx *api.Context) {
	file := ctx.File
	call, cond := perfKotlinAssertCondition(file, ctx.Idx)
	if cond == nil {
		return
	}
	if expensive, known := perfIsExpensive(file, cond, 0); !expensive || !known {
		return
	}
	if perfWithinAssertionStatusCheck(file, call.node) {
		return
	}
	msg := "Kotlin assertion arguments are always evaluated, even when assertions are off"
	if perfInsideClass(file, call.node) {
		msg += ". Consider surrounding assertion with `if (javaClass.desiredAssertionStatus()) { assert(...) }`"
	}
	ctx.Emit(perfFindingAt(file, cond.start, cond.end, msg))
}

// perfCheapCallNames are library calls Lint treats as cheap.
var perfCheapCallNames = map[string]bool{"isEmpty": true, "isNotEmpty": true, "contains": true}

// perfKotlinStdlibFunctions are kotlin stdlib top-level functions; an unqualified call to one of these names that the
// file does not declare is a library call, which Lint counts as expensive.
var perfKotlinStdlibFunctions = map[string]bool{
	"listOf": true, "listOfNotNull": true, "mutableListOf": true, "arrayListOf": true, "setOf": true,
	"mutableSetOf": true, "hashSetOf": true, "linkedSetOf": true, "mapOf": true, "mutableMapOf": true,
	"hashMapOf": true, "linkedMapOf": true, "emptyList": true, "emptySet": true, "emptyMap": true, "arrayOf": true,
	"intArrayOf": true, "longArrayOf": true, "sequenceOf": true, "maxOf": true, "minOf": true, "run": true,
	"with": true, "require": true, "requireNotNull": true, "check": true, "checkNotNull": true, "lazy": true,
	"repeat": true, "buildList": true, "buildSet": true, "buildMap": true, "buildString": true, "runCatching": true,
	"synchronized": true, "println": true, "print": true, "error": true, "enumValues": true, "enumValueOf": true,
}

// perfIsExpensive is Lint's isExpensive walk. known is false when the verdict depends on a call krit cannot follow.
func perfIsExpensive(file *scanner.File, e *perfExpr, depth int) (expensive, known bool) {
	if depth == 4 {
		return true, true
	}
	if e == nil {
		return true, true
	}
	switch e.kind {
	case perfLiteral, perfInstance, perfIsCheck, perfName, perfQualified:
		return false, true
	case perfCast:
		return perfIsExpensive(file, e.kids[0], depth+1)
	case perfBinary, perfAssign:
		return perfAnyExpensive(file, e.kids, depth+1)
	case perfParen:
		return perfIsExpensive(file, e.kids[0], depth)
	case perfUnary:
		return perfIsExpensive(file, e.kids[0], depth+1)
	case perfIndex:
		return perfAnyExpensive(file, e.kids, depth+1)
	case perfCall:
		if e.recv != nil {
			// A qualified call is a qualified reference expression in UAST, which Lint treats as cheap.
			return false, true
		}
		if candidates := perfEnclosingFunctions(file, e.node, e.name, false); len(candidates) > 0 {
			fn := perfResolveFunction(file, candidates, len(e.kids))
			if fn == 0 {
				return false, false
			}
			body := perfFunctionBody(file, fn)
			if body == nil {
				return true, true
			}
			if len(body.kids) == 1 && body.kids[0] != nil && body.kids[0].kind == perfReturn {
				if ret := body.kids[0].kids[0]; ret != nil {
					return perfIsExpensive(file, ret, depth+1)
				}
			}
			return true, true
		}
		if e.name != "" && e.name[0] >= 'A' && e.name[0] <= 'Z' {
			// A constructor call: Lint finds no single-return body there.
			return true, true
		}
		if perfCheapCallNames[e.name] {
			return false, true
		}
		if perfKotlinStdlibFunctions[e.name] && lintImportsOf(file).bySimple[e.name] == "" {
			return true, true
		}
		return false, false
	}
	return true, true
}

func perfAnyExpensive(file *scanner.File, kids []*perfExpr, depth int) (expensive, known bool) {
	known = true
	for _, k := range kids {
		exp, ok := perfIsExpensive(file, k, depth)
		if exp && ok {
			return true, true
		}
		if !ok {
			known = false
		}
	}
	return false, known
}

// perfWithinAssertionStatusCheck reports whether an enclosing `if` condition, or every condition of an enclosing
// `when` branch, mentions desiredAssertionStatus. An `else` branch has no conditions, so it counts, as in Lint.
func perfWithinAssertionStatusCheck(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "if_expression":
			for c := file.FlatFirstChild(cur); c != 0; c = file.FlatNextSib(c) {
				if file.FlatIsNamed(c) && file.FlatType(c) != "control_structure_body" {
					if strings.Contains(file.FlatNodeText(c), "desiredAssertionStatus") {
						return true
					}
					break
				}
			}
		case "when_entry":
			all := true
			for c := file.FlatFirstChild(cur); c != 0; c = file.FlatNextSib(c) {
				if file.FlatType(c) == "when_condition" && !strings.Contains(file.FlatNodeText(c), "desiredAssertionStatus") {
					all = false
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}

// perfInsideClass reports whether idx sits inside a class, object or object expression rather than a top-level
// function.
func perfInsideClass(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "companion_object", "object_literal":
			return true
		}
	}
	return false
}

// AssertionSideEffectRule mirrors Lint's AssertionSideEffect: an assertion condition (Java `assert` or Kotlin
// `assert(...)`) that assigns, increments, or calls a mutating method.
type AssertionSideEffectRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: Kotlin library calls are reported only with a proven
// receiver type and calls are followed only into functions of the same file.
func (r *AssertionSideEffectRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *AssertionSideEffectRule) check(ctx *api.Context) {
	file := ctx.File
	var cond *perfExpr
	if file.FlatType(ctx.Idx) == "assert_statement" {
		if kids := perfNamedChildren(file, ctx.Idx); len(kids) > 0 {
			cond = perfBuild(file, kids[0])
		}
	} else {
		_, cond = perfKotlinAssertCondition(file, ctx.Idx)
	}
	if cond == nil {
		return
	}
	at, what := perfSideEffect(file, cond, 0, 0)
	if at == nil {
		return
	}
	ctx.Emit(perfFindingAt(file, at.start, at.end, "Assertion condition has a side effect: "+what))
}

// perfMayHaveSideEffects is Lint's name list for library calls that mutate.
func perfMayHaveSideEffects(name string) bool {
	switch name {
	case "add", "remove", "put", "delete", "mkdir", "mkdirs":
		return true
	}
	return len(name) > 3 && strings.HasPrefix(name, "set") && name[3] >= 'A' && name[3] <= 'Z'
}

// perfSideEffect is Lint's getSideEffect walk. It returns the expression to report and the text Lint quotes.
func perfSideEffect(file *scanner.File, e *perfExpr, depth, callDepth int) (*perfExpr, string) {
	if e == nil || depth == 10 {
		return nil, ""
	}
	switch e.kind {
	case perfUnary:
		switch e.op {
		case "++", "--":
			if callDepth > 0 && perfIsLocalName(file, e.kids[0]) {
				return nil, ""
			}
			return e, e.text(file)
		}
	case perfAssign:
		if callDepth == 0 || len(e.kids) == 0 || !perfIsLocalName(file, e.kids[0]) {
			return e, e.text(file)
		}
		return perfFirstSideEffect(file, e.kids, depth+1, callDepth)
	case perfBinary:
		return perfFirstSideEffect(file, e.kids, depth+1, callDepth)
	case perfQualified:
		if len(e.kids) > 0 {
			return perfSideEffect(file, e.kids[0], depth+1, callDepth)
		}
	case perfParen:
		return perfSideEffect(file, e.kids[0], depth+1, callDepth)
	case perfCall:
		if e.java || callDepth > 2 {
			return nil, ""
		}
		return perfCallSideEffect(file, e, depth, callDepth)
	case perfReturn:
		return perfSideEffect(file, e.kids[0], depth+1, callDepth)
	case perfIf:
		return perfFirstSideEffect(file, e.kids, depth+1, callDepth)
	case perfTry:
		return perfFirstSideEffect(file, e.kids, depth+1, callDepth)
	case perfBlock:
		kids := e.kids
		if len(kids) > 20 {
			kids = kids[:20]
		}
		for _, k := range kids {
			if at, what := perfSideEffect(file, k, depth+1, callDepth); at != nil {
				return e, what
			}
		}
	case perfLoop:
		return perfSideEffect(file, e.kids[0], depth+1, callDepth)
	}
	return nil, ""
}

func perfFirstSideEffect(file *scanner.File, kids []*perfExpr, depth, callDepth int) (*perfExpr, string) {
	for _, k := range kids {
		if at, what := perfSideEffect(file, k, depth, callDepth); at != nil {
			return at, what
		}
	}
	return nil, ""
}

// perfCallSideEffect follows a call into a function of the file, or reports a mutating library call whose receiver
// is proven to be a JDK or Android class.
func perfCallSideEffect(file *scanner.File, e *perfExpr, depth, callDepth int) (*perfExpr, string) {
	var candidates []uint32
	switch {
	case e.recv == nil:
		candidates = perfEnclosingFunctions(file, e.node, e.name, false)
	case e.recv.kind == perfInstance && e.recv.text(file) == "this":
		candidates = perfEnclosingFunctions(file, e.node, e.name, true)
	}
	if len(candidates) > 0 {
		fn := perfResolveFunction(file, candidates, len(e.kids))
		if fn == 0 {
			return nil, ""
		}
		if _, what := perfSideEffect(file, perfFunctionBody(file, fn), depth+1, callDepth+1); what != "" {
			return e, what
		}
		return nil, ""
	}
	if e.recv == nil || !perfMayHaveSideEffects(e.name) {
		return nil, ""
	}
	if !perfReceiverIsPlatformClass(file, e) {
		return nil, ""
	}
	return e, perfCallSelectorText(file, e)
}

// perfKotlinJavaTypeAliases are the kotlin.collections and kotlin.text typealiases of JDK classes, which resolve to
// the JDK members.
var perfKotlinJavaTypeAliases = map[string]bool{
	"ArrayList": true, "HashMap": true, "HashSet": true, "LinkedHashMap": true, "LinkedHashSet": true,
	"StringBuilder": true,
}

// perfPlatformPackages are library packages whose members are compiled (not source) and outside kotlin.*.
var perfPlatformPackages = []string{"java.", "javax.", "android.", "androidx.", "dalvik."}

// perfReceiverIsPlatformClass reports whether the receiver of a Kotlin call has a declared or constructed type in a
// JDK or Android package.
func perfReceiverIsPlatformClass(file *scanner.File, e *perfExpr) bool {
	call, ok := lintCallAt(file, e.node)
	if !ok || call.ReceiverNode == 0 {
		return false
	}
	text := lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(lintReceiverTypeText(file, call)), "?"))
	if text == "" {
		return false
	}
	fqn := text
	if !strings.Contains(text, ".") {
		if lintSameFileClass(file, text) != 0 || lintSameFileTypeAlias(file, text) != "" {
			return false
		}
		if imported, ok := lintImportsOf(file).bySimple[text]; ok {
			fqn = imported
		} else if perfKotlinJavaTypeAliases[text] {
			return true
		} else {
			return false
		}
	}
	for _, p := range perfPlatformPackages {
		if strings.HasPrefix(fqn, p) {
			return true
		}
	}
	return false
}
