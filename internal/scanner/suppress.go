package scanner

import (
	"bytes"
	"sort"
	"strings"
)

// Suppression represents a range of bytes where specific rules are suppressed.
type Suppression struct {
	StartByte int
	EndByte   int
	Rules     map[string]bool // rule names that are suppressed; nil = suppress all
}

// SuppressionIndex provides O(log n) lookup for whether a finding is suppressed.
type SuppressionIndex struct {
	suppressions []Suppression // sorted by StartByte
}

var suppressionNeedles = [][]byte{
	[]byte("Suppress("),
	[]byte("SuppressWarnings("),
}

// BuildSuppressionIndexFlat walks the flat tree once to find all
// @Suppress/@SuppressWarnings annotations and builds an index of suppressed
// byte ranges.
func BuildSuppressionIndexFlat(tree *FlatTree, content []byte) *SuppressionIndex {
	idx := &SuppressionIndex{
		suppressions: make([]Suppression, 0, 16),
	}
	if tree == nil || len(tree.Nodes) == 0 {
		return idx
	}
	flatWalkForSuppressions(tree, 0, content, idx)
	sort.Slice(idx.suppressions, func(i, j int) bool {
		return idx.suppressions[i].StartByte < idx.suppressions[j].StartByte
	})
	return idx
}

// IsSuppressed checks if a finding at the given byte offset is suppressed for the given rule.
func (idx *SuppressionIndex) IsSuppressed(byteOffset int, ruleName string, ruleSetName string) bool {
	if len(idx.suppressions) == 0 {
		return false
	}

	// Binary search for the rightmost suppression starting at or before byteOffset
	i := sort.Search(len(idx.suppressions), func(i int) bool {
		return idx.suppressions[i].StartByte > byteOffset
	})

	// Check all suppressions that could contain this offset (walk backwards)
	for j := i - 1; j >= 0; j-- {
		s := idx.suppressions[j]
		if s.EndByte < byteOffset {
			break // past this suppression's range
		}
		if byteOffset >= s.StartByte && byteOffset <= s.EndByte {
			if suppressionMatches(s.Rules, ruleName, ruleSetName) {
				return true
			}
		}
	}
	return false
}

// kotlinCompilerWarningAliases maps Kotlin compiler warning identifiers
// (used in `@Suppress("FOO")`) to the krit rule names that detect the
// same issue. Without this, Signal-style `@Suppress("UNUSED_PARAMETER")`
// suppressions would be ignored by krit.
var kotlinCompilerWarningAliases = map[string][]string{
	"UNUSED_PARAMETER":           {"UnusedParameter"},
	"UNUSED_VARIABLE":            {"UnusedVariable"},
	"UNUSED_EXPRESSION":          {"UnusedPrivateProperty"},
	"UNUSED_ANONYMOUS_PARAMETER": {"UnusedParameter"},
	"NAME_SHADOWING":             {"NoNameShadowing"},
	"UNCHECKED_CAST":             {"UnsafeCast"},
	"UNUSED_VALUE":               {"UnusedPrivateProperty"},
	"UNUSED_IMPORT":              {"UnusedImport"},
	// IntelliJ/Kotlin inspection id (lowercase) used by Signal via
	// `@Suppress("unused")`. Covers all unused-declaration krit rules.
	"unused": {"UnusedParameter", "UnusedPrivateProperty", "UnusedPrivateMember", "UnusedPrivateFunction", "UnusedPrivateClass", "UnusedVariable", "UnusedImport"},

	// IntelliJ Kotlin inspection IDs used in `@Suppress(...)` for naming
	// conventions. These pair up with krit's naming rules.
	"EnumEntryName":     {"EnumNaming"},
	"ClassName":         {"ClassNaming"},
	"FunctionName":      {"FunctionNaming"},
	"PropertyName":      {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
	"PrivatePropertyName": {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
	"ObjectPropertyName":  {"ObjectPropertyNaming"},
	"LocalVariableName":   {"VariableNaming"},
	"ConstPropertyName":   {"TopLevelPropertyNaming", "ObjectPropertyNaming"},
}

func suppressionMatches(rules map[string]bool, ruleName string, ruleSetName string) bool {
	if rules == nil {
		return true // suppress all
	}
	if rules[ruleName] || rules["all"] || rules["ALL"] || rules["All"] {
		return true
	}
	if ruleSetName != "" {
		if rules[ruleSetName+"."+ruleName] || rules[ruleSetName+":"+ruleName] {
			return true
		}
	}
	if rules["detekt."+ruleName] || rules["detekt:"+ruleName] {
		return true
	}
	// Check Kotlin compiler warning aliases.
	for alias, mapped := range kotlinCompilerWarningAliases {
		if !rules[alias] {
			continue
		}
		for _, m := range mapped {
			if m == ruleName {
				return true
			}
		}
	}
	return false
}

func flatWalkForSuppressions(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) {
	if tree == nil || int(idx) >= len(tree.Nodes) {
		return
	}
	nodeType := tree.Nodes[idx].TypeName()

	switch nodeType {
	case "prefix_expression", "annotation":
		if flatProcessSuppressionNode(tree, idx, content, out) {
			return
		}
		return
	case "file_annotation":
		flatProcessFileAnnotation(tree, idx, content, out)
		return
	case "source_file", "class_body", "statements", "function_body", "modifiers",
		"class_declaration", "function_declaration", "property_declaration",
		"object_declaration", "companion_object", "secondary_constructor",
		"primary_constructor", "anonymous_initializer", "enum_entry", "lambda_literal",
		"class_member_declarations", "block",
		"control_structure_body", "catch_block", "finally_block",
		"call_expression", "annotated_lambda", "call_suffix",
		"if_expression", "when_expression", "when_entry", "try_expression":
	default:
		return
	}

	for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		switch tree.Nodes[child].TypeName() {
		case "prefix_expression", "annotation":
			flatProcessSuppressionNode(tree, child, content, out)
		case "file_annotation":
			flatProcessFileAnnotation(tree, child, content, out)
		case "modifiers", "source_file", "class_body", "statements", "function_body",
			"class_declaration", "function_declaration", "property_declaration",
			"object_declaration", "companion_object", "secondary_constructor",
			"primary_constructor", "anonymous_initializer", "enum_entry",
			"class_member_declarations", "block",
			"lambda_literal", "control_structure_body", "catch_block", "finally_block",
			"call_expression", "annotated_lambda", "call_suffix",
			"if_expression", "when_expression", "when_entry", "try_expression":
			flatWalkForSuppressions(tree, child, content, out)
		}
	}
}

func flatProcessFileAnnotation(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) {
	textBytes := FlatNodeBytes(tree, idx, content)
	if !hasSuppressionNeedle(textBytes) {
		return
	}
	rules := extractSuppressedRules(string(textBytes))
	target := idx
	for parent := tree.Nodes[target].Parent; parent != 0; parent = tree.Nodes[target].Parent {
		target = parent
	}
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: int(tree.Nodes[target].StartByte),
		EndByte:   int(tree.Nodes[target].EndByte),
		Rules:     rules,
	})
}

func flatProcessSuppressionNode(tree *FlatTree, idx uint32, content []byte, out *SuppressionIndex) bool {
	textBytes := FlatNodeBytes(tree, idx, content)
	if !hasSuppressionNeedle(textBytes) {
		return false
	}
	parseText := textBytes
	if tree.Nodes[idx].TypeName() == "prefix_expression" {
		for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if tree.Nodes[child].TypeName() == "annotation" && flatAnnotationHasArgs(tree, child) {
				parseText = FlatNodeBytes(tree, child, content)
				break
			}
		}
	}
	rules := extractSuppressedRules(string(parseText))
	target := flatFindAnnotationTarget(tree, idx)
	if target == 0 && idx != 0 {
		return true
	}
	out.suppressions = append(out.suppressions, Suppression{
		StartByte: int(tree.Nodes[target].StartByte),
		EndByte:   int(tree.Nodes[target].EndByte),
		Rules:     rules,
	})
	return true
}

func flatAnnotationHasArgs(tree *FlatTree, idx uint32) bool {
	for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
		if tree.Nodes[child].TypeName() == "constructor_invocation" {
			return true
		}
	}
	return false
}

func flatFindAnnotationTarget(tree *FlatTree, idx uint32) uint32 {
	if tree.Nodes[idx].TypeName() == "prefix_expression" {
		var annChild uint32
		hasOperand := false
		for child := tree.Nodes[idx].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if tree.Nodes[child].TypeName() == "annotation" {
				annChild = child
			} else {
				hasOperand = true
			}
		}
		if annChild != 0 && hasOperand && flatAnnotationHasArgs(tree, annChild) {
			return idx
		}
	}

	parent := tree.Nodes[idx].Parent
	if idx == 0 && parent == 0 {
		return 0
	}
	if tree.Nodes[parent].TypeName() == "modifiers" {
		return tree.Nodes[parent].Parent
	}
	parentType := tree.Nodes[parent].TypeName()
	if parentType == "source_file" || parentType == "statements" || parentType == "class_body" {
		foundSelf := false
		for child := tree.Nodes[parent].FirstChild; child != 0; child = tree.Nodes[child].NextSib {
			if child == idx {
				foundSelf = true
				continue
			}
			if !foundSelf {
				continue
			}
			childType := tree.Nodes[child].TypeName()
			if childType == "prefix_expression" || childType == "annotation" {
				continue
			}
			return child
		}
		return parent
	}
	return parent
}

func hasSuppressionNeedle(text []byte) bool {
	for _, needle := range suppressionNeedles {
		if bytes.Contains(text, needle) {
			return true
		}
	}
	return false
}

// extractSuppressedRules parses rule names from @Suppress("Rule1", "Rule2") or @SuppressWarnings("...")
func extractSuppressedRules(text string) map[string]bool {
	// Find the arguments between ( and )
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start < 0 || end <= start {
		return nil
	}
	args := text[start+1 : end]

	rules := make(map[string]bool)
	for _, arg := range strings.Split(args, ",") {
		arg = strings.TrimSpace(arg)
		// Remove quotes
		arg = strings.Trim(arg, "\"'")
		// Remove detekt prefix
		arg = strings.TrimPrefix(arg, "detekt.")
		arg = strings.TrimPrefix(arg, "detekt:")
		if arg != "" {
			rules[arg] = true
		}
	}

	// Special case: "all" suppresses everything
	if rules["all"] || rules["ALL"] || rules["All"] {
		return nil // nil means suppress all
	}

	return rules
}
