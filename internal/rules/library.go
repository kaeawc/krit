package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ForbiddenPublicDataClassRule detects public data classes in library code.
// Data classes expose their properties as part of the API, making them hard to evolve.
type ForbiddenPublicDataClassRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Library-hygiene rule. Detection matches on known library package names
// and API shapes without confirming the actual import target. Classified
// per roadmap/17.
func (r *ForbiddenPublicDataClassRule) Confidence() float64 { return api.ConfidenceMedium }

// LibraryEntitiesShouldNotBePublicRule detects public top-level classes, functions,
// and properties that could be internal in library code.
type LibraryEntitiesShouldNotBePublicRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Library-hygiene rule. Detection matches on known library package names
// and API shapes without confirming the actual import target. Classified
// per roadmap/17.
func (r *LibraryEntitiesShouldNotBePublicRule) Confidence() float64 { return api.ConfidenceMedium }

// LibraryCodeMustSpecifyReturnTypeRule detects public functions and properties
// without explicit return types. In libraries, implicit return types are part of
// the public API and can change unexpectedly.
type LibraryCodeMustSpecifyReturnTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Library-hygiene rule. Detection matches on known library package names
// and API shapes without confirming the actual import target. Classified
// per roadmap/17.
func (r *LibraryCodeMustSpecifyReturnTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// hasExplicitType checks whether a function_declaration or property_declaration
// has an explicit type annotation by looking for a ":" token followed by a type node.
// For functions, the colon and type are direct children of function_declaration.
// For properties, the colon and type are inside a variable_declaration child.
func hasExplicitTypeFlat(file *scanner.File, idx uint32) bool {
	if hasColonTypeFlat(file, idx) {
		return true
	}
	// For property_declaration, check inside variable_declaration
	found := false
	file.FlatForEachChild(idx, func(child uint32) {
		if found || file.FlatType(child) != "variable_declaration" {
			return
		}
		if hasColonTypeFlat(file, child) {
			found = true
		}
	})
	return found
}

// hasColonType checks a node's direct children for a ":" followed by a type node.
func hasColonTypeFlat(file *scanner.File, idx uint32) bool {
	colonSeen := false
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == ":" {
			colonSeen = true
			continue
		}
		if colonSeen {
			ct := file.FlatType(child)
			if ct == "user_type" || ct == "nullable_type" || ct == "function_type" || ct == "parenthesized_type" {
				return true
			}
			break
		}
	}
	return false
}

// hasAnnotationFlat checks whether a node has a specific annotation by exact simple name.
// It checks both child modifiers and the run of annotation siblings directly before the node.
// `@Test` does not match `@TestOnly`, and a use-site target (`@field:Inject`) or qualifier
// (`@javax.inject.Inject`) does not change the name.
func hasAnnotationFlat(file *scanner.File, idx uint32, annotationName string) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "modifiers" && annotationTextListHasName(file.FlatNodeText(child), annotationName) {
			return true
		}
	}
	for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
		prevType := file.FlatType(prev)
		if prevType != "modifiers" && prevType != "annotation" {
			break
		}
		if annotationTextListHasName(file.FlatNodeText(prev), annotationName) {
			return true
		}
	}
	return false
}

// annotationTextListHasName reports whether text, the source of a modifier list or a single annotation,
// contains an annotation whose simple name is name.
func annotationTextListHasName(text, name string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != '@' {
			continue
		}
		rest := text[i+1:]
		// Skip arguments so an annotation mentioned inside another annotation's argument list is ignored.
		end := 0
		for end < len(rest) && (isIdentByte(rest[end]) || rest[end] == '.' || rest[end] == ':') {
			end++
		}
		ref := rest[:end]
		if colon := strings.LastIndex(ref, ":"); colon >= 0 {
			ref = ref[colon+1:]
		}
		if strings.Contains(name, ".") {
			// A dotted name such as `DependencyGraph.Factory` matches itself or a package-qualified form.
			if ref == name || strings.HasSuffix(ref, "."+name) {
				return true
			}
		} else if dot := strings.LastIndex(ref, "."); dot >= 0 {
			ref = ref[dot+1:]
		}
		if ref == name {
			return true
		}
		i += end
		if end < len(rest) && rest[end] == '(' {
			i += 1 + annotationArgumentsLength(rest[end+1:])
		}
	}
	return false
}

// annotationArgumentsLength returns the byte length of an annotation argument list body, up to its closing paren.
func annotationArgumentsLength(rest string) int {
	depth := 1
	inString := false
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; {
		case inString:
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(rest)
}

func extractIdentifierFlat(file *scanner.File, idx uint32) string {
	// Linear sibling walk (FirstChild/NextSib) instead of indexed
	// FlatNamedChild(idx, i) loop which was O(k) per child access and
	// O(N²) across the iteration. This helper is called on essentially
	// every declaration by many rules, so the constant factor matters.
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier", "identifier":
			return file.FlatNodeString(child, nil)
		case "variable_declaration":
			for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeString(gc, nil)
				}
			}
		}
	}
	return ""
}
