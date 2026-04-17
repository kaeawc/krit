package rules

import (
	"fmt"
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

func (r *DatabaseInstanceRecreatedRule) NodeTypes() []string { return []string{"call_expression"} }

func (r *DatabaseInstanceRecreatedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isRoomDatabaseBuilderCallFlat(file, idx) {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if _, ok := flatEnclosingAncestor(file, idx, "function_body"); !ok {
		return nil
	}
	if hasAnnotationFlat(file, fn, "Provides") {
		return nil
	}
	if hasModuleAnnotatedAncestorFlat(file, fn) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Room.databaseBuilder(...) inside a regular function recreates the database; create it once via @Provides or a singleton holder.",
	)}
}

// DaoNotInterfaceRule detects Room DAOs declared as abstract classes.
type DaoNotInterfaceRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *DaoNotInterfaceRule) Confidence() float64 { return 0.75 }

func (r *DaoNotInterfaceRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *DaoNotInterfaceRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasAnnotationFlat(file, idx, "Dao") {
		return nil
	}
	if file.FlatHasChildOfType(idx, "interface") {
		return nil
	}

	name := extractIdentifierFlat(file, idx)
	if name == "" {
		name = "DAO"
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		1,
		fmt.Sprintf("@Dao '%s' should be declared as an interface, not an abstract class.", name),
	)}
}

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

func (r *DaoWithoutAnnotationsRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *DaoWithoutAnnotationsRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !hasAnnotationFlat(file, idx, "Dao") {
		return nil
	}

	body := file.FlatFindChild(idx, "class_body")
	if body == 0 {
		return nil
	}

	daoName := extractIdentifierFlat(file, idx)
	if daoName == "" {
		daoName = "DAO"
	}

	var findings []scanner.Finding
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if daoFunctionHasAllowedAnnotationFlat(file, child) {
			continue
		}

		funcName := extractIdentifierFlat(file, child)
		if funcName == "" {
			funcName = "function"
		}

		findings = append(findings, r.Finding(
			file,
			file.FlatRow(child)+1,
			1,
			fmt.Sprintf("@Dao '%s' function '%s' must be annotated with @Query, @Insert, @Update, @Delete, or @Transaction.", daoName, funcName),
		))
	}

	return findings
}

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

func (r *JdbcPreparedStatementNotClosedRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *JdbcPreparedStatementNotClosedRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	stmtName := extractIdentifierFlat(file, idx)
	if stmtName == "" {
		return nil
	}
	if !jdbcPreparedStatementCallFlat(file, idx) {
		return nil
	}
	if jdbcPreparedStatementHasCleanupFlat(file, idx, stmtName) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		1,
		fmt.Sprintf("PreparedStatement '%s' should be wrapped in use { } or explicitly closed with .close().", stmtName),
	)}
}

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
