package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnsanitizedFilenameFromContentProviderRule mirrors Lint's UnsanitizedFilenameFromContentProvider
// (UnsafeFilenameDetector): a ContentProvider display name read from a Cursor and passed to a java.io.File
// constructor before anything could have sanitized it.
type UnsanitizedFilenameFromContentProviderRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: the Cursor receiver is proven from declarations, and the
// data flow is followed through local variables of one function.
func (r *UnsanitizedFilenameFromContentProviderRule) Confidence() float64 {
	return api.ConfidenceMedium
}

const (
	cursorFQN          = "android.database.Cursor"
	contentResolverFQN = "android.content.ContentResolver"
	displayNameColumn  = "_display_name"
)

func (r *UnsanitizedFilenameFromContentProviderRule) check(ctx *api.Context) {
	file := ctx.File
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || (call.Name != "getColumnIndex" && call.Name != "getColumnIndexOrThrow") || len(call.Args) != 1 {
		return
	}
	fn := filenameFlowScope(file, ctx.Idx)
	if fn == 0 || !cursorReceiver(file, call) || !displayNameArgument(file, call.Args[0]) {
		return
	}
	// The column index flows into Cursor.getString(int); each such call starts a flow of the filename.
	lintTrackFlow(file, fn, ctx.Idx, func(ev lintFlowEvent) {
		if ev.receiver {
			return
		}
		get, ok := lintCallAt(file, ev.call)
		if !ok || get.Name != "getString" || len(get.Args) != 1 || !cursorReceiver(file, get) {
			return
		}
		r.checkFilename(ctx, fn, ev.call)
	})
}

// checkFilename follows the value of one getString call: it reports the first uses that reach a File constructor,
// until the value is used in any other call, which Lint takes as a possible sanitization.
func (r *UnsanitizedFilenameFromContentProviderRule) checkFilename(ctx *api.Context, fn, getString uint32) {
	file := ctx.File
	sanitized := false
	lintTrackFlow(file, fn, getString, func(ev lintFlowEvent) {
		if !ev.receiver && !sanitized && fileConstructorCall(file, ev.call) {
			ref := strings.TrimSpace(file.FlatNodeText(ev.ref))
			ctx.Emit(lintFindingAtNode(ctx, ev.ref, "", "Using `"+ref+"` is unsafe as it is a filename obtained "+
				"directly from a `ContentProvider`. You should sanitize it before using it for creating a `File`."))
			return
		}
		sanitized = true
	})
}

// filenameFlowScope returns the function, method or constructor whose body Lint's data flow walks.
func filenameFlowScope(file *scanner.File, idx uint32) uint32 {
	fn, _ := flatEnclosingAncestor(file, idx, "function_declaration", "method_declaration", "constructor_declaration",
		"secondary_constructor")
	return fn
}

// displayNameArgument reports whether the column argument evaluates to "_display_name": a literal, a same-file
// constant, or the DISPLAY_NAME column constants of android.provider.
func displayNameArgument(file *scanner.File, arg uint32) bool {
	if v, ok := lintEvalString(file, arg); ok && v == displayNameColumn {
		return true
	}
	text := strings.Join(strings.Fields(file.FlatNodeText(arg)), "")
	last := text[strings.LastIndex(text, ".")+1:]
	if last != "DISPLAY_NAME" && last != "COLUMN_DISPLAY_NAME" {
		return false
	}
	owner := strings.TrimSuffix(text, "."+last)
	if owner == text {
		// A bare constant needs a static import of one of the column classes.
		full, ok := lintImportsOf(file).bySimple[last]
		if !ok {
			return false
		}
		owner = strings.TrimSuffix(full, "."+last)
		text = full
	}
	for _, cls := range []string{"android.provider.OpenableColumns", "android.provider.MediaStore",
		"android.provider.DocumentsContract"} {
		head := owner
		if i := strings.Index(owner, "."); i >= 0 && !strings.HasPrefix(owner, "android.") {
			head = owner[:i]
		}
		if owner == cls || strings.HasPrefix(owner, cls+".") || lintTypeRefMatches(file, head, cls) {
			return true
		}
	}
	return false
}

// cursorReceiver reports whether the call's receiver is an android.database.Cursor: a declaration visible at the call
// with that type or with an initializer returning one, or the parameter of a use/let/also lambda on such a value.
func cursorReceiver(file *scanner.File, call lintCall) bool {
	if call.ReceiverNode == 0 {
		return false
	}
	// The declaration visible at the call decides the type, so a same-named declaration in a sibling block, a nested
	// function or another class does not (#1621).
	if d, ok := lintVisibleReceiverDecl(file, call); ok {
		switch {
		case d.typ != "":
			return lintTypeRefMatches(file, d.typ, cursorFQN)
		case d.lambda != 0:
			return cursorLambdaParameter(file, d.lambda)
		case d.init != 0:
			return cursorCallResult(file, d.init) || lintTypeRefMatches(file, lintConstructorCallType(file, d.init), cursorFQN)
		}
		return false
	}
	// Nothing visible declares the name (an inherited member, say): fall back to the file's declarations.
	if typ := lintReceiverTypeText(file, call); typ != "" {
		return lintTypeRefMatches(file, strings.TrimSuffix(strings.TrimSpace(typ), "?"), cursorFQN)
	}
	name := strings.TrimSpace(call.ReceiverText)
	if !lintIdentOnly(name) {
		return false
	}
	lambda, ok := lintLambdaBinding(file, call.Idx, name)
	if !ok {
		return false
	}
	return cursorLambdaParameter(file, lambda)
}

// cursorLambdaParameter reports whether the parameter of a use/let/also lambda is a Cursor: the scope function is
// called on a call returning a Cursor, or on a Cursor value.
func cursorLambdaParameter(file *scanner.File, lambda uint32) bool {
	scopeCall, ok := flatEnclosingAncestor(file, lambda, "call_expression")
	if !ok {
		return false
	}
	scope, ok := lintCallAt(file, scopeCall)
	if !ok || (scope.Name != "use" && scope.Name != "let" && scope.Name != "also") || scope.ReceiverNode == 0 {
		return false
	}
	recv := lintSkipParensJS(file, scope.ReceiverNode)
	if _, isCall := lintCallAt(file, recv); isCall {
		return cursorCallResult(file, recv)
	}
	return cursorReceiver(file, lintCall{File: file, Idx: scopeCall, ReceiverNode: scope.ReceiverNode,
		ReceiverText: scope.ReceiverText})
}

// cursorCallResult reports whether the expression is a call returning an android.database.Cursor:
// ContentResolver.query, SQLiteDatabase.query/rawQuery and their variants, or a call to a function declared in the
// file with a Cursor return type. Calls into other files are not followed, so the verdict depends on this file only.
// A trailing `!!`, or an Elvis whose right side returns, throws, breaks or continues, keeps the call's type.
func cursorCallResult(file *scanner.File, expr uint32) bool {
	for {
		expr = lintSkipParensJS(file, expr)
		if file.FlatType(expr) == "postfix_expression" && strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(expr)), "!!") {
			expr = file.FlatNamedChild(expr, 0)
			continue
		}
		if file.FlatType(expr) == "elvis_expression" && file.FlatNamedChildCount(expr) == 2 &&
			file.FlatType(lintSkipParensJS(file, file.FlatNamedChild(expr, 1))) == "jump_expression" {
			expr = file.FlatNamedChild(expr, 0)
			continue
		}
		break
	}
	call, ok := lintCallAt(file, expr)
	if !ok {
		return false
	}
	switch call.Name {
	case "query":
		if contentResolverReceiver(file, call) || sqliteDatabaseReceiver(file, call) {
			return true
		}
	case "rawQuery", "rawQueryWithFactory", "queryWithFactory":
		if sqliteDatabaseReceiver(file, call) {
			return true
		}
	}
	if call.ReceiverNode != 0 && strings.TrimSpace(call.ReceiverText) != "this" {
		return false
	}
	return lintVisibleFunctionsReturn(file, call, cursorFQN)
}

// sqliteDatabaseReceiver reports whether a call is made on an android.database.sqlite.SQLiteDatabase.
func sqliteDatabaseReceiver(file *scanner.File, call lintCall) bool {
	const fqn = "android.database.sqlite.SQLiteDatabase"
	if d, ok := lintVisibleReceiverDecl(file, call); ok {
		return lintVisibleDeclIs(file, d, fqn)
	}
	typ := lintReceiverTypeText(file, call)
	return typ != "" && lintTypeRefMatches(file, strings.TrimSuffix(strings.TrimSpace(typ), "?"), fqn)
}

// contentResolverReceiver reports whether a query call is made on a ContentResolver. A receiver named
// contentResolver is Context.contentResolver unless a declaration visible at the call gives it another type.
func contentResolverReceiver(file *scanner.File, call lintCall) bool {
	if d, ok := lintVisibleReceiverDecl(file, call); ok {
		return lintVisibleDeclIs(file, d, contentResolverFQN)
	}
	recv := strings.TrimSpace(call.ReceiverText)
	recv = strings.TrimSuffix(recv, "!!")
	if recv == "contentResolver" || recv == "this.contentResolver" || recv == "getContentResolver()" ||
		strings.HasSuffix(recv, ".contentResolver") || strings.HasSuffix(recv, ".getContentResolver()") {
		return true
	}
	if typ := lintReceiverTypeText(file, call); typ != "" {
		return lintTypeRefMatches(file, strings.TrimSuffix(strings.TrimSpace(typ), "?"), contentResolverFQN)
	}
	return false
}

// lintFunctionReturnTypeText returns the declared return type of a Kotlin function or a Java method, or "".
func lintFunctionReturnTypeText(file *scanner.File, fn uint32) string {
	if file.FlatType(fn) == "method_declaration" {
		return lintChildTypeText(file, fn)
	}
	afterParams := false
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_value_parameters":
			afterParams = true
		case "user_type", "nullable_type":
			if afterParams {
				return file.FlatNodeText(child)
			}
		case "function_body":
			return ""
		}
	}
	return ""
}

// lintVisibleFunctionsReturn reports whether a receiverless (or `this.`) call names, among the functions visible from
// it, only functions declared to return fqn. The innermost scope with a function of that name and a matching
// argument count decides: a local function declared earlier in an enclosing block, then a member of an enclosing
// class, then a top-level function. Overloads in that scope that disagree on the return type abstain.
func lintVisibleFunctionsReturn(file *scanner.File, call lintCall, fqn string) bool {
	membersOnly := call.ReceiverNode != 0
	arity := len(call.Args)
	if suffix, ok := file.FlatFindChild(call.Idx, "call_suffix"); ok {
		if _, lambda := file.FlatFindChild(suffix, "annotated_lambda"); lambda {
			arity++
		}
	}
	var cands []uint32
	add := func(fn uint32) {
		switch file.FlatType(fn) {
		case "function_declaration", "method_declaration":
			if javascriptInterfaceMemberName(file, fn) == call.Name && lintFunctionAcceptsArity(file, fn, arity) {
				cands = append(cands, fn)
			}
		}
	}
	prev := call.Idx
	for cur, ok := file.FlatParent(call.Idx); ok && len(cands) == 0; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_body", "enum_class_body", "interface_body", "enum_body", "enum_body_declarations", "source_file":
			for child := file.FlatFirstChild(cur); child != 0; child = file.FlatNextSib(child) {
				add(child)
			}
		case "class_declaration", "object_declaration", "object_literal", "program":
		default:
			if !membersOnly {
				for child := file.FlatFirstChild(cur); child != 0 && child != prev; child = file.FlatNextSib(child) {
					add(child)
				}
			}
		}
		prev = cur
	}
	if len(cands) == 0 {
		return false
	}
	for _, fn := range cands {
		typ := strings.TrimSuffix(strings.TrimSpace(lintFunctionReturnTypeText(file, fn)), "?")
		if !lintTypeRefMatches(file, lintTrimGenerics(typ), fqn) {
			return false
		}
	}
	return true
}

// lintFunctionAcceptsArity reports whether a Kotlin function or Java method can be called with n arguments,
// counting Kotlin default values and varargs.
func lintFunctionAcceptsArity(file *scanner.File, fn uint32, n int) bool {
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		params, ok = file.FlatFindChild(fn, "formal_parameters")
	}
	if !ok {
		return false
	}
	total, required, vararg := 0, 0, false
	nextVararg, lastRequired := false, false
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter_modifiers":
			nextVararg = strings.Contains(file.FlatNodeText(child), "vararg")
		case "parameter", "formal_parameter":
			total++
			lastRequired = !nextVararg
			if nextVararg {
				vararg = true
			} else {
				required++
			}
			nextVararg = false
		case "spread_parameter":
			total++
			vararg, lastRequired = true, false
		case "=":
			// A Kotlin default value: the parameter before it may be omitted.
			if lastRequired {
				required--
				lastRequired = false
			}
		}
	}
	return n >= required && (vararg || n <= total)
}

// lintVisibleDecl is the declaration of a name visible at a use site.
type lintVisibleDecl struct {
	node uint32
	// typ is the declared type without type arguments or `?`, or "" when the type is inferred.
	typ string
	// init is the initializer expression, or 0.
	init uint32
	// lambda is the Kotlin lambda whose declared or implicit (`it`) parameter this is, or 0.
	lambda uint32
}

// lintVisibleReceiverDecl resolves an identifier receiver (`x`, `x!!` or `this.x`) of the call to its visible
// declaration.
func lintVisibleReceiverDecl(file *scanner.File, call lintCall) (lintVisibleDecl, bool) {
	name := strings.TrimSuffix(strings.TrimSpace(call.ReceiverText), "!!")
	membersOnly := false
	if rest, ok := strings.CutPrefix(name, "this."); ok {
		name, membersOnly = rest, true
	}
	if !lintIdentOnly(name) || name == "this" || name == "super" {
		return lintVisibleDecl{}, false
	}
	return lintResolveVisibleName(file, call.Idx, name, membersOnly)
}

// lintVisibleDeclIs reports whether a visible declaration has the type fqn, declared or constructed by its
// initializer.
func lintVisibleDeclIs(file *scanner.File, d lintVisibleDecl, fqn string) bool {
	if d.typ != "" {
		return lintTypeRefMatches(file, d.typ, fqn)
	}
	return d.init != 0 && lintTypeRefMatches(file, lintConstructorCallType(file, d.init), fqn)
}

// lintResolveVisibleName finds the declaration of name visible at idx, walking out through the enclosing scopes:
// locals declared earlier in the same or an enclosing block, parameters of enclosing functions and lambdas, loop and
// catch variables, Java resources, members and primary-constructor parameters of enclosing classes, and top-level
// properties. A declaration in a sibling block or inside a nested function, lambda or class is not visible. With
// membersOnly (a `this.` receiver), only class members and top-level properties are considered.
func lintResolveVisibleName(file *scanner.File, idx uint32, name string, membersOnly bool) (lintVisibleDecl, bool) {
	prev := idx
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		var d lintVisibleDecl
		found := false
		switch file.FlatType(cur) {
		case "class_body", "enum_class_body", "interface_body", "enum_body", "enum_body_declarations", "source_file":
			for child := file.FlatFirstChild(cur); child != 0 && !found; child = file.FlatNextSib(child) {
				d, found = lintLocalDeclNamed(file, child, name)
			}
		case "class_declaration", "object_declaration":
			if ctor, ok := file.FlatFindChild(cur, "primary_constructor"); ok {
				if params, ok := file.FlatFindChild(ctor, "class_parameters"); ok {
					ctor = params
				}
				for p := file.FlatFirstChild(ctor); p != 0 && !found; p = file.FlatNextSib(p) {
					if file.FlatType(p) == "class_parameter" && lintDeclaredName(file, p) == name {
						d, found = lintVisibleDecl{node: p, typ: lintChildTypeText(file, p)}, true
					}
				}
			}
		case "program":
		default:
			if membersOnly {
				break
			}
			d, found = lintScopeDeclNamed(file, cur, prev, name)
		}
		if found {
			return d, true
		}
		prev = cur
	}
	return lintVisibleDecl{}, false
}

// lintScopeDeclNamed looks for name among the declarations a block-like or binding node makes visible to its child
// prev: parameters of functions and lambdas, loop and catch variables, and locals or resources declared before prev.
func lintScopeDeclNamed(file *scanner.File, cur, prev uint32, name string) (lintVisibleDecl, bool) {
	switch file.FlatType(cur) {
	case "function_declaration", "secondary_constructor", "anonymous_function", "method_declaration",
		"constructor_declaration":
		params, ok := file.FlatFindChild(cur, "function_value_parameters")
		if !ok {
			params, ok = file.FlatFindChild(cur, "formal_parameters")
		}
		if !ok {
			return lintVisibleDecl{}, false
		}
		return lintParamListDeclNamed(file, params, 0, name)
	case "lambda_literal":
		params, ok := file.FlatFindChild(cur, "lambda_parameters")
		if !ok {
			// A lambda without declared parameters binds the implicit `it`.
			return lintVisibleDecl{node: cur, lambda: cur}, name == "it"
		}
		return lintParamListDeclNamed(file, params, cur, name)
	case "lambda_expression":
		params := file.FlatNamedChild(cur, 0)
		if params == 0 || params == prev {
			return lintVisibleDecl{}, false
		}
		if file.FlatType(params) == "formal_parameters" {
			return lintParamListDeclNamed(file, params, 0, name)
		}
		return lintVisibleDecl{node: params}, lintParamsDeclare(file, params, name)
	case "catch_block":
		return lintVisibleDecl{node: cur, typ: lintChildTypeText(file, cur)}, lintDeclaredName(file, cur) == name
	case "catch_clause":
		p, _ := file.FlatFindChild(cur, "catch_formal_parameter")
		id, ok := file.FlatFindChild(p, "identifier")
		return lintVisibleDecl{node: p}, p != 0 && ok && file.FlatNodeText(id) == name
	case "for_statement":
		return lintParamListDeclNamed(file, cur, 0, name)
	case "enhanced_for_statement":
		id, ok := file.FlatFindChild(cur, "identifier")
		return lintVisibleDecl{node: cur, typ: lintJavaInferredType(lintChildTypeText(file, cur))},
			ok && id != prev && file.FlatNodeText(id) == name
	}
	// A block: the nearest earlier declaration wins.
	var d lintVisibleDecl
	found := false
	for child := file.FlatFirstChild(cur); child != 0 && child != prev; child = file.FlatNextSib(child) {
		decls := []uint32{child}
		if file.FlatType(child) == "resource_specification" {
			decls = decls[:0]
			for res := file.FlatFirstChild(child); res != 0; res = file.FlatNextSib(res) {
				decls = append(decls, res)
			}
		}
		for _, decl := range decls {
			if got, ok := lintLocalDeclNamed(file, decl, name); ok {
				d, found = got, true
			}
		}
	}
	return d, found
}

// lintParamListDeclNamed finds name among the children of a Kotlin or Java parameter list, Kotlin lambda parameters
// (lambda is the lambda, or 0), or a Kotlin for loop's variables.
func lintParamListDeclNamed(file *scanner.File, params, lambda uint32, name string) (lintVisibleDecl, bool) {
	for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
		switch file.FlatType(p) {
		case "parameter", "formal_parameter", "spread_parameter":
			if lintParamNamed(file, p, name) {
				return lintVisibleDecl{node: p, typ: lintChildTypeText(file, p)}, true
			}
		case "variable_declaration":
			if lintDeclaredName(file, p) == name {
				return lintVisibleDecl{node: p, typ: lintChildTypeText(file, p), lambda: lambda}, true
			}
		case "multi_variable_declaration":
			if lintMultiDeclares(file, p, name) {
				return lintVisibleDecl{node: p}, true
			}
		}
	}
	return lintVisibleDecl{}, false
}

// lintLocalDeclNamed reports whether decl is a Kotlin property, a Java local, field or resource declaring name.
func lintLocalDeclNamed(file *scanner.File, decl uint32, name string) (lintVisibleDecl, bool) {
	switch file.FlatType(decl) {
	case "property_declaration":
		if vd, ok := file.FlatFindChild(decl, "variable_declaration"); ok {
			if lintDeclaredName(file, vd) != name {
				return lintVisibleDecl{}, false
			}
			return lintVisibleDecl{node: decl, typ: lintChildTypeText(file, vd), init: lintInitializerAfterEquals(file, decl)}, true
		}
		if mv, ok := file.FlatFindChild(decl, "multi_variable_declaration"); ok && lintMultiDeclares(file, mv, name) {
			return lintVisibleDecl{node: decl}, true
		}
	case "local_variable_declaration", "field_declaration", "constant_declaration":
		typ := lintJavaInferredType(lintChildTypeText(file, decl))
		for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "variable_declarator" {
				continue
			}
			if id, ok := file.FlatFindChild(child, "identifier"); ok && file.FlatNodeText(id) == name {
				return lintVisibleDecl{node: child, typ: typ, init: lintInitializerAfterEquals(file, child)}, true
			}
		}
	case "resource":
		if _, ok := file.FlatFindChild(decl, "="); !ok {
			return lintVisibleDecl{}, false
		}
		if id, ok := file.FlatFindChild(decl, "identifier"); ok && file.FlatNodeText(id) == name {
			return lintVisibleDecl{node: decl, typ: lintJavaInferredType(lintChildTypeText(file, decl)),
				init: lintInitializerAfterEquals(file, decl)}, true
		}
	}
	return lintVisibleDecl{}, false
}

// lintParamNamed reports whether p is a Kotlin or Java parameter called name.
func lintParamNamed(file *scanner.File, p uint32, name string) bool {
	switch file.FlatType(p) {
	case "parameter":
		return lintDeclaredName(file, p) == name
	case "formal_parameter":
		id, ok := file.FlatFindChild(p, "identifier")
		return ok && file.FlatNodeText(id) == name
	case "spread_parameter":
		if vd, ok := file.FlatFindChild(p, "variable_declarator"); ok {
			id, ok := file.FlatFindChild(vd, "identifier")
			return ok && file.FlatNodeText(id) == name
		}
	}
	return false
}

// lintMultiDeclares reports whether a Kotlin destructuring declaration binds name.
func lintMultiDeclares(file *scanner.File, mv uint32, name string) bool {
	for vd := file.FlatFirstChild(mv); vd != 0; vd = file.FlatNextSib(vd) {
		if file.FlatType(vd) == "variable_declaration" && lintDeclaredName(file, vd) == name {
			return true
		}
	}
	return false
}

// lintJavaInferredType maps Java's `var` to "" (inferred from the initializer).
func lintJavaInferredType(typ string) string {
	if typ == "var" {
		return ""
	}
	return typ
}

// lintLambdaBinding returns the innermost Kotlin lambda enclosing idx that binds name, as its declared parameter or
// as the implicit `it`.
func lintLambdaBinding(file *scanner.File, idx uint32, name string) (uint32, bool) {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "lambda_literal" {
			continue
		}
		params, hasParams := file.FlatFindChild(cur, "lambda_parameters")
		if !hasParams {
			if name == "it" {
				return cur, true
			}
			continue
		}
		if lintParamsDeclare(file, params, name) {
			return cur, true
		}
	}
	return 0, false
}

// fileConstructorCall reports whether the call constructs a java.io.File or a class in the file that extends it.
func fileConstructorCall(file *scanner.File, idx uint32) bool {
	call, ok := lintCallAt(file, idx)
	if !ok {
		return false
	}
	switch file.FlatType(idx) {
	case "object_creation_expression":
	case "call_expression":
		if call.ReceiverNode != 0 && !strings.Contains(call.TypeText, ".") {
			return false
		}
	default:
		return false
	}
	return lintTypeExtendsFile(file, call.TypeText, 0)
}

func lintTypeExtendsFile(file *scanner.File, typ string, depth int) bool {
	typ = strings.TrimSpace(typ)
	if typ == "" || depth > 6 {
		return false
	}
	if !strings.Contains(typ, ".") {
		if cls := lintSameFileClass(file, typ); cls != 0 {
			for _, super := range javascriptInterfaceSuperclassNames(file, cls) {
				if lintTypeExtendsFile(file, super, depth+1) {
					return true
				}
			}
			return false
		}
	}
	return lintTypeRefMatches(file, typ, "java.io.File")
}

// lintFlowEvent is a use of a tracked value: as the receiver of a call, or as one of its arguments.
type lintFlowEvent struct {
	call, ref uint32
	receiver  bool
}

// lintTrackFlow is a small port of Lint's DataFlowAnalyzer over one function body. It tracks the start expression
// and the local variables it is assigned to (reassigning a variable drops it), and calls fn, in source order, for
// every call that has a tracked value as its receiver or as an argument. Lambdas and local functions are walked, as
// Lint walks them; local classes are not entered.
func lintTrackFlow(file *scanner.File, root, start uint32, fn func(lintFlowEvent)) {
	f := &lintFlow{file: file, root: root, start: start, tracked: map[string]bool{}, fn: fn}
	f.visit(root)
}

type lintFlow struct {
	file        *scanner.File
	root, start uint32
	tracked     map[string]bool
	fn          func(lintFlowEvent)
}

func (f *lintFlow) isTracked(n uint32) bool {
	n = lintSkipParensJS(f.file, n)
	if n == f.start {
		return true
	}
	switch f.file.FlatType(n) {
	case "simple_identifier", "identifier":
		return f.tracked[f.file.FlatNodeText(n)]
	}
	return false
}

func (f *lintFlow) visit(n uint32) {
	file := f.file
	for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
		// Lint's analyzer also walks local functions, which can capture the value; local classes are separate
		// declarations.
		switch file.FlatType(child) {
		case "class_declaration", "object_declaration", "local_class_declaration":
			if n != f.root {
				continue
			}
		}
		f.visit(child)
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
	case "call_expression", "method_invocation", "object_creation_expression":
		f.call(n)
	}
}

// assign drops a variable that is reassigned, or tracks it when the new value is tracked.
func (f *lintFlow) assign(n uint32) {
	file := f.file
	lhs := file.FlatNamedChild(n, 0)
	rhs := file.FlatNamedChild(n, file.FlatNamedChildCount(n)-1)
	switch file.FlatType(lhs) {
	case "directly_assignable_expression", "identifier", "simple_identifier":
		if name := strings.TrimSpace(file.FlatNodeText(lhs)); lintIdentOnly(name) {
			f.tracked[name] = f.isTracked(rhs)
		}
	}
}

// call reports the tracked values a call uses as its receiver and as its arguments.
func (f *lintFlow) call(n uint32) {
	call, ok := lintCallAt(f.file, n)
	if !ok {
		return
	}
	if call.ReceiverNode != 0 && f.isTracked(call.ReceiverNode) {
		f.fn(lintFlowEvent{call: n, ref: call.ReceiverNode, receiver: true})
	}
	for _, arg := range call.Args {
		if arg != 0 && f.isTracked(arg) {
			f.fn(lintFlowEvent{call: n, ref: lintSkipParensJS(f.file, arg)})
		}
	}
}
