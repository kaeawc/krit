package rules

import (
	"fmt"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// check mirrors detekt's RedundantVisibilityModifierRule. An explicit `public` is reported on classes,
// interfaces and enums at any depth, and on functions and properties that are not top-level or `override`;
// objects, constructors, constructor properties, accessors and top-level functions and properties are skipped.
// An `internal` modifier is reported on any declaration whose containing class or object is private or local.
func (r *RedundantVisibilityModifierRule) check(ctx *api.Context) {
	file, mods := ctx.File, ctx.Idx
	decl, ok := file.FlatParent(mods)
	if !ok {
		return
	}
	var public, internal uint32
	for c := file.FlatFirstChild(mods); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "visibility_modifier" {
			continue
		}
		switch file.FlatNodeText(c) {
		case "public":
			public = c
		case "internal":
			internal = c
		}
	}
	if public != 0 {
		r.checkPublic(ctx, decl, public)
	}
	if internal != 0 {
		r.checkInternal(ctx, decl)
	}
}

func (r *RedundantVisibilityModifierRule) checkPublic(ctx *api.Context, decl, public uint32) {
	file := ctx.File
	var message string
	switch file.FlatType(decl) {
	case "class_declaration":
		message = "is explicitly marked as public. Public is the default visibility for classes. The public modifier is redundant."
	case "function_declaration", "property_declaration":
		if file.FlatHasModifier(decl, "override") || isTopLevelDeclaration(file, decl) {
			return
		}
		if file.FlatType(decl) == "function_declaration" {
			message = "is explicitly marked as public. Functions are public by default so this modifier is redundant."
		} else {
			message = "is explicitly marked as public. Properties are public by default so this modifier is redundant."
		}
	default:
		return
	}
	anchor := declAnchor(file, decl)
	f := r.Finding(file, file.FlatRow(anchor)+1, file.FlatCol(anchor)+1,
		fmt.Sprintf("%s %s", extractIdentifierFlat(file, decl), message))
	start := int(file.FlatStartByte(public))
	end := int(file.FlatEndByte(public))
	// Also consume the whitespace after the keyword.
	for end < len(file.Content) && (file.Content[end] == ' ' || file.Content[end] == '\t') {
		end++
	}
	f.Fix = &scanner.Fix{ByteMode: true, StartByte: start, EndByte: end, Replacement: ""}
	ctx.Emit(f)
}

func (r *RedundantVisibilityModifierRule) checkInternal(ctx *api.Context, decl uint32) {
	file := ctx.File
	switch file.FlatType(decl) {
	case "class_declaration", "object_declaration", "companion_object", "function_declaration", "property_declaration",
		"secondary_constructor", "primary_constructor", "class_parameter", "type_alias":
	default:
		// Accessors are declarations detekt does not report.
		return
	}
	owner := containingClassOrObject(file, decl)
	if owner == 0 {
		return
	}
	if file.FlatType(owner) != "object_literal" && !isLocalClassOrObject(file, owner) && !file.FlatHasModifier(owner, "private") {
		return
	}
	start := decl
	if kdoc, ok := flatPrecedingKDoc(file, decl); ok && file.FlatType(kdoc) == "multiline_comment" {
		start = kdoc
	}
	ctx.Emit(r.Finding(file, file.FlatRow(start)+1, file.FlatCol(start)+1,
		fmt.Sprintf("The `internal` modifier on %s is redundant and should be removed.", visibilityDeclName(file, decl, owner))))
}

// visibilityDeclName is the name detekt prints: constructors use their class name and a bare companion is "Companion".
func visibilityDeclName(file *scanner.File, decl, owner uint32) string {
	switch file.FlatType(decl) {
	case "primary_constructor", "secondary_constructor":
		return extractIdentifierFlat(file, owner)
	case "companion_object":
		if name := extractIdentifierFlat(file, decl); name != "" {
			return name
		}
		return "Companion"
	}
	return extractIdentifierFlat(file, decl)
}

// isTopLevelDeclaration reports whether decl is a direct child of the file.
func isTopLevelDeclaration(file *scanner.File, decl uint32) bool {
	p, ok := file.FlatParent(decl)
	return ok && file.FlatType(p) == "source_file"
}

// containingClassOrObject returns the class, object or object literal that directly declares decl, or 0.
// A constructor parameter or primary constructor belongs to the class that owns the constructor.
func containingClassOrObject(file *scanner.File, decl uint32) uint32 {
	p, ok := file.FlatParent(decl)
	if !ok {
		return 0
	}
	if file.FlatType(decl) == "primary_constructor" {
		return classOrObjectNode(file, p)
	}
	switch file.FlatType(p) {
	case "class_body", "enum_class_body":
		if owner, ok := file.FlatParent(p); ok {
			return classOrObjectNode(file, owner)
		}
	case "primary_constructor":
		if owner, ok := file.FlatParent(p); ok {
			return classOrObjectNode(file, owner)
		}
	case "class_declaration":
		return classOrObjectNode(file, p)
	}
	return 0
}

// classOrObjectNode returns n when it declares a class, object, companion or object literal, else 0.
func classOrObjectNode(file *scanner.File, n uint32) uint32 {
	switch file.FlatType(n) {
	case "class_declaration", "object_declaration", "companion_object", "object_literal":
		return n
	}
	return 0
}

// isLocalClassOrObject reports whether a class or object is declared inside a function body or inside another local class.
func isLocalClassOrObject(file *scanner.File, n uint32) bool {
	for {
		p, ok := file.FlatParent(n)
		if !ok {
			return false
		}
		switch file.FlatType(p) {
		case "source_file":
			return false
		case "class_body", "enum_class_body":
			owner, ok := file.FlatParent(p)
			if !ok {
				return false
			}
			switch file.FlatType(owner) {
			case "object_literal":
				return true
			case "class_declaration", "object_declaration", "companion_object", "enum_entry":
				n = owner
				continue
			}
			return true
		}
		return true
	}
}
