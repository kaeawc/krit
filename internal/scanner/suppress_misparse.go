package scanner

import (
	"bytes"
	"strings"
)

// tree-sitter-kotlin ends a class_declaration at its name when a primary
// constructor starts on a later line or carries a modifier, as in
// `class X\n@Inject\nconstructor(...) : Y {`. The constructor and body then
// become sibling expressions. The helpers here recover the real end of such a
// class from the source bytes, so a class-level @Suppress still covers the body
// without moving any node offsets.

// suppressionTargetEnd returns the end byte of the declaration an annotation
// on target covers, including parts the grammar left outside target.
func suppressionTargetEnd(tree *FlatTree, target uint32, content []byte) int {
	end := int(tree.EndBytes[target])
	switch nodeTypeName(tree.Types[target]) {
	case "class_declaration":
		return max(end, misparsedClassEnd(tree, target, content))
	case "property_declaration":
		return propertyAccessorsEnd(tree, target)
	case "prefix_expression":
		// `@A data class X` followed by a detached constructor can parse as an
		// annotated expression that ends at the class name or inside the header.
		if nameEnd := classHeaderNameEnd(content, int(tree.StartBytes[target]), end); nameEnd > 0 {
			return max(end, scanDetachedConstructor(content, nameEnd))
		}
	}
	return end
}

// propertyAccessorsEnd returns the end of the property at idx extended over
// the getter and setter the grammar emits as its following siblings.
func propertyAccessorsEnd(tree *FlatTree, idx uint32) int {
	end := int(tree.EndBytes[idx])
	for sib := tree.NextSibs[idx]; sib != 0; sib = tree.NextSibs[sib] {
		switch nodeTypeName(tree.Types[sib]) {
		case "getter", "setter":
			end = int(tree.EndBytes[sib])
		case "line_comment", "multiline_comment":
		default:
			return end
		}
	}
	return end
}

// classHeaderNameEnd returns the offset after the class name when
// content[start:end] begins with annotations, modifiers and `class Name`, or
// -1 otherwise.
func classHeaderNameEnd(content []byte, start, end int) int {
	i := skipBlankAndComments(content, start)
	for i < end {
		switch {
		case content[i] == '@':
			i = skipAnnotationUse(content, i)
		case wordAt(content, i, "class"):
			i = skipBlankAndComments(content, i+len("class"))
			if i >= end || !isIdentStart(content[i]) {
				return -1
			}
			return skipWord(content, i)
		case isIdentStart(content[i]) && isClassModifier(content, i):
			i = skipWord(content, i)
		default:
			return -1
		}
		i = skipBlankAndComments(content, i)
	}
	return -1
}

var classModifierWords = []string{
	"data", "sealed", "open", "abstract", "final", "inner", "value", "annotation", "enum",
	"private", "internal", "protected", "public", "expect", "actual",
}

func isClassModifier(content []byte, i int) bool {
	for _, w := range classModifierWords {
		if wordAt(content, i, w) {
			return true
		}
	}
	return false
}

// misparsedClassEnd returns the end byte of the class at idx, extended over a
// constructor and body the grammar left outside the class_declaration.
func misparsedClassEnd(tree *FlatTree, idx uint32, content []byte) int {
	end := int(tree.EndBytes[idx])
	for c := tree.FirstChildren[idx]; c != 0; c = tree.NextSibs[c] {
		switch nodeTypeName(tree.Types[c]) {
		case "class_body", "primary_constructor", "enum_class_body":
			return end
		}
	}
	if ext := scanDetachedConstructor(content, end); ext > end {
		return ext
	}
	return end
}

// scanDetachedConstructor returns the offset after the constructor and body
// that start at or after pos, or pos when no detached constructor follows.
func scanDetachedConstructor(content []byte, pos int) int {
	i := skipBlankAndComments(content, pos)
	// Leading annotations and visibility modifiers, then the constructor keyword.
	for i < len(content) {
		switch {
		case content[i] == '@':
			i = skipAnnotationUse(content, i)
		case wordAt(content, i, "constructor"):
			return scanConstructorAndBody(content, i+len("constructor"), pos)
		case wordAt(content, i, "private"), wordAt(content, i, "internal"),
			wordAt(content, i, "protected"), wordAt(content, i, "public"):
			i = skipWord(content, i)
		default:
			return pos
		}
		i = skipBlankAndComments(content, i)
	}
	return pos
}

// detachedConstructorParamsEnd returns the offset after the parameter list of
// the constructor that starts at or after pos, or -1 when none follows.
func detachedConstructorParamsEnd(content []byte, pos int) int {
	i := skipBlankAndComments(content, pos)
	for i < len(content) {
		switch {
		case content[i] == '@':
			i = skipAnnotationUse(content, i)
		case wordAt(content, i, "constructor"):
			i = skipBlankAndComments(content, i+len("constructor"))
			if i >= len(content) || content[i] != '(' {
				return -1
			}
			if end, ok := matchBalanced(content, i); ok {
				return end
			}
			return -1
		case wordAt(content, i, "private"), wordAt(content, i, "internal"),
			wordAt(content, i, "protected"), wordAt(content, i, "public"):
			i = skipWord(content, i)
		default:
			return -1
		}
		i = skipBlankAndComments(content, i)
	}
	return -1
}

// constructorSuppressionSpan returns the byte range a suppression on a class
// constructor covers: the class header through the parameter list. It lets a
// finding about the constructor, which anchors on the class name, match. ok is
// false when target is not a constructor of a class.
func constructorSuppressionSpan(tree *FlatTree, target uint32, content []byte) (start, end int, ok bool) {
	switch nodeTypeName(tree.Types[target]) {
	case "primary_constructor":
		parent := tree.Parents[target]
		if nodeTypeName(tree.Types[parent]) != "class_declaration" {
			return 0, 0, false
		}
		return int(tree.StartBytes[parent]), int(tree.EndBytes[target]), true
	case "prefix_expression":
		// The grammar ends the class at its name and parses the annotated
		// constructor that follows as a sibling expression.
		outer := target
		for p := tree.Parents[outer]; p != outer && nodeTypeName(tree.Types[p]) == "prefix_expression"; p = tree.Parents[outer] {
			outer = p
		}
		prev := uint32(0)
		for c := tree.FirstChildren[tree.Parents[outer]]; c != 0 && c != outer; c = tree.NextSibs[c] {
			if !isCommentNode(tree, c) {
				prev = c
			}
		}
		if prev == 0 || nodeTypeName(tree.Types[prev]) != "class_declaration" {
			return 0, 0, false
		}
		for c := tree.FirstChildren[prev]; c != 0; c = tree.NextSibs[c] {
			switch nodeTypeName(tree.Types[c]) {
			case "class_body", "primary_constructor", "enum_class_body":
				return 0, 0, false
			}
		}
		paramsEnd := detachedConstructorParamsEnd(content, int(tree.EndBytes[prev]))
		if paramsEnd < 0 {
			return 0, 0, false
		}
		return int(tree.StartBytes[prev]), paramsEnd, true
	}
	return 0, 0, false
}

func scanConstructorAndBody(content []byte, i, fallback int) int {
	i = skipBlankAndComments(content, i)
	if i >= len(content) || content[i] != '(' {
		return fallback
	}
	end, ok := matchBalanced(content, i)
	if !ok {
		return fallback
	}
	afterParams := end
	i = skipBlankAndComments(content, end)
	if i < len(content) && content[i] == ':' {
		i = scanSupertypesToBody(content, i+1)
	}
	if i < 0 || i >= len(content) || content[i] != '{' {
		return afterParams
	}
	if bodyEnd, ok := matchBalanced(content, i); ok {
		return bodyEnd
	}
	return afterParams
}

// scanSupertypesToBody returns the offset of the body's opening brace, or -1
// when a new declaration starts first (the class has no body).
func scanSupertypesToBody(content []byte, i int) int {
	depth := 0
	for i < len(content) {
		i = skipBlankAndComments(content, i)
		if i >= len(content) {
			break
		}
		switch c := content[i]; {
		case c == '(' || c == '<' || c == '[':
			depth++
			i++
		case c == ')' || c == '>' || c == ']':
			depth--
			i++
		case c == '{' && depth <= 0:
			return i
		case c == '"' || c == '\'':
			next, ok := skipLiteral(content, i)
			if !ok {
				return -1
			}
			i = next
		case isIdentStart(c):
			if depth <= 0 && isDeclarationStart(content, i) {
				return -1
			}
			i = skipWord(content, i)
		default:
			i++
		}
	}
	return -1
}

var declarationStartWords = []string{
	"fun", "val", "var", "class", "interface", "object", "typealias", "import", "package",
	"enum", "data", "sealed", "abstract", "open", "private", "internal", "protected", "public",
}

func isDeclarationStart(content []byte, i int) bool {
	for _, w := range declarationStartWords {
		if wordAt(content, i, w) {
			return true
		}
	}
	return false
}

// skipAnnotationUse skips `@Name`, an optional `.Name` qualifier and an
// optional argument list.
func skipAnnotationUse(content []byte, i int) int {
	i++
	for i < len(content) && (isIdentPart(content[i]) || content[i] == '.' || content[i] == ':') {
		i++
	}
	if i < len(content) && content[i] == '(' {
		if end, ok := matchBalanced(content, i); ok {
			return end
		}
	}
	return i
}

// matchBalanced returns the offset after the bracket that closes the one at
// open, skipping strings, character literals and comments.
func matchBalanced(content []byte, open int) (int, bool) {
	depth := 0
	for i := open; i < len(content); {
		switch c := content[i]; c {
		case '(', '{', '[':
			depth++
			i++
		case ')', '}', ']':
			depth--
			i++
			if depth == 0 {
				return i, true
			}
		case '"', '\'', '/':
			next, ok := skipLiteral(content, i)
			if !ok {
				i++
				continue
			}
			i = next
		default:
			i++
		}
	}
	return 0, false
}

// skipLiteral skips a string, character literal or comment starting at i.
// ok is false when the byte at i does not start one.
func skipLiteral(content []byte, i int) (int, bool) {
	switch content[i] {
	case '/':
		return skipComment(content, i)
	case '\'':
		return skipCharLiteral(content, i)
	case '"':
		return skipStringLiteral(content, i)
	}
	return i, false
}

// skipComment skips a line or nested block comment starting at the '/' at i.
func skipComment(content []byte, i int) (int, bool) {
	if i+1 >= len(content) {
		return i, false
	}
	switch content[i+1] {
	case '/':
		for i < len(content) && content[i] != '\n' {
			i++
		}
		return i, true
	case '*':
		depth := 0
		for i+1 < len(content) {
			switch {
			case content[i] == '/' && content[i+1] == '*':
				depth++
				i += 2
			case content[i] == '*' && content[i+1] == '/':
				depth--
				i += 2
				if depth == 0 {
					return i, true
				}
			default:
				i++
			}
		}
		return len(content), true
	}
	return i, false
}

// skipCharLiteral skips a character literal starting at the quote at i.
func skipCharLiteral(content []byte, i int) (int, bool) {
	for j := i + 1; j < len(content) && content[j] != '\n'; j++ {
		if content[j] == '\\' {
			j++
			continue
		}
		if content[j] == '\'' {
			return j + 1, true
		}
	}
	return i, false
}

// skipStringLiteral skips a single-line or raw string literal starting at the quote at i.
func skipStringLiteral(content []byte, i int) (int, bool) {
	if i+2 < len(content) && content[i+1] == '"' && content[i+2] == '"' {
		for j := i + 3; j+2 < len(content); j++ {
			if content[j] == '"' && content[j+1] == '"' && content[j+2] == '"' {
				return j + 3, true
			}
		}
		return len(content), true
	}
	for j := i + 1; j < len(content) && content[j] != '\n'; j++ {
		switch content[j] {
		case '\\':
			j++
		case '"':
			return j + 1, true
		}
	}
	return i, false
}

func skipBlankAndComments(content []byte, i int) int {
	for i < len(content) {
		switch content[i] {
		case ' ', '\t', '\n', '\r':
			i++
		case '/':
			next, ok := skipLiteral(content, i)
			if !ok {
				return i
			}
			i = next
		default:
			return i
		}
	}
	return i
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

func skipWord(content []byte, i int) int {
	for i < len(content) && isIdentPart(content[i]) {
		i++
	}
	return i
}

// wordAt reports whether the identifier at i is exactly word.
func wordAt(content []byte, i int, word string) bool {
	end := i + len(word)
	if end > len(content) || string(content[i:end]) != word {
		return false
	}
	return (i == 0 || !isIdentPart(content[i-1])) && (end == len(content) || !isIdentPart(content[end]))
}

// Inside a misparsed body an annotation can be split from the declaration it
// annotates: `@Suppress("X")` may become a bare `@Suppress` followed by a
// parenthesized expression, either inside a prefix_expression or as sibling
// statements, and the declaration follows as the next statement.

// detachedArgsEnd returns the end of the argument list parsed as the node
// right after the bare annotation ann, or -1 when there is none.
func detachedArgsEnd(tree *FlatTree, ann uint32, content []byte) int {
	if nodeTypeName(tree.Types[ann]) != "annotation" || flatAnnotationHasArgs(tree, ann) {
		return -1
	}
	next := tree.NextSibs[ann]
	annEnd := int(tree.EndBytes[ann])
	if next == 0 || int(tree.StartBytes[next]) != annEnd || annEnd >= len(content) || content[annEnd] != '(' {
		return -1
	}
	end, ok := matchBalanced(content, annEnd)
	if !ok || end != int(tree.EndBytes[next]) {
		return -1
	}
	return end
}

// isDetachedArgs reports whether the node at idx is the argument list of the
// bare annotation right before it.
func isDetachedArgs(tree *FlatTree, idx uint32, content []byte) bool {
	parent := tree.Parents[idx]
	var prev uint32
	for c := tree.FirstChildren[parent]; c != 0 && c != idx; c = tree.NextSibs[c] {
		prev = c
	}
	return prev != 0 && detachedArgsEnd(tree, prev, content) == int(tree.EndBytes[idx])
}

func isCommentNode(tree *FlatTree, idx uint32) bool {
	switch nodeTypeName(tree.Types[idx]) {
	case "line_comment", "multiline_comment":
		return true
	}
	return false
}

// isAnnotationOnly reports whether the node at idx holds only annotations and
// comments, so it annotates whatever statement follows it.
func isAnnotationOnly(tree *FlatTree, idx uint32, content []byte) bool {
	switch nodeTypeName(tree.Types[idx]) {
	case "annotation":
		return detachedArgsEnd(tree, idx, content) > 0
	case "prefix_expression":
	default:
		return false
	}
	for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
		switch {
		case nodeTypeName(tree.Types[child]) == "annotation", isCommentNode(tree, child):
		case nodeTypeName(tree.Types[child]) == "prefix_expression" && isAnnotationOnly(tree, child, content):
		case isDetachedArgs(tree, child, content):
		default:
			return false
		}
	}
	return true
}

// nextStatementSibling returns the statement that the annotation-only node at
// idx annotates: the next sibling that is not a comment, annotation or
// detached argument list, looking through enclosing annotation-only
// prefix_expressions. It returns 0 unless that statement sits in a statement
// list.
func nextStatementSibling(tree *FlatTree, idx uint32, content []byte) uint32 {
	for {
		for sib := tree.NextSibs[idx]; sib != 0; sib = tree.NextSibs[sib] {
			if isCommentNode(tree, sib) || isDetachedArgs(tree, sib, content) ||
				nodeTypeName(tree.Types[sib]) == "annotation" ||
				nodeTypeName(tree.Types[sib]) == "prefix_expression" && isAnnotationOnly(tree, sib, content) {
				continue
			}
			if !isStatementList(tree, tree.Parents[sib]) {
				return 0
			}
			return sib
		}
		parent := tree.Parents[idx]
		if parent == idx {
			return 0
		}
		switch {
		case nodeTypeName(tree.Types[parent]) == "prefix_expression" && isAnnotationOnly(tree, parent, content):
			idx = parent
		case endsDeclarationBody(tree, parent, idx):
			// `fun f() =\n @A\n try {}` can leave the expression as the declaration's next sibling.
			return expressionAfterDeclaration(tree, parent)
		default:
			return 0
		}
	}
}

// endsDeclarationBody reports whether parent is a function body or property
// that ends exactly where the annotation-only node at idx ends.
func endsDeclarationBody(tree *FlatTree, parent, idx uint32) bool {
	switch nodeTypeName(tree.Types[parent]) {
	case "function_body", "property_declaration":
		return tree.EndBytes[parent] == tree.EndBytes[idx]
	}
	return false
}

// expressionAfterDeclaration returns the expression statement that follows the
// declaration holding body, or 0 when the next statement is not an expression.
func expressionAfterDeclaration(tree *FlatTree, body uint32) uint32 {
	decl := body
	if nodeTypeName(tree.Types[body]) == "function_body" {
		decl = tree.Parents[body]
		if nodeTypeName(tree.Types[decl]) != "function_declaration" || tree.EndBytes[decl] != tree.EndBytes[body] {
			return 0
		}
	}
	for sib := tree.NextSibs[decl]; sib != 0; sib = tree.NextSibs[sib] {
		if isCommentNode(tree, sib) {
			continue
		}
		if !isStatementList(tree, tree.Parents[sib]) || strings.HasSuffix(nodeTypeName(tree.Types[sib]), "_declaration") {
			return 0
		}
		return sib
	}
	return 0
}

// splitReturnOperand returns the statement the grammar detached from an
// annotated bare `return` or `return@label` on the same line, as in
// `@A return f(\n *x,\n)`. It returns 0 when idx is not that shape.
func splitReturnOperand(tree *FlatTree, idx uint32, content []byte) uint32 {
	if nodeTypeName(tree.Types[idx]) != "prefix_expression" {
		return 0
	}
	jump := uint32(0)
	for child := tree.FirstChildren[idx]; child != 0; child = tree.NextSibs[child] {
		if nodeTypeName(tree.Types[child]) == "jump_expression" {
			jump = child
		}
	}
	if jump == 0 || tree.NextSibs[jump] != 0 || !isBareReturn(tree, jump) {
		return 0
	}
	jumpEnd := int(tree.EndBytes[jump])
	for sib := tree.NextSibs[idx]; sib != 0; sib = tree.NextSibs[sib] {
		if isCommentNode(tree, sib) {
			continue
		}
		start := int(tree.StartBytes[sib])
		if start < jumpEnd || bytes.IndexByte(content[jumpEnd:start], '\n') >= 0 || !isStatementList(tree, tree.Parents[sib]) {
			return 0
		}
		return sib
	}
	return 0
}

// isBareReturn reports whether the jump_expression holds only a return keyword
// and an optional label.
func isBareReturn(tree *FlatTree, jump uint32) bool {
	first := true
	for child := tree.FirstChildren[jump]; child != 0; child = tree.NextSibs[child] {
		switch nodeTypeName(tree.Types[child]) {
		case "return", "return@":
			if !first {
				return false
			}
		case "label":
		default:
			return false
		}
		first = false
	}
	return !first
}

func isStatementList(tree *FlatTree, idx uint32) bool {
	switch nodeTypeName(tree.Types[idx]) {
	case "statements", "class_body", "source_file":
		return true
	}
	return false
}
