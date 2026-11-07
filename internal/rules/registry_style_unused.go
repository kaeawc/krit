package rules

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleUnusedRules() {

	// --- from style_unused.go ---
	{
		r := &UnusedImportRule{BaseRule: BaseRule{RuleName: "UnusedImport", RuleSetName: "style", Sev: "warning", Desc: "Detects import statements where the imported name is not referenced in the file."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"import_header"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Tags: []string{"precompile"},
			Check: func(ctx *api.Context) {
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
				f := r.Finding(file, importLine, file.FlatCol(idx)+1,
					fmt.Sprintf("Unused import '%s'.", shortName))
				startByte := int(file.FlatStartByte(idx))
				endByte := int(file.FlatEndByte(idx))
				// The node can swallow a following KDoc; imports are one line.
				if nl := bytes.IndexByte(file.Content[startByte:endByte], '\n'); nl >= 0 {
					endByte = startByte + nl
					if endByte > startByte && file.Content[endByte-1] == '\r' {
						endByte--
					}
				}
				if endByte < len(file.Content) && file.Content[endByte] == '\r' {
					endByte++
				}
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixNone, Implementation: r,
			Tags: []string{"precompile"},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
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
				if unusedParameterInMisparsedDeclarationFlat(file, idx) {
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
				if unusedParameterBodyIsStub(file, summary.body) {
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
					if parameterHasUnusedSuppression(file, param.idx) {
						continue
					}
					used := false
					unknown := false
					callable := param.isFunctionType || unusedParameterTypeMayBeFunctionAlias(file, param.idx)
					if summary.paramsNode != 0 {
						used, unknown = unusedParameterUsageFlat(file, summary.paramsNode, param.idx, paramName, callable)
					}
					if !used && !unknown {
						used, unknown = unusedParameterUsageFlat(file, summary.body, param.idx, paramName, callable)
					}
					// Error recovery can strand part of the body in ERROR children of the function.
					for c := file.FlatFirstChild(idx); !used && !unknown && c != 0; c = file.FlatNextSib(c) {
						if file.FlatType(c) == "ERROR" {
							used, unknown = unusedParameterUsageFlat(file, c, param.idx, paramName, callable)
						}
					}
					if !used && !unknown && unusedParameterStrandedSiblingsMentionName(file, idx, paramName) {
						unknown = true
					}
					if unknown {
						continue
					}
					if !used {
						msg := fmt.Sprintf("Parameter '%s' is unused.", paramName)
						if unusedParameterShadowedByLambdaParameter(file, summary.body, paramName) {
							msg += " A lambda parameter of the same name shadows it; its uses refer to the lambda parameter."
						}
						ctx.EmitAt(file.FlatRow(param.idx)+1, file.FlatCol(param.idx)+1, msg)
					}
				}
			},
		})
	}
	{
		r := &UnusedVariableRule{BaseRule: BaseRule{RuleName: "UnusedVariable", RuleSetName: "style", Sev: "warning", Desc: "Detects local variables that are declared but never used."}, AllowedNames: regexp.MustCompile(`^(ignored|_)$`)}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "variable_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Tags: []string{"precompile"},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
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
					ctx.EmitAtNode(target.nameNode, fmt.Sprintf("Local variable '%s' is never used.", target.name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateClassRule{BaseRule: BaseRule{RuleName: "UnusedPrivateClass", RuleSetName: "style", Sev: "warning", Desc: "Detects private classes that are never referenced in the file."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Private class %s is unused.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateFunctionRule{BaseRule: BaseRule{RuleName: "UnusedPrivateFunction", RuleSetName: "style", Sev: "warning", Desc: "Detects private functions that are never called in the file."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
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
				if privateOperatorFunctionHasSymbolUse(file, idx, name) {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Private function `%s` is unused.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivatePropertyRule{BaseRule: BaseRule{RuleName: "UnusedPrivateProperty", RuleSetName: "style", Sev: "warning", Desc: "Detects private properties that are never referenced in the file."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "class_parameter"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "class_parameter" {
					checkUnusedConstructorParameter(ctx, r, idx)
					return
				}
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				// Error recovery can yield property nodes for class headers; real ones have val/var.
				if !propertyDeclarationHasValVar(file, idx) {
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
				// Deleting the property would also delete the initializer's side effect.
				if unusedPropertyInitializerHasSideEffects(file, idx) {
					return
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Private property `%s` is unused.", name))
				}
			},
		})
	}
	{
		r := &UnusedPrivateMemberRule{BaseRule: BaseRule{RuleName: "UnusedPrivateMember", RuleSetName: "style", Sev: "warning", Desc: "Detects private members (classes, functions, properties) that are never used."}, IgnoreAnnotated: DefaultUnusedMemberIgnoreAnnotated}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "function_declaration", "property_declaration"}, Confidence: api.ConfidenceMedium, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !file.FlatHasModifier(idx, "private") {
					return
				}
				if file.FlatType(idx) == "property_declaration" && !propertyDeclarationHasValVar(file, idx) {
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
				// Apply the same exemptions as UnusedPrivateFunction/UnusedPrivateProperty.
				switch file.FlatType(idx) {
				case "function_declaration":
					if scanner.IsTestFile(file.Path) ||
						flatHasEntryPointAnnotation(file, idx) ||
						privateOperatorFunctionHasSymbolUse(file, idx, name) {
						return
					}
				case "property_declaration":
					if flatHasFrameworkAnnotation(file, idx) {
						return
					}
					if name == "TAG" && propertyInitializerCallCalleeName(file, idx) == "tag" {
						return
					}
					if unusedPropertyInitializerHasSideEffects(file, idx) {
						return
					}
				}
				if !fileHasReferenceNameOutsideNode(file, name, idx) {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						fmt.Sprintf("Private member '%s' is never used.", name))
				}
			},
		})
	}
}

// checkUnusedConstructorParameter reports a private constructor property, or a plain primary-constructor parameter,
// that nothing in the file refers to. detekt reports both as unused private properties.
func checkUnusedConstructorParameter(ctx *api.Context, r *UnusedPrivatePropertyRule, param uint32) {
	file := ctx.File
	ctor, ok := file.FlatParent(param)
	if !ok || file.FlatType(ctor) != "primary_constructor" {
		return
	}
	class, ok := file.FlatParent(ctor)
	if !ok || file.FlatType(class) != "class_declaration" {
		return
	}
	for _, m := range []string{"annotation", "expect", "actual", "data", "value", "inline"} {
		if file.FlatHasModifier(class, m) {
			return
		}
	}
	binding, _ := file.FlatFindChild(param, "binding_pattern_kind")
	isProperty := binding != 0
	if isProperty && !file.FlatHasModifier(param, "private") {
		return
	}
	if flatHasFrameworkAnnotation(file, param) {
		return
	}
	name, nameNode := "", uint32(0)
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "simple_identifier" {
			name, nameNode = file.FlatNodeText(c), c
			break
		}
	}
	if name == "" || (r.AllowedNames != nil && r.AllowedNames.MatchString(name)) {
		return
	}
	if !fileHasReferenceNameOutsideNode(file, name, param) {
		ctx.EmitAtNode(nameNode, fmt.Sprintf("Private property `%s` is unused.", name))
	}
}
