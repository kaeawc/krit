package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerDatabaseRules() {

	// --- from database.go ---
	{
		r := &DatabaseInstanceRecreatedRule{BaseRule: BaseRule{RuleName: "DatabaseInstanceRecreated", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Room.databaseBuilder calls inside regular functions that recreate the database on each invocation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isRoomDatabaseBuilderCallFlat(file, idx) {
					return
				}
				fn, ok := flatEnclosingFunction(file, idx)
				if !ok {
					return
				}
				if _, ok := flatEnclosingAncestor(file, idx, "function_body"); !ok {
					return
				}
				if hasAnnotationFlat(file, fn, "Provides") {
					return
				}
				if hasModuleAnnotatedAncestorFlat(file, fn) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "Room.databaseBuilder(...) inside a regular function recreates the database; create it once via @Provides or a singleton holder.")
			},
		})
	}
	{
		r := &DaoNotInterfaceRule{BaseRule: BaseRule{RuleName: "DaoNotInterface", RuleSetName: "database", Sev: "info", Desc: "Detects Room DAOs declared as abstract classes instead of interfaces."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Dao") {
					return
				}
				if file.FlatHasChildOfType(idx, "interface") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "DAO"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Dao '%s' should be declared as an interface, not an abstract class.", name))
			},
		})
	}
	{
		r := &DaoWithoutAnnotationsRule{BaseRule: BaseRule{RuleName: "DaoWithoutAnnotations", RuleSetName: "database", Sev: "warning", Desc: "Detects Room DAO member functions missing required annotations like @Query, @Insert, @Update, or @Delete."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Dao") {
					return
				}
				body, _ := file.FlatFindChild(idx, "class_body")
				if body == 0 {
					return
				}
				daoName := extractIdentifierFlat(file, idx)
				if daoName == "" {
					daoName = "DAO"
				}
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
					ctx.EmitAt(file.FlatRow(child)+1, 1, fmt.Sprintf("@Dao '%s' function '%s' must be annotated with @Query, @Insert, @Update, @Delete, or @Transaction.", daoName, funcName))
				}
			},
		})
	}
	{
		r := &JdbcPreparedStatementNotClosedRule{BaseRule: BaseRule{RuleName: "JdbcPreparedStatementNotClosed", RuleSetName: "database", Sev: "warning", Desc: "Detects JDBC prepared statements assigned to local properties without .use {} or .close() in the same scope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				stmtName := extractIdentifierFlat(file, idx)
				if stmtName == "" {
					return
				}
				if !jdbcPreparedStatementCallFlat(file, idx) {
					return
				}
				if jdbcPreparedStatementHasCleanupFlat(file, idx, stmtName) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("PreparedStatement '%s' should be wrapped in use { } or explicitly closed with .close().", stmtName))
			},
		})
	}
}
