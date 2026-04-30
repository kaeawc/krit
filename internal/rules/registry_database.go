package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerDatabaseRules() {

	// --- from database.go ---
	{
		r := &DatabaseInstanceRecreatedRule{BaseRule: BaseRule{RuleName: "DatabaseInstanceRecreated", RuleSetName: "resource-cost", Sev: "warning", Desc: "Detects Room.databaseBuilder calls inside regular functions that recreate the database on each invocation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isRoomDatabaseBuilderCallFlat(file, idx) {
					return
				}
				fn, ok := flatEnclosingCallable(file, idx)
				if !ok {
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
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
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
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
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
		r := &ForeignKeyWithoutOnDeleteRule{BaseRule: BaseRule{RuleName: "ForeignKeyWithoutOnDelete", RuleSetName: "database", Sev: "warning", Desc: "Detects Room @ForeignKey(...) without an onDelete argument; the default NO_ACTION usually leaves stale rows."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "constructor_invocation"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var args uint32
				switch file.FlatType(idx) {
				case "call_expression":
					if !flatCallExpressionNameEquals(file, idx, "ForeignKey") {
						return
					}
					_, args = flatCallExpressionParts(file, idx)
				case "constructor_invocation":
					if annotationConstructorName(file, idx) != "ForeignKey" {
						return
					}
					args, _ = file.FlatFindChild(idx, "value_arguments")
				default:
					return
				}
				if args == 0 {
					return
				}
				if foreignKeyHasOnDeleteArg(file, args) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "@ForeignKey is missing onDelete; default NO_ACTION leaves stale rows. Set onDelete to CASCADE, RESTRICT, or SET_NULL.")
			},
		})
	}
	{
		r := &RoomConflictStrategyReplaceOnFkRule{BaseRule: BaseRule{RuleName: "RoomConflictStrategyReplaceOnFk", RuleSetName: "database", Sev: "warning", Desc: "Detects @Insert(onConflict = REPLACE) on a Room entity that declares foreign keys; REPLACE deletes and re-inserts, cascading FK deletes."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &JdbcResultSetLeakedFromFunctionRule{BaseRule: BaseRule{RuleName: "JdbcResultSetLeakedFromFunction", RuleSetName: "database", Sev: "warning", Desc: "Detects functions whose declared return type is java.sql.ResultSet; callers almost always forget to close the ResultSet."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !functionHasBodyFlat(file, idx) {
					return
				}
				if !functionReturnsResultSetFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "function"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("Function '%s' returns ResultSet; callers almost always forget to close it. Accept a (ResultSet) -> R block and call .use {} instead.", name))
			},
		})
	}
	{
		r := &EntityPrimaryKeyNotStableRule{BaseRule: BaseRule{RuleName: "EntityPrimaryKeyNotStable", RuleSetName: "database", Sev: "warning", Desc: "Detects @Entity @PrimaryKey declared as var without autoGenerate = true; mutable primary keys break equals/hashCode."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_parameter", "property_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "PrimaryKey") {
					return
				}
				if !declarationIsVarFlat(file, idx) {
					return
				}
				if entityPrimaryKeyAutoGeneratedFlat(file, idx) {
					return
				}
				if !enclosingEntityClassFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "primary key"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("@PrimaryKey '%s' is declared as var without autoGenerate = true; use val or set autoGenerate = true to keep equals/hashCode stable.", name))
			},
		})
	}
	{
		r := &EntityMutableColumnRule{BaseRule: BaseRule{RuleName: "EntityMutableColumn", RuleSetName: "database", Sev: "info", Desc: "Detects Room @Entity class primary-constructor parameters declared as var, which prevents straightforward copy-on-write."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Entity") {
					return
				}
				ctor, _ := file.FlatFindChild(idx, "primary_constructor")
				if ctor == 0 {
					return
				}
				className := extractIdentifierFlat(file, idx)
				if className == "" {
					className = "Entity"
				}
				for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
					param := file.FlatNamedChild(ctor, i)
					if param == 0 || file.FlatType(param) != "class_parameter" {
						continue
					}
					if !classParameterIsVarFlat(file, param) {
						continue
					}
					name := extractIdentifierFlat(file, param)
					if name == "" {
						name = "column"
					}
					ctx.EmitAt(file.FlatRow(param)+1, file.FlatCol(param)+1, fmt.Sprintf("@Entity '%s' column '%s' should be declared as `val` to support copy-on-write.", className, name))
				}
			},
		})
	}
	{
		r := &RoomExportSchemaDisabledRule{BaseRule: BaseRule{RuleName: "RoomExportSchemaDisabled", RuleSetName: "database", Sev: "warning", Desc: "Detects Room @Database(exportSchema = false); disabling schema export loses migration history."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Database") {
					return
				}
				if !roomDatabaseExportSchemaDisabledFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					name = "database"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, fmt.Sprintf("@Database '%s' sets exportSchema = false; schema history is lost. Set exportSchema = true and configure room.schemaLocation.", name))
			},
		})
	}
	{
		r := &RoomEntityChangedMigrationMissingRule{BaseRule: BaseRule{RuleName: "RoomEntityChangedMigrationMissing", RuleSetName: "database", Sev: "warning", Desc: "Detects @Entity columns whose names do not appear in any Room Migration(M, N) declaration in the project."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &RoomRelationWithoutIndexRule{BaseRule: BaseRule{RuleName: "RoomRelationWithoutIndex", RuleSetName: "database", Sev: "warning", Desc: "Detects @Relation(entityColumn = ...) referencing a column that is not declared in the target @Entity's indices."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &SqliteCursorWithoutCloseRule{BaseRule: BaseRule{RuleName: "SqliteCursorWithoutClose", RuleSetName: "database", Sev: "warning", Desc: "Detects SQLiteDatabase rawQuery/query cursors assigned to local properties without .use {} or .close() in the same scope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				cursorName := extractIdentifierFlat(file, idx)
				if cursorName == "" {
					return
				}
				if !sqliteCursorCallFlat(file, idx) {
					return
				}
				if sqliteCursorRHSWrappedInUseFlat(file, idx) {
					return
				}
				if jdbcPreparedStatementHasCleanupFlat(file, idx, cursorName) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Cursor '%s' from rawQuery/query should be wrapped in use { } or explicitly closed with .close().", cursorName))
			},
		})
	}
	{
		r := &RoomDatabaseVersionNotBumpedRule{BaseRule: BaseRule{RuleName: "RoomDatabaseVersionNotBumped", RuleSetName: "database", Sev: "warning", Desc: "Detects @Database whose version is unchanged while @Entity sources have changed since HEAD. CI-only: skipped unless KRIT_CI_MODE=1."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsParsedFiles, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &JdbcPreparedStatementNotClosedRule{BaseRule: BaseRule{RuleName: "JdbcPreparedStatementNotClosed", RuleSetName: "database", Sev: "warning", Desc: "Detects JDBC prepared statements assigned to local properties without .use {} or .close() in the same scope."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Implementation: r,
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
