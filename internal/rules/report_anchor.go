package rules

import "github.com/kaeawc/krit/internal/scanner"

// declAnchor returns the node a user navigates to for a declaration: its name
// identifier, or its keyword when it has no name. Other nodes return idx.
// Skipping modifiers and annotations keeps the line off an annotation row.
func declAnchor(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return idx
	}
	switch file.FlatType(idx) {
	case "function_declaration", "class_declaration", "object_declaration",
		"property_declaration", "secondary_constructor", "companion_object",
		"type_alias", "anonymous_initializer", "enum_entry",
		"method_declaration", "constructor_declaration", "interface_declaration",
		"enum_declaration", "record_declaration", "annotation_type_declaration":
	default:
		return idx
	}
	java := file.Language == scanner.LangJava
	keyword := uint32(0)
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "modifiers", "annotation", "type_parameters", "comment", "line_comment", "multiline_comment":
			continue
		case "identifier":
			return c
		case "simple_identifier", "type_identifier":
			// Java return types are type_identifier nodes that precede the name.
			if !java {
				return c
			}
		case "variable_declaration", "multi_variable_declaration":
			for gc := file.FlatFirstChild(c); gc != 0; gc = file.FlatNextSib(gc) {
				if file.FlatType(gc) == "simple_identifier" {
					return gc
				}
			}
			return c
		}
		if keyword == 0 && !file.FlatIsNamed(c) {
			keyword = c
		}
	}
	if keyword != 0 {
		return keyword
	}
	return idx
}

// declLine returns the 1-based line of declAnchor(file, idx).
func declLine(file *scanner.File, idx uint32) int { return file.FlatRow(declAnchor(file, idx)) + 1 }

// declCol returns the 1-based column of declAnchor(file, idx).
func declCol(file *scanner.File, idx uint32) int { return file.FlatCol(declAnchor(file, idx)) + 1 }

// bangBangNode returns the `!!` operator token of a postfix_expression, or
// idx when there is none, so findings land on the operator itself.
func bangBangNode(file *scanner.File, idx uint32) uint32 {
	for c := file.FlatFirstChild(idx); c != 0; c = file.FlatNextSib(c) {
		if !file.FlatIsNamed(c) && file.FlatType(c) == "!!" {
			return c
		}
	}
	return idx
}

// callAnchor returns the callee name of a call_expression with a receiver, so
// the finding lands on the called method and not on the receiver or the first
// line of a chain. Bare calls and other nodes return idx.
func callAnchor(file *scanner.File, idx uint32) uint32 { return calleeNameAnchor(file, idx) }

// calleeNameAnchor returns the callee name of a call_expression even when the
// call starts on the same line as its receiver chain, so the finding lands on
// `.clickable` and not on the chain root. Other nodes return idx.
func calleeNameAnchor(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return idx
	}
	nav := file.FlatFirstChild(idx)
	if nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return idx
	}
	var last uint32
	for c := file.FlatFirstChild(nav); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) != "navigation_suffix" {
			continue
		}
		for gc := file.FlatFirstChild(c); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatType(gc) == "simple_identifier" {
				last = gc
			}
		}
	}
	return anchorOr(last, idx)
}

// calleeStartAnchor is calleeNameAnchor limited to calls that start on the same
// row as the callee, so a multi-line chain keeps its reported start line.
func calleeStartAnchor(file *scanner.File, idx uint32) uint32 {
	at := calleeNameAnchor(file, idx)
	if at != idx && file.FlatRow(at) != file.FlatRow(idx) {
		return idx
	}
	return at
}

// calleeLine returns the 1-based line of calleeStartAnchor(file, idx).
func calleeLine(file *scanner.File, idx uint32) int {
	return file.FlatRow(calleeStartAnchor(file, idx)) + 1
}

// calleeCol returns the 1-based column of calleeStartAnchor(file, idx).
func calleeCol(file *scanner.File, idx uint32) int {
	return file.FlatCol(calleeStartAnchor(file, idx)) + 1
}

// callLine returns the 1-based line of callAnchor(file, idx).
func callLine(file *scanner.File, idx uint32) int { return file.FlatRow(callAnchor(file, idx)) + 1 }

// callCol returns the 1-based column of callAnchor(file, idx).
func callCol(file *scanner.File, idx uint32) int { return file.FlatCol(callAnchor(file, idx)) + 1 }

// anchorOr returns node when it is set, otherwise fallback.
func anchorOr(node, fallback uint32) uint32 {
	if node != 0 {
		return node
	}
	return fallback
}

// symbolAnchor returns the 1-based position of an indexed symbol's name, or
// its recorded line at column 1 when the declaration node cannot be found.
func symbolAnchor(file *scanner.File, sym scanner.Symbol) (line, col int) {
	if node := deadCodeSymbolNode(sym, file); node != 0 {
		at := declAnchor(file, node)
		return file.FlatRow(at) + 1, file.FlatCol(at) + 1
	}
	return sym.Line, 1
}

// commentOffsetPosition maps a byte offset inside a node's text to a 1-based
// line and column in the file.
func commentOffsetPosition(file *scanner.File, node uint32, offset int) (line, col int) {
	abs := int(file.FlatStartByte(node)) + offset
	row := file.RowForByte(abs)
	return row + 1, abs - file.LineOffset(row) + 1
}

// withNodeSpan moves f onto node and gives it that node's byte span.
func withNodeSpan(f scanner.Finding, file *scanner.File, node uint32) scanner.Finding {
	f.Line, f.Col = file.FlatRow(node)+1, file.FlatCol(node)+1
	f.StartByte, f.EndByte = int(file.FlatStartByte(node)), int(file.FlatEndByte(node))
	return f
}

// catchKeywordNode returns the `catch` keyword of a catch clause, or the clause when the keyword is absent.
func catchKeywordNode(file *scanner.File, catch uint32) uint32 {
	for c := file.FlatFirstChild(catch); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "catch" {
			return c
		}
	}
	return catch
}
