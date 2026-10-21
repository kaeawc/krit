package rules

import (
	"bytes"
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

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
func (r *RoomConflictStrategyReplaceOnFkRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule. Detection matches on annotation names and function
// calls without confirming the declared type is a Room DAO or entity.
// Classified per roadmap/17.
func (r *EntityMutableColumnRule) Confidence() float64 { return api.ConfidenceMedium }

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

func (r *RoomConflictStrategyReplaceOnFkRule) check(ctx *api.Context) {
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

// RoomMultipleWritesMissingTransactionRule detects DAO functions whose body
// contains 2+ calls to sibling DAO methods annotated @Insert/@Update/@Delete
// while the enclosing function itself is not @Transaction.
type RoomMultipleWritesMissingTransactionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection is name-based against sibling DAO methods and does not resolve
// types across files.
func (r *RoomMultipleWritesMissingTransactionRule) Confidence() float64 { return api.ConfidenceMedium }

// daoWriteAnnotatedSiblings returns a set of sibling function names in the
// enclosing DAO class body that are annotated @Insert/@Update/@Delete.
func daoWriteAnnotatedSiblings(file *scanner.File, classIdx uint32) map[string]struct{} {
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return nil
	}
	out := make(map[string]struct{})
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if !hasAnnotationFlat(file, child, "Insert") && !hasAnnotationFlat(file, child, "Update") && !hasAnnotationFlat(file, child, "Delete") {
			continue
		}
		name := extractIdentifierFlat(file, child)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

// RoomSuspendQueryInTransactionRule detects DAO functions that are both
// `@Transaction` and `suspend` and call into sibling `suspend @Query`
// functions. Room already wraps suspending queries in a transaction, so the
// outer `@Transaction` double-wraps and breaks cancellation.
type RoomSuspendQueryInTransactionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Database/Room rule.
// Detection is name-based against sibling DAO suspend @Query methods and does
// not resolve types across files.
func (r *RoomSuspendQueryInTransactionRule) Confidence() float64 { return api.ConfidenceMedium }

// daoSuspendQueryAnnotatedSiblings returns sibling function names in the
// enclosing DAO class body that are `suspend` and annotated `@Query`.
func daoSuspendQueryAnnotatedSiblings(file *scanner.File, classIdx uint32) map[string]struct{} {
	body, _ := file.FlatFindChild(classIdx, "class_body")
	if body == 0 {
		return nil
	}
	out := make(map[string]struct{})
	for i := 0; i < file.FlatChildCount(body); i++ {
		child := file.FlatChild(body, i)
		if file.FlatType(child) != "function_declaration" {
			continue
		}
		if !hasAnnotationFlat(file, child, "Query") {
			continue
		}
		if !file.FlatHasModifier(child, "suspend") {
			continue
		}
		name := extractIdentifierFlat(file, child)
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
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
func (r *EntityPrimaryKeyNotStableRule) Confidence() float64 { return api.ConfidenceMedium }

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

// RoomRelationWithoutIndexRule detects @Relation properties whose referenced
// entityColumn is not declared in the target @Entity's indices list.
type RoomRelationWithoutIndexRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection matches on
// annotation names without confirming the resolved type is a Room entity.
func (r *RoomRelationWithoutIndexRule) Confidence() float64 { return api.ConfidenceMedium }

type roomEntityFacts struct {
	indexed map[string]struct{}
}

func (r *RoomRelationWithoutIndexRule) check(ctx *api.Context) {
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
