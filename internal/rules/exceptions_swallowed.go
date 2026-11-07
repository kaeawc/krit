package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SwallowedExceptionRule ports detekt 1.23.8 SwallowedException: a catch whose exception is never
// referenced, or that throws a new exception built only from the exception's members (`e.message`).
type SwallowedExceptionRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be swallowed
	IgnoredExceptionTypes     []string       // exception types that are allowed to be swallowed
}

// Confidence reports a tier-2 (medium) base confidence. Exceptions rule. Detection matches exception type names and catch/ throw
// shapes via structural AST + name-list lookups. Classified per
// roadmap/17.
func (r *SwallowedExceptionRule) Confidence() float64 { return api.ConfidenceMedium }

const swallowedExceptionMessage = "The caught exception is swallowed. The original exception could be lost."

// swallowedIsReference reports a simple_identifier that is a name reference rather than a declaration.
func swallowedIsReference(file *scanner.File, id uint32) bool {
	p, ok := file.FlatParent(id)
	if !ok {
		return true
	}
	switch file.FlatType(p) {
	case "variable_declaration", "parameter", "class_parameter", "function_declaration", "catch_block":
		return false
	}
	return true
}

// swallowedIsNameReference reports a node that is a KtNameReferenceExpression in the PSI: an identifier use,
// a `$name` template entry, or the type position of `name::class`.
func swallowedIsNameReference(file *scanner.File, n uint32) bool {
	switch file.FlatType(n) {
	case "simple_identifier":
		return swallowedIsReference(file, n)
	case "interpolated_identifier":
		return true
	case "type_identifier":
		p, ok := file.FlatParent(n)
		return ok && file.FlatType(p) == "callable_reference"
	}
	return false
}

// swallowedQualified returns the expression a receiver reference belongs to: the navigation (or the call
// wrapping it) when ref is the receiver of a `.` access, otherwise ref itself.
func swallowedQualified(file *scanner.File, ref uint32) (uint32, bool) {
	p, ok := file.FlatParent(ref)
	if !ok || file.FlatType(p) != "navigation_expression" || file.FlatFirstChild(p) != ref {
		return ref, false
	}
	for c := file.FlatFirstChild(p); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "navigation_suffix" && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(c)), "?.") {
			return ref, false
		}
	}
	// Climb through a chain `e.cause.message` and a trailing call `e.toString()`.
	cur := p
	for {
		pp, ok := file.FlatParent(cur)
		if !ok {
			break
		}
		switch file.FlatType(pp) {
		case "navigation_expression":
			if file.FlatFirstChild(pp) == cur {
				cur = pp
				continue
			}
		case "call_expression":
			if file.FlatFirstChild(pp) == cur {
				cur = pp
				continue
			}
		}
		break
	}
	return cur, true
}

func swallowedParentIsThrow(file *scanner.File, n uint32) bool {
	p, ok := file.FlatParent(n)
	if !ok || file.FlatType(p) != "jump_expression" {
		return false
	}
	return file.FlatType(file.FlatFirstChild(p)) == "throw"
}

func (r *SwallowedExceptionRule) isExceptionUnused(file *scanner.File, catchNode uint32, name string) bool {
	used := false
	seenClose := false
	for c := file.FlatFirstChild(catchNode); c != 0 && !used; c = file.FlatNextSib(c) {
		if file.FlatType(c) == ")" {
			seenClose = true
			continue
		}
		if !seenClose {
			continue
		}
		file.FlatWalkAllNodes(c, func(id uint32) {
			if used || !swallowedIsNameReference(file, id) {
				return
			}
			text := file.FlatNodeText(id)
			if text == name {
				used = true
				return
			}
			for _, t := range r.IgnoredExceptionTypes {
				if text == t {
					used = true
				}
			}
		})
	}
	return !used
}

// swallowedReferences lists what detekt's parameterReferences finds for a throw: the qualified expression of
// each reference to the exception, and the same for the variables the thrown expression reads when their
// initializer references the exception.
func swallowedReferences(file *scanner.File, thrown, catchNode uint32, name string) []uint32 {
	var refs []uint32
	seen := map[string]bool{}
	file.FlatWalkNodes(thrown, "simple_identifier", func(id uint32) {
		if !swallowedIsReference(file, id) {
			return
		}
		text := file.FlatNodeText(id)
		if text == name {
			q, _ := swallowedQualified(file, id)
			refs = append(refs, q)
			return
		}
		if seen[text] {
			return
		}
		seen[text] = true
		file.FlatWalkNodes(catchNode, "property_declaration", func(prop uint32) {
			if swallowedPropertyDeclarationName(file, prop) != text {
				return
			}
			init := swallowedPropertyInitializer(file, prop)
			if init == 0 {
				return
			}
			file.FlatWalkNodes(init, "simple_identifier", func(inner uint32) {
				if file.FlatNodeText(inner) == name && swallowedIsReference(file, inner) {
					q, _ := swallowedQualified(file, inner)
					refs = append(refs, q)
				}
			})
		})
	})
	return refs
}

func (r *SwallowedExceptionRule) isExceptionSwallowed(file *scanner.File, catchNode uint32, name string) bool {
	swallowed := false
	file.FlatWalkNodes(catchNode, "jump_expression", func(j uint32) {
		if swallowed || file.FlatType(file.FlatFirstChild(j)) != "throw" {
			return
		}
		var thrown uint32
		for c := file.FlatNextSib(file.FlatFirstChild(j)); c != 0; c = file.FlatNextSib(c) {
			if file.FlatIsNamed(c) {
				thrown = c
				break
			}
		}
		if thrown == 0 {
			return
		}
		refs := swallowedReferences(file, thrown, catchNode, name)
		if len(refs) == 0 {
			return
		}
		for _, ref := range refs {
			t := file.FlatType(ref)
			if (t != "navigation_expression" && t != "call_expression") || swallowedParentIsThrow(file, ref) {
				return
			}
		}
		swallowed = true
	})
	return swallowed
}

func (r *SwallowedExceptionRule) checkSwallowedException(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	name := extractCaughtVarNameFlat(file, idx)
	if name == "" {
		return
	}
	typeText := ""
	sawColon := false
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == ":" {
			sawColon = true
		} else if sawColon && file.FlatIsNamed(c) {
			typeText = file.FlatNodeText(c)
			break
		}
	}
	for _, ignored := range r.IgnoredExceptionTypes {
		ignored = strings.TrimSuffix(strings.TrimPrefix(ignored, "*"), "*")
		if strings.Contains(strings.ToLower(typeText), strings.ToLower(ignored)) {
			return
		}
	}
	unused := r.isExceptionUnused(file, idx, name)
	if !unused && !r.isExceptionSwallowed(file, idx, name) {
		return
	}
	if r.AllowedExceptionNameRegex != nil && r.AllowedExceptionNameRegex.MatchString(name) {
		return
	}
	f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, swallowedExceptionMessage)
	if unused {
		// Rethrowing the exception is only a fix when the body never touched it.
		f.Fix = catchRethrowFix(file, idx, name)
	}
	ctx.Emit(f)
}
