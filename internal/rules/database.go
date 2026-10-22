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
	return hasAnnotationFlat(file, idx, "Query") ||
		hasAnnotationFlat(file, idx, "Insert") ||
		hasAnnotationFlat(file, idx, "Update") ||
		hasAnnotationFlat(file, idx, "Delete") ||
		hasAnnotationFlat(file, idx, "Transaction")
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
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if flatValueArgumentLabel(file, arg) == "onDelete" {
			return true
		}
	}
	return false
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
	typeText := strings.TrimSpace(directExplicitTypeTextFlat(file, idx))
	if typeText == "" {
		return false
	}
	typeText = strings.TrimSuffix(typeText, "?")
	if dot := strings.LastIndex(typeText, "."); dot >= 0 {
		typeText = typeText[dot+1:]
	}
	return typeText == "ResultSet"
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

func jdbcPreparedStatementCallFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatWalkNodes(idx, "call_expression", func(callIdx uint32) {
		if found || flatCallExpressionName(file, callIdx) != "prepareStatement" {
			return
		}
		found = true
	})
	return found
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
	for _, suffix := range []string{".close(", ".use"} {
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
			if hasAnnotationFlat(file, cur, "Module") {
				return true
			}
		}
	}
	return false
}
