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

func unusedParameterHasReferenceFlat(file *scanner.File, body uint32, paramIdx uint32, paramName string) bool {
	if file == nil || body == 0 || paramIdx == 0 || paramName == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(body, func(candidate uint32) {
		if found {
			return
		}
		refName := unusedParameterReferenceNameFlat(file, candidate)
		if refName != paramName {
			return
		}
		if file.FlatType(candidate) == "simple_identifier" &&
			!unusedParameterIdentifierIsReferenceFlat(file, candidate) {
			return
		}
		if unusedParameterInsideNestedFunctionFlat(file, candidate, body) {
			return
		}
		if unusedParameterReferenceShadowedFlat(file, body, candidate, paramName) {
			return
		}
		found = true
	})
	return found
}

func unusedParameterReferenceNameFlat(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "simple_identifier":
		return file.FlatNodeText(idx)
	case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "$")
		text = strings.TrimPrefix(text, "{")
		text = strings.TrimSuffix(text, "}")
		return text
	default:
		return ""
	}
}

func unusedParameterIdentifierIsReferenceFlat(file *scanner.File, ident uint32) bool {
	parent, ok := file.FlatParent(ident)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "parameter", "class_parameter", "variable_declaration",
		"function_declaration", "class_declaration", "object_declaration",
		"type_identifier", "user_type", "nullable_type", "function_type",
		"type_parameter", "type_parameters", "function_value_parameters",
		"lambda_parameters", "value_argument_label", "import_header",
		"package_header", "navigation_suffix":
		return false
	case "value_argument":
		if file.FlatNamedChildCount(parent) >= 2 && file.FlatNamedChild(parent, 0) == ident {
			return false
		}
	}
	for cur, ok := file.FlatParent(ident); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "user_type", "nullable_type", "function_type", "type_parameter", "type_parameters":
			return false
		case "function_body", "lambda_literal", "function_declaration":
			return true
		}
	}
	return true
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

// countIdentifierOccurrences counts word-boundary occurrences of name in s.
// A match requires that the character before and after the match is not
// part of an identifier.
func countIdentifierOccurrences(s, name string) int {
	if name == "" || len(s) < len(name) {
		return 0
	}
	isIdent := func(b byte) bool {
		return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	}
	count := 0
	for i := 0; i+len(name) <= len(s); {
		j := strings.Index(s[i:], name)
		if j < 0 {
			break
		}
		pos := i + j
		start := pos
		end := pos + len(name)
		beforeOK := start == 0 || !isIdent(s[start-1])
		afterOK := end == len(s) || !isIdent(s[end])
		if beforeOK && afterOK {
			count++
		}
		i = end
	}
	return count
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
