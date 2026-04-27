package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerDiHygieneRules() {

	// --- from di_hygiene.go ---
	{
		r := &AnvilMergeComponentEmptyScopeRule{BaseRule: BaseRule{RuleName: "AnvilMergeComponentEmptyScope", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @MergeComponent scopes with no matching @ContributesTo or @ContributesBinding declarations."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &AnvilContributesBindingWithoutScopeRule{BaseRule: BaseRule{RuleName: "AnvilContributesBindingWithoutScope", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @ContributesBinding scope mismatches with the @ContributesTo scope on the bound interface."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				bindingScope := anvilAnnotationScopeFlat(file, idx, "ContributesBinding")
				if bindingScope == "" {
					return
				}
				interfaceScopes := anvilContributedInterfaceScopesFlat(file)
				if len(interfaceScopes) == 0 {
					return
				}
				for _, iface := range anvilImplementedTypesFlat(file, idx) {
					if iface == "" {
						continue
					}
					ifaceScope, ok := interfaceScopes[iface]
					if !ok || ifaceScope == "" || ifaceScope == bindingScope {
						continue
					}
					name := extractIdentifierFlat(file, idx)
					if name == "" {
						name = "binding"
					}
					ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@ContributesBinding(%s::class) on '%s' binds '%s', which is only contributed to %s::class.", bindingScope, name, iface, ifaceScope))
					return
				}
			},
		})
	}
	{
		r := &BindsMismatchedArityRule{BaseRule: BaseRule{RuleName: "BindsMismatchedArity", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects @Binds functions that do not declare exactly one parameter as required by Dagger."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !hasAnnotationFlat(file, idx, "Binds") {
					return
				}
				paramCount := 0
				if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
					walkFunctionParametersFlat(file, params, func(_ uint32) {
						paramCount++
					})
				}
				if paramCount == 1 {
					return
				}
				name := extractIdentifierFlat(file, idx)
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("@Binds function '%s' must declare exactly one parameter; found %d.", name, paramCount))
			},
		})
	}
	{
		r := &HiltEntryPointOnNonInterfaceRule{BaseRule: BaseRule{RuleName: "HiltEntryPointOnNonInterface", RuleSetName: diHygieneRuleSet, Sev: "warning", Desc: "Detects Hilt @EntryPoint annotations on classes or objects instead of interfaces."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "prefix_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				kind, name, line, ok := hiltEntryPointDeclarationFlat(file, idx)
				if !ok || kind == "interface" {
					return
				}
				if name == "" {
					name = "entry point"
				}
				ctx.EmitAt(line, 1, fmt.Sprintf("@EntryPoint '%s' must be declared as an interface, not a %s.", name, kind))
			},
		})
	}
}
