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

// RoomExportSchemaDisabledRule detects Room @Database(exportSchema = false)
// declarations. Disabling schema export loses migration history.
type RoomExportSchemaDisabledRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection matches on annotation names without confirming the annotated
// class is androidx.room.Database.
func (r *RoomExportSchemaDisabledRule) Confidence() float64 { return 0.75 }

func roomDatabaseExportSchemaDisabledFlat(file *scanner.File, idx uint32) bool {
	text := findAnnotationTextFlat(file, idx, "Database")
	if text == "" {
		return false
	}
	compact := strings.Join(strings.Fields(text), "")
	return strings.Contains(compact, "exportSchema=false")
}

// RoomEntityChangedMigrationMissingRule detects @Entity columns whose names
// do not appear in any Room Migration(M, N) declaration in the project. A
// newly added or removed column without a corresponding migration update
// will fail Room's schema validation at runtime.
type RoomEntityChangedMigrationMissingRule struct {
	FlatDispatchBase
	BaseRule
}
// Confidence reports a tier-3 (low) base confidence. Detection cannot tell
// when a column was introduced; pre-existing columns left out of migration
// SQL also match. Inactive by default.
func (r *RoomEntityChangedMigrationMissingRule) Confidence() float64 { return 0.6 }

func (r *RoomEntityChangedMigrationMissingRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}

	migrationCorpus := collectRoomMigrationCorpus(index.Files)
	if migrationCorpus == "" {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Entity")) {
			continue
		}
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
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
			tableName := roomEntityTableName(file, idx, className)
			for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
				param := file.FlatNamedChild(ctor, i)
				if param == 0 || file.FlatType(param) != "class_parameter" {
					continue
				}
				if hasAnnotationFlat(file, param, "Ignore") {
					continue
				}
				column := roomEntityColumnName(file, param)
				if column == "" {
					continue
				}
				if migrationCorpusMentions(migrationCorpus, column) {
					continue
				}
				ctx.Emit(r.Finding(
					file,
					file.FlatRow(param)+1,
					file.FlatCol(param)+1,
					fmt.Sprintf("@Entity '%s' column '%s' is not referenced by any Migration(...) in the project; add a Migration(N-1, N) that updates table '%s' for this column.", className, column, tableName),
				))
			}
		})
	}
}

func collectRoomMigrationCorpus(files []*scanner.File) string {
	var b strings.Builder
	for _, file := range files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Migration")) {
			continue
		}
		appendMigrationDeclTexts(file, "object_declaration", &b)
		appendMigrationDeclTexts(file, "class_declaration", &b)
	}
	return b.String()
}

func appendMigrationDeclTexts(file *scanner.File, nodeType string, b *strings.Builder) {
	file.FlatWalkNodes(0, nodeType, func(idx uint32) {
		if !declarationExtendsRoomMigrationFlat(file, idx) {
			return
		}
		body, _ := file.FlatFindChild(idx, "class_body")
		if body == 0 {
			return
		}
		b.WriteString(file.FlatNodeText(body))
		b.WriteByte('\n')
	})
}

func declarationExtendsRoomMigrationFlat(file *scanner.File, idx uint32) bool {
	found := false
	file.FlatForEachChild(idx, func(child uint32) {
		if found {
			return
		}
		if file.FlatType(child) != "delegation_specifier" {
			return
		}
		text := strings.Join(strings.Fields(file.FlatNodeText(child)), "")
		open := strings.Index(text, "Migration(")
		if open < 0 {
			return
		}
		if open > 0 {
			prev := text[open-1]
			if prev != '.' && prev != ':' && prev != ' ' {
				return
			}
		}
		args := text[open+len("Migration("):]
		close := strings.Index(args, ")")
		if close < 0 {
			return
		}
		parts := strings.Split(args[:close], ",")
		if len(parts) != 2 {
			return
		}
		if !isIntLiteral(parts[0]) || !isIntLiteral(parts[1]) {
			return
		}
		found = true
	})
	return found
}

func isIntLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func roomEntityTableName(file *scanner.File, idx uint32, fallback string) string {
	if name := annotationStringArg(file, idx, "Entity", "tableName"); name != "" {
		return name
	}
	return fallback
}

func roomEntityColumnName(file *scanner.File, param uint32) string {
	if name := annotationStringArg(file, param, "ColumnInfo", "name"); name != "" {
		return name
	}
	return extractIdentifierFlat(file, param)
}

func annotationStringArg(file *scanner.File, idx uint32, annotation, arg string) string {
	text := findAnnotationTextFlat(file, idx, annotation)
	if text == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(text), "")
	key := arg + "=\""
	start := strings.Index(compact, key)
	if start < 0 {
		return ""
	}
	rest := compact[start+len(key):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func migrationCorpusMentions(corpus, column string) bool {
	if column == "" {
		return true
	}
	pos := 0
	for {
		rel := strings.Index(corpus[pos:], column)
		if rel < 0 {
			return false
		}
		idx := pos + rel
		end := idx + len(column)
		var before, after byte = ' ', ' '
		if idx > 0 {
			before = corpus[idx-1]
		}
		if end < len(corpus) {
			after = corpus[end]
		}
		if !isIdentChar(before) && !isIdentChar(after) {
			return true
		}
		pos = end
	}
}

// RoomRelationWithoutIndexRule detects @Relation properties whose referenced
// entityColumn is not declared in the target @Entity's indices list.
type RoomRelationWithoutIndexRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection matches on
// annotation names without confirming the resolved type is a Room entity.
func (r *RoomRelationWithoutIndexRule) Confidence() float64 { return 0.75 }

type roomEntityFacts struct {
	indexed map[string]struct{}
}

func (r *RoomRelationWithoutIndexRule) check(ctx *v2.Context) {
	index := ctx.CodeIndex
	if index == nil || len(index.Files) == 0 {
		return
	}

	entities := make(map[string]roomEntityFacts)
	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Entity")) {
			continue
		}
		file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
			entityText := findAnnotationTextFlat(file, idx, "Entity")
			if entityText == "" {
				return
			}
			name := extractIdentifierFlat(file, idx)
			if name == "" {
				return
			}
			facts := roomEntityFacts{indexed: make(map[string]struct{})}
			for _, c := range collectStringsInArg(entityText, "indices") {
				facts.indexed[c] = struct{}{}
			}
			for _, c := range collectStringsInArg(entityText, "primaryKeys") {
				facts.indexed[c] = struct{}{}
			}
			if ctor, _ := file.FlatFindChild(idx, "primary_constructor"); ctor != 0 {
				for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
					p := file.FlatNamedChild(ctor, i)
					if p == 0 || file.FlatType(p) != "class_parameter" {
						continue
					}
					if hasAnnotationFlat(file, p, "PrimaryKey") {
						if pname := extractIdentifierFlat(file, p); pname != "" {
							facts.indexed[pname] = struct{}{}
						}
					}
				}
			}
			entities[name] = facts
		})
	}
	if len(entities) == 0 {
		return
	}

	for _, file := range index.Files {
		if file == nil || file.FlatTree == nil {
			continue
		}
		if !bytes.Contains(file.Content, []byte("Relation")) {
			continue
		}
		check := func(node uint32) {
			relText := findAnnotationTextFlat(file, node, "Relation")
			if relText == "" {
				return
			}
			entityCol := extractAnnotationStringNamedArg(relText, "entityColumn")
			if entityCol == "" {
				return
			}
			entityName := extractAnnotationClassNamedArg(relText, "entity")
			if entityName == "" {
				entityName = innerEntityTypeName(explicitTypeTextFlat(file, node))
			}
			if entityName == "" {
				return
			}
			facts, ok := entities[entityName]
			if !ok {
				return
			}
			if _, ok := facts.indexed[entityCol]; ok {
				return
			}
			ctx.Emit(r.Finding(
				file,
				file.FlatRow(node)+1,
				file.FlatCol(node)+1,
				fmt.Sprintf("@Relation(entityColumn = %q) targets entity '%s' which has no Index for that column; add Index(%q) to the @Entity indices.", entityCol, entityName, entityCol),
			))
		}
		file.FlatWalkNodes(0, "class_parameter", check)
		file.FlatWalkNodes(0, "property_declaration", check)
	}
}

// namedArgValueStart returns the substring of annotationText starting at the
// value of `argName = ...`, with leading whitespace trimmed. Returns "" if the
// named argument is not present as a standalone identifier followed by `=`.
func namedArgValueStart(annotationText, argName string) string {
	rest := annotationText
	for {
		i := strings.Index(rest, argName)
		if i < 0 {
			return ""
		}
		if i > 0 {
			prev := rest[i-1]
			if prev == '_' || prev == '.' || (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') {
				rest = rest[i+len(argName):]
				continue
			}
		}
		rest = rest[i+len(argName):]
		break
	}
	rest = strings.TrimLeft(rest, " \t\n\r")
	if !strings.HasPrefix(rest, "=") {
		return ""
	}
	return strings.TrimLeft(rest[1:], " \t\n\r")
}

// collectStringsInArg returns all double-quoted string literals inside the
// bracketed value of a named annotation argument, e.g. extracting "userId"
// from `indices = [Index("userId")]`. Returns an empty slice if the named
// argument is absent or its value is not bracket/paren-delimited.
func collectStringsInArg(annotationText, argName string) []string {
	rest := namedArgValueStart(annotationText, argName)
	if rest == "" {
		return nil
	}
	if strings.HasPrefix(rest, "arrayOf") {
		p := strings.Index(rest, "(")
		if p < 0 {
			return nil
		}
		rest = rest[p:]
	}
	if len(rest) == 0 {
		return nil
	}
	var open, closeCh byte
	switch rest[0] {
	case '[':
		open, closeCh = '[', ']'
	case '(':
		open, closeCh = '(', ')'
	default:
		return nil
	}
	depth := 0
	end := -1
	for j := 0; j < len(rest); j++ {
		c := rest[j]
		if c == open {
			depth++
		} else if c == closeCh {
			depth--
			if depth == 0 {
				end = j
				break
			}
		}
	}
	if end < 0 {
		return nil
	}
	section := rest[1:end]
	var out []string
	for j := 0; j < len(section); j++ {
		if section[j] != '"' {
			continue
		}
		k := j + 1
		var b strings.Builder
		for k < len(section) {
			c := section[k]
			if c == '\\' && k+1 < len(section) {
				b.WriteByte(section[k+1])
				k += 2
				continue
			}
			if c == '"' {
				break
			}
			b.WriteByte(c)
			k++
		}
		out = append(out, b.String())
		j = k
	}
	return out
}

// extractAnnotationStringNamedArg returns the string-literal value of a named
// annotation argument like `entityColumn = "userId"`.
func extractAnnotationStringNamedArg(annotationText, argName string) string {
	rest := namedArgValueStart(annotationText, argName)
	if !strings.HasPrefix(rest, "\"") {
		return ""
	}
	rest = rest[1:]
	var b strings.Builder
	for k := 0; k < len(rest); k++ {
		c := rest[k]
		if c == '\\' && k+1 < len(rest) {
			b.WriteByte(rest[k+1])
			k++
			continue
		}
		if c == '"' {
			return b.String()
		}
		b.WriteByte(c)
	}
	return ""
}

// extractAnnotationClassNamedArg returns the class name from a `name = X::class`
// argument, with any qualifier prefix stripped.
func extractAnnotationClassNamedArg(annotationText, argName string) string {
	rest := namedArgValueStart(annotationText, argName)
	if rest == "" {
		return ""
	}
	end := strings.Index(rest, "::class")
	if end < 0 {
		return ""
	}
	expr := strings.TrimSpace(rest[:end])
	if dot := strings.LastIndex(expr, "."); dot >= 0 {
		expr = expr[dot+1:]
	}
	return expr
}

// innerEntityTypeName extracts the element type name from a property type text
// like `List<Post>` or `Post?`, returning the bare entity identifier.
func innerEntityTypeName(typeText string) string {
	t := strings.TrimSpace(typeText)
	t = strings.TrimSuffix(t, "?")
	for strings.Contains(t, "<") {
		i := strings.Index(t, "<")
		j := strings.LastIndex(t, ">")
		if j <= i {
			break
		}
		inner := t[i+1 : j]
		if c := strings.LastIndex(inner, ","); c >= 0 {
			inner = inner[c+1:]
		}
		t = strings.TrimSpace(inner)
		t = strings.TrimSuffix(t, "?")
	}
	if dot := strings.LastIndex(t, "."); dot >= 0 {
		t = t[dot+1:]
	}
	return t
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
