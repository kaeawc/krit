package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// deprecatedKDocTagRe matches the literal `@deprecated` KDoc block tag.
// The tag must begin at a non-identifier boundary (start of text,
// whitespace, or the `*` line marker) and end at a non-identifier
// boundary (whitespace, end of text, or punctuation like `.` or `:`).
// This rejects substring matches inside other tag names like
// `@deprecatedSince` and inside Markdown code spans where the `@` is
// preceded by a backtick.
var deprecatedKDocTagRe = regexp.MustCompile(`(?:^|[\s*])@deprecated(?:$|[\s.:])`)

func registerCommentsRules() {

	// --- from comments.go ---
	{
		r := &AbsentOrWrongFileLicenseRule{BaseRule: BaseRule{RuleName: "AbsentOrWrongFileLicense", RuleSetName: "comments", Sev: "warning", Desc: "Detects files that are missing a valid license header comment."}, LicenseTemplate: "Copyright"}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsLinePass, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &DeprecatedBlockTagRule{BaseRule: BaseRule{RuleName: "DeprecatedBlockTag", RuleSetName: "comments", Sev: "warning", Desc: "Detects @deprecated KDoc tags that should use the @Deprecated annotation instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				if !flatIsKDoc(file, idx) {
					return
				}
				text := file.FlatNodeText(idx)
				if !deprecatedKDocTagRe.MatchString(text) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"Use @Deprecated annotation instead of @deprecated KDoc tag.")
				if fix := buildDeprecatedBlockTagFix(file, idx, text); fix != nil {
					f.Fix = fix
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &DocumentationOverPrivateFunctionRule{BaseRule: BaseRule{RuleName: "DocumentationOverPrivateFunction", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc documentation on private functions where it is unnecessary."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
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
				if r.Pattern.MatchString(firstLine) {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, 1,
					"KDoc first sentence should end with proper punctuation.")
				if insertAt := endOfSentenceInsertOffsetFlat(file, idx); insertAt >= 0 {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   insertAt,
						EndByte:     insertAt,
						Replacement: ".",
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &KDocReferencesNonPublicPropertyRule{BaseRule: BaseRule{RuleName: "KDocReferencesNonPublicProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc bracket references that point to non-public properties."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *api.Context) {
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
		r := &SampleAnnotationFreshnessRule{BaseRule: BaseRule{RuleName: "SampleAnnotationFreshness", RuleSetName: "comments", Sev: "info", Desc: "Detects KDoc @sample tags whose fully-qualified target function is not present in analysed sources."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile | api.NeedsParsedFiles, Languages: []scanner.Language{scanner.LangKotlin},
			NodeTypes: []string{"multiline_comment"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &KdocLinkValidationRule{BaseRule: BaseRule{RuleName: "KdocLinkValidation", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc bracket links that do not resolve to source symbols."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile | api.NeedsParsedFiles, Languages: []scanner.Language{scanner.LangKotlin},
			NodeTypes: []string{"multiline_comment"}, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &OutdatedDocumentationRule{BaseRule: BaseRule{RuleName: "OutdatedDocumentation", RuleSetName: "comments", Sev: "warning", Desc: "Detects @param tags in KDoc that do not match the actual function parameters."}, MatchDeclarationsOrder: true, MatchTypeParameters: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				prev, ok := flatPrecedingKDoc(file, idx)
				if !ok {
					return
				}
				kdoc := file.FlatNodeText(prev)
				docParamIdx := paramTagRe.FindAllStringSubmatchIndex(kdoc, -1)
				if len(docParamIdx) == 0 {
					return
				}
				summary := getFunctionDeclSummaryFlat(file, idx)
				actualParams := make(map[string]bool)
				orderedNames := make([]string, 0)
				// Type parameters are declared `<T, R>` before the
				// value-parameter list and conventionally documented in
				// that order. Add them first when MatchTypeParameters is
				// on so the order check sees the canonical sequence.
				if r.MatchTypeParameters {
					for _, name := range outdatedDocCollectTypeParameterNamesFlat(file, idx) {
						actualParams[name] = true
						orderedNames = append(orderedNames, name)
					}
				}
				for _, param := range summary.params {
					if param.name != "" {
						actualParams[param.name] = true
						orderedNames = append(orderedNames, param.name)
					}
				}
				kdocStart := int(file.FlatStartByte(prev))
				for _, m := range docParamIdx {
					name := kdoc[m[2]:m[3]]
					if actualParams[name] {
						continue
					}
					f := r.Finding(file, file.FlatRow(prev)+1, 1,
						"KDoc @param \""+name+"\" does not match any actual parameter.")
					// Delete the full line containing this @param tag from the KDoc.
					tagStart := m[0]
					lineStart := tagStart
					for lineStart > 0 && kdoc[lineStart-1] != '\n' {
						lineStart--
					}
					lineEnd := m[1]
					for lineEnd < len(kdoc) && kdoc[lineEnd] != '\n' {
						lineEnd++
					}
					if lineEnd < len(kdoc) && kdoc[lineEnd] == '\n' {
						lineEnd++
					}
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   kdocStart + lineStart,
						EndByte:     kdocStart + lineEnd,
						Replacement: "",
					}
					ctx.Emit(f)
				}
				docParams := make([][]string, 0, len(docParamIdx))
				for _, m := range docParamIdx {
					docParams = append(docParams, []string{kdoc[m[0]:m[1]], kdoc[m[2]:m[3]]})
				}
				// MatchDeclarationsOrder: ensure the @param tags that
				// reference real (or — when MatchTypeParameters is on —
				// type) parameters appear in the same order as the
				// declared parameters. A monotonic index walk catches
				// out-of-order tags.
				if r.MatchDeclarationsOrder && len(orderedNames) > 1 {
					position := make(map[string]int, len(orderedNames))
					for i, name := range orderedNames {
						position[name] = i
					}
					prevIdx := -1
					for _, dp := range docParams {
						name := dp[1]
						pos, ok := position[name]
						if !ok {
							continue
						}
						if pos < prevIdx {
							ctx.EmitAt(file.FlatRow(prev)+1, 1,
								"KDoc @param \""+name+"\" is out of declaration order.")
							break
						}
						prevIdx = pos
					}
				}
			},
		})
	}
	{
		r := &UndocumentedPublicClassRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicClass", RuleSetName: "comments", Sev: "warning", Desc: "Detects public classes that are missing KDoc documentation."}, SearchInNestedClass: true, SearchInInnerClass: true, SearchInInnerObject: true, SearchInInnerInterface: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPublicDeclarationFlat(file, idx) {
					return
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				if shouldSkipUndocumentedNestedClass(file, idx,
					r.SearchInNestedClass, r.SearchInInnerClass,
					r.SearchInInnerObject, r.SearchInInnerInterface) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *api.Context) {
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *api.Context) {
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

// shouldSkipUndocumentedNestedClass returns true when the class
// at idx is contained within another declaration whose corresponding
// search-in flag is false. The four flags map to the kind of immediate
// enclosing declaration:
//
//   - searchInInnerClass: the class carries `inner` modifier (Kotlin
//     inner classes; capture an outer instance)
//   - searchInNestedClass: the class is nested in a class without the
//     `inner` modifier
//   - searchInInnerObject: the class is declared inside an object
//   - searchInInnerInterface: the class is declared inside an interface
//
// When all four flags are true (the default), no nested class is
// skipped and the rule fires on every undocumented public class.
func shouldSkipUndocumentedNestedClass(file *scanner.File, idx uint32, searchNested, searchInner, searchObject, searchInterface bool) bool {
	if file == nil {
		return false
	}
	owner, ok := flatEnclosingAncestor(file, idx, "class_declaration", "object_declaration", "companion_object")
	if !ok || owner == 0 {
		return false
	}
	switch file.FlatType(owner) {
	case "object_declaration", "companion_object":
		return !searchObject
	case "class_declaration":
		// Differentiate interface containers via the `interface` modifier.
		if file.FlatHasChildOfType(owner, "interface") {
			return !searchInterface
		}
		// Differentiate inner from nested classes by the inner modifier
		// on the candidate class itself, NOT on the enclosing owner.
		if file.FlatHasModifier(idx, "inner") {
			return !searchInner
		}
		return !searchNested
	}
	return false
}

func shouldSkipPublicDocumentationRule(file *scanner.File, idx uint32) bool {
	if file == nil {
		return true
	}
	if scanner.IsTestFile(file.Path) {
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
