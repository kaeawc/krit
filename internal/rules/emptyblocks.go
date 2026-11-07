package rules

import (
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// nodeLineRange returns the start and end byte offsets that cover the full line(s)
// of a node, including leading whitespace and the trailing newline. Useful for
// byte-mode deletion that should remove whole lines.
func nodeLineRange(content []byte, startByte, endByte int) (int, int) {
	s := startByte
	for s > 0 && content[s-1] != '\n' {
		s--
	}
	e := endByte
	e = lineBreakEnd(content, e)
	return s, e
}

// detectIndent returns the whitespace indentation at the line containing the given byte offset.
func detectIndent(content []byte, byteOffset int) string {
	// Walk backwards to find the start of the line
	lineStart := byteOffset
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	// Collect leading whitespace
	var indent []byte
	for i := lineStart; i < len(content) && (content[i] == ' ' || content[i] == '\t'); i++ {
		indent = append(indent, content[i])
	}
	return string(indent)
}

// stripCommentsBlockRe and stripCommentsLineRe are compiled once at
// package init so stripComments — called for every block_body /
// catch_block / class_body the dispatcher visits across every Kotlin
// and Java file — does not recompile the same patterns on each call.
var (
	stripCommentsBlockRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	stripCommentsLineRe  = regexp.MustCompile(`//[^\n]*`)
)

// stripComments removes line comments and block comments from a string.
func stripComments(s string) string {
	s = stripCommentsBlockRe.ReplaceAllString(s, "")
	s = stripCommentsLineRe.ReplaceAllString(s, "")
	return s
}

// directBraceBody returns the source between the first direct `{` child of idx and the next direct
// `}` child, so braces inside parameter defaults or a `when` subject are not mistaken for the body.
// ok is false when idx has no braced body of its own.
func directBraceBody(file *scanner.File, idx uint32) (body string, ok bool) {
	open := -1
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "{":
			if open < 0 {
				open = int(file.FlatEndByte(child))
			}
		case "}":
			if open >= 0 {
				return string(file.Content[open:file.FlatStartByte(child)]), true
			}
		}
	}
	return "", false
}

// braceNode returns the first direct `{` child of idx, which is where detekt reports an empty block, or idx.
func braceNode(file *scanner.File, idx uint32) uint32 {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "{" {
			return c
		}
	}
	return idx
}

// directBraceBodyIsEmpty reports a braced body that holds only whitespace; any comment makes it
// non-empty because it documents an intentional no-op.
func directBraceBodyIsEmpty(file *scanner.File, idx uint32) bool {
	body, ok := directBraceBody(file, idx)
	return ok && strings.TrimSpace(body) == ""
}

func isBlockEmptyFlat(file *scanner.File, idx uint32) bool {
	text := file.FlatNodeText(idx)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return true
	}
	body := strings.TrimSpace(text[start+1 : end])
	cleaned := stripComments(body)
	return strings.TrimSpace(cleaned) == ""
}

// isWhileBodyEmpty reports whether a while statement's body does nothing. A brace-less body such as
// `while (cond) x = next()` is a statement, so it is not empty.
func isWhileBodyEmpty(text string) bool {
	open := strings.Index(text, "(")
	if open < 0 {
		return true
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if whileConditionHasSideEffectCall(text[open+1:i]) || whileBodyHasComment(text[i+1:]) {
					return false
				}
				body := strings.TrimSpace(stripComments(text[i+1:]))
				if body == ";" {
					return true
				}
				if !strings.HasPrefix(body, "{") {
					return body == ""
				}
				end := strings.LastIndex(body, "}")
				return end > 0 && strings.TrimSpace(body[1:end]) == ""
			}
		}
	}
	return true
}

var whileConditionCallRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// whileConditionHasSideEffectCall reports a call in the condition whose name is not a plain query, as
// in `while (awaitItem() != null) {}`, where the condition does the work.
func whileConditionHasSideEffectCall(cond string) bool {
	for _, m := range whileConditionCallRe.FindAllStringSubmatch(cond, -1) {
		name := m[1]
		if strings.HasPrefix(name, "is") || strings.HasPrefix(name, "has") || name == "contains" || name == "get" {
			continue
		}
		return true
	}
	return false
}

// whileBodyHasComment reports a comment in the loop body, which documents an intentional spin.
func whileBodyHasComment(body string) bool {
	return strings.Contains(body, "//") || strings.Contains(body, "/*")
}

// kotlinBracedBodyIsEmpty reports whether a control_structure_body is `{ }` holding no statement and no comment.
func kotlinBracedBodyIsEmpty(file *scanner.File, body uint32) bool {
	return controlBodyHasBraces(file, body) && bracedBodyTextIsEmpty(file, body) && !blockHasCommentFlat(file, body)
}

// bracedBodyTextIsEmpty reports whether the source between a body's outer braces is only whitespace
// and comments. The check reads text, not child nodes: a soft keyword used as an identifier, such as
// `{ internal }`, parses as a modifier with no `statements` node but is still code.
func bracedBodyTextIsEmpty(file *scanner.File, body uint32) bool {
	text := file.FlatNodeText(body)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return false
	}
	return strings.TrimSpace(stripComments(text[start+1:end])) == ""
}

func blockHasCommentFlat(file *scanner.File, idx uint32) bool {
	hasComment := false
	file.FlatWalkAllNodes(idx, func(n uint32) {
		if hasComment {
			return
		}
		switch file.FlatType(n) {
		case "line_comment", "multiline_comment", "block_comment", "comment":
			hasComment = true
		}
	})
	return hasComment
}

// javaMethodHasOverrideAnnotation reports whether a Java method_declaration
// carries an `@Override` annotation. The annotation is a structural AST node —
// `modifiers > (marker_annotation | annotation) > identifier "Override"` — so
// this matches only a real annotation, not the token `Override` appearing in a
// string literal, comment, or identifier elsewhere in the method text.
func javaMethodHasOverrideAnnotation(file *scanner.File, idx uint32) bool {
	mods, ok := file.FlatFindChild(idx, "modifiers")
	if !ok {
		return false
	}
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t != "marker_annotation" && t != "annotation" {
			continue
		}
		for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatType(gc) == "identifier" && file.FlatNodeTextEquals(gc, "Override") {
				return true
			}
		}
	}
	return false
}

// EmptyCatchBlockRule detects catch blocks with empty body.
type EmptyCatchBlockRule struct {
	FlatDispatchBase
	BaseRule
	AllowedExceptionNameRegex *regexp.Regexp // exception names matching this are allowed to be empty
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyCatchBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyClassBlockRule detects classes with empty body.
type EmptyClassBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyClassBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyDefaultConstructorRule detects explicit empty default constructors.
type EmptyDefaultConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyDefaultConstructorRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyDoWhileBlockRule detects do-while loops with empty body.
type EmptyDoWhileBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyDoWhileBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyElseBlockRule detects else blocks with empty body.
type EmptyElseBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyElseBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyFinallyBlockRule detects finally blocks with empty body.
type EmptyFinallyBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyFinallyBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyForBlockRule detects for loops with empty body.
type EmptyForBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyForBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyFunctionBlockRule detects functions with empty body.
type EmptyFunctionBlockRule struct {
	FlatDispatchBase
	BaseRule
	IgnoreOverridden bool // if true, skip override functions with empty body
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyFunctionBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyIfBlockRule detects if blocks with empty body.
type EmptyIfBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyIfBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyInitBlockRule detects init blocks with empty body.
type EmptyInitBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyInitBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyKotlinFileRule detects files with no meaningful code.
type EmptyKotlinFileRule struct {
	LineBase
	BaseRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 — the check walks the AST root for any non-package/import/
// comment child, which is a precise structural determination.
func (r *EmptyKotlinFileRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *EmptyKotlinFileRule) check(ctx *api.Context) {
	file := ctx.File
	if file == nil || file.FlatTree == nil {
		return
	}
	// Skip Spotless / format-tool template files (e.g. spotless/copyright.kt
	// spotless/copyright.kts). These are copyright-header templates with a
	// .kt/.kts extension for syntax highlighting, not real Kotlin source.
	if isSpotlessTemplateFile(file.Path) {
		return
	}
	explained := false
	for i := 0; i < file.FlatChildCount(0); i++ {
		child := file.FlatChild(0, i)
		t := file.FlatType(child)
		// Skip package, imports, and comments
		if t == "package_header" || t == "import_header" || t == "import_list" ||
			t == "line_comment" || t == "multiline_comment" {
			continue
		}
		// Any other node means the file has content
		return
	}
	// A comment saying the file is a deliberate placeholder is the file's content. The grammar may
	// nest a comment inside the package header, so every comment in the file counts.
	file.FlatWalkAllNodes(0, func(n uint32) {
		if t := file.FlatType(n); (t == "line_comment" || t == "multiline_comment") &&
			emptyFilePlaceholderCommentRe.MatchString(file.FlatNodeText(n)) {
			explained = true
		}
	})
	if explained {
		return
	}
	ctx.Emit(r.Finding(file, 1, 1, "Empty Kotlin file detected."))
}

var emptyFilePlaceholderCommentRe = regexp.MustCompile(`(?i)placeholder|intentionally|do not (delete|remove)|keep this file|this file (exists|is kept|is here)|deliberately`)

// isSpotlessTemplateFile reports whether the path looks like a Spotless
// copyright/license template — these files have a .kt or .kts extension
// but are template inputs for the Spotless formatter plugin, not source.
func isSpotlessTemplateFile(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	// Directory markers.
	if strings.Contains(p, "/spotless/") {
		base := strings.ToLower(filepathBase(p))
		if strings.HasPrefix(base, "copyright.") ||
			strings.HasPrefix(base, "license.") ||
			strings.HasPrefix(base, "header.") {
			return true
		}
	}
	return false
}

// filepathBase is a stdlib-free path basename helper to keep this file's
// imports unchanged.
func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// EmptySecondaryConstructorRule detects secondary constructors with empty body.
type EmptySecondaryConstructorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptySecondaryConstructorRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyTryBlockRule detects try blocks with empty body.
type EmptyTryBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyTryBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyWhenBlockRule detects when expressions with empty body.
type EmptyWhenBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyWhenBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }

// EmptyWhileBlockRule detects while loops with empty body.
type EmptyWhileBlockRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence holds the 0.95 dispatch default. Empty-block rule. Detection checks AST child count for
// statements/expressions inside a block — purely structural. No heuristic
// path. Classified per roadmap/17.
func (r *EmptyWhileBlockRule) Confidence() float64 { return api.ConfidenceVeryHigh }
