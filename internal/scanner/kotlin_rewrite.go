package scanner

import (
	"bytes"
)

// The vendored tree-sitter-kotlin grammar predates or mishandles several
// syntax forms. kotlinRewriter rewrites the offending tokens in a copy of the
// source before parsing. Every rewrite keeps the byte length, so offsets in
// the parsed tree match the original file. Rows and columns are recomputed
// from the original source afterwards (see restorePositions).
//
// Rewrites only touch bytes the lexer classifies as code, never string,
// comment or backtick contents.

// kotlinRewriter holds a lazily copied working buffer plus follow-up work the
// tree needs after parsing.
type kotlinRewriter struct {
	orig []byte
	out  []byte
	lx   *kotlinLexer

	// prefixes are blanked token runs that belong to the declaration that
	// follows them (`fun` of `fun interface`, a `context(...)` clause).
	prefixes []declPrefix
	// delegations are `by expr` clauses blanked so the class or object body
	// parses; the tree gets them back after parsing.
	delegations []delegationRepair
	// parenFixes are call-chain assignment targets wrapped in parentheses.
	parenFixes []parenFix
	// semis are the offsets of ';' bytes that replaced whitespace.
	semis []uint32
	// signCtx is built when a line starts with `+` or `-`.
	signCtx *signLineContext
}

// parenFix records `(` at open and `)` at close put around an assignment
// target that starts at start.
type parenFix struct {
	open, close, start uint32
	kind               parenFixKind
}

type parenFixKind uint8

const (
	// parenFixAssignment wraps the target of an assignment.
	parenFixAssignment parenFixKind = iota
	// parenFixExpression wraps a plain expression.
	parenFixExpression
)

// declPrefix is a blanked span [start, end) that the declaration beginning at
// declStart should cover.
type declPrefix struct {
	start, end, declStart uint32
	// context marks a `context(...)` clause whose parameters repairTree
	// rebuilds as a context_parameters node.
	context bool
	grafted bool
}

// ContextParametersNodeType is the krit-specific node type that holds the
// named parameters of a `context(...)` clause as `parameter` children. It is
// the first child of the declaration the clause belongs to.
const ContextParametersNodeType = "context_parameters"

// delegationRepair describes a blanked `by expr` clause. typeEnd is the end
// of the delegated supertype; [byStart, exprEnd) is the clause.
type delegationRepair struct {
	typeEnd, byStart, exprStart, exprEnd uint32
}

// rewriteKotlin returns src with grammar workarounds applied, or src itself
// when nothing changed.
func rewriteKotlin(src []byte) ([]byte, *kotlinRewriter) {
	r := &kotlinRewriter{orig: src}
	r.run()
	if r.out == nil {
		return src, nil
	}
	return r.out, r
}

func (r *kotlinRewriter) run() {
	o := r.orig
	needLex := hasNonASCII(o)
	for _, w := range [][]byte{[]byte("..<"), []byte("0b"), []byte("0B"), []byte("interface"), []byte("context"), []byte("$$"),
		[]byte("(val "), []byte("(var "), []byte("[val "), []byte("[var "), []byte("val ["), []byte("var ["), []byte("{ ["),
		[]byte("class "), []byte("object")} {
		if bytes.Contains(o, w) {
			needLex = true
			break
		}
	}
	if needLex {
		r.lexer()
		r.rewriteTokens()
		r.rewriteHeaders()
	}
	r.rewriteCommasAndRefs()
	r.rewriteModifierIdentifiers()
	r.rewriteGenericCallableRefs()
	r.rewriteFunctionTypeReceivers()
	r.rewriteLines()
	if r.lx != nil {
		for _, e := range r.lx.edits {
			r.set(e.pos, e.b)
		}
	}
}

func (r *kotlinRewriter) lexer() *kotlinLexer {
	if r.lx == nil {
		r.lx = lexKotlin(r.orig)
	}
	return r.lx
}

// set writes b at i in the working copy.
func (r *kotlinRewriter) set(i int, b byte) {
	if r.out == nil {
		r.out = make([]byte, len(r.orig))
		copy(r.out, r.orig)
	}
	r.out[i] = b
}

// blank replaces [from, to) with spaces, keeping line breaks.
func (r *kotlinRewriter) blank(from, to int) {
	for i := from; i < to && i < len(r.orig); i++ {
		if r.orig[i] != '\n' && r.orig[i] != '\r' {
			r.set(i, ' ')
		}
	}
}

// isCode reports whether byte i is outside strings, comments and backticks.
func (r *kotlinRewriter) isCode(i int) bool {
	return i >= 0 && i < len(r.orig) && r.lexer().mask[i]
}

// wordAt reports whether the identifier word w starts at i in code.
func (r *kotlinRewriter) wordAt(i int, w string) bool {
	o := r.orig
	if i < 0 || i+len(w) > len(o) || string(o[i:i+len(w)]) != w {
		return false
	}
	if i > 0 && isIdentByte(o[i-1]) {
		return false
	}
	if i+len(w) < len(o) && isIdentByte(o[i+len(w)]) {
		return false
	}
	return r.isCode(i)
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func (r *kotlinRewriter) skipWS(i int) int {
	for i < len(r.orig) && isSpaceByte(r.orig[i]) {
		i++
	}
	return i
}

// prevCode returns the index of the last non-space byte before i, or -1.
func (r *kotlinRewriter) prevNonSpace(i int) int {
	for i--; i >= 0; i-- {
		if !isSpaceByte(r.orig[i]) {
			return i
		}
	}
	return -1
}

// skipBalanced returns the index after the bracket matching the one at i, or
// -1. Only code bytes count.
func (r *kotlinRewriter) skipBalanced(i int, open, closing byte) int {
	depth := 0
	for j := i; j < len(r.orig); j++ {
		if !r.isCode(j) {
			continue
		}
		switch r.orig[j] {
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return j + 1
			}
		}
	}
	return -1
}

// skipAngle returns the index after the '>' matching the '<' at i, or -1
// when the text does not look like type arguments.
func (r *kotlinRewriter) skipAngle(i int) int {
	depth := 0
	for j := i; j < len(r.orig) && j-i < 400; j++ {
		if !r.isCode(j) {
			continue
		}
		switch c := r.orig[j]; c {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return j + 1
			}
		case '-':
			if j+1 < len(r.orig) && r.orig[j+1] == '>' {
				j++
			}
		case '{', '}', ';', '=':
			return -1
		}
	}
	return -1
}

// rewriteTokens applies the token-level rewrites that need no context.
func (r *kotlinRewriter) rewriteTokens() {
	o := r.orig
	// `..<` (rangeUntil) becomes `.. `.
	for pos := 0; ; {
		k := bytes.Index(o[pos:], []byte("..<"))
		if k < 0 {
			break
		}
		at := pos + k
		pos = at + 3
		if r.isCode(at) {
			r.set(at+2, ' ')
		}
	}
	// Binary literals `0b101` become `0x101`.
	for _, pfx := range []string{"0b", "0B"} {
		for pos := 0; ; {
			k := bytes.Index(o[pos:], []byte(pfx))
			if k < 0 {
				break
			}
			at := pos + k
			pos = at + 2
			if at > 0 && (isIdentByte(o[at-1]) || o[at-1] == '.') {
				continue
			}
			if at+2 < len(o) && (o[at+2] == '0' || o[at+2] == '1') && r.isCode(at) {
				r.set(at+1, 'x')
			}
		}
	}
	// Non-ASCII bytes in code can only be identifier characters. The grammar
	// rejects some letters, so they all become 'a'.
	for i, b := range o {
		if b >= 0x80 && r.isCode(i) {
			r.set(i, 'a')
		}
	}
	// Multi-dollar string prefixes `$$"` lose the dollars.
	for _, sp := range r.lexer().dollarPrefixes {
		r.blank(sp[0], sp[1])
	}
	r.rewriteFunInterface()
	r.rewriteContextClauses()
	r.rewriteNameBasedDestructuring()
}

// rewriteFunInterface blanks the `fun` of `fun interface`, which the grammar
// cannot parse. The interface declaration is widened to cover it afterwards.
func (r *kotlinRewriter) rewriteFunInterface() {
	o := r.orig
	for pos := 0; ; {
		k := bytes.Index(o[pos:], []byte("interface"))
		if k < 0 {
			break
		}
		at := pos + k
		pos = at + len("interface")
		if !r.wordAt(at, "interface") {
			continue
		}
		p := r.prevNonSpace(at)
		if p < 2 || !r.wordAt(p-2, "fun") || at == p+1 {
			continue
		}
		r.blank(p-2, p+1)
		r.prefixes = append(r.prefixes, declPrefix{start: uint32(p - 2), end: uint32(p + 1), declStart: uint32(at)})
	}
}

var contextFollowWords = map[string]bool{
	"fun": true, "class": true, "interface": true, "object": true, "val": true, "var": true, "typealias": true,
}

var contextModifierWords = map[string]bool{
	"private": true, "public": true, "internal": true, "protected": true, "abstract": true, "open": true,
	"final": true, "override": true, "suspend": true, "inline": true, "operator": true, "infix": true,
	"tailrec": true, "external": true, "data": true, "sealed": true, "enum": true, "annotation": true,
	"value": true, "inner": true, "lateinit": true, "const": true, "expect": true, "actual": true,
}

// rewriteContextClauses blanks a `context(...)` clause that starts a line and
// precedes a declaration. The grammar reads it as a call expression.
func (r *kotlinRewriter) rewriteContextClauses() {
	o := r.orig
	for pos := 0; ; {
		k := bytes.Index(o[pos:], []byte("context"))
		if k < 0 {
			break
		}
		at := pos + k
		pos = at + len("context")
		if !r.wordAt(at, "context") {
			continue
		}
		if ls := lineStartOf(o, at); len(bytes.TrimSpace(o[ls:at])) != 0 {
			continue
		}
		open := r.skipWS(at + len("context"))
		if open >= len(o) || o[open] != '(' {
			continue
		}
		end := r.skipBalanced(open, '(', ')')
		if end < 0 {
			continue
		}
		declStart := r.skipWS(end)
		if !r.startsDeclaration(declStart) {
			continue
		}
		r.blank(at, end)
		r.prefixes = append(r.prefixes, declPrefix{start: uint32(at), end: uint32(end), declStart: uint32(declStart), context: true})
		pos = end
	}
}

// startsDeclaration reports whether modifiers and annotations starting at i
// lead to a declaration keyword.
func (r *kotlinRewriter) startsDeclaration(i int) bool {
	o := r.orig
	for n := 0; n < 24 && i < len(o); n++ {
		i = r.skipWS(i)
		if i >= len(o) {
			return false
		}
		if o[i] == '@' {
			j := i + 1
			for j < len(o) && (isIdentByte(o[j]) || o[j] == '.' || o[j] == ':') {
				j++
			}
			if j < len(o) && o[j] == '(' {
				if j = r.skipBalanced(j, '(', ')'); j < 0 {
					return false
				}
			}
			i = j
			continue
		}
		j := i
		for j < len(o) && isIdentByte(o[j]) {
			j++
		}
		if j == i {
			return false
		}
		w := string(o[i:j])
		if contextFollowWords[w] {
			return r.isCode(i)
		}
		if !contextModifierWords[w] {
			return false
		}
		i = j
	}
	return false
}

// rewriteNameBasedDestructuring rewrites the Kotlin 2.4 forms
// `(val a, val b) = x` and `[val a, val b] = x` into `val (a,     b) = x`,
// `val [a, b] = x` into `val (a, b) = x`, and the `for` and lambda variants
// into plain `(a, b)` lists.
func (r *kotlinRewriter) rewriteNameBasedDestructuring() {
	o := r.orig
	for i := 0; i < len(o); i++ {
		if (o[i] != '(' && o[i] != '[') || !r.isCode(i) {
			continue
		}
		closeCh := byte(')')
		if o[i] == '[' {
			closeCh = ']'
			// `val [a, b] = x`: square-bracket positional destructuring.
			if p := r.prevNonSpace(i); p >= 2 && (r.wordAt(p-2, "val") || r.wordAt(p-2, "var")) {
				if end := r.skipBalanced(i, '[', ']'); end > 0 && r.followedByAssign(end) {
					r.set(i, '(')
					r.set(end-1, ')')
				}
				continue
			}
		}
		j := r.skipWS(i + 1)
		if !(r.wordAt(j, "val") || r.wordAt(j, "var")) {
			continue
		}
		end := r.skipBalanced(i, o[i], closeCh)
		if end < 0 || !isDestructuringNameList(o[i+1:end-1]) {
			continue
		}
		after := r.skipWS(end)
		assign := r.followedByAssign(end)
		isFor := r.wordAt(after, "in")
		isLambda := after+1 < len(o) && o[after] == '-' && o[after+1] == '>'
		if !assign && !isFor && !isLambda {
			continue
		}
		r.rewriteNameList(i, end, assign)
		i = end - 1
	}
}

// isDestructuringNameList reports whether s holds only val/var words, names,
// commas and `name = property` entries.
func isDestructuringNameList(s []byte) bool {
	for _, c := range s {
		if !isIdentByte(c) && c != ',' && c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '=' && c != '.' && c != '`' {
			return false
		}
	}
	return true
}

func (r *kotlinRewriter) followedByAssign(end int) bool {
	a := r.skipWS(end)
	return a < len(r.orig) && r.orig[a] == '=' && (a+1 >= len(r.orig) || r.orig[a+1] != '=')
}

// rewriteNameList removes the `val`/`var` words inside the list at
// [open, end). With keyword set the first word moves in front of the list:
// "(val a" becomes "val (a". The list always ends up in parentheses.
func (r *kotlinRewriter) rewriteNameList(open, end int, keyword bool) {
	o := r.orig
	var spans [][2]int
	for j := open + 1; j < end-1; j++ {
		if r.wordAt(j, "val") || r.wordAt(j, "var") {
			e := j + 3
			for e < end-1 && (o[e] == ' ' || o[e] == '\t') {
				e++
			}
			spans = append(spans, [2]int{j, e})
			j = e - 1
		}
	}
	if len(spans) == 0 {
		return
	}
	if keyword {
		first := spans[0]
		if first[1] < open+5 {
			return
		}
		kw := string(o[first[0] : first[0]+3])
		for k := 0; k < 3; k++ {
			r.set(open+k, kw[k])
		}
		r.set(open+3, ' ')
		r.set(open+4, '(')
		for k := open + 5; k < first[1]; k++ {
			r.set(k, ' ')
		}
		spans = spans[1:]
	} else if o[open] == '[' {
		r.set(open, '(')
	}
	for _, s := range spans {
		r.blank(s[0], s[1])
	}
	if o[end-1] == ']' {
		r.set(end-1, ')')
	}
}

func hasNonASCII(o []byte) bool {
	for _, b := range o {
		if b >= 0x80 {
			return true
		}
	}
	return false
}
