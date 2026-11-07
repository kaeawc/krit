package rules

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// Helpers kept from the previous SwallowedException implementation; other rules use them.

// catchRethrowFix appends `throw <caught>` before the closing brace of a catch block. It returns nil when the
// block does not end in `}` or the caught variable has no usable name.
func catchRethrowFix(file *scanner.File, catchNode uint32, caughtVar string) *scanner.Fix {
	if caughtVar == "" || caughtVar == "_" {
		return nil
	}
	endByte := int(file.FlatEndByte(catchNode))
	if endByte <= 0 || endByte > len(file.Content) || file.Content[endByte-1] != '}' {
		return nil
	}
	closeBrace := endByte - 1
	lineStart := closeBrace
	for lineStart > 0 && (file.Content[lineStart-1] == ' ' || file.Content[lineStart-1] == '\t') {
		lineStart--
	}
	indent := detectIndent(file.Content, int(file.FlatStartByte(catchNode)))
	replacement := "    throw " + caughtVar + "\n" + indent + "}"
	if lineStart > 0 && file.Content[lineStart-1] != '\n' {
		// The brace follows code on the same line, so start a new line for the throw.
		replacement = "\n" + indent + replacement
	}
	return &scanner.Fix{ByteMode: true, StartByte: closeBrace, EndByte: endByte, Replacement: replacement}
}

func swallowedPropertyDeclarationName(file *scanner.File, prop uint32) string {
	if decl, ok := file.FlatFindChild(prop, "variable_declaration"); ok {
		if ident, ok := file.FlatFindChild(decl, "simple_identifier"); ok {
			return file.FlatNodeString(ident, nil)
		}
	}
	return ""
}

func swallowedPropertyInitializer(file *scanner.File, prop uint32) uint32 {
	afterEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			afterEquals = true
			continue
		}
		if afterEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func swallowedJumpExpressionValue(file *scanner.File, node uint32) uint32 {
	seenKeyword := false
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if !seenKeyword {
			t := file.FlatType(child)
			seenKeyword = t == "throw" || swallowedIsReturnKeyword(t)
			continue
		}
		// Skip the `label` node that follows a `return@` token; the returned
		// value is the first *other* named child.
		if file.FlatType(child) == "label" {
			continue
		}
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

// swallowedIsReturnKeyword matches both bare and labeled return tokens
// (`return` and `return@label`, the latter parsed as a `return@` token).
func swallowedIsReturnKeyword(t string) bool {
	return t == "return" || t == "return@"
}

func swallowedHasNamedStatement(file *scanner.File, statements uint32) bool {
	for child := file.FlatFirstChild(statements); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return true
		}
	}
	return false
}
