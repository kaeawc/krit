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

// DocumentationOverPrivateFunctionRule detects KDoc on private functions.
type DocumentationOverPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivateFunctionRule) Confidence() float64 { return 0.95 }

// DocumentationOverPrivatePropertyRule detects KDoc on private properties.
type DocumentationOverPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivatePropertyRule) Confidence() float64 { return 0.95 }

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

// KDocReferencesNonPublicPropertyRule finds KDoc [ref] to non-public properties.
type KDocReferencesNonPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *KDocReferencesNonPublicPropertyRule) Confidence() float64 { return 0.95 }

var kdocRefRe = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

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

// UndocumentedPublicFunctionRule detects public functions without KDoc.
type UndocumentedPublicFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicFunctionRule) Confidence() float64 { return 0.95 }

// UndocumentedPublicPropertyRule detects public properties without KDoc.
type UndocumentedPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicPropertyRule) Confidence() float64 { return 0.95 }
