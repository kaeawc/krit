package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerLibraryRules() {

	// --- from library.go ---
	{
		r := &ForbiddenPublicDataClassRule{BaseRule: BaseRule{RuleName: "ForbiddenPublicDataClass", RuleSetName: "libraries", Sev: "warning", Desc: "Detects public data classes in library code whose exposed properties make the API hard to evolve."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "data") || scanner.IsTestFile(file.Path) {
					return
				}
				if !libraryEffectivelyPublic(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Public data class '%s' exposes its properties as part of the API. Consider using a regular class.", name))
			},
		})
	}
	{
		r := &LibraryEntitiesShouldNotBePublicRule{BaseRule: BaseRule{RuleName: "LibraryEntitiesShouldNotBePublic", RuleSetName: "libraries", Sev: "warning", Desc: "Detects public top-level declarations in library code that could be made internal."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "function_declaration", "property_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				parent, ok := file.FlatParent(idx)
				if !ok || file.FlatType(parent) != "source_file" || scanner.IsTestFile(file.Path) {
					return
				}
				if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "internal") || file.FlatHasModifier(idx, "protected") {
					return
				}
				if hasAnnotationFlat(file, idx, "PublishedApi") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				kind := strings.TrimSuffix(file.FlatType(idx), "_declaration")
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Public %s '%s' could be made internal in library code.", kind, name))
			},
		})
	}
	{
		r := &LibraryCodeMustSpecifyReturnTypeRule{BaseRule: BaseRule{RuleName: "LibraryCodeMustSpecifyReturnType", RuleSetName: "libraries", Sev: "warning", Desc: "Detects public functions and properties in library code without explicit return type annotations."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "property_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				parent, ok := file.FlatParent(idx)
				if !ok || scanner.IsTestFile(file.Path) {
					return
				}
				// Top-level declarations and members of classes; locals have other parents.
				if t := file.FlatType(parent); t != "source_file" && t != "class_body" {
					return
				}
				if !libraryEffectivelyPublic(file, idx) || file.FlatHasModifier(idx, "override") {
					return
				}
				if hasExplicitTypeFlat(file, idx) {
					return
				}
				// A block body with no declared type returns Unit, which explicit API mode accepts.
				if file.FlatType(idx) == "function_declaration" && functionHasBlockBody(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				kind := strings.TrimSuffix(file.FlatType(idx), "_declaration")
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), fmt.Sprintf("Public %s '%s' has no explicit return type. Add an explicit type to the public API.", kind, name))
			},
		})
	}
}

// libraryEffectivelyPublic reports a declaration that is not private, internal or protected itself
// and sits in no enclosing class or object that is.
func libraryEffectivelyPublic(file *scanner.File, idx uint32) bool {
	hidden := func(n uint32) bool {
		return file.FlatHasModifier(n, "private") || file.FlatHasModifier(n, "internal") || file.FlatHasModifier(n, "protected")
	}
	if hidden(idx) {
		return false
	}
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration", "companion_object":
			if hidden(p) {
				return false
			}
		case "function_body", "lambda_literal", "statements":
			return false
		}
	}
	return true
}

func functionHasBlockBody(file *scanner.File, fn uint32) bool {
	body, ok := file.FlatFindChild(fn, "function_body")
	return ok && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(body)), "{")
}
