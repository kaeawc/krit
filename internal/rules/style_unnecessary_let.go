package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnnecessaryLet follows detekt 1.23's rule of the same name. detekt needs type
// resolution; this syntactic version is the fallback when the FIR checker has no
// verdict. The decision table is shared with the FIR checker:
//
//	more than one use of the lambda parameter  -> no finding
//	safe call (`a?.let`):
//	  no trailing lambda                       -> no finding
//	  parameter unused, call result unused     -> "can be replaced with a simple if"
//	  parameter used once, body is `p.member`  -> "can be omitted"
//	other receiver (`a.let`, `let`):
//	  parameter unused, or body is `p.member`,
//	  or the call is not part of a call chain  -> "can be omitted"
const (
	unnecessaryLetOmittedMessage  = "let expression can be omitted"
	unnecessaryLetSimpleIfMessage = "let expression can be replaced with a simple if"
)

// unnecessaryLetNoParamCallees are callees whose lambda receives no implicit
// `it`, so an `it` inside such a nested lambda is the enclosing lambda's.
var unnecessaryLetNoParamCallees = map[string]bool{
	"run": true, "apply": true, "with": true, "item": true, "stickyHeader": true, "launch": true, "async": true, "runBlocking": true,
	"withContext": true, "synchronized": true, "lazy": true, "runCatching": true, "thread": true,
	"Thread": true, "buildString": true, "buildList": true, "buildMap": true, "buildSet": true,
	"coroutineScope": true, "supervisorScope": true, "remember": true, "LaunchedEffect": true,
	"DisposableEffect": true, "SideEffect": true, "produce": true, "flow": true, "channelFlow": true,
}

// unnecessaryLetValueCallees are callees whose lambda result is used as a value.
// A lambda passed to any other call is taken to return Unit, as UI callbacks and
// content lambdas do.
var unnecessaryLetValueCallees = map[string]bool{
	"let": true, "run": true, "with": true, "runCatching": true, "map": true, "mapNotNull": true,
	"mapIndexed": true, "flatMap": true, "filter": true, "filterNot": true, "first": true,
	"firstOrNull": true, "last": true, "lastOrNull": true, "find": true, "any": true, "all": true,
	"none": true, "count": true, "sumOf": true, "associate": true, "associateBy": true,
	"associateWith": true, "groupBy": true, "sortedBy": true, "sortedByDescending": true, "maxBy": true,
	"minBy": true, "maxOf": true, "minOf": true, "fold": true, "reduce": true, "lazy": true,
	"remember": true, "derivedStateOf": true, "async": true, "withContext": true, "runBlocking": true,
	"coroutineScope": true, "supervisorScope": true, "use": true, "synchronized": true, "takeIf": true,
	"takeUnless": true, "getOrElse": true, "getOrPut": true, "ifEmpty": true, "orElse": true,
	"mapLatest": true, "flatMapLatest": true, "mapTo": true, "partition": true, "indexOfFirst": true,
	"indexOfLast": true, "distinctBy": true, "takeWhile": true, "dropWhile": true, "mapKeys": true,
	"mapValues": true, "computeIfAbsent": true, "singleOrNull": true, "single": true, "then": true,
}

// unnecessaryLetCheck reports one call_expression when it is a `let` call that
// detekt's UnnecessaryLet flags.
func unnecessaryLetCheck(r *UnnecessaryLetRule, ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	letIdent, navSuffix, safe := unnecessaryLetCallee(file, idx)
	if letIdent == 0 {
		return
	}
	suffix, _ := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return
	}
	lambdaLit, params, stmts := flatTrailingLambdaParts(file, suffix)
	names, ok := unnecessaryLetParamNames(file, params)
	if !ok {
		return
	}
	count := 0
	if lambdaLit != 0 && stmts != 0 {
		count = unnecessaryLetCountReferences(file, stmts, names)
	}
	if count > 1 {
		return
	}
	var body uint32
	if stmts != 0 && unnecessaryLetStatementCount(file, stmts) == 1 {
		body = unnecessaryLetFirstStatement(file, stmts)
	}
	canReplace := body != 0 && unnecessaryLetBodyRootMatches(file, body, names)

	var message string
	switch {
	case safe:
		if lambdaLit == 0 {
			return
		}
		if count == 0 && !unnecessaryLetReceiverUsed(file, idx) {
			message = unnecessaryLetSimpleIfMessage
		} else if count == 1 && canReplace {
			message = unnecessaryLetOmittedMessage
		}
	case count == 0 || canReplace || !unnecessaryLetInCallChain(file, idx):
		message = unnecessaryLetOmittedMessage
	}
	if message == "" || unnecessaryLetShadowed(file) {
		return
	}
	f := r.Finding(file, callLine(file, idx), callCol(file, idx), message)
	f.Fix = unnecessaryLetFix(file, idx, navSuffix, safe, count, body, names)
	ctx.Emit(f)
}

// unnecessaryLetCallee returns the `let` identifier of a call named let, the
// navigation suffix that holds it (0 for a bare call), and whether it is `?.`.
func unnecessaryLetCallee(file *scanner.File, idx uint32) (letIdent, navSuffix uint32, safe bool) {
	first := file.FlatFirstChild(idx)
	switch file.FlatType(first) {
	case "simple_identifier":
		if file.FlatNodeTextEquals(first, "let") {
			return first, 0, false
		}
	case "navigation_expression":
		suffix := flatLastChildOfType(file, first, "navigation_suffix")
		if suffix == 0 {
			return 0, 0, false
		}
		ident, _ := file.FlatFindChild(suffix, "simple_identifier")
		if ident == 0 || !file.FlatNodeTextEquals(ident, "let") {
			return 0, 0, false
		}
		return ident, suffix, strings.HasPrefix(file.FlatNodeText(suffix), "?.")
	}
	return 0, 0, false
}

// unnecessaryLetShadowed reports whether the file declares its own `let`
// function, which a resolving checker would not treat as kotlin.let.
func unnecessaryLetShadowed(file *scanner.File) bool {
	shadowed := false
	file.FlatWalkNodes(0, "function_declaration", func(decl uint32) {
		if !shadowed && extractIdentifierFlat(file, decl) == "let" {
			shadowed = true
		}
	})
	return shadowed
}

// unnecessaryLetParamNames returns the names the lambda parameter is known by:
// `it` without an explicit parameter, the name for one parameter, and the
// destructured names for a destructuring parameter. ok is false when the lambda
// declares several parameters, which `let` does not accept.
func unnecessaryLetParamNames(file *scanner.File, params uint32) (names []string, ok bool) {
	if params == 0 || file.FlatNamedChildCount(params) == 0 {
		return []string{"it"}, true
	}
	if file.FlatNamedChildCount(params) != 1 {
		return nil, false
	}
	param := file.FlatNamedChild(params, 0)
	switch file.FlatType(param) {
	case "multi_variable_declaration":
		file.FlatForEachChild(param, func(c uint32) {
			if file.FlatType(c) != "variable_declaration" {
				return
			}
			if id, found := file.FlatFindChild(c, "simple_identifier"); found {
				names = append(names, file.FlatNodeText(id))
			}
		})
		return names, true
	default:
		id, found := file.FlatFindChild(param, "simple_identifier")
		if !found {
			return nil, false
		}
		return []string{file.FlatNodeText(id)}, true
	}
}

func unnecessaryLetNameIn(names []string, name string) bool {
	for _, n := range names {
		if n == name && n != "_" {
			return true
		}
	}
	return false
}

// unnecessaryLetStatementCount counts the non-comment statements of a body.
func unnecessaryLetStatementCount(file *scanner.File, stmts uint32) int {
	n := 0
	file.FlatForEachChild(stmts, func(c uint32) {
		if unnecessaryLetIsStatement(file, c) {
			n++
		}
	})
	return n
}

func unnecessaryLetFirstStatement(file *scanner.File, stmts uint32) uint32 {
	var first uint32
	file.FlatForEachChild(stmts, func(c uint32) {
		if first == 0 && unnecessaryLetIsStatement(file, c) {
			first = c
		}
	})
	return first
}

func unnecessaryLetIsStatement(file *scanner.File, node uint32) bool {
	if !file.FlatIsNamed(node) {
		return false
	}
	switch file.FlatType(node) {
	case "line_comment", "multiline_comment", "comment":
		return false
	}
	return true
}

// unnecessaryLetCountReferences counts uses of the parameter names in a lambda
// body. A name declared again by a nested lambda hides the parameter, and so
// does the implicit `it` of a nested lambda whose callee passes one argument.
func unnecessaryLetCountReferences(file *scanner.File, stmts uint32, names []string) int {
	count := 0
	var walk func(n uint32)
	walk = func(n uint32) {
		switch file.FlatType(n) {
		case "lambda_literal":
			if unnecessaryLetNestedLambdaHides(file, n, names) {
				return
			}
		case "simple_identifier", "interpolated_identifier":
			if unnecessaryLetNameIn(names, file.FlatNodeText(n)) && unnecessaryLetIsReference(file, n) {
				count++
			}
			return
		}
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			walk(c)
		}
	}
	walk(stmts)
	return count
}

// unnecessaryLetIsReference reports whether an identifier reads a variable, not
// a member name, a declaration, or a named-argument label.
func unnecessaryLetIsReference(file *scanner.File, ident uint32) bool {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return true
	}
	switch file.FlatType(parent) {
	case "navigation_suffix", "variable_declaration", "parameter", "class_parameter", "function_declaration",
		"user_type", "type_identifier", "import_header", "package_header", "class_declaration",
		"object_declaration", "type_alias", "catch_block", "annotation", "label", "lambda_parameters":
		return false
	case "value_argument":
		if next := file.FlatNextSib(ident); next != 0 && file.FlatType(next) == "=" {
			return false
		}
	case "callable_reference":
		// `::it` names a member, `it::foo` reads the variable.
		if prev, found := file.FlatPrevSibling(ident); found && file.FlatType(prev) == "::" {
			return false
		}
	}
	return true
}

func unnecessaryLetNestedLambdaHides(file *scanner.File, lambda uint32, names []string) bool {
	params, _ := file.FlatFindChild(lambda, "lambda_parameters")
	if params != 0 && file.FlatNamedChildCount(params) > 0 {
		hides := false
		file.FlatWalkNodes(params, "simple_identifier", func(id uint32) {
			if unnecessaryLetNameIn(names, file.FlatNodeText(id)) {
				hides = true
			}
		})
		return hides
	}
	if !unnecessaryLetNameIn(names, "it") {
		return false
	}
	callee := unnecessaryLetLambdaCallee(file, lambda)
	if params, _, found := unnecessaryLetDeclaredLambda(file, callee); found {
		return params == 1
	}
	if unnecessaryLetNoParamCallees[callee] {
		return false
	}
	// A trailing lambda of a capitalized call is a UI content lambda without a
	// parameter.
	return callee == "" || callee[0] < 'A' || callee[0] > 'Z' || !unnecessaryLetIsTrailing(file, lambda)
}

func unnecessaryLetIsTrailing(file *scanner.File, lambda uint32) bool {
	parent, ok := file.FlatParent(lambda)
	if !ok {
		return false
	}
	if file.FlatType(parent) == "annotated_lambda" {
		return true
	}
	return file.FlatType(parent) == "call_suffix"
}

// unnecessaryLetDeclaredLambda reads the function-typed parameter of a function
// the file declares by that name: the number of implicit-it values its lambda
// receives (receiver types count as none) and whether it returns Unit.
func unnecessaryLetDeclaredLambda(file *scanner.File, name string) (params int, unit, found bool) {
	if name == "" {
		return 0, false, false
	}
	ambiguous := false
	file.FlatWalkNodes(0, "function_declaration", func(decl uint32) {
		if ambiguous || extractIdentifierFlat(file, decl) != name {
			return
		}
		list, ok := file.FlatFindChild(decl, "function_value_parameters")
		if !ok {
			return
		}
		var fn uint32
		file.FlatForEachChild(list, func(p uint32) {
			if file.FlatType(p) != "parameter" {
				return
			}
			if t := unnecessaryLetFunctionType(file, p); t != 0 {
				fn = t
			}
		})
		if fn == 0 {
			return
		}
		count, isUnit := unnecessaryLetFunctionTypeShape(file, fn)
		if found && (count != params || isUnit != unit) {
			ambiguous = true
			return
		}
		params, unit, found = count, isUnit, true
	})
	if ambiguous {
		return 0, false, false
	}
	return params, unit, found
}

// unnecessaryLetFunctionType finds the function_type of a parameter, through
// nullable and parenthesized wrappers.
func unnecessaryLetFunctionType(file *scanner.File, node uint32) uint32 {
	var out uint32
	file.FlatForEachChild(node, func(c uint32) {
		switch file.FlatType(c) {
		case "function_type":
			out = c
		case "nullable_type", "parenthesized_type":
			if inner := unnecessaryLetFunctionType(file, c); inner != 0 {
				out = inner
			}
		}
	})
	return out
}

func unnecessaryLetFunctionTypeShape(file *scanner.File, fn uint32) (params int, unit bool) {
	list, _ := file.FlatFindChild(fn, "function_type_parameters")
	if list != 0 {
		params = file.FlatNamedChildCount(list)
	}
	last := file.FlatNamedChild(fn, file.FlatNamedChildCount(fn)-1)
	return params, last != 0 && strings.TrimSpace(file.FlatNodeText(last)) == "Unit"
}

// unnecessaryLetLambdaCallee returns the name of the call a lambda is passed to,
// or "" when it is not a call argument.
func unnecessaryLetLambdaCallee(file *scanner.File, lambda uint32) string {
	node := lambda
	for {
		parent, ok := file.FlatParent(node)
		if !ok {
			return ""
		}
		switch file.FlatType(parent) {
		case "annotated_lambda", "value_argument", "value_arguments", "call_suffix":
			node = parent
			continue
		case "call_expression":
			return flatCallExpressionName(file, unnecessaryLetCallWithName(file, parent))
		}
		return ""
	}
}

// unnecessaryLetCallBase returns what a call is made on. The grammar nests a
// call with both arguments and a trailing lambda, `f(x) { }`, in a second call;
// the base is looked for through that nesting but not through `f(x)(y)`, which
// calls the result of `f(x)`.
func unnecessaryLetCallBase(file *scanner.File, call uint32) uint32 {
	for {
		inner := file.FlatFirstChild(call)
		if inner == 0 || file.FlatType(inner) != "call_expression" || !unnecessaryLetSplitCall(file, call, inner) {
			return inner
		}
		call = inner
	}
}

// unnecessaryLetSplitCall reports whether outer holds only the trailing lambda of
// the call whose arguments are in inner.
func unnecessaryLetSplitCall(file *scanner.File, outer, inner uint32) bool {
	outerSuffix, _ := file.FlatFindChild(outer, "call_suffix")
	innerSuffix, _ := file.FlatFindChild(inner, "call_suffix")
	if outerSuffix == 0 || innerSuffix == 0 {
		return false
	}
	return flatCallSuffixLambdaNode(file, outerSuffix) != 0 && !file.FlatHasChildOfType(outerSuffix, "value_arguments") &&
		file.FlatHasChildOfType(innerSuffix, "value_arguments") && flatCallSuffixLambdaNode(file, innerSuffix) == 0
}

// unnecessaryLetCallWithName returns the call that holds the callee name of a
// call whose arguments and trailing lambda are nested.
func unnecessaryLetCallWithName(file *scanner.File, call uint32) uint32 {
	for {
		inner := file.FlatFirstChild(call)
		if inner == 0 || file.FlatType(inner) != "call_expression" || !unnecessaryLetSplitCall(file, call, inner) {
			return call
		}
		call = inner
	}
}

// unnecessaryLetIsQualified reports whether node is a dot or safe-call
// expression (detekt's KtQualifiedExpression).
func unnecessaryLetIsQualified(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "navigation_expression":
		return true
	case "call_expression":
		return file.FlatType(unnecessaryLetCallBase(file, node)) == "navigation_expression"
	}
	return false
}

// unnecessaryLetBodyRootMatches reports whether the body is a qualified chain
// whose leftmost receiver is the lambda parameter.
func unnecessaryLetBodyRootMatches(file *scanner.File, body uint32, names []string) bool {
	if !unnecessaryLetIsQualified(file, body) {
		return false
	}
	root := body
	for unnecessaryLetIsQualified(file, root) {
		if file.FlatType(root) == "call_expression" {
			root = unnecessaryLetCallBase(file, root)
		}
		root = file.FlatFirstChild(root)
	}
	text := file.FlatNodeText(root)
	for _, n := range names {
		if text == n {
			return true
		}
	}
	return false
}

// unnecessaryLetInCallChain reports whether the let call sits in a longer
// qualified chain: its receiver is qualified or it is itself a receiver.
func unnecessaryLetInCallChain(file *scanner.File, call uint32) bool {
	first := file.FlatFirstChild(call)
	if file.FlatType(first) != "navigation_expression" {
		return false
	}
	if recv := file.FlatFirstChild(first); recv != 0 && unnecessaryLetIsQualified(file, recv) {
		return true
	}
	parent, ok := file.FlatParent(call)
	return ok && file.FlatType(parent) == "navigation_expression" && file.FlatFirstChild(parent) == call
}

// unnecessaryLetReceiverUsed reports whether the value of `a?.let { }` is used,
// as detekt's receiverIsUsed does: a call that is not a statement is used, and a
// statement is used only as the value of an enclosing expression.
func unnecessaryLetReceiverUsed(file *scanner.File, call uint32) bool {
	parent, ok := file.FlatParent(call)
	if !ok || file.FlatType(parent) != "statements" {
		return true
	}
	return unnecessaryLetStatementUsed(file, call, parent)
}

// unnecessaryLetStatementUsed reports whether a statement of the block stmts is
// used as a value.
func unnecessaryLetStatementUsed(file *scanner.File, stmt, stmts uint32) bool {
	for {
		if last := unnecessaryLetLastStatement(file, stmts); last != stmt {
			return false
		}
		owner, ok := file.FlatParent(stmts)
		if !ok {
			return false
		}
		switch file.FlatType(owner) {
		case "lambda_literal":
			return !unnecessaryLetLambdaReturnsUnit(file, owner)
		case "control_structure_body":
			expr, ok := file.FlatParent(owner)
			if !ok {
				return false
			}
			switch file.FlatType(expr) {
			case "when_entry":
				expr, _ = file.FlatParent(expr)
			case "if_expression", "when_expression":
			default:
				return false
			}
			stmt = expr
		case "try_expression":
			stmt = owner
		case "catch_block":
			stmt, _ = file.FlatParent(owner)
		default:
			return false
		}
		parent, ok := file.FlatParent(stmt)
		if !ok || file.FlatType(parent) != "statements" {
			return true
		}
		stmts = parent
	}
}

// unnecessaryLetLambdaReturnsUnit guesses from syntax that a lambda has a Unit
// return type: it is passed to a stdlib callee with a Unit lambda, or it
// initializes a property declared with a `-> Unit` function type.
func unnecessaryLetLambdaReturnsUnit(file *scanner.File, lambda uint32) bool {
	callee := unnecessaryLetLambdaCallee(file, lambda)
	if _, unit, found := unnecessaryLetDeclaredLambda(file, callee); found {
		return unit
	}
	if callee != "" {
		return !unnecessaryLetValueCallees[callee]
	}
	parent, ok := file.FlatParent(lambda)
	if !ok || file.FlatType(parent) != "property_declaration" {
		return false
	}
	decl, found := file.FlatFindChild(parent, "variable_declaration")
	if !found {
		return false
	}
	typ := strings.ReplaceAll(file.FlatNodeText(decl), " ", "")
	return strings.HasSuffix(typ, "->Unit")
}

func unnecessaryLetLastStatement(file *scanner.File, stmts uint32) uint32 {
	var last uint32
	file.FlatForEachChild(stmts, func(c uint32) {
		if unnecessaryLetIsStatement(file, c) {
			last = c
		}
	})
	return last
}

// unnecessaryLetFix returns the edit for the two shapes whose replacement keeps
// the value: `a.let { it }` and `a.let { it.member }` (also with `?.`).
func unnecessaryLetFix(file *scanner.File, call, navSuffix uint32, safe bool, count int, body uint32, names []string) *scanner.Fix {
	if navSuffix == 0 || count != 1 || body == 0 || len(names) != 1 {
		return nil
	}
	name := names[0]
	text := strings.TrimSpace(file.FlatNodeText(body))
	fix := func(replacement string) *scanner.Fix {
		return &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(navSuffix)),
			EndByte:     int(file.FlatEndByte(call)),
			Replacement: replacement,
		}
	}
	if text == name && !safe {
		return fix("")
	}
	if !strings.HasPrefix(text, name+".") || strings.Contains(text, "{") || !unnecessaryLetBodyRootMatches(file, body, names) {
		return nil
	}
	remainder := text[len(name):]
	if safe {
		return fix("?" + remainder)
	}
	return fix(remainder)
}
