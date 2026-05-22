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
	if e < len(content) && content[e] == '\n' {
		e++
	}
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
	// Skip Spotless / format-tool template files (e.g. spotless/copyright.kt
	// spotless/copyright.kts). These are copyright-header templates with a
	// .kt/.kts extension for syntax highlighting, not real Kotlin source.
	if isSpotlessTemplateFile(file.Path) {
		return
	}
	if file == nil || file.FlatTree == nil {
		return
	}
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
	ctx.Emit(r.Finding(file, 1, 1, "Empty Kotlin file detected."))
}

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
