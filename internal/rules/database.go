package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// DatabaseInstanceRecreatedRule detects Room.databaseBuilder calls in regular
// functions where the database would be rebuilt on each invocation.
type DatabaseInstanceRecreatedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DatabaseInstanceRecreatedRule) Confidence() float64 { return api.ConfidenceMedium }

// DaoNotInterfaceRule detects Room DAOs declared as abstract classes.
type DaoNotInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DaoNotInterfaceRule) Confidence() float64 { return api.ConfidenceMedium }

// DaoWithoutAnnotationsRule detects Room DAO member functions that are missing
// any Room operation annotation.
type DaoWithoutAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DaoWithoutAnnotationsRule) Confidence() float64 { return api.ConfidenceMedium }

func daoFunctionHasAllowedAnnotationFlat(file *scanner.File, idx uint32) bool {
	return hasExactAnnotationFlat(file, idx, "Query") ||
		hasExactAnnotationFlat(file, idx, "Insert") ||
		hasExactAnnotationFlat(file, idx, "Update") ||
		hasExactAnnotationFlat(file, idx, "Delete") ||
		hasExactAnnotationFlat(file, idx, "Upsert") ||
		hasExactAnnotationFlat(file, idx, "RawQuery") ||
		hasExactAnnotationFlat(file, idx, "Transaction")
}

// ForeignKeyWithoutOnDeleteRule detects Room @ForeignKey(...) constructions
// that omit the named onDelete argument, falling back to NO_ACTION.
type ForeignKeyWithoutOnDeleteRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection is
// name-based and may match unrelated ForeignKey constructors.
func (r *ForeignKeyWithoutOnDeleteRule) Confidence() float64 { return api.ConfidenceMedium }

func foreignKeyHasOnDeleteArg(file *scanner.File, args uint32) bool {
	if args == 0 {
		return false
	}
	positional := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, arg)
		if label == "onDelete" {
			return true
		}
		if label == "" {
			positional++
		}
	}
	// onDelete is the fourth constructor parameter (entity, parentColumns, childColumns, onDelete).
	return positional >= 4
}

// JdbcResultSetLeakedFromFunctionRule detects functions whose declared return
// type is java.sql.ResultSet. Returning a ResultSet to a caller almost always
// leaks the underlying cursor because the caller forgets to close it.
type JdbcResultSetLeakedFromFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *JdbcResultSetLeakedFromFunctionRule) Confidence() float64 { return api.ConfidenceMedium }

func functionReturnsResultSetFlat(file *scanner.File, idx uint32) bool {
	if !sourceImportsOrMentions(file, "java.sql.") {
		return false
	}
	typeText := strings.TrimSpace(directExplicitTypeTextFlat(file, idx))
	if typeText == "" {
		return false
	}
	typeText = strings.TrimSuffix(typeText, "?")
	qualified := strings.Contains(typeText, ".")
	if dot := strings.LastIndex(typeText, "."); dot >= 0 {
		typeText = typeText[dot+1:]
	}
	if typeText != "ResultSet" {
		return false
	}
	// An explicit import of another package's ResultSet names that class.
	if !qualified {
		if fqn := unaliasedImportFqn(file, "ResultSet"); fqn != "" && fqn != "java.sql.ResultSet" {
			return false
		}
	}
	return !resultSetOverridesJdbcAPIFlat(file, idx)
}

// unaliasedImportFqn returns the path of the import that brings simple into
// scope under its own name, or "" when none does.
func unaliasedImportFqn(file *scanner.File, simple string) string {
	for _, line := range file.Lines {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "import" && strings.HasSuffix(strings.TrimSuffix(fields[1], ";"), "."+simple) {
			return strings.TrimSuffix(fields[1], ";")
		}
	}
	return ""
}

// jdbcDirectSupertypesFlat returns the class's direct supertypes, an
// interface implemented by delegation (`Statement by delegate`) included.
func jdbcDirectSupertypesFlat(file *scanner.File, cls uint32) []androidSupertypeRef {
	supertypes := androidDirectSupertypesFlat(file, cls)
	file.FlatWalkNodes(cls, "explicit_delegation", func(delegation uint32) {
		spec, ok := file.FlatParent(delegation)
		if !ok || file.FlatType(spec) != "delegation_specifier" {
			return
		}
		if owner, ok := flatEnclosingAncestor(file, spec, "class_declaration", "object_declaration", "object_literal"); !ok || owner != cls {
			return
		}
		if ut, ok := file.FlatFindChild(delegation, "user_type"); ok {
			name := androidUserTypeNameFlat(file, ut)
			supertypes = append(supertypes, androidSupertypeRef{name: name, simple: androidSimpleName(name), qualified: strings.Contains(name, ".")})
		}
	})
	return supertypes
}

// jdbcAPITypes are the java.sql and javax.sql types whose members return a
// ResultSet.
var jdbcAPITypes = map[string]bool{
	"Statement": true, "PreparedStatement": true, "CallableStatement": true, "DatabaseMetaData": true,
	"Array": true, "RowSet": true, "CachedRowSet": true, "JdbcRowSet": true, "WebRowSet": true,
	"FilteredRowSet": true, "JoinRowSet": true,
}

// resultSetOverridesJdbcAPIFlat reports whether the function at idx is an
// override in a class whose direct supertypes include a java.sql or
// javax.sql type: the JDBC API fixes its return type.
func resultSetOverridesJdbcAPIFlat(file *scanner.File, idx uint32) bool {
	if !file.FlatHasModifier(idx, "override") {
		return false
	}
	cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "object_literal")
	if !ok {
		return false
	}
	for _, super := range jdbcDirectSupertypesFlat(file, cls) {
		if !jdbcAPITypes[super.simple] {
			continue
		}
		fqn := super.name
		if !super.qualified {
			fqn = unaliasedImportFqn(file, super.simple)
		}
		if strings.HasPrefix(fqn, "java.sql.") || strings.HasPrefix(fqn, "javax.sql.") {
			return true
		}
		if !super.qualified && (sourceImportsOrMentions(file, "java.sql.*") || sourceImportsOrMentions(file, "javax.sql.*")) {
			return true
		}
	}
	return false
}

func functionHasBodyFlat(file *scanner.File, idx uint32) bool {
	body, _ := file.FlatFindChild(idx, "function_body")
	return body != 0
}

// JdbcPreparedStatementNotClosedRule detects JDBC prepared statements assigned
// to local properties without a later .use {} or .close() in the same scope.
type JdbcPreparedStatementNotClosedRule struct {
	FlatDispatchBase
	BaseRule
}

// SqliteCursorWithoutCloseRule detects Android SQLiteDatabase rawQuery/query
// results assigned to local properties without a later .use {} or .close() in
// the same scope.
type SqliteCursorWithoutCloseRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection matches on
// method name without confirming the receiver is SQLiteDatabase.
func (r *SqliteCursorWithoutCloseRule) Confidence() float64 { return api.ConfidenceMedium }

// sqliteCursorCallFlat reports whether the property initializer at idx has a
// top-level call_expression named rawQuery or query, and if so returns the
// matching call node's flat index. Only call_expression nodes that sit on the
// property initializer's top-level call/navigation chain qualify — calls
// nested inside lambdas, anonymous functions, or local declarations belong to
// a different scope and do not determine the property's static type, so they
// must not count.
func sqliteCursorCallFlat(file *scanner.File, idx uint32) (uint32, bool) {
	var match uint32
	found := false
	flatWalkLocalScope(file, idx, func(node uint32) {
		if found || file.FlatType(node) != "call_expression" {
			return
		}
		name := flatCallExpressionName(file, node)
		if name == "rawQuery" || name == "query" {
			match = node
			found = true
		}
	})
	return match, found
}

func sqliteCursorRHSWrappedInUseFlat(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	return strings.Contains(text, ".use(") || strings.Contains(text, ".use {") || strings.Contains(text, ".use{")
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *JdbcPreparedStatementNotClosedRule) Confidence() float64 { return api.ConfidenceMedium }

// jdbcPreparedStatementCallFlat returns the first prepareStatement call
// anywhere in the property at idx.
func jdbcPreparedStatementCallFlat(file *scanner.File, idx uint32) (uint32, bool) {
	var match uint32
	found := false
	file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
		if found || flatCallExpressionName(file, callIdx) != "prepareStatement" {
			return
		}
		match = callIdx
		found = true
	})
	return match, found
}

// receiverReturningScopeCalls return the receiver they are called on.
var receiverReturningScopeCalls = map[string]bool{"apply": true, "also": true, "takeIf": true, "takeUnless": true}

// resourceHandedOverAfter reports whether the property decl named name is
// handed to other code after its declaration in owner: read other than as a
// receiver or an assignment target (localEscapesAfterDeclaration), or
// returned through calls that return their receiver
// (`return cursor.apply { moveToFirst() }`).
func resourceHandedOverAfter(file *scanner.File, owner, decl uint32, name string) bool {
	if localEscapesAfterDeclaration(file, owner, decl, name) {
		return true
	}
	declEnd := file.FlatEndByte(decl)
	returned := false
	file.FlatWalkNodes(owner, "simple_identifier", func(id uint32) {
		if returned || file.FlatStartByte(id) < declEnd || file.FlatNodeText(id) != name {
			return
		}
		current := id
		for {
			nav, ok := file.FlatParent(current)
			if !ok || file.FlatType(nav) != "navigation_expression" || file.FlatFirstChild(nav) != current {
				break
			}
			call, ok := file.FlatParent(nav)
			if !ok || file.FlatType(call) != "call_expression" || !receiverReturningScopeCalls[flatCallExpressionName(file, call)] {
				break
			}
			current = call
		}
		if current == id {
			return
		}
		if parent, ok := file.FlatParent(current); ok && file.FlatType(parent) == "jump_expression" &&
			strings.HasPrefix(file.FlatNodeText(parent), "return") {
			returned = true
		}
	})
	return returned
}

// resourceReturningScopeCalls return (or hand to their lambda) the receiver
// they are called on, so the property still holds the resource.
var resourceReturningScopeCalls = map[string]bool{
	"apply": true, "also": true, "takeIf": true, "takeUnless": true, "let": true, "run": true,
}

// propertyMayHoldCallResult reports whether the property at prop may hold
// the value of call itself, so handing the property over hands that value
// over. It is false only when the initializer reads a member of the call's
// result (`rawQuery(...).count`, `prepareStatement(sql).executeQuery()`),
// whose value is not the resource; any other shape may hold it.
func propertyMayHoldCallResult(file *scanner.File, prop, call uint32) bool {
	expr := propertyInitializerExpression(file, prop)
	for expr != 0 {
		expr = flatUnwrapParenExpr(file, expr)
		if expr == call {
			return true
		}
		switch file.FlatType(expr) {
		case "postfix_expression", "elvis_expression", "as_expression":
			expr = file.FlatNamedChild(expr, 0)
		case "call_expression":
			nav, _ := flatCallExpressionParts(file, expr)
			receiver := flatNavigationReceiver(file, nav)
			if !flatNodeWithin(file, receiver, call) {
				return true
			}
			if !resourceReturningScopeCalls[flatNavigationExpressionLastIdentifier(file, nav)] {
				return false
			}
			expr = receiver
		case "navigation_expression":
			return !flatNodeWithin(file, flatNavigationReceiver(file, expr), call)
		default:
			return true
		}
	}
	return true
}

func jdbcPreparedStatementHasCleanupFlat(file *scanner.File, idx uint32, stmtName string) bool {
	if stmtName == "" {
		return false
	}
	scope, ok := file.FlatParent(idx)
	if !ok {
		return false
	}

	end := file.FlatEndByte(idx)
	for i := 0; i < file.FlatChildCount(scope); i++ {
		child := file.FlatChild(scope, i)
		if file.FlatStartByte(child) <= end {
			continue
		}

		childText := file.FlatNodeText(child)
		if jdbcCloseReceiverPresent(childText, stmtName) {
			return true
		}
	}

	return false
}

// jdbcCloseReceiverPresent reports whether text contains a `<stmtName>.close(`
// or `<stmtName>.use` call where stmtName matches as a whole identifier token
// (not just a trailing substring of an unrelated identifier such as
// `apsCloser.close(` for a stmtName of `Closer`, or `xstmt.close(` for `stmt`).
func jdbcCloseReceiverPresent(text, stmtName string) bool {
	if stmtName == "" {
		return false
	}
	for _, suffix := range []string{".close(", "?.close(", "!!.close(", ".use", "?.use", "!!.use"} {
		needle := stmtName + suffix
		start := 0
		for {
			rel := strings.Index(text[start:], needle)
			if rel < 0 {
				break
			}
			pos := start + rel
			if pos == 0 || !isIdentChar(text[pos-1]) {
				return true
			}
			start = pos + 1
		}
	}
	return false
}

func isRoomDatabaseBuilderCallFlat(file *scanner.File, idx uint32) bool {
	if !sourceImportsOrMentions(file, "androidx.room.Room") {
		return false
	}
	if file.FlatType(idx) == "method_invocation" {
		if databaseCallName(file, idx) != "databaseBuilder" {
			return false
		}
		receiver := databaseCallReceiverName(file, idx)
		return receiver == "Room" || strings.HasSuffix(receiver, ".Room")
	}
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "databaseBuilder" {
		return false
	}

	navText := strings.Join(strings.Fields(file.FlatNodeText(navExpr)), "")
	navText = strings.ReplaceAll(navText, "?.", ".")
	parts := strings.Split(navText, ".")
	if len(parts) < 2 {
		return false
	}

	return parts[len(parts)-2] == "Room" || strings.HasSuffix(parts[len(parts)-2], "androidx.room.Room")
}

func hasModuleAnnotatedAncestorFlat(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			if hasExactAnnotationFlat(file, cur, "Module") {
				return true
			}
		}
	}
	return false
}

// roomBuilderStoredOnceFlat reports whether the databaseBuilder call at idx
// builds a cached instance: it sits inside an elvis fallback, a synchronized
// block, or an assignment to an existing variable, or in an app-start hook.
func roomBuilderStoredOnceFlat(file *scanner.File, idx, fn uint32) bool {
	switch extractIdentifierFlat(file, fn) {
	case "onCreate", "attachBaseContext":
		return true
	}
	for cur, ok := file.FlatParent(idx); ok && cur != fn; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "elvis_expression", "assignment":
			return true
		case "call_expression":
			switch flatCallExpressionName(file, cur) {
			case "synchronized", "withLock", "getOrPut", "lazy":
				return true
			}
		}
	}
	return false
}
