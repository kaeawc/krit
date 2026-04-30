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
		r := &RoomFallbackToDestructiveMigrationRule{BaseRule: BaseRule{RuleName: "RoomFallbackToDestructiveMigration", RuleSetName: "database", Sev: "warning", Desc: "Detects Room database builders calling fallbackToDestructiveMigration outside debug source sets; silent data loss on schema version bump."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "method_invocation"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				name := databaseCallName(file, idx)
				if name != "fallbackToDestructiveMigration" &&
					name != "fallbackToDestructiveMigrationFrom" &&
					name != "fallbackToDestructiveMigrationOnDowngrade" {
					return
				}
				if !sourceImportsOrMentions(file, "androidx.room") {
					return
				}
				if isDebugSourceFile(file.Path) {
					return
				}
				if enclosedInBuildConfigDebugGuard(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Room.%s() drops all user data on a schema version bump; provide a Migration or restrict the call to debug builds.", name))
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
		r := &RoomMultipleWritesMissingTransactionRule{BaseRule: BaseRule{RuleName: "RoomMultipleWritesMissingTransaction", RuleSetName: "database", Sev: "warning", Desc: "Detects Room DAO functions that perform 2+ @Insert/@Update/@Delete calls without being annotated @Transaction."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if hasAnnotationFlat(file, idx, "Transaction") {
					return
				}
				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				cls, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration")
				if !ok {
					return
				}
				if !hasAnnotationFlat(file, cls, "Dao") {
					return
				}
				writers := daoWriteAnnotatedSiblings(file, cls)
				if len(writers) == 0 {
					return
				}
				count := 0
				wrapped := false
				file.FlatWalkNodes(body, "call_expression", func(callIdx uint32) {
					name := flatCallExpressionName(file, callIdx)
					if name == "" {
						return
					}
					if name == "withTransaction" || name == "runInTransaction" {
						wrapped = true
						return
					}
					if _, ok := writers[name]; ok {
						count++
					}
				})
				if wrapped || count < 2 {
					return
				}
				fnName := extractIdentifierFlat(file, idx)
				if fnName == "" {
					fnName = "function"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("DAO function '%s' performs multiple @Insert/@Update/@Delete calls but is not @Transaction; wrap it with @Transaction to make the writes atomic.", fnName))
			},
		})
	}
	{
		r := &RoomMigrationUsesExecSqlWithInterpolationRule{BaseRule: BaseRule{RuleName: "RoomMigrationUsesExecSqlWithInterpolation", RuleSetName: "database", Sev: "warning", Desc: "Detects db.execSQL(...) calls inside a Room Migration that use Kotlin string interpolation in the SQL string."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Languages: []scanner.Language{scanner.LangKotlin}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !flatCallExpressionNameEquals(file, idx, "execSQL") {
					return
				}
				_, args := flatCallExpressionParts(file, idx)
				if args == 0 {
					return
				}
				firstArg := uint32(0)
				for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
					if file.FlatType(c) == "value_argument" {
						firstArg = c
						break
					}
				}
				expr := flatValueArgumentExpression(file, firstArg)
				if expr == 0 || file.FlatType(expr) != "string_literal" {
					return
				}
				if !flatContainsStringInterpolation(file, expr) {
					return
				}
				if !enclosingMigrationOwnerFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1, "execSQL inside a Room Migration uses string interpolation; use a non-interpolated literal or bindArgs to avoid SQL injection and migration drift.")
			},
		})
	}
	{
		r := &RoomQueryMissingWhereForUpdateRule{BaseRule: BaseRule{RuleName: "RoomQueryMissingWhereForUpdate", RuleSetName: "database", Sev: "warning", Desc: "Detects Room @Query(\"UPDATE ...\")/@Query(\"DELETE ...\") whose SQL text omits a WHERE clause."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := findAnnotationTextFlat(file, idx, "Query")
				if text == "" {
					return
				}
				sql := roomQueryAnnotationSQL(text)
				if sql == "" {
					return
				}
				keyword, missing := roomQueryMissingWhereSQL(sql)
				if !missing {
					return
				}
				fnName := extractIdentifierFlat(file, idx)
				if roomQueryFunctionNameAllowsFullTable(fnName) {
					return
				}
				if fnName == "" {
					fnName = "function"
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("@Query on '%s' issues a %s without a WHERE clause; add a WHERE clause or rename the function to deleteAll/clearAll if a full-table operation is intended.", fnName, keyword))
			},
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
