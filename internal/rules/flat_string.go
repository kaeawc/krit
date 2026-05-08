package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func stripKotlinComments(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	inLineComment := false
	inBlockComment := false
	inString := false
	escaped := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				b.WriteByte(ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(text) && text[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(text) {
			switch text[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}
		b.WriteByte(ch)
	}
	return b.String()
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
