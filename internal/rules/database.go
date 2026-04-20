package rules

import (
	"strings"

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
func (r *DatabaseInstanceRecreatedRule) Confidence() float64 { return 0.75 }

// DaoNotInterfaceRule detects Room DAOs declared as abstract classes.
type DaoNotInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DaoNotInterfaceRule) Confidence() float64 { return 0.75 }

// DaoWithoutAnnotationsRule detects Room DAO member functions that are missing
// any Room operation annotation.
type DaoWithoutAnnotationsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DaoWithoutAnnotationsRule) Confidence() float64 { return 0.75 }

func daoFunctionHasAllowedAnnotationFlat(file *scanner.File, idx uint32) bool {
	return hasAnnotationFlat(file, idx, "Query") ||
		hasAnnotationFlat(file, idx, "Insert") ||
		hasAnnotationFlat(file, idx, "Update") ||
		hasAnnotationFlat(file, idx, "Delete") ||
		hasAnnotationFlat(file, idx, "Transaction")
}

// JdbcPreparedStatementNotClosedRule detects JDBC prepared statements assigned
// to local properties without a later .use {} or .close() in the same scope.
type JdbcPreparedStatementNotClosedRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *JdbcPreparedStatementNotClosedRule) Confidence() float64 { return 0.75 }

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
		if strings.Contains(childText, stmtName+".close(") || strings.Contains(childText, stmtName+".use") {
			return true
		}
	}

	return false
}

func isRoomDatabaseBuilderCallFlat(file *scanner.File, idx uint32) bool {
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

	return parts[len(parts)-2] == "Room"
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
