package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// bindsInsteadOfProvidesHostIsConcrete reports whether the function's module is an object or a concrete class,
// where @Binds cannot be declared in place.
func bindsInsteadOfProvidesHostIsConcrete(file *scanner.File, fn uint32) bool {
	for cur, ok := file.FlatParent(fn); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "object_declaration":
			// A companion object can sit beside @Binds in its abstract or interface outer module.
			if !file.FlatHasModifier(cur, "companion") {
				return true
			}
		case "class_declaration":
			if file.FlatHasModifier(cur, "abstract") {
				return false
			}
			for child := file.FlatFirstChild(cur); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "interface" {
					return false
				}
			}
			return true
		}
	}
	return false
}

// bindsInsteadOfProvidesInCompanion reports a function declared directly in a companion object, where @Binds is not allowed.
func bindsInsteadOfProvidesInCompanion(file *scanner.File, fn uint32) bool {
	for cur, ok := file.FlatParent(fn); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "companion_object":
			return true
		case "object_declaration":
			return file.FlatHasModifier(cur, "companion")
		case "class_declaration":
			return false
		}
	}
	return false
}

// bindsInsteadOfProvidesChangesQualifierOrScope reports whether the @Provides
// function carries a scope or an annotation that its parameter does not share,
// so the pass-through is not a plain alias.
func bindsInsteadOfProvidesChangesQualifierOrScope(file *scanner.File, fn uint32) bool {
	if firstScopeAnnotationFlat(file, fn) != "" {
		return true
	}
	fnAnns := bindsInsteadOfProvidesAnnotationNames(file, fn)
	delete(fnAnns, "Provides")
	params, ok := file.FlatFindChild(fn, "function_value_parameters")
	if !ok {
		return false
	}
	paramAnns := map[string]bool{}
	for name := range bindsInsteadOfProvidesAnnotationNames(file, params) {
		paramAnns[name] = true
	}
	if len(fnAnns) != len(paramAnns) {
		return true
	}
	for name := range fnAnns {
		if !paramAnns[name] {
			return true
		}
	}
	return false
}

func bindsInsteadOfProvidesAnnotationNames(file *scanner.File, idx uint32) map[string]bool {
	names := map[string]bool{}
	var visit func(n uint32, depth int)
	visit = func(n uint32, depth int) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "annotation":
				text := strings.TrimPrefix(strings.TrimSpace(file.FlatNodeText(c)), "@")
				text = strings.TrimPrefix(text, "param:")
				if i := strings.IndexAny(text, "( \t\r\n"); i >= 0 {
					text = text[:i]
				}
				names[text] = true
			case "modifiers", "parameter_modifiers":
				if depth < 2 {
					visit(c, depth+1)
				}
			}
		}
	}
	visit(idx, 0)
	return names
}
