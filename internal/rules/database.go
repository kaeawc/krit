package rules

import (
	"bytes"
	"fmt"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
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

// ForeignKeyWithoutOnDeleteRule detects Room @ForeignKey(...) constructions
// that omit the named onDelete argument, falling back to NO_ACTION.
type ForeignKeyWithoutOnDeleteRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection is
// name-based and may match unrelated ForeignKey constructors.
func (r *ForeignKeyWithoutOnDeleteRule) Confidence() float64 { return 0.75 }

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
func (r *JdbcResultSetLeakedFromFunctionRule) Confidence() float64 { return 0.75 }

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

// RoomConflictStrategyReplaceOnFkRule detects @Insert(onConflict = REPLACE)
// on Room DAO methods whose target entity declares foreign keys. REPLACE
// deletes and re-inserts the row, which cascades FK deletes.
type RoomConflictStrategyReplaceOnFkRule struct {
	FlatDispatchBase
	BaseRule
}

// EntityMutableColumnRule detects Room @Entity class declarations whose
// primary-constructor parameters use `var`, preventing straightforward
// copy-on-write semantics for entity rows.
type EntityMutableColumnRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection matches on annotation names without confirming the parameter
// resolves to a Room @Entity class.
func (r *RoomConflictStrategyReplaceOnFkRule) Confidence() float64 { return 0.75 }

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *EntityMutableColumnRule) Confidence() float64 { return 0.75 }

func classParameterIsVarFlat(file *scanner.File, param uint32) bool {
	bpk, _ := file.FlatFindChild(param, "binding_pattern_kind")
	if bpk == 0 {
		return false
	}
	for c := file.FlatFirstChild(bpk); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "var" {
			return true
		}
	}
	return false
}

func (r *RoomConflictStrategyReplaceOnFkRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}

	entitiesWithFk := make(map[string]struct{})
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Entity")) || !bytes.Contains(file.Content, []byte("foreignKeys")) {
			continue
		}
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			text := findAnnotationTextFlat(file, idx, "Entity")
			if text == "" || !strings.Contains(text, "foreignKeys") {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			entitiesWithFk[name] = struct{}{}
		})
	}
	if len(entitiesWithFk) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Insert")) || !bytes.Contains(file.Content, []byte("REPLACE")) {
			continue
		}
		file.FlatWalkNodes(0, "function_declaration", func(idx uint32) {
			insertText := findAnnotationTextFlat(file, idx, "Insert")
			if insertText == "" || !strings.Contains(insertText, "REPLACE") {
				return
			}
			entityName := roomInsertEntityParameterTypeFlat(file, idx)
			if entityName == "" {
				return
			}
			if _, ok := entitiesWithFk[entityName]; !ok {
				return
			}
			fnName := flatFunctionName(file, idx)
			if fnName == "" {
				fnName = "insert"
			}
			ctx.Emit(r.Finding(
				file,
				file.FlatRow(idx)+1,
				1,
				fmt.Sprintf("@Insert(onConflict = REPLACE) on '%s' targets entity '%s' which declares foreign keys; REPLACE deletes and re-inserts, cascading FK deletes. Use OnConflictStrategy.IGNORE or @Update.", fnName, entityName),
			))
		})
	}
}

func roomInsertEntityParameterTypeFlat(file *scanner.File, fn uint32) string {
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return ""
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		userType, _ := file.FlatFindChild(child, "user_type")
		if userType == 0 {
			return ""
		}
		ident := flatLastChildOfType(file, userType, "type_identifier")
		if ident == 0 {
			return ""
		}
		return file.FlatNodeText(ident)
	}
	return ""
}

// EntityPrimaryKeyNotStableRule detects Room @Entity primary keys declared as
// `var` without `autoGenerate = true`. A mutable primary key breaks
// equals/hashCode contracts when the row is inserted and assigned a real id.
type EntityPrimaryKeyNotStableRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection matches on annotation names without confirming the declared type
// is a Room entity. Classified per roadmap/17.
func (r *EntityPrimaryKeyNotStableRule) Confidence() float64 { return 0.75 }

func entityPrimaryKeyAutoGeneratedFlat(file *scanner.File, idx uint32) bool {
	text := findAnnotationTextFlat(file, idx, "PrimaryKey")
	if text == "" {
		return false
	}
	compact := strings.Join(strings.Fields(text), "")
	return strings.Contains(compact, "autoGenerate=true")
}

func enclosingEntityClassFlat(file *scanner.File, idx uint32) bool {
	cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
	if !ok {
		return false
	}
	return hasAnnotationFlat(file, cls, "Entity")
}

func declarationIsVarFlat(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "var") {
			return true
		}
	}
	return false
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
