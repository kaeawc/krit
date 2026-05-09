package typeinfer

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func flatFindIdentifier(file *scanner.File, idx uint32) string {
	if ident := flatFindNamedChildOfType(file, idx, "simple_identifier"); ident != 0 {
		return file.FlatNodeText(ident)
	}
	return ""
}

func forEachFlatNamedChild(file *scanner.File, parent uint32, fn func(child uint32)) {
	if file == nil || file.FlatTree == nil || fn == nil {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		if child := file.FlatNamedChild(parent, i); child != 0 {
			fn(child)
		}
	}
}

func flatForEachRelevantDeclarationChild(file *scanner.File, idx uint32, fn func(child uint32)) {
	if file == nil || file.FlatTree == nil || idx == 0 || fn == nil {
		return
	}
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "property_declaration", "function_declaration", "class_declaration",
			"object_declaration", "type_alias", "class_body", "class_member_declarations",
			"statements", "function_body", "lambda_literal",
			"control_structure_body", "catch_block", "finally_block",
			"primary_constructor", "secondary_constructor",
			"anonymous_initializer":
			fn(child)
		}
	}
}

func flatVariableDeclNameAndType(file *scanner.File, idx uint32) (string, uint32) {
	if file == nil || file.FlatTree == nil || idx == 0 {
		return "", 0
	}
	var name string
	var typeIdx uint32
	for i := 0; i < file.FlatNamedChildCount(idx); i++ {
		child := file.FlatNamedChild(idx, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier":
			if name == "" {
				name = file.FlatNodeText(child)
			}
		case "user_type", "nullable_type", "type_identifier":
			if typeIdx == 0 {
				typeIdx = child
			}
		}
	}
	return name, typeIdx
}

func extractVisibility(text string) string {
	if strings.Contains(text, "private ") {
		return "private"
	}
	if strings.Contains(text, "internal ") {
		return "internal"
	}
	if strings.Contains(text, "protected ") {
		return "protected"
	}
	return "public"
}

type modifierFlags struct {
	visibility string
	override   bool
	abstract   bool
	sealed     bool
	data       bool
	inner      bool
	open       bool
	enum       bool
}

func flatReadModifierFlags(file *scanner.File, idx uint32) modifierFlags {
	flags := modifierFlags{visibility: "public"}
	if file == nil || file.FlatTree == nil || idx == 0 {
		return flags
	}
	// Tree-sitter parses `enum class X` and `sealed class X` with the
	// enum/sealed keyword as a direct child of class_declaration rather
	// than wrapped inside a `modifiers` node. Walk both: the direct
	// children for class-level keywords, and the dedicated `modifiers`
	// subtree for visibility/data/inner/etc.
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if t := file.FlatType(child); t == "enum" || t == "sealed" || t == "data" || t == "inner" || t == "abstract" || t == "open" || t == "private" || t == "internal" || t == "protected" || t == "override" {
			applyModifierText(&flags, file.FlatNodeText(child))
		}
	}
	mods := flatFindNamedChildOfType(file, idx, "modifiers")
	if mods == 0 {
		return flags
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if child == 0 {
			continue
		}
		applyModifierText(&flags, file.FlatNodeText(child))
		for j := 0; j < file.FlatChildCount(child); j++ {
			gc := file.FlatChild(child, j)
			if gc != 0 {
				applyModifierText(&flags, file.FlatNodeText(gc))
			}
		}
	}
	return flags
}

func applyModifierText(flags *modifierFlags, text string) {
	switch text {
	case "private":
		flags.visibility = "private"
	case "internal":
		flags.visibility = "internal"
	case "protected":
		flags.visibility = "protected"
	case "override":
		flags.override = true
	case "abstract":
		flags.abstract = true
	case "sealed":
		flags.sealed = true
	case "data":
		flags.data = true
	case "inner":
		flags.inner = true
	case "open":
		flags.open = true
	case "enum":
		flags.enum = true
	}
}
