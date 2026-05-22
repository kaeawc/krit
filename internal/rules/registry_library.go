package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
				if !file.FlatHasModifier(idx, "data") {
					return
				}
				if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "internal") || file.FlatHasModifier(idx, "protected") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Public data class '%s' exposes its properties as part of the API. Consider using a regular class.", name))
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
				if !ok || file.FlatType(parent) != "source_file" {
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
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Public %s '%s' could be made internal in library code.", kind, name))
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
				if !ok || file.FlatType(parent) != "source_file" {
					return
				}
				if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "internal") || file.FlatHasModifier(idx, "protected") {
					return
				}
				if hasExplicitTypeFlat(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				kind := strings.TrimSuffix(file.FlatType(idx), "_declaration")
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Public %s '%s' has no explicit return type. Add an explicit type to the public API.", kind, name))
			},
		})
	}
}
