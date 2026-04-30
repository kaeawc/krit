package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"regexp"
	"strings"
)

func registerStyleUnusedRules() {

	// --- from style_unused.go ---
	{
		r := &UnusedImportRule{BaseRule: BaseRule{RuleName: "UnusedImport", RuleSetName: "style", Sev: "warning", Desc: "Detects import statements where the imported name is not referenced in the file."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: 0.75, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				shortName := importShortNameFlat(file, idx)
				if shortName == "" {
					return
				}
				if unusedImportImplicitlyUsedByKotlin(shortName) {
					return
				}
				importLine := file.FlatRow(idx) + 1
				if r.hasReferenceName(file, shortName) {
					return
				}
				f := r.Finding(file, importLine, 1,
					fmt.Sprintf("Unused import '%s'.", shortName))
				startByte := int(file.FlatStartByte(idx))
				endByte := int(file.FlatEndByte(idx))
				if endByte < len(file.Content) && file.Content[endByte] == '\n' {
					endByte++
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   startByte,
					EndByte:     endByte,
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnusedParameterRule{BaseRule: BaseRule{RuleName: "UnusedParameter", RuleSetName: "style", Sev: "warning", Desc: "Detects function parameters that are never used in the function body."}, AllowedNames: regexp.MustCompile(`^(ignored|expected|_)$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				summary := getFunctionDeclSummaryFlat(file, idx)
				if summary.hasOverride || summary.hasOpen || summary.hasAbstract || summary.hasOperator {
					return
				}
				if file.FlatHasModifier(idx, "actual") ||
					file.FlatHasModifier(idx, "expect") {
					return
				}
				if summary.hasEntryPoint {
					return
				}
				if summary.hasComposable {
					return
				}
				if summary.hasSubscribeLike {
					return
				}
				for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
					if file.FlatType(p) == "class_declaration" {
						for i := 0; i < file.FlatChildCount(p); i++ {
							c := file.FlatChild(p, i)
							if file.FlatType(c) == "interface" || (file.FlatType(c) == "class" && file.FlatNodeTextEquals(c, "interface")) {
								return
							}
						}
						break
					}
				}
				if summary.body == 0 {
					return
				}
				bodyText := file.FlatNodeText(summary.body)
				trimmedBody := strings.TrimSpace(bodyText)
				if trimmedBody == "= Unit" || trimmedBody == "{}" || trimmedBody == "{ }" {
					return
				}
				if strings.Contains(trimmedBody, "throw ") &&
					(strings.HasPrefix(trimmedBody, "{") && strings.Count(trimmedBody, "\n") <= 3) {
					return
				}
				if strings.HasPrefix(trimmedBody, "= throw ") ||
					strings.HasPrefix(trimmedBody, "= TODO(") ||
					strings.HasPrefix(trimmedBody, "= error(") {
					return
				}
				if summary.paramsNode == 0 {
					return
				}
				if hasSiblingOverloadFlat(file, idx, summary.name) {
					return
				}
				for _, param := range summary.params {
					paramName := param.name
					if paramName == "" {
						continue
					}
					if r.AllowedNames != nil && r.AllowedNames.MatchString(paramName) {
						continue
					}
					paramText := file.FlatNodeText(param.idx)
					if strings.Contains(paramText, "@Suppress") &&
						(strings.Contains(paramText, "\"unused\"") ||
							strings.Contains(paramText, "\"UNUSED_PARAMETER\"")) {
						continue
					}
					used := false
					unknown := false
					if summary.paramsNode != 0 {
						used, unknown = unusedParameterUsageFlat(file, summary.paramsNode, param.idx, paramName, param.isFunctionType)
					}
					if !used && !unknown {
						used, unknown = unusedParameterUsageFlat(file, summary.body, param.idx, paramName, param.isFunctionType)
					}
					if unknown {
						continue
					}
					if !used {
						ctx.EmitAt(file.FlatRow(param.idx)+1, 1,
							fmt.Sprintf("Parameter '%s' is unused.", paramName))
					}
				}
			},
		})
	}
	{
		r := &UnusedVariableRule{BaseRule: BaseRule{RuleName: "UnusedVariable", RuleSetName: "style", Sev: "warning", Desc: "Detects local variables that are declared but never used."}, AllowedNames: regexp.MustCompile(`^(ignored|_)$`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "variable_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				target, ok := unusedVariableDeclaration(file, idx)
				if !ok {
					return
				}
				if r.AllowedNames != nil && r.AllowedNames.MatchString(target.name) {
					return
				}
				used, unknown := unusedVariableUsage(file, target)
				if !used && !unknown {
					ctx.EmitAt(file.FlatRow(target.emitNode)+1, 1,
						fmt.Sprintf("Local variable '%s' is never used.", target.name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateClassRule{BaseRule: BaseRule{RuleName: "UnusedPrivateClass", RuleSetName: "style", Sev: "warning", Desc: "Detects private classes that are never referenced in the file."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Private class '%s' is never used.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateFunctionRule{BaseRule: BaseRule{RuleName: "UnusedPrivateFunction", RuleSetName: "style", Sev: "warning", Desc: "Detects private functions that are never called in the file."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
					return
				}
				if flatHasEntryPointAnnotation(file, idx) {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Private function '%s' is never called.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivatePropertyRule{BaseRule: BaseRule{RuleName: "UnusedPrivateProperty", RuleSetName: "style", Sev: "warning", Desc: "Detects private properties that are never referenced in the file."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				if flatHasFrameworkAnnotation(file, idx) {
					return
				}
				name := propertyDeclarationNameFlat(file, idx)
				if name == "" {
					return
				}
				if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
					return
				}
				if name == "TAG" {
					if propertyInitializerCallCalleeName(file, idx) == "tag" {
						return
					}
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Private property '%s' is never used.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateMemberRule{BaseRule: BaseRule{RuleName: "UnusedPrivateMember", RuleSetName: "style", Sev: "warning", Desc: "Detects private members (classes, functions, properties) that are never used."}, IgnoreAnnotated: DefaultUnusedMemberIgnoreAnnotated}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "function_declaration", "property_declaration"}, Confidence: 0.75, Fix: v2.FixSemantic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				mods, _ := file.FlatFindChild(idx, "modifiers")
				modsText := ""
				if mods != 0 {
					modsText = file.FlatNodeText(mods)
				}
				for _, ann := range r.IgnoreAnnotated {
					if strings.Contains(modsText, ann) {
						return
					}
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" && file.FlatType(idx) == "property_declaration" {
					name = propertyDeclarationNameFlat(file, idx)
				}
				if name == "" {
					return
				}
				if r.AllowedNames != nil && r.AllowedNames.MatchString(name) {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						fmt.Sprintf("Private member '%s' is never used.", name))
				}
			},
		})
	}
}
