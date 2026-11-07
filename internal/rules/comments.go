package rules

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func flatIsKDoc(file *scanner.File, idx uint32) bool {
	return strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "/**")
}

// kdocRemovalRange returns the byte range that deletes a KDoc. When the comment owns its lines the
// range covers the whole lines, so the declaration keeps its indent; otherwise only the comment goes.
func kdocRemovalRange(file *scanner.File, kdoc uint32) (int, int) {
	content := file.Content
	start, end := int(file.FlatStartByte(kdoc)), int(file.FlatEndByte(kdoc))
	lineStart := start
	for lineStart > 0 && (content[lineStart-1] == ' ' || content[lineStart-1] == '\t') {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(content) && (content[lineEnd] == ' ' || content[lineEnd] == '\t') {
		lineEnd++
	}
	if lineEnd < len(content) && content[lineEnd] == '\r' {
		lineEnd++
	}
	atLineStart := lineStart == 0 || content[lineStart-1] == '\n'
	atLineEnd := lineEnd < len(content) && content[lineEnd] == '\n'
	if atLineStart && atLineEnd {
		return lineStart, lineEnd + 1
	}
	return start, end
}

// kdocFirstParagraph returns the KDoc lines of the first paragraph: the first content line up to a
// blank line or a block tag. Each item holds the line text without the comment decoration.
func kdocFirstParagraph(raw string) []string {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "/**")
	raw = strings.TrimSuffix(raw, "*/")
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "- ") || (trimmed == "" && len(out) > 0) {
			break
		}
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// endOfSentenceInsertOffsetFlat returns the byte offset after the last line of the first KDoc paragraph.
func endOfSentenceInsertOffsetFlat(file *scanner.File, idx uint32) int {
	startByte := int(file.FlatStartByte(idx))
	endByte := int(file.FlatEndByte(idx))
	if startByte < 0 || endByte > len(file.Content) || startByte >= endByte {
		return -1
	}
	raw := file.Content[startByte:endByte]
	pos := 0
	result := -1
	for pos < len(raw) {
		nl := pos
		for nl < len(raw) && raw[nl] != '\n' {
			nl++
		}
		line := raw[pos:nl]
		stripped := strings.TrimSpace(string(line))
		stripped = strings.TrimPrefix(stripped, "/**")
		stripped = strings.TrimSpace(stripped)
		stripped = strings.TrimSuffix(stripped, "*/")
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, "*"))
		if stripped == "" || strings.HasPrefix(stripped, "@") || strings.HasPrefix(stripped, "- ") {
			if result >= 0 || stripped != "" {
				break
			}
			pos = nl + 1
			continue
		}
		// Insert after trailing whitespace and a closing `*/` on the line.
		e := len(line)
		for e > 0 && (line[e-1] == ' ' || line[e-1] == '\t' || line[e-1] == '\r') {
			e--
		}
		if e >= 2 && line[e-2] == '*' && line[e-1] == '/' {
			e -= 2
			for e > 0 && (line[e-1] == ' ' || line[e-1] == '\t') {
				e--
			}
		}
		result = startByte + pos + e
		pos = nl + 1
	}
	return result
}

type kdocLinkToken struct {
	Target string
	Offset int
}

func iterateKdocLinks(text string, yield func(kdocLinkToken)) {
	if yield == nil {
		return
	}
	inCode := false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
			continue
		case '`':
			inCode = !inCode
			continue
		case '[':
			if inCode {
				continue
			}
			if i+1 < len(text) && text[i+1] == '[' {
				continue
			}
			closeBracket := findKdocLinkClose(text, i+1)
			if closeBracket < 0 {
				continue
			}
			if closeBracket+1 < len(text) && text[closeBracket+1] == '(' {
				i = closeBracket
				continue
			}
			target := strings.TrimSpace(text[i+1 : closeBracket])
			if isKdocLinkTarget(target) {
				yield(kdocLinkToken{Target: target, Offset: i})
			}
			i = closeBracket
		}
	}
}

func findKdocLinkClose(text string, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == ']' {
			return i
		}
	}
	return -1
}

func isKdocLinkTarget(target string) bool {
	if target == "" || strings.ContainsAny(target, " \t\r\n[]()") {
		return false
	}
	for _, r := range target {
		if r == '.' || r == '#' || r == '_' || r == '-' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
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
	if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "protected") || file.FlatHasModifier(idx, "internal") {
		return false
	}
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_body", "statements", "lambda_literal", "control_structure_body":
			return false
		case "class_declaration", "object_declaration", "companion_object":
			// A member of a non-public container is not public API.
			if file.FlatHasModifier(parent, "private") || file.FlatHasModifier(parent, "internal") || file.FlatHasModifier(parent, "protected") {
				return false
			}
		case "source_file":
			return true
		}
	}
	return true
}

func isPrivateDeclarationFlat(file *scanner.File, idx uint32) bool {
	return file.FlatHasModifier(idx, "private")
}

// AbsentOrWrongFileLicenseRule checks that the first comment matches a license template.
type AbsentOrWrongFileLicenseRule struct {
	LineBase
	BaseRule
	LicenseTemplate string
	// LicenseTemplateFile names a file whose trimmed text replaces LicenseTemplate when it can be read.
	LicenseTemplateFile string
	IsRegex             bool
	compiledRegex       *regexp.Regexp // cached compiled regex when IsRegex=true
	templateOnce        sync.Once
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *AbsentOrWrongFileLicenseRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *AbsentOrWrongFileLicenseRule) check(ctx *api.Context) {
	file := ctx.File
	r.templateOnce.Do(func() {
		if r.LicenseTemplateFile == "" {
			return
		}
		if data, err := os.ReadFile(r.LicenseTemplateFile); err == nil {
			r.LicenseTemplate = strings.TrimSpace(string(data))
		}
	})
	// Only active if a license template is configured; skip by default
	if r.LicenseTemplate == "" {
		return
	}
	// Find the first comment node
	var firstComment uint32
	hasFirstComment := false
	// A script's `#!` line stays first: the header follows it.
	headerStart := 0
	for i := 0; i < file.FlatChildCount(0); i++ {
		child := file.FlatChild(0, i)
		t := file.FlatType(child)
		if t == "shebang_line" {
			headerStart = scriptLineEnd(file.Content, int(file.FlatEndByte(child)))
			continue
		}
		if t == "multiline_comment" || t == "line_comment" {
			firstComment = child
			hasFirstComment = true
			break
		}
		// The header must come first: a comment after the package line is documentation, not a license.
		break
	}
	if !hasFirstComment {
		f := r.Finding(file, 1, 1, "Expected license not found or incorrect in the file: "+filepath.Base(file.Path))
		// Auto-fix only when LicenseTemplate is a literal header — a regex
		// pattern is not safe to inline as Kotlin source.
		if !r.IsRegex && !strings.Contains(r.LicenseTemplate, "*/") {
			// Insert after a leading BOM, and with the file's own line ending.
			insertAt := headerStart
			if insertAt == 0 && bytes.HasPrefix(file.Content, []byte("\xef\xbb\xbf")) {
				insertAt = 3
			}
			eol := "\n"
			if i := bytes.IndexByte(file.Content, '\n'); i > 0 && file.Content[i-1] == '\r' {
				eol = "\r\n"
			}
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   insertAt,
				EndByte:     insertAt,
				Replacement: "/* " + r.LicenseTemplate + " */" + eol,
			}
		}
		ctx.Emit(f)
		return
	}
	text := file.FlatNodeText(firstComment)
	if r.IsRegex {
		if r.compiledRegex == nil {
			var err error
			r.compiledRegex, err = regexp.Compile(r.LicenseTemplate)
			if err != nil {
				return // invalid regex pattern, skip
			}
		}
		if r.compiledRegex.MatchString(text) {
			return
		}
	} else {
		if strings.Contains(text, r.LicenseTemplate) {
			return
		}
	}
	f := r.Finding(file, 1, 1, "Expected license not found or incorrect in the file: "+filepath.Base(file.Path))
	if !r.IsRegex && !strings.Contains(r.LicenseTemplate, "*/") {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(firstComment)),
			EndByte:     int(file.FlatEndByte(firstComment)),
			Replacement: "/* " + r.LicenseTemplate + " */",
		}
	}
	ctx.Emit(f)
}

// scriptLineEnd returns the offset just past the line break that ends the line holding offset, or
// len(content) when that line is the last one.
func scriptLineEnd(content []byte, offset int) int {
	if offset >= len(content) {
		return len(content)
	}
	if i := bytes.IndexByte(content[offset:], '\n'); i >= 0 {
		return offset + i + 1
	}
	return len(content)
}

// DeprecatedBlockTagRule detects @deprecated in KDoc comments.
type DeprecatedBlockTagRule struct {
	FlatDispatchBase
	BaseRule
}

// buildDeprecatedBlockTagFix returns a single byte-range Fix that rewrites
// the KDoc and inserts a `@Deprecated("<message>")` annotation in front of
// the following declaration. Returns nil when the surrounding shape is not
// safe to rewrite (no following declaration, declaration already annotated,
// or message extraction fails).
func buildDeprecatedBlockTagFix(file *scanner.File, kdocIdx uint32, kdocText string) *scanner.Fix {
	target, ok := flatDeclarationAfter(file, kdocIdx)
	if !ok {
		return nil
	}
	if hasAnnotationNamed(file, target, "Deprecated") {
		return nil
	}
	message, cleanedKDoc, okExtract := stripDeprecatedKDocTag(kdocText)
	if !okExtract {
		return nil
	}

	kdocStart := int(file.FlatStartByte(kdocIdx))
	declStart := int(file.FlatStartByte(target))
	if declStart <= kdocStart || declStart > len(file.Content) {
		return nil
	}
	indent := detectIndent(file.Content, declStart)
	annotation := buildDeprecatedAnnotation(message)
	replacement := cleanedKDoc + "\n" + indent + annotation + "\n" + indent
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   kdocStart,
		EndByte:     declStart,
		Replacement: replacement,
	}
}

// flatDeclarationAfter returns the next sibling that looks like a declaration
// the KDoc is documenting. Skips intervening line comments and whitespace
// siblings.
func flatDeclarationAfter(file *scanner.File, idx uint32) (uint32, bool) {
	// A KDoc before the first declaration can sit at the end of the package or import header, so climb out
	// of such a header and continue with its siblings.
	for cur := idx; ; {
		for sib, ok := file.FlatNextSibling(cur); ok; sib, ok = file.FlatNextSibling(sib) {
			switch file.FlatType(sib) {
			case "line_comment", "multiline_comment":
				continue
			case "class_declaration",
				"object_declaration",
				"function_declaration",
				"property_declaration",
				"type_alias",
				"secondary_constructor":
				return sib, true
			default:
				return 0, false
			}
		}
		parent, ok := file.FlatParent(cur)
		if !ok {
			return 0, false
		}
		if t := file.FlatType(parent); t != "package_header" && t != "import_list" && t != "import_header" {
			return 0, false
		}
		cur = parent
	}
}

// stripDeprecatedKDocTag removes the `@deprecated` line (and continuation
// lines that belong to the same tag) from a KDoc block, returning the
// extracted message text and the cleaned KDoc body. Continuation lines are
// any subsequent ` * ...` lines that do not start with another `@<tag>`
// and are not the closing `*/`.
func stripDeprecatedKDocTag(text string) (message, cleaned string, ok bool) {
	lines := strings.Split(text, "\n")
	var msgParts []string
	var out []string
	inTag := false
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Strip leading "*" / "* " for tag detection.
		body := trimmed
		if strings.HasPrefix(body, "*") && !strings.HasPrefix(body, "*/") {
			body = strings.TrimPrefix(body, "*")
			body = strings.TrimPrefix(body, " ")
		}
		if !inTag && strings.HasPrefix(body, "@deprecated") {
			found = true
			inTag = true
			rest := strings.TrimSpace(strings.TrimPrefix(body, "@deprecated"))
			if rest != "" {
				msgParts = append(msgParts, rest)
			}
			continue
		}
		if inTag {
			// End the tag when we hit another @tag, the closing */, or a
			// blank-looking continuation that's empty.
			if strings.HasPrefix(body, "@") || strings.HasPrefix(trimmed, "*/") || trimmed == "*" || trimmed == "" {
				inTag = false
				// Fall through so this line is preserved.
			} else {
				if body = strings.TrimSpace(body); body != "" {
					msgParts = append(msgParts, body)
				}
				continue
			}
		}
		out = append(out, line)
	}
	if !found {
		return "", "", false
	}
	return strings.Join(msgParts, " "), strings.Join(out, "\n"), true
}

// buildDeprecatedAnnotation renders a `@Deprecated("...")` annotation,
// escaping embedded quotes and backslashes in the message. When the message
// is empty the annotation still receives an empty string argument so the
// resulting annotation compiles.
func buildDeprecatedAnnotation(message string) string {
	escaped := strings.ReplaceAll(message, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	// A bare `$` would start a string template.
	escaped = strings.ReplaceAll(escaped, `$`, `\$`)
	return `@Deprecated("` + escaped + `")`
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DeprecatedBlockTagRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// DocumentationOverPrivateFunctionRule detects KDoc on private functions.
type DocumentationOverPrivateFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivateFunctionRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// DocumentationOverPrivatePropertyRule detects KDoc on private properties.
type DocumentationOverPrivatePropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *DocumentationOverPrivatePropertyRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// endOfSentenceDefaultPattern finds a sentence terminator followed by a break, or ending the paragraph.
const endOfSentenceDefaultPattern = `([.?!][ \t\n\r\f<])|([.?!:]$)`

// EndOfSentenceFormatRule checks KDoc first sentence ends with proper punctuation.
type EndOfSentenceFormatRule struct {
	FlatDispatchBase
	BaseRule
	// Pattern is the regex applied to the first paragraph of a KDoc to verify
	// it ends with proper punctuation. Configurable via the
	// `endOfSentenceFormat` YAML option.
	Pattern *regexp.Regexp
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *EndOfSentenceFormatRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// KDocReferencesNonPublicPropertyRule finds KDoc [ref] to non-public properties.
type KDocReferencesNonPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *KDocReferencesNonPublicPropertyRule) Confidence() float64 { return api.ConfidenceVeryHigh }

var kdocRefRe = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

// OutdatedDocumentationRule detects @param tags that don't match actual function parameters.
type OutdatedDocumentationRule struct {
	FlatDispatchBase
	BaseRule
	MatchDeclarationsOrder bool
	MatchTypeParameters    bool
	// AllowParamOnConstructorProperties counts @param tags of val/var constructor parameters as documented.
	AllowParamOnConstructorProperties bool
}

var paramTagRe = regexp.MustCompile(`@param\s+([A-Za-z_][A-Za-z0-9_]*)`)

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *OutdatedDocumentationRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// outdatedDocCollectTypeParameterNamesFlat returns the type-parameter
// identifiers declared on a function (e.g. `T`, `R` in `fun <T, R> map`).
// Tree-sitter Kotlin emits a `type_parameters` child of
// `function_declaration` with one `type_parameter` per name, each
// containing a `type_identifier`.
func outdatedDocCollectTypeParameterNamesFlat(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	tps, ok := file.FlatFindChild(idx, "type_parameters")
	if !ok || tps == 0 {
		return nil
	}
	var names []string
	for child := file.FlatFirstChild(tps); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "type_parameter" {
			continue
		}
		ident, _ := file.FlatFindChild(child, "type_identifier")
		if ident == 0 {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(ident))
		if text != "" {
			names = append(names, text)
		}
	}
	return names
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
func (r *UndocumentedPublicClassRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// UndocumentedPublicFunctionRule detects public functions without KDoc.
type UndocumentedPublicFunctionRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicFunctionRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// UndocumentedPublicPropertyRule detects public properties without KDoc.
type UndocumentedPublicPropertyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Documentation/comment rule. Detection checks presence and
// well-formedness of doc comments on declarations — purely structural. No
// heuristic path. Classified per roadmap/17.
func (r *UndocumentedPublicPropertyRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// kdocSectionStart returns where detekt reports a KDoc: right after `/**` when text follows on that line,
// otherwise the first non-blank character of the next line.
func kdocSectionStart(file *scanner.File, idx uint32) (row, col int) {
	row, col = file.FlatRow(idx)+1, file.FlatCol(idx)+1
	text := file.FlatNodeText(idx)
	firstLine, rest, hasMore := strings.Cut(text, "\n")
	body := strings.TrimPrefix(firstLine, "/**")
	if strings.TrimSpace(strings.TrimSuffix(body, "*/")) != "" || !hasMore {
		return row, col + 3
	}
	line, _, _ := strings.Cut(rest, "\n")
	return row + 1, len(line) - len(strings.TrimLeft(line, " \t")) + 1
}

// outdatedDocMatches reports that the documented @param names equal the declared ones, as a sequence when
// order matters and as a set otherwise.
func outdatedDocMatches(tags, expected []string, ordered bool) bool {
	if len(tags) != len(expected) {
		return false
	}
	if ordered {
		for i := range tags {
			if tags[i] != expected[i] {
				return false
			}
		}
		return true
	}
	seen := make(map[string]int, len(expected))
	for _, n := range expected {
		seen[n]++
	}
	for _, n := range tags {
		if seen[n] == 0 {
			return false
		}
		seen[n]--
	}
	return true
}

// kdocHasTag reports a `@tag name` block tag in the KDoc text.
func kdocHasTag(kdoc, tag, name string) bool {
	re := regexp.MustCompile(`@` + regexp.QuoteMeta(tag) + `\s+` + regexp.QuoteMeta(name) + `\b`)
	return re.MatchString(kdoc)
}
