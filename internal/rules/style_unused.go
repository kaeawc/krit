package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// UnusedImportRule detects import statements where the imported name is not used.
type UnusedImportRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedImportRule) Confidence() float64 { return 0.75 }

// UnusedParameterRule detects function parameters that are never used in the body.
type UnusedParameterRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-1 (high) base confidence. Parameter usage is
// detected from reference-shaped AST identifiers with local shadowing handled
// structurally, so comments, strings, and substring collisions do not count as
// usage.
func (r *UnusedParameterRule) Confidence() float64 { return 0.95 }

type unusedParameterReferenceMatch uint8

const (
	unusedParameterNoMatch unusedParameterReferenceMatch = iota
	unusedParameterMatchesParam
	unusedParameterMatchesOtherDeclaration
	unusedParameterUnknownMatch
)

func unusedParameterUsageFlat(file *scanner.File, body uint32, paramIdx uint32, paramName string) (used bool, unknown bool) {
	if file == nil || body == 0 || paramIdx == 0 || paramName == "" {
		return false, false
	}
	for _, nodeType := range []string{"simple_identifier", "interpolated_identifier", "line_str_ref", "multi_line_str_ref"} {
		file.FlatWalkNodes(body, nodeType, func(candidate uint32) {
			if used || unknown {
				return
			}
			switch unusedParameterResolveReferenceFlat(file, body, paramIdx, candidate, paramName) {
			case unusedParameterMatchesParam:
				used = true
			case unusedParameterUnknownMatch:
				unknown = true
			}
		})
		if used || unknown {
			break
		}
	}
	return used, unknown
}

func unusedParameterResolveReferenceFlat(file *scanner.File, body uint32, paramIdx uint32, ref uint32, paramName string) unusedParameterReferenceMatch {
	if unusedParameterReferenceNameFlat(file, ref) != paramName {
		return unusedParameterNoMatch
	}
	if unusedParameterInsideNestedFunctionFlat(file, ref, body) {
		return unusedParameterMatchesOtherDeclaration
	}
	if file.FlatType(ref) == "simple_identifier" {
		reference, known := unusedParameterIdentifierIsReferenceFlat(file, ref)
		if !known {
			return unusedParameterUnknownMatch
		}
		if !reference {
			return unusedParameterNoMatch
		}
	}
	if unusedParameterReferenceShadowedFlat(file, body, ref, paramName) {
		return unusedParameterMatchesOtherDeclaration
	}
	if unusedParameterSameFileDeclarationMatchFlat(file, body, paramIdx, ref, paramName) {
		return unusedParameterMatchesParam
	}
	return unusedParameterUnknownMatch
}

func unusedParameterReferenceNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "simple_identifier":
		return strings.Trim(file.FlatNodeText(idx), "`")
	case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "$")
		text = strings.TrimPrefix(text, "{")
		text = strings.TrimSuffix(text, "}")
		return strings.Trim(text, "`")
	default:
		return ""
	}
}

func unusedParameterIdentifierIsReferenceFlat(file *scanner.File, ident uint32) (isReference bool, known bool) {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return false, false
	}
	switch file.FlatType(parent) {
	case "parameter", "class_parameter", "variable_declaration",
		"function_declaration", "class_declaration", "object_declaration",
		"type_identifier", "user_type", "nullable_type", "function_type",
		"type_parameter", "type_parameters", "function_value_parameters",
		"lambda_parameters", "value_argument_label", "import_header",
		"package_header", "navigation_suffix":
		return false, true
	case "value_argument":
		if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == ident {
			return false, true
		}
	}
	for cur, ok := file.FlatParent(ident); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "user_type", "nullable_type", "function_type", "type_parameter", "type_parameters":
			return false, true
		case "function_body", "lambda_literal", "function_declaration":
			return true, true
		}
	}
	return false, false
}

func unusedParameterInsideNestedFunctionFlat(file *scanner.File, ident uint32, body uint32) bool {
	for cur, ok := file.FlatParent(ident); ok && cur != body; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "anonymous_function":
			return true
		}
	}
	return false
}

func unusedParameterReferenceShadowedFlat(file *scanner.File, body uint32, ref uint32, name string) bool {
	for cur, ok := file.FlatParent(ref); ok && cur != body; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if unusedParameterLambdaDeclaresNameFlat(file, cur, name) {
				return true
			}
		case "for_statement":
			if unusedParameterForDeclaresNameFlat(file, cur, name) {
				return true
			}
		case "catch_block", "catch_clause":
			if unusedParameterCatchDeclaresNameFlat(file, cur, name) {
				return true
			}
		}
	}
	return unusedParameterPriorLocalDeclarationShadowsFlat(file, body, ref, name)
}

func unusedParameterLambdaDeclaresNameFlat(file *scanner.File, lambda uint32, name string) bool {
	params, ok := file.FlatFindChild(lambda, "lambda_parameters")
	if !ok {
		return name == "it"
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(child, name) {
				return true
			}
		case "variable_declaration":
			if extractIdentifierFlat(file, child) == name {
				return true
			}
		}
	}
	return false
}

func unusedParameterForDeclaresNameFlat(file *scanner.File, forStmt uint32, name string) bool {
	for child := file.FlatFirstChild(forStmt); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			if file.FlatNodeTextEquals(child, name) {
				return true
			}
		case "variable_declaration", "multi_variable_declaration":
			if unusedParameterDeclarationNodeHasNameFlat(file, child, name) {
				return true
			}
		}
	}
	return false
}

func unusedParameterCatchDeclaresNameFlat(file *scanner.File, catchNode uint32, name string) bool {
	for child := file.FlatFirstChild(catchNode); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "parameter" && extractIdentifierFlat(file, child) == name {
			return true
		}
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeTextEquals(child, name) {
			return true
		}
	}
	return false
}

func unusedParameterPriorLocalDeclarationShadowsFlat(file *scanner.File, body uint32, ref uint32, name string) bool {
	refStart := file.FlatStartByte(ref)
	shadowed := false
	file.FlatWalkAllNodes(body, func(candidate uint32) {
		if shadowed || file.FlatStartByte(candidate) >= refStart {
			return
		}
		switch file.FlatType(candidate) {
		case "property_declaration":
			if unusedParameterNodeContainsFlat(file, candidate, ref) {
				return
			}
			if unusedParameterDeclarationNodeHasNameFlat(file, candidate, name) &&
				unusedParameterDeclarationScopeContainsRefFlat(file, body, candidate, ref) {
				shadowed = true
			}
		case "function_declaration":
			if extractIdentifierFlat(file, candidate) == name &&
				unusedParameterDeclarationScopeContainsRefFlat(file, body, candidate, ref) {
				shadowed = true
			}
		}
	})
	return shadowed
}

func unusedParameterDeclarationNodeHasNameFlat(file *scanner.File, node uint32, name string) bool {
	if extractIdentifierFlat(file, node) == name {
		return true
	}
	found := false
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		if found || file.FlatType(candidate) != "variable_declaration" {
			return
		}
		if extractIdentifierFlat(file, candidate) == name {
			found = true
		}
	})
	return found
}

func unusedParameterDeclarationScopeContainsRefFlat(file *scanner.File, stop uint32, decl uint32, ref uint32) bool {
	scope := unusedParameterLexicalScopeFlat(file, stop, decl)
	return scope != 0 && unusedParameterNodeContainsFlat(file, scope, ref)
}

func unusedParameterSameFileDeclarationMatchFlat(file *scanner.File, body uint32, paramIdx uint32, ref uint32, name string) bool {
	return extractIdentifierFlat(file, paramIdx) == name &&
		unusedParameterNodeContainsFlat(file, body, ref) &&
		!unusedParameterReferenceShadowedFlat(file, body, ref, name)
}

func unusedParameterLexicalScopeFlat(file *scanner.File, stop uint32, node uint32) uint32 {
	for cur, ok := file.FlatParent(node); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "statements", "function_body", "control_structure_body",
			"lambda_literal", "catch_block", "catch_clause", "for_statement":
			return cur
		}
		if cur == stop {
			return cur
		}
	}
	return 0
}

func unusedParameterNodeContainsFlat(file *scanner.File, outer uint32, inner uint32) bool {
	return file.FlatStartByte(outer) <= file.FlatStartByte(inner) &&
		file.FlatEndByte(inner) <= file.FlatEndByte(outer)
}

func hasSiblingOverloadFlat(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	parent, ok := file.FlatParent(idx)
	for ok && file.FlatType(parent) != "class_body" && file.FlatType(parent) != "source_file" &&
		file.FlatType(parent) != "class_member_declarations" {
		parent, ok = file.FlatParent(parent)
	}
	if !ok {
		return false
	}
	// Linear sibling walk via FirstChild/NextSib. The previous form used
	// FlatNamedChild(parent, i) in a for-i loop, which is O(k) per call and
	// O(N²) across the iteration. For files with many siblings under one
	// parent (generated code, Dagger modules with lots of @Binds methods)
	// this was a latent quadratic.
	for sib := file.FlatFirstChild(parent); sib != 0; sib = file.FlatNextSib(sib) {
		if !file.FlatIsNamed(sib) || sib == idx {
			continue
		}
		if file.FlatType(sib) != "function_declaration" {
			continue
		}
		if extractIdentifierFlat(file, sib) == name {
			return true
		}
	}
	return false
}

// UnusedVariableRule detects local variables that are never used.
type UnusedVariableRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedVariableRule) Confidence() float64 { return 0.75 }

type unusedVariableDecl struct {
	name     string
	stmt     uint32
	nameNode uint32
	emitNode uint32
	scope    uint32
}

func unusedVariableDeclaration(file *scanner.File, idx uint32) (unusedVariableDecl, bool) {
	var target unusedVariableDecl
	if file == nil || idx == 0 {
		return target, false
	}
	nodeType := file.FlatType(idx)
	stmt := idx
	nameNode := uint32(0)
	switch nodeType {
	case "property_declaration":
		if _, ok := file.FlatFindChild(idx, "multi_variable_declaration"); ok {
			return target, false
		}
		varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
		if varDecl == 0 {
			return target, false
		}
		nameNode, _ = file.FlatFindChild(varDecl, "simple_identifier")
	case "variable_declaration":
		parent, ok := file.FlatParent(idx)
		if !ok || file.FlatType(parent) != "multi_variable_declaration" {
			return target, false
		}
		prop, ok := flatEnclosingAncestor(file, idx, "property_declaration")
		if !ok {
			return target, false
		}
		stmt = prop
		nameNode, _ = file.FlatFindChild(idx, "simple_identifier")
	default:
		return target, false
	}
	if nameNode == 0 {
		return target, false
	}
	name := file.FlatNodeString(nameNode, nil)
	if name == "" {
		return target, false
	}
	if !unusedVariableIsLocalProperty(file, stmt) {
		return target, false
	}
	scope, ok := unusedVariableLexicalScope(file, stmt)
	if !ok {
		return target, false
	}
	return unusedVariableDecl{
		name:     name,
		stmt:     stmt,
		nameNode: nameNode,
		emitNode: idx,
		scope:    scope,
	}, true
}

func unusedVariableIsLocalProperty(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	parentType := file.FlatType(parent)
	if parentType == "source_file" ||
		parentType == "class_body" || parentType == "enum_class_body" ||
		parentType == "companion_object" || parentType == "object_declaration" ||
		parentType == "class_member_declarations" {
		return false
	}
	propLine := file.FlatRow(idx)
	depth := 0
	for i := propLine - 1; i >= 0 && i >= propLine-200; i-- {
		line := file.Lines[i]
		depth += strings.Count(line, "}") - strings.Count(line, "{")
		if depth < 0 {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "companion object") ||
				strings.HasPrefix(trimmed, "object ") ||
				strings.Contains(trimmed, " object ") {
				return false
			}
			break
		}
	}
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		t := file.FlatType(a)
		if t == "delegation_specifier" || t == "explicit_delegation" {
			return false
		}
		if t == "class_body" || t == "enum_class_body" ||
			t == "companion_object" || t == "object_declaration" ||
			t == "class_member_declarations" {
			return false
		}
		if t == "function_body" || t == "function_declaration" ||
			t == "anonymous_function" || t == "source_file" {
			break
		}
	}
	trimmed := strings.TrimSpace(file.FlatNodeText(idx))
	return strings.HasPrefix(trimmed, "val ") || strings.HasPrefix(trimmed, "var ")
}

func unusedVariableLexicalScope(file *scanner.File, stmt uint32) (uint32, bool) {
	scope, ok := file.FlatParent(stmt)
	if !ok {
		return 0, false
	}
	for {
		switch file.FlatType(scope) {
		case "statements", "function_body", "lambda_literal", "control_structure_body", "source_file":
			return scope, true
		}
		next, ok := file.FlatParent(scope)
		if !ok {
			return 0, false
		}
		scope = next
	}
}

func unusedVariableHasReference(file *scanner.File, target unusedVariableDecl) bool {
	found := false
	stmtEnd := file.FlatEndByte(target.stmt)
	file.FlatWalkAllNodes(target.scope, func(candidate uint32) {
		if found || file.FlatStartByte(candidate) <= stmtEnd {
			return
		}
		if !unusedVariableReferenceNameMatches(file, candidate, target.name) {
			return
		}
		if !unusedVariableIsReferenceIdentifier(file, candidate) {
			return
		}
		if unusedVariableCrossesUnsupportedOwner(file, candidate, target.scope) {
			return
		}
		if unusedVariableIsShadowedAtReference(file, target, candidate) {
			return
		}
		found = true
	})
	return found
}

func unusedVariableReferenceNameMatches(file *scanner.File, idx uint32, name string) bool {
	switch file.FlatType(idx) {
	case "simple_identifier", "interpolated_identifier":
		return file.FlatNodeTextEquals(idx, name)
	default:
		return false
	}
}

func unusedVariableIsReferenceIdentifier(file *scanner.File, idx uint32) bool {
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "variable_declaration", "parameter", "function_declaration", "class_declaration",
		"object_declaration", "type_alias", "import_header", "package_header",
		"user_type", "nullable_type", "type_reference", "value_argument_label",
		"navigation_suffix", "annotation", "label":
		return false
	}
	if next, ok := file.FlatNextSibling(idx); ok && file.FlatType(next) == "=" {
		if file.FlatType(parent) == "value_argument" {
			return false
		}
	}
	for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
		switch file.FlatType(a) {
		case "user_type", "nullable_type", "type_reference", "type_arguments",
			"type_projection", "function_type", "receiver_type", "annotation",
			"import_header", "package_header":
			return false
		case "property_declaration", "function_declaration", "lambda_literal",
			"statements", "control_structure_body", "function_body", "source_file":
			return true
		}
	}
	return true
}

func unusedVariableCrossesUnsupportedOwner(file *scanner.File, idx, scope uint32) bool {
	for a, ok := file.FlatParent(idx); ok && a != scope; a, ok = file.FlatParent(a) {
		switch file.FlatType(a) {
		case "class_declaration", "object_declaration", "companion_object",
			"function_declaration", "anonymous_function":
			return true
		}
	}
	return false
}

func unusedVariableIsShadowedAtReference(file *scanner.File, target unusedVariableDecl, ref uint32) bool {
	refStart := file.FlatStartByte(ref)
	shadowed := false
	file.FlatWalkAllNodes(target.scope, func(candidate uint32) {
		if shadowed || candidate == target.stmt || candidate == target.nameNode {
			return
		}
		if unusedVariableNodeContains(file, target.stmt, candidate) {
			return
		}
		if file.FlatStartByte(candidate) <= file.FlatStartByte(target.stmt) ||
			file.FlatStartByte(candidate) >= refStart {
			return
		}
		if !unusedVariableDeclaresName(file, candidate, target.name) {
			return
		}
		candidateStmt := unusedVariableShadowDeclarationStatement(file, candidate)
		if candidateStmt != 0 && unusedVariableNodeContains(file, candidateStmt, ref) {
			return
		}
		shadowScope := unusedVariableShadowScope(file, candidate)
		if shadowScope == 0 || !unusedVariableNodeContains(file, shadowScope, ref) {
			return
		}
		shadowed = true
	})
	return shadowed
}

func unusedVariableDeclaresName(file *scanner.File, idx uint32, name string) bool {
	switch file.FlatType(idx) {
	case "property_declaration":
		if _, ok := file.FlatFindChild(idx, "multi_variable_declaration"); ok {
			return false
		}
		varDecl, _ := file.FlatFindChild(idx, "variable_declaration")
		if varDecl == 0 {
			return false
		}
		ident, _ := file.FlatFindChild(varDecl, "simple_identifier")
		return ident != 0 && file.FlatNodeTextEquals(ident, name)
	case "variable_declaration", "parameter":
		ident, _ := file.FlatFindChild(idx, "simple_identifier")
		return ident != 0 && file.FlatNodeTextEquals(ident, name)
	default:
		return false
	}
}

func unusedVariableShadowDeclarationStatement(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "property_declaration":
		return idx
	case "variable_declaration":
		if prop, ok := flatEnclosingAncestor(file, idx, "property_declaration"); ok {
			return prop
		}
		return idx
	case "parameter":
		return idx
	default:
		return 0
	}
}

func unusedVariableShadowScope(file *scanner.File, idx uint32) uint32 {
	switch file.FlatType(idx) {
	case "property_declaration", "variable_declaration":
		scope, ok := unusedVariableLexicalScope(file, idx)
		if ok {
			return scope
		}
	case "parameter":
		for a, ok := file.FlatParent(idx); ok; a, ok = file.FlatParent(a) {
			switch file.FlatType(a) {
			case "lambda_literal", "function_declaration", "anonymous_function":
				return a
			case "source_file":
				return 0
			}
		}
	}
	return 0
}

func unusedVariableNodeContains(file *scanner.File, ancestor, idx uint32) bool {
	return file.FlatStartByte(ancestor) <= file.FlatStartByte(idx) &&
		file.FlatEndByte(ancestor) >= file.FlatEndByte(idx)
}

// UnusedPrivateClassRule detects private classes that are never referenced.
type UnusedPrivateClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateClassRule) Confidence() float64 { return 0.75 }

// entryPointAnnotationNames lists annotation names that mark a declaration as
// a framework entry point (called via reflection, preview tooling, test
// runners, etc.). Private declarations with these annotations should NOT be
// flagged as unused.
var entryPointAnnotationNames = map[string]bool{
	"Preview":           true, // androidx.compose.ui.tooling.preview.Preview
	"SignalPreview":     true, // Signal-specific preview wrapper
	"ComposePreview":    true,
	"PreviewParameter":  true,
	"PreviewLightDark":  true,
	"DarkPreview":       true,
	"LightPreview":      true,
	"NightPreview":      true,
	"DayPreview":        true,
	"Test":              true, // JUnit @Test
	"ParameterizedTest": true,
	"BeforeEach":        true,
	"AfterEach":         true,
	"BeforeAll":         true,
	"AfterAll":          true,
	"Before":            true,
	"After":             true,
	"BeforeClass":       true,
	"AfterClass":        true,
	"ParameterizedRobolectricTestRunner.Parameters": true,
	"Parameters":    true,
	"Provides":      true, // Dagger
	"Binds":         true,
	"BindsInstance": true,
	"Module":        true,
	"JvmStatic":     true,
	"JvmName":       true,
	"JvmField":      true,
	// Reflection/proguard retention markers.
	"Keep":              true, // androidx.annotation.Keep
	"UsedByReflection":  true,
	"UsedByNative":      true,
	"VisibleForTesting": true, // accessed from test module
	"SerializedName":    true, // Gson/Moshi
	"JsonCreator":       true, // Jackson
	"JsonProperty":      true,
}

func flatAnnotationListContains(parentText string, childText string, name string) bool {
	text := childText
	text = strings.TrimPrefix(text, "@")
	if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
		text = text[:parenIdx]
	}
	if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
		text = text[colonIdx+1:]
	}
	text = strings.TrimSpace(text)
	if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
		text = text[dotIdx+1:]
	}
	return text == name || (parentText == text && text == name)
}

func flatHasAnnotationNamed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 {
		return false
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		for i := 0; i < file.FlatChildCount(mods); i++ {
			child := file.FlatChild(mods, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			if flatAnnotationListContains("", file.FlatNodeText(child), name) {
				return true
			}
		}
	}
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		t := file.FlatType(child)
		if t != "annotation" && t != "modifier" {
			continue
		}
		if flatAnnotationListContains("", file.FlatNodeText(child), name) {
			return true
		}
	}
	return false
}

func flatHasEntryPointAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		text = strings.TrimPrefix(text, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if entryPointAnnotationNames[text] {
			return true
		}
	}
	return false
}

func flatHasFrameworkAnnotation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		raw := file.FlatNodeText(child)
		text := strings.TrimPrefix(raw, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if frameworkAnnotationNames[text] {
			return true
		}
		if text == "Suppress" || text == "SuppressWarnings" {
			if strings.Contains(raw, `"unused"`) ||
				strings.Contains(raw, `"UNUSED_PARAMETER"`) ||
				strings.Contains(raw, `"UNUSED_VARIABLE"`) ||
				strings.Contains(raw, `"UnusedPrivateProperty"`) ||
				strings.Contains(raw, `"UnusedPrivateMember"`) ||
				strings.Contains(raw, `"UnusedPrivateFunction"`) ||
				strings.Contains(raw, `"UnusedVariable"`) {
				return true
			}
		}
	}
	return false
}

// UnusedPrivateFunctionRule detects private functions that are never called.
type UnusedPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateFunctionRule) Confidence() float64 { return 0.75 }

// UnusedPrivatePropertyRule detects private properties that are never referenced.
type UnusedPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames *regexp.Regexp
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivatePropertyRule) Confidence() float64 { return 0.75 }

// UnusedPrivateMemberRule is a combined check for unused private members.
type UnusedPrivateMemberRule struct {
	FlatDispatchBase
	BaseRule
	AllowedNames    *regexp.Regexp
	IgnoreAnnotated []string
}

// Confidence reports a tier-2 (medium) base confidence. Style/unused rule. Detection uses substring presence in the enclosing
// scope body to decide whether a declaration is referenced, which
// false-positives on substring collisions. Classified per roadmap/17.
func (r *UnusedPrivateMemberRule) Confidence() float64 { return 0.75 }
