package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func flatIsKDoc(file *scanner.File, idx uint32) bool {
	return strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "/**")
}

func flatKdocText(file *scanner.File, idx uint32) string {
	text := file.FlatNodeText(idx)
	text = strings.TrimPrefix(strings.TrimSpace(text), "/**")
	text = strings.TrimSuffix(text, "*/")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "*")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

func flatPrecedingKDoc(file *scanner.File, idx uint32) (uint32, bool) {
	prev, ok := file.FlatPrevSibling(idx)
	for ok {
		t := file.FlatType(prev)
		if t == "multiline_comment" && flatIsKDoc(file, prev) {
			return prev, true
		}
		if t == "line_comment" {
			prev, ok = file.FlatPrevSibling(prev)
			continue
		}
		if file.FlatChildCount(prev) > 0 {
			lastChild := file.FlatChild(prev, file.FlatChildCount(prev)-1)
			if file.FlatType(lastChild) == "multiline_comment" && flatIsKDoc(file, lastChild) {
				return lastChild, true
			}
			if file.FlatChildCount(lastChild) > 0 {
				deepLast := file.FlatChild(lastChild, file.FlatChildCount(lastChild)-1)
				if file.FlatType(deepLast) == "multiline_comment" && flatIsKDoc(file, deepLast) {
					return deepLast, true
				}
			}
		}
		break
	}
	return 0, false
}

func isPublicDeclarationFlat(file *scanner.File, idx uint32) bool {
	return !file.FlatHasModifier(idx, "private") && !file.FlatHasModifier(idx, "protected") && !file.FlatHasModifier(idx, "internal")
}

func isPrivateDeclarationFlat(file *scanner.File, idx uint32) bool {
	return file.FlatHasModifier(idx, "private")
}

// AbsentOrWrongFileLicenseRule checks that the first comment matches a license template.
type AbsentOrWrongFileLicenseRule struct {
	LineBase
	BaseRule
	LicenseTemplate string
	IsRegex         bool
	compiledRegex   *regexp.Regexp // cached compiled regex when IsRegex=true
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *AbsentOrWrongFileLicenseRule) Confidence() float64 { return 0.95 }

func (r *AbsentOrWrongFileLicenseRule) CheckLines(file *scanner.File) []scanner.Finding {
	// Only active if a license template is configured; skip by default
	if r.LicenseTemplate == "" {
		return nil
	}
	// Find the first comment node
	var firstComment uint32
	hasFirstComment := false
	for i := 0; i < file.FlatChildCount(0); i++ {
		child := file.FlatChild(0, i)
		t := file.FlatType(child)
		if t == "multiline_comment" || t == "line_comment" {
			firstComment = child
			hasFirstComment = true
			break
		}
		// Skip package_header, but if we hit a real declaration first, no comment
		if t != "package_header" && t != "import_header" {
			break
		}
	}
	licenseComment := "/* " + r.LicenseTemplate + " */\n"
	if !hasFirstComment {
		f := r.Finding(file, 1, 1, "File does not have a valid license header.")
		// Auto-fix: insert the license template at byte 0
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     0,
			Replacement: licenseComment,
		}
		return []scanner.Finding{f}
	}
	text := file.FlatNodeText(firstComment)
	if r.IsRegex {
		if r.compiledRegex == nil {
			var err error
			r.compiledRegex, err = regexp.Compile(r.LicenseTemplate)
			if err != nil {
				return nil // invalid regex pattern, skip
			}
		}
		if r.compiledRegex.MatchString(text) {
			return nil
		}
	} else {
		if strings.Contains(text, r.LicenseTemplate) {
			return nil
		}
	}
	f := r.Finding(file, 1, 1, "File does not have a valid license header.")
	// Auto-fix: replace existing wrong license comment with correct one
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(firstComment)),
		EndByte:     int(file.FlatEndByte(firstComment)),
		Replacement: "/* " + r.LicenseTemplate + " */",
	}
	return []scanner.Finding{f}
}

// DeprecatedBlockTagRule detects @deprecated in KDoc comments.
type DeprecatedBlockTagRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DeprecatedBlockTagRule) Confidence() float64 { return 0.95 }

func (r *DeprecatedBlockTagRule) NodeTypes() []string { return []string{"multiline_comment"} }

func (r *DeprecatedBlockTagRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatIsKDoc(file, idx) {
		return nil
	}
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "@deprecated") {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"Use @Deprecated annotation instead of @deprecated KDoc tag.")}
	}
	return nil
}

// DocumentationOverPrivateFunctionRule detects KDoc on private functions.
type DocumentationOverPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivateFunctionRule) Confidence() float64 { return 0.95 }

func (r *DocumentationOverPrivateFunctionRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *DocumentationOverPrivateFunctionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isPrivateDeclarationFlat(file, idx) {
		return nil
	}
	if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Private function should not have KDoc documentation.")
		// Remove KDoc including trailing newline
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
		return []scanner.Finding{f}
	}
	return nil
}

func (r *DocumentationOverPrivateFunctionRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// DocumentationOverPrivatePropertyRule detects KDoc on private properties.
type DocumentationOverPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivatePropertyRule) Confidence() float64 { return 0.95 }

func (r *DocumentationOverPrivatePropertyRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *DocumentationOverPrivatePropertyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isPrivateDeclarationFlat(file, idx) {
		return nil
	}
	if kdocIdx, ok := flatPrecedingKDoc(file, idx); ok {
		f := r.Finding(file, file.FlatRow(idx)+1, 1,
			"Private property should not have KDoc documentation.")
		// Remove KDoc including trailing newline
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
		return []scanner.Finding{f}
	}
	return nil
}

func (r *DocumentationOverPrivatePropertyRule) Check(file *scanner.File) []scanner.Finding {
	return nil
}

// EndOfSentenceFormatRule checks KDoc first sentence ends with proper punctuation.
type EndOfSentenceFormatRule struct {
	FlatDispatchBase
	BaseRule
	Pattern             *regexp.Regexp
	EndOfSentenceFormat string
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *EndOfSentenceFormatRule) Confidence() float64 { return 0.95 }

func (r *EndOfSentenceFormatRule) NodeTypes() []string { return []string{"multiline_comment"} }

func (r *EndOfSentenceFormatRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatIsKDoc(file, idx) {
		return nil
	}
	text := flatKdocText(file, idx)
	if text == "" {
		return nil
	}
	// Get first line (first sentence)
	firstLine := strings.SplitN(text, "\n", 2)[0]
	// Skip @-tag lines
	if strings.HasPrefix(firstLine, "@") {
		return nil
	}
	if !r.Pattern.MatchString(firstLine) {
		return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1,
			"KDoc first sentence should end with proper punctuation.")}
	}
	return nil
}

// KDocReferencesNonPublicPropertyRule finds KDoc [ref] to non-public properties.
type KDocReferencesNonPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *KDocReferencesNonPublicPropertyRule) Confidence() float64 { return 0.95 }

func (r *KDocReferencesNonPublicPropertyRule) NodeTypes() []string {
	return []string{"multiline_comment"}
}

var kdocRefRe = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

func (r *KDocReferencesNonPublicPropertyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !flatIsKDoc(file, idx) {
		return nil
	}
	text := file.FlatNodeText(idx)
	matches := kdocRefRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Collect non-public property names
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
		return nil
	}

	var findings []scanner.Finding
	for _, m := range matches {
		name := m[1]
		if nonPublic[name] {
			findings = append(findings, r.Finding(file, file.FlatRow(idx)+1, 1,
				"KDoc references non-public property \""+name+"\"."))
		}
	}
	return findings
}

// OutdatedDocumentationRule detects @param tags that don't match actual function parameters.
type OutdatedDocumentationRule struct {
	FlatDispatchBase
	BaseRule
	MatchDeclarationsOrder bool
	MatchTypeParameters    bool
}

var paramTagRe = regexp.MustCompile(`@param\s+([A-Za-z_][A-Za-z0-9_]*)`)

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *OutdatedDocumentationRule) Confidence() float64 { return 0.95 }

func (r *OutdatedDocumentationRule) NodeTypes() []string { return []string{"function_declaration"} }

func (r *OutdatedDocumentationRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	// Check for preceding KDoc
	prev, ok := flatPrecedingKDoc(file, idx)
	if !ok {
		return nil
	}

	kdoc := file.FlatNodeText(prev)
	docParams := paramTagRe.FindAllStringSubmatch(kdoc, -1)
	if len(docParams) == 0 {
		return nil
	}

	// Collect actual parameter names
	actualParams := make(map[string]bool)
	summary := getFunctionDeclSummaryFlat(file, idx)
	for _, param := range summary.params {
		if param.name != "" {
			actualParams[param.name] = true
		}
	}

	// Check each documented @param against actual params
	var findings []scanner.Finding
	for _, dp := range docParams {
		name := dp[1]
		if !actualParams[name] {
			findings = append(findings, r.Finding(file, file.FlatRow(prev)+1, 1,
				"KDoc @param \""+name+"\" does not match any actual parameter."))
		}
	}
	return findings
}

// UndocumentedPublicClassRule detects public classes without KDoc.
type UndocumentedPublicClassRule struct {
	FlatDispatchBase
	BaseRule
	SearchInNestedClass    bool
	SearchInInnerClass     bool
	SearchInInnerObject    bool
	SearchInInnerInterface bool
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicClassRule) Confidence() float64 { return 0.95 }

func (r *UndocumentedPublicClassRule) NodeTypes() []string { return []string{"class_declaration"} }

func (r *UndocumentedPublicClassRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isPublicDeclarationFlat(file, idx) {
		return nil
	}
	// Skip override declarations using AST modifier check
	if file.FlatHasModifier(idx, "override") {
		return nil
	}
	if _, ok := flatPrecedingKDoc(file, idx); ok {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	msg := "Public class is not documented with KDoc."
	if name != "" {
		msg = "Public class '" + name + "' is not documented with KDoc."
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, msg)}
}

// UndocumentedPublicFunctionRule detects public functions without KDoc.
type UndocumentedPublicFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicFunctionRule) Confidence() float64 { return 0.95 }

func (r *UndocumentedPublicFunctionRule) NodeTypes() []string {
	return []string{"function_declaration"}
}

func (r *UndocumentedPublicFunctionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isPublicDeclarationFlat(file, idx) {
		return nil
	}
	// Skip override functions using AST modifier check
	if file.FlatHasModifier(idx, "override") {
		return nil
	}
	if _, ok := flatPrecedingKDoc(file, idx); ok {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	msg := "Public function is not documented with KDoc."
	if name != "" {
		msg = "Public function '" + name + "' is not documented with KDoc."
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, msg)}
}

// UndocumentedPublicPropertyRule detects public properties without KDoc.
type UndocumentedPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicPropertyRule) Confidence() float64 { return 0.95 }

func (r *UndocumentedPublicPropertyRule) NodeTypes() []string {
	return []string{"property_declaration"}
}

func (r *UndocumentedPublicPropertyRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if !isPublicDeclarationFlat(file, idx) {
		return nil
	}
	// Skip override properties using AST modifier check
	if file.FlatHasModifier(idx, "override") {
		return nil
	}
	if _, ok := flatPrecedingKDoc(file, idx); ok {
		return nil
	}
	name := extractIdentifierFlat(file, idx)
	msg := "Public property is not documented with KDoc."
	if name != "" {
		msg = "Public property '" + name + "' is not documented with KDoc."
	}
	return []scanner.Finding{r.Finding(file, file.FlatRow(idx)+1, 1, msg)}
}
