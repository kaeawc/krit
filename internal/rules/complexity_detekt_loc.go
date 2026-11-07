package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// detektCodeLines counts the distinct lines that hold at least one non-comment token inside root, as
// detekt's linesOfCode does. A token counts on the line where it starts, but every line of a multi-line
// token (a raw string) counts. With skipLeadingEq the `=` of an expression body is not counted.
func detektCodeLines(file *scanner.File, root uint32, skipLeadingEq bool) int {
	if file == nil || file.FlatTree == nil || root == 0 {
		return 0
	}
	startRow := file.FlatRow(root)
	rows := make([]bool, strings.Count(file.FlatNodeText(root), "\n")+1)
	mark := func(from, to int) {
		for r := from; r <= to; r++ {
			if i := r - startRow; i >= 0 && i < len(rows) {
				rows[i] = true
			}
		}
	}
	var walk func(n uint32, top bool)
	walk = func(n uint32, top bool) {
		switch file.FlatType(n) {
		case "line_comment", "multiline_comment", "comment":
			return
		}
		first := file.FlatFirstChild(n)
		if first == 0 {
			if file.FlatEndByte(n) == file.FlatStartByte(n) {
				return
			}
			if skipLeadingEq && top && file.FlatType(n) == "=" {
				return
			}
			row := file.FlatRow(n)
			end := row
			if file.FlatEndByte(n)-file.FlatStartByte(n) > 1 {
				end += strings.Count(file.FlatNodeText(n), "\n")
			}
			mark(row, end)
			return
		}
		for c := first; c != 0; c = file.FlatNextSib(c) {
			walk(c, n == root)
		}
	}
	walk(root, false)
	count := 0
	for _, hit := range rows {
		if hit {
			count++
		}
	}
	return count
}

// isNamedFunctionNode reports a declaration detekt models as KtNamedFunction.
func isNamedFunctionNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "function_declaration", "anonymous_function":
		return true
	}
	return false
}

// functionBodyCodeLines is detekt's per-function size: the code lines of the body, minus the body lines of
// functions nested in it. A function without a body has no size.
func functionBodyCodeLines(file *scanner.File, fn uint32) (int, bool) {
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok {
		return 0, false
	}
	lines := detektCodeLines(file, body, true)
	var sub func(n uint32)
	sub = func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if isNamedFunctionNode(file, c) {
				// Measured against detekt 1.23.8: it removes the lines of the whole nested declaration,
				// signature included, not only its body.
				if _, ok := file.FlatFindChild(c, "function_body"); ok {
					lines -= detektCodeLines(file, c, false)
				}
				continue
			}
			sub(c)
		}
	}
	sub(body)
	return lines, true
}

// isClassOrObjectNode reports a declaration detekt models as KtClassOrObject.
func isClassOrObjectNode(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "class_declaration", "object_declaration", "companion_object", "object_literal":
		return true
	}
	return false
}

// classCodeLines is detekt's per-class size: the code lines of the whole declaration, minus the lines of
// classes and objects nested in it.
func classCodeLines(file *scanner.File, cls uint32) int {
	lines := detektCodeLines(file, cls, false)
	var sub func(n uint32)
	sub = func(n uint32) {
		for c := file.FlatFirstChild(n); c != 0; c = file.FlatNextSib(c) {
			if isClassOrObjectNode(file, c) {
				lines -= detektCodeLines(file, c, false)
				continue
			}
			sub(c)
		}
	}
	sub(cls)
	return lines
}

// detektFunctionName is the name detekt prints for a function, which is empty for anonymous ones.
func detektFunctionName(file *scanner.File, idx uint32) string {
	return extractIdentifierFlat(file, idx)
}

// detektClassName is the name detekt prints for a class or object. An unnamed companion is "Companion".
func detektClassName(file *scanner.File, idx uint32) string {
	name := extractIdentifierFlat(file, idx)
	if name == "" && file.FlatType(idx) == "companion_object" {
		return "Companion"
	}
	return name
}
