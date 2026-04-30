package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"regexp"
	"strings"
)

func registerCommentsRules() {

	// --- from comments.go ---
	{
		r := &AbsentOrWrongFileLicenseRule{BaseRule: BaseRule{RuleName: "AbsentOrWrongFileLicense", RuleSetName: "comments", Sev: "warning", Desc: "Detects files that are missing a valid license header comment."}, LicenseTemplate: "Copyright"}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsLinePass, Fix: v2.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DeprecatedBlockTagRule{BaseRule: BaseRule{RuleName: "DeprecatedBlockTag", RuleSetName: "comments", Sev: "warning", Desc: "Detects @deprecated KDoc tags that should use the @Deprecated annotation instead."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				if !flatIsKDoc(file, idx) {
					return
				}
				text := file.FlatNodeText(idx)
				if strings.Contains(text, "@deprecated") {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Use @Deprecated annotation instead of @deprecated KDoc tag.")
				}
			},
		})
	}
	{
		r := &DocumentationOverPrivateFunctionRule{BaseRule: BaseRule{RuleName: "DocumentationOverPrivateFunction", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc documentation on private functions where it is unnecessary."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPrivateDeclarationFlat(file, idx) {
					return
				}
				if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Private function should not have KDoc documentation.")
					endByte := int(file.FlatEndByte(kdocIdx))
					if endByte < len(file.Content) && file.Content[endByte] == '\n' {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(kdocIdx)),
						EndByte:     endByte,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &DocumentationOverPrivatePropertyRule{BaseRule: BaseRule{RuleName: "DocumentationOverPrivateProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc documentation on private properties where it is unnecessary."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, Fix: v2.FixIdiomatic, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPrivateDeclarationFlat(file, idx) {
					return
				}
				if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
					f := r.Finding(file, file.FlatRow(idx)+1, 1,
						"Private property should not have KDoc documentation.")
					endByte := int(file.FlatEndByte(kdocIdx))
					if endByte < len(file.Content) && file.Content[endByte] == '\n' {
						endByte++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(kdocIdx)),
						EndByte:     endByte,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EndOfSentenceFormatRule{BaseRule: BaseRule{RuleName: "EndOfSentenceFormat", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc first sentences that do not end with proper punctuation."}, Pattern: regexp.MustCompile(`([.?!][ \t\n\r])|([.?!]$)`)}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				if !flatIsKDoc(file, idx) {
					return
				}
				text := flatKdocText(file, idx)
				if text == "" {
					return
				}
				firstLine := strings.SplitN(text, "\n", 2)[0]
				if strings.HasPrefix(firstLine, "@") {
					return
				}
				if !r.Pattern.MatchString(firstLine) {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"KDoc first sentence should end with proper punctuation.")
				}
			},
		})
	}
	{
		r := &KDocReferencesNonPublicPropertyRule{BaseRule: BaseRule{RuleName: "KDocReferencesNonPublicProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc bracket references that point to non-public properties."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !flatIsKDoc(file, idx) {
					return
				}
				text := file.FlatNodeText(idx)
				matches := kdocRefRe.FindAllStringSubmatch(text, -1)
				if len(matches) == 0 {
					return
				}
				nonPublic := make(map[string]bool)
				file.FlatWalkNodes(0, "property_declaration", func(pidx uint32) {
					if isPublicDeclarationFlat(file, pidx) {
						return
					}
					if name := extractIdentifierFlat(file, pidx); name != "" {
						nonPublic[name] = true
					}
				})
				if len(nonPublic) == 0 {
					return
				}
				for _, m := range matches {
					name := m[1]
					if nonPublic[name] {
						ctx.EmitAt(file.FlatRow(idx)+1, 1,
							"KDoc references non-public property \""+name+"\".")
					}
				}
			},
		})
	}
	{
		r := &OutdatedDocumentationRule{BaseRule: BaseRule{RuleName: "OutdatedDocumentation", RuleSetName: "comments", Sev: "warning", Desc: "Detects @param tags in KDoc that do not match the actual function parameters."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				prev, ok := flatPrecedingKDoc(file, idx)
				if !ok {
					return
				}
				kdoc := file.FlatNodeText(prev)
				docParams := paramTagRe.FindAllStringSubmatch(kdoc, -1)
				if len(docParams) == 0 {
					return
				}
				actualParams := make(map[string]bool)
				summary := getFunctionDeclSummaryFlat(file, idx)
				for _, param := range summary.params {
					if param.name != "" {
						actualParams[param.name] = true
					}
				}
				for _, dp := range docParams {
					name := dp[1]
					if !actualParams[name] {
						ctx.EmitAt(file.FlatRow(prev)+1, 1,
							"KDoc @param \""+name+"\" does not match any actual parameter.")
					}
				}
			},
		})
	}
	{
		r := &UndocumentedPublicClassRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicClass", RuleSetName: "comments", Sev: "warning", Desc: "Detects public classes that are missing KDoc documentation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPublicDeclarationFlat(file, idx) {
					return
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				msg := "Public class is not documented with KDoc."
				if name != "" {
					msg = "Public class '" + name + "' is not documented with KDoc."
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, msg)
			},
		})
	}
	{
		r := &UndocumentedPublicFunctionRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicFunction", RuleSetName: "comments", Sev: "warning", Desc: "Detects public functions that are missing KDoc documentation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPublicDeclarationFlat(file, idx) {
					return
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				msg := "Public function is not documented with KDoc."
				if name != "" {
					msg = "Public function '" + name + "' is not documented with KDoc."
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, msg)
			},
		})
	}
	{
		r := &UndocumentedPublicPropertyRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects public properties that are missing KDoc documentation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPublicDeclarationFlat(file, idx) {
					return
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				msg := "Public property is not documented with KDoc."
				if name != "" {
					msg = "Public property '" + name + "' is not documented with KDoc."
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, msg)
			},
		})
	}
}

func shouldSkipPublicDocumentationRule(file *scanner.File, idx uint32) bool {
	if file == nil {
		return true
	}
	if isTestFile(file.Path) {
		return true
	}
	if isGradleBuildScript(file.Path) {
		return true
	}
	if file.FlatHasModifier(idx, "override") {
		return true
	}
	if _, ok := flatPrecedingKDoc(file, idx); ok {
		return true
	}
	if deadCodeDeclarationHasDIAnnotation(file, idx) {
		return true
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) == "source_file" {
			break
		}
		if deadCodeDeclarationHasDIAnnotation(file, parent) {
			return true
		}
	}
	return deadCodeByteInsideDIAnnotatedContainer(int(file.FlatStartByte(idx)), file)
}
