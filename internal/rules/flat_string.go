package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// stripKotlinComments removes Kotlin line and block comments from text
// while preserving string-literal content. Triple-quoted raw strings
// are recognized as a single delimiter pair so that embedded `"`, `//`,
// and `/* */` inside the raw body are not misinterpreted as code or
// comments.
func stripKotlinComments(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	n := len(text)
	for i := 0; i < n; {
		ch := text[i]
		if ch == '/' && i+1 < n && text[i+1] == '/' {
			i = skipLineComment(text, i)
			continue
		}
		if ch == '/' && i+1 < n && text[i+1] == '*' {
			i = skipBlockComment(text, i, &b)
			continue
		}
		if ch == '"' && i+2 < n && text[i+1] == '"' && text[i+2] == '"' {
			i = copyRawString(text, i, &b)
			continue
		}
		if ch == '"' {
			i = copyLineString(text, i, &b)
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func skipLineComment(text string, i int) int {
	for i < len(text) && text[i] != '\n' {
		i++
	}
	return i
}

func skipBlockComment(text string, i int, b *strings.Builder) int {
	n := len(text)
	i += 2
	for i+1 < n && (text[i] != '*' || text[i+1] != '/') {
		if text[i] == '\n' {
			b.WriteByte('\n')
		}
		i++
	}
	if i+1 < n {
		return i + 2
	}
	// Unterminated block comment: still preserve a trailing newline so
	// downstream line counts stay stable.
	for ; i < n; i++ {
		if text[i] == '\n' {
			b.WriteByte('\n')
		}
	}
	return n
}

func copyRawString(text string, i int, b *strings.Builder) int {
	n := len(text)
	b.WriteString(`"""`)
	i += 3
	for i+2 < n && (text[i] != '"' || text[i+1] != '"' || text[i+2] != '"') {
		b.WriteByte(text[i])
		i++
	}
	if i+2 < n {
		b.WriteString(`"""`)
		return i + 3
	}
	for ; i < n; i++ {
		b.WriteByte(text[i])
	}
	return n
}

func copyLineString(text string, i int, b *strings.Builder) int {
	n := len(text)
	b.WriteByte('"')
	i++
	for i < n {
		c := text[i]
		if c == '\\' && i+1 < n {
			b.WriteByte(c)
			b.WriteByte(text[i+1])
			i += 2
			continue
		}
		b.WriteByte(c)
		i++
		if c == '"' || c == '\n' {
			break
		}
	}
	return i
}

func flatContainsStringInterpolation(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "interpolated_identifier", "interpolated_expression",
			"line_string_expression", "multi_line_string_expression",
			"line_str_ref", "multi_line_str_ref":
			found = true
		}
	})
	return found
}

// stringLiteralContent returns the concatenated text of every
// string_content child under a string_literal node, which is the
// runtime value of a non-interpolated string. Callers should first
// verify there is no interpolation (flatContainsStringInterpolation)
// since this ignores interpolated segments entirely.
func stringLiteralContent(file *scanner.File, idx uint32) string {
	if file == nil || file.FlatType(idx) != "string_literal" {
		return ""
	}
	var b strings.Builder
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "string_content" || file.FlatType(child) == "string_fragment" {
			b.WriteString(file.FlatNodeText(child))
		}
	}
	if b.Len() == 0 {
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if unquoted, err := strconv.Unquote(text); err == nil {
			return unquoted
		}
		if len(text) >= 6 && strings.HasPrefix(text, `"""`) && strings.HasSuffix(text, `"""`) {
			return text[3 : len(text)-3]
		}
	}
	return b.String()
}

// stringLiteralIsRaw returns true when the string_literal node is a
// triple-quoted raw string (`"""..."""`). Raw strings don't process
// backslash escapes, so rules that analyze escape sequences should
// skip them.
func stringLiteralIsRaw(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "string_literal" {
		return false
	}
	start := file.FlatStartByte(idx)
	if int(start)+3 > len(file.Content) {
		return false
	}
	return file.Content[start] == '"' && file.Content[start+1] == '"' && file.Content[start+2] == '"'
}

// infixLeftStringLiteralContent returns the content of the left-hand
// string_literal of an infix_expression like `"key" to value`, or "" if
// the left operand is not a non-interpolated string.
func infixLeftStringLiteralContent(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 || file.FlatType(idx) != "infix_expression" {
		return ""
	}
	left := file.FlatFirstChild(idx)
	for left != 0 && !file.FlatIsNamed(left) {
		left = file.FlatNextSib(left)
	}
	if left == 0 || file.FlatType(left) != "string_literal" {
		return ""
	}
	if flatContainsStringInterpolation(file, left) {
		return ""
	}
	return stringLiteralContent(file, left)
}
