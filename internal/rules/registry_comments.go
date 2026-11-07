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
			NodeTypes: []string{"multiline_comment"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
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
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"@deprecated tag block does not properly report deprecation in Kotlin, use @Deprecated annotation instead")
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
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPrivateDeclarationFlat(file, idx) {
					return
				}
				if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
					f := r.Finding(file, declLine(file, idx), declCol(file, idx),
						"The function "+extractIdentifierFlat(file, idx)+" has a comment. Prefer renaming the function giving it a more self-explanatory name.")
					startByte, endByte := kdocRemovalRange(file, kdocIdx)
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
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
			NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPrivateDeclarationFlat(file, idx) {
					return
				}
				if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
					f := r.Finding(file, declLine(file, idx), declCol(file, idx),
						"Private properties should be named in a self-explanatory manner without the need for a  comment.")
					startByte, endByte := kdocRemovalRange(file, kdocIdx)
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   startByte,
						EndByte:     endByte,
						Replacement: "",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &EndOfSentenceFormatRule{BaseRule: BaseRule{RuleName: "EndOfSentenceFormat", RuleSetName: "comments", Sev: "warning", Desc: "Detects KDoc first sentences that do not end with proper punctuation."}, Pattern: regexp.MustCompile(endOfSentenceDefaultPattern)}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"multiline_comment"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) || isGradleBuildScript(file.Path) {
					return
				}
				if !flatIsKDoc(file, idx) {
					return
				}
				paragraph := kdocFirstParagraph(file.FlatNodeText(idx))
				if len(paragraph) == 0 || strings.HasPrefix(paragraph[0], "<") {
					return
				}
				// A sentence wrapped over several lines ends on a later line, so test the whole paragraph.
				if r.Pattern.MatchString(strings.Join(paragraph, "\n")) {
					return
				}
				row, col := kdocSectionStart(file, idx)
				f := r.Finding(file, row, col,
					"The first sentence of this KDoc does not end with the correct punctuation.")
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
		r := &KDocReferencesNonPublicPropertyRule{BaseRule: BaseRule{RuleName: "KDocReferencesNonPublicProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects private and internal class properties that the class KDoc references."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				kdoc, ok := flatPrecedingKDoc(file, idx)
				if !ok {
					return
				}
				refs := map[string]bool{}
				for _, m := range kdocRefRe.FindAllStringSubmatch(file.FlatNodeText(kdoc), -1) {
					refs[m[1]] = true
				}
				if len(refs) == 0 {
					return
				}
				body, _ := file.FlatFindChild(idx, "class_body")
				if body == 0 {
					return
				}
				forEachDirectClassBodyNodeFlat(file, body, func(member uint32) {
					if file.FlatType(member) != "property_declaration" {
						return
					}
					name := extractIdentifierFlat(file, member)
					if !refs[name] || !file.FlatHasModifier(member, "private") && !file.FlatHasModifier(member, "internal") {
						return
					}
					ctx.EmitAt(declLine(file, member), declCol(file, member),
						"The property "+name+" is non-public and should not be referenced from KDoc comments.")
				})
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
		r := &OutdatedDocumentationRule{BaseRule: BaseRule{RuleName: "OutdatedDocumentation", RuleSetName: "comments", Sev: "warning", Desc: "Detects functions and classes whose KDoc @param tags do not match their parameters."}, MatchDeclarationsOrder: true, MatchTypeParameters: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration", "class_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				prev, ok := flatPrecedingKDoc(file, idx)
				if !ok {
					return
				}
				var tags []string
				for _, m := range paramTagRe.FindAllStringSubmatch(file.FlatNodeText(prev), -1) {
					tags = append(tags, m[1])
				}
				// detekt only compares when the documentation has @param tags.
				if len(tags) == 0 {
					return
				}
				var expected []string
				if r.MatchTypeParameters {
					expected = append(expected, outdatedDocCollectTypeParameterNamesFlat(file, idx)...)
				}
				if file.FlatType(idx) == "function_declaration" {
					if params, ok := file.FlatFindChild(idx, "function_value_parameters"); ok {
						for _, p := range functionParamEntries(file, params, nil) {
							expected = append(expected, paramName(file, p.node))
						}
					}
				} else if ctor, ok := file.FlatFindChild(idx, "primary_constructor"); ok {
					// @param on a constructor property is outdated: such parameters are documented with @property.
					for _, p := range classParamEntries(file, ctor, nil) {
						if r.AllowParamOnConstructorProperties || !file.FlatHasChildOfType(p.node, "binding_pattern_kind") {
							expected = append(expected, paramName(file, p.node))
						}
					}
				}
				if outdatedDocMatches(tags, expected, r.MatchDeclarationsOrder) {
					return
				}
				ctx.EmitAt(declLine(file, idx), declCol(file, idx),
					"Documentation of "+extractIdentifierFlat(file, idx)+" is outdated")
			},
		})
	}
	{
		r := &UndocumentedPublicClassRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicClass", RuleSetName: "comments", Sev: "warning", Desc: "Detects public classes that are missing KDoc documentation."}, SearchInNestedClass: true, SearchInInnerClass: true, SearchInInnerObject: true, SearchInInnerInterface: true}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "object_declaration", "companion_object"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
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
				kind := "class"
				if file.FlatType(idx) == "object_declaration" {
					kind = "object"
				}
				_ = kind
				if name == "" {
					name = "companion"
				}
				msg := name + " is missing required documentation."
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), msg)
			},
		})
	}
	{
		r := &UndocumentedPublicFunctionRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicFunction", RuleSetName: "comments", Sev: "warning", Desc: "Detects public functions that are missing KDoc documentation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isPublicDeclarationFlat(file, idx) {
					return
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				name := extractIdentifierFlat(file, idx)
				msg := "The function " + name + " is missing documentation."
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), msg)
			},
		})
	}
	{
		r := &UndocumentedPublicPropertyRule{BaseRule: BaseRule{RuleName: "UndocumentedPublicProperty", RuleSetName: "comments", Sev: "warning", Desc: "Detects public properties that are missing KDoc documentation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration", "class_parameter", "enum_entry"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				if name == "" {
					return
				}
				switch file.FlatType(idx) {
				case "class_parameter":
					// Only val/var parameters are properties, and the class KDoc may document them with @property.
					if !file.FlatHasChildOfType(idx, "binding_pattern_kind") || !isPublicDeclarationFlat(file, idx) {
						return
					}
					if ctor, ok := file.FlatParent(idx); ok {
						if cls, ok := file.FlatParent(ctor); ok {
							if !isPublicDeclarationFlat(file, cls) {
								return
							}
							if kdoc, ok := flatPrecedingKDoc(file, cls); ok && kdocHasTag(file.FlatNodeText(kdoc), "property", name) {
								return
							}
						}
					}
				case "enum_entry":
					if cls, ok := flatEnclosingAncestor(file, idx, "class_declaration"); ok && !isPublicDeclarationFlat(file, cls) {
						return
					}
				default:
					if !isPublicDeclarationFlat(file, idx) {
						return
					}
					// detekt reads a script's top-level properties as locals, which are never public API.
					if parent, ok := file.FlatParent(idx); ok && file.IsScript() && file.FlatType(parent) == "source_file" {
						return
					}
				}
				if shouldSkipPublicDocumentationRule(file, idx) {
					return
				}
				ctx.EmitAt(declLine(file, idx), declCol(file, idx), "The property "+name+" is missing documentation.")
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
	// Test sources are left out by the rules' default excludes, not here, and detekt has no DI exemption.
	if file.FlatHasModifier(idx, "override") {
		return true
	}
	_, ok := flatPrecedingKDoc(file, idx)
	return ok
}
