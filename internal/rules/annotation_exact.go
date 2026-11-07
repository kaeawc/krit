package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// hasExactAnnotationFlat reports whether the declaration at idx carries the annotation annotationName.
// Unlike hasAnnotationFlat it matches whole names, so `@DaoHelper` does not match `Dao`, and it accepts
// package-qualified (`@androidx.room.Dao`) and use-site-targeted (`@get:Dao`) uses.
func hasExactAnnotationFlat(file *scanner.File, idx uint32, annotationName string) bool {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "modifiers" && annotationTextMentions(file.FlatNodeText(child), annotationName) {
			return true
		}
	}
	// Some grammars emit the modifier list as the preceding sibling.
	for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "modifiers", "annotation":
			if annotationTextMentions(file.FlatNodeText(prev), annotationName) {
				return true
			}
		case "line_comment", "multiline_comment", "block_comment":
		default:
			return false
		}
	}
	return false
}

// findExactAnnotationTextFlat returns the source text of the annotation annotationName on idx, matching
// whole names as hasExactAnnotationFlat does, or "" when it is absent.
func findExactAnnotationTextFlat(file *scanner.File, idx uint32, annotationName string) string {
	if file == nil || idx == 0 {
		return ""
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if text := findExactAnnotationTextInParent(file, mods, annotationName); text != "" {
			return text
		}
	}
	return findExactAnnotationTextInParent(file, idx, annotationName)
}

func findExactAnnotationTextInParent(file *scanner.File, parent uint32, annotationName string) string {
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		switch file.FlatType(child) {
		case "annotation", "modifier":
			text := strings.TrimSpace(file.FlatNodeText(child))
			if annotationTextMentions(text, annotationName) {
				return text
			}
		}
	}
	return ""
}

// annotationTextMentions reports whether text contains an annotation use of
// annotationName: `@Name`, `@pkg.Name` or `@target:Name`, where Name ends at a
// non-identifier character. A dotted annotationName such as `Outer.Inner` matches the
// trailing segments. String literal contents are ignored.
func annotationTextMentions(text, annotationName string) bool {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '"':
			i = skipQuotedLiteral(text, i)
		case '@':
			j := i + 1
			chain := ""
			for j < len(text) {
				start := j
				for j < len(text) && isIdentChar(text[j]) {
					j++
				}
				if j == start {
					break
				}
				if j < len(text) && text[j] == ':' && chain == "" {
					// Use-site target such as `@get:`.
					j++
					continue
				}
				chain += text[start:j]
				if j < len(text) && text[j] == '.' {
					chain += "."
					j++
					continue
				}
				break
			}
			if chain == annotationName || strings.HasSuffix(chain, "."+annotationName) {
				return true
			}
		}
	}
	return false
}

// skipQuotedLiteral returns the index of the closing quote of the string
// literal that starts at text[start], or len(text)-1 when it is unterminated.
func skipQuotedLiteral(text string, start int) int {
	if strings.HasPrefix(text[start:], `"""`) {
		if end := strings.Index(text[start+3:], `"""`); end >= 0 {
			return start + 3 + end + 2
		}
		return len(text) - 1
	}
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return len(text) - 1
}
