package scanner

import "bytes"

// Line-leading tokens the grammar attaches to the previous line.
//
//   - `as T` / `as? T` on its own line continues the expression above; the
//     grammar rejects the line break, so the whitespace is replaced by spaces.
//   - `in x ->`, `!in x ->`, `else ->` and `.5f` start a new statement, but
//     the grammar continues the expression above (`a in b`, an `else` for an
//     `if`, a member access), so a ';' ends the previous line.
//   - `is X if cond ->` (when guard) becomes `is X , cond ->`.
//   - `{ [a, b] -> ...` (square-bracket destructuring) uses parentheses.
//   - `(x).f()` on its own line is a new statement, not a call suffix of the line above.

var guardStopWords = map[string]bool{
	"val": true, "var": true, "return": true, "else": true, "fun": true, "throw": true, "for": true,
	"while": true, "do": true, "try": true, "class": true, "object": true, "when": true, "if": true,
	"import": true, "package": true, "break": true, "continue": true, "private": true, "public": true,
	"internal": true, "protected": true, "override": true, "const": true, "lateinit": true,
}

func (r *kotlinRewriter) rewriteLines() {
	o := r.orig
	for ls := 0; ls < len(o); {
		le := bytes.IndexByte(o[ls:], '\n')
		if le < 0 {
			le = len(o)
		} else {
			le += ls
		}
		p := ls
		for p < le && (o[p] == ' ' || o[p] == '\t') {
			p++
		}
		if p < le {
			r.rewriteLine(ls, p, le)
			if q := r.skipLeadingBlockComments(p, le); q != p && q < le && (o[q] == '+' || o[q] == '-') {
				r.rewriteLeadingSign(ls, q)
			}
		}
		ls = le + 1
	}
}

func (r *kotlinRewriter) rewriteLine(ls, p, le int) {
	r.rewriteLeadingToken(ls, p, le)
	r.rewriteLineShapes(p, le)
}

// rewriteLeadingToken handles a line whose first token the grammar attaches to the previous line.
func (r *kotlinRewriter) rewriteLeadingToken(ls, p, le int) {
	o := r.orig
	line := o[p:le]
	switch o[p] {
	case 'a':
		if startsCastContinuation(line) {
			r.joinWithPrevious(ls, p)
		}
	case 'i':
		if bytes.HasPrefix(line, []byte("in ")) && bytes.Contains(line, []byte("->")) {
			r.terminatePrevious(ls)
		}
	case '!':
		if startsNegatedWhenBranch(line) {
			r.terminatePrevious(ls)
			r.wrapNegatedIdentifier(p, le)
		}
	case 'w':
		if bytes.HasPrefix(line, []byte("where ")) && bytes.Contains(line, []byte(":")) {
			r.joinWithPrevious(ls, p)
		}
	case 'e':
		if r.startsElseBranch(p, line) {
			r.terminatePrevious(ls)
		}
	case '+', '-':
		r.rewriteLeadingSign(ls, p)
	case '(':
		r.rewriteLeadingParen(ls, p)
	case '.':
		if startsFractionLiteral(line) {
			r.terminatePrevious(ls)
		}
	}
}

// rewriteLineShapes handles constructs anywhere on the line starting at p.
func (r *kotlinRewriter) rewriteLineShapes(p, le int) {
	line := r.orig[p:le]
	if bytes.Contains(line, []byte(" if ")) && bytes.Contains(line, []byte("->")) {
		r.rewriteWhenGuard(p, le)
	}
	if bytes.Contains(line, []byte("(")) && bytes.Contains(line, []byte("):")) || bytes.Contains(line, []byte(") :")) {
		if bytes.Contains(line, []byte("->")) {
			r.rewriteTypedDestructuringParam(p, le)
		}
	}
	if bytes.Contains(line, []byte("{ [")) {
		r.rewriteBracketLambda(p, le)
	}
	if (hasCallSuffix(line) || startsChainContinuation(line)) && hasAssignOp(line) {
		r.rewriteCallChainAssignment(p, le)
	}
}

// startsCastContinuation reports a line starting with `as T` or `as? T`.
func startsCastContinuation(line []byte) bool {
	return len(line) > 2 && line[1] == 's' && (line[2] == ' ' || line[2] == '\t' || line[2] == '?')
}

// startsNegatedWhenBranch reports a when branch that starts with a negation, as in `!in x ->`.
func startsNegatedWhenBranch(line []byte) bool {
	return len(line) > 1 && line[1] != '=' && line[1] != '!' && bytes.Contains(line, []byte("->"))
}

// startsElseBranch reports an `else ->` when branch at p.
func (r *kotlinRewriter) startsElseBranch(p int, line []byte) bool {
	if !bytes.HasPrefix(line, []byte("else")) {
		return false
	}
	k := r.skipWS(p + 4)
	return k+1 < len(r.orig) && r.orig[k] == '-' && r.orig[k+1] == '>'
}

// startsFractionLiteral reports a line starting with a literal like `.5f`.
func startsFractionLiteral(line []byte) bool {
	return len(line) > 1 && line[1] >= '0' && line[1] <= '9'
}

// startsChainContinuation reports a line that continues a call chain from the line above, as in
// `.interpolator = X()` after `view.animate()`.
func startsChainContinuation(line []byte) bool {
	return len(line) > 1 && (line[0] == '.' || line[0] == '?' && line[1] == '.')
}

func hasCallSuffix(line []byte) bool {
	for _, s := range []string{").", ")[", ")?.", ")!!", "}["} {
		if bytes.Contains(line, []byte(s)) {
			return true
		}
	}
	return false
}

func hasAssignOp(line []byte) bool {
	for i := 1; i < len(line); i++ {
		if line[i] == '=' && (line[i-1] == ' ' || line[i-1] == '\t' || bytes.IndexByte([]byte("+-*/%"), line[i-1]) >= 0) {
			return true
		}
	}
	return false
}

// lastCodeBefore returns the index of the last non-space byte before ls when
// it is code, else -1.
func (r *kotlinRewriter) lastCodeBefore(ls int) int {
	prev := r.prevNonSpace(ls)
	if prev < 0 || !r.isCode(prev) {
		return -1
	}
	return prev
}

func (r *kotlinRewriter) joinWithPrevious(ls, p int) {
	prev := r.lastCodeBefore(ls)
	if prev < 0 || !r.isCode(p) {
		return
	}
	for i := prev + 1; i < p; i++ {
		r.set(i, ' ')
	}
}

// lastCodeSkippingComments returns the index of the last code byte before ls
// that is not whitespace, looking past comments, or -1.
func (r *kotlinRewriter) lastCodeSkippingComments(ls int) int {
	for j := ls - 1; j >= 0; j-- {
		if r.isCode(j) && !isSpaceByte(r.orig[j]) {
			return j
		}
	}
	return -1
}

// terminatePrevious ends the statement above the line starting at ls with a
// ';' so the line starts a new statement.
func (r *kotlinRewriter) terminatePrevious(ls int) {
	prev := r.lastCodeSkippingComments(ls)
	if prev < 0 || prev+1 >= len(r.orig) || !isSpaceByte(r.orig[prev+1]) {
		return
	}
	switch c := r.orig[prev]; {
	case isIdentByte(c), c == ')', c == ']', c == '"', c == '\'', c == '?', c == '!', c == '}':
	default:
		return
	}
	r.set(prev+1, ';')
	r.semis = append(r.semis, uint32(prev+1))
}

func (r *kotlinRewriter) rewriteWhenGuard(p, le int) {
	o := r.orig
	for q := p + 1; q+3 < le; q++ {
		if o[q] != 'i' || o[q+1] != 'f' || !r.wordAt(q, "if") || !isSpaceByte(o[q-1]) {
			continue
		}
		head := o[p : q-1]
		if !validGuardHead(head) {
			return
		}
		// A '->' must follow at bracket depth 0 on the same line.
		depth := 0
		found := false
		for j := q + 2; j+1 < le && !found; j++ {
			if !r.isCode(j) {
				continue
			}
			switch o[j] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
				if depth < 0 {
					return
				}
			case '-':
				if depth == 0 && o[j+1] == '>' {
					found = true
				}
			}
		}
		if !found {
			return
		}
		r.set(q, ',')
		r.set(q+1, ' ')
		return
	}
}

// validGuardHead reports whether h can be the condition before a when guard.
func validGuardHead(h []byte) bool {
	h = bytes.TrimSpace(h)
	if len(h) == 0 || bytes.Contains(h, []byte("->")) {
		return false
	}
	w := h
	for i, c := range h {
		if !isIdentByte(c) {
			w = h[:i]
			break
		}
	}
	if guardStopWords[string(w)] {
		return false
	}
	for i, c := range h {
		if c != '=' {
			continue
		}
		prev, next := byte(0), byte(0)
		if i > 0 {
			prev = h[i-1]
		}
		if i+1 < len(h) {
			next = h[i+1]
		}
		if next == '=' || prev == '=' || prev == '!' || prev == '<' || prev == '>' {
			continue
		}
		return false
	}
	return true
}

func (r *kotlinRewriter) rewriteBracketLambda(p, le int) {
	o := r.orig
	for from := p; from < le; {
		k := bytes.Index(o[from:le], []byte("{ ["))
		if k < 0 {
			return
		}
		b := from + k + 2
		from = b + 1
		if !r.isCode(b) {
			continue
		}
		end := r.skipBalanced(b, '[', ']')
		if end < 0 {
			continue
		}
		after := r.skipWS(end)
		if after+1 < len(o) && o[after] == '-' && o[after+1] == '>' && isDestructuringList(o[b+1:end-1]) {
			r.set(b, '(')
			r.set(end-1, ')')
		}
	}
}

func isDestructuringList(s []byte) bool {
	for _, c := range s {
		if !isIdentByte(c) && c != ',' && c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '`' && c != ':' && c != '?' && c != '<' && c != '>' && c != '.' {
			return false
		}
	}
	return true
}

// rewriteCallChainAssignment handles `f(x).y = v`, `a.b().c += v` and
// `f()[i] = v`. The grammar only accepts such a left-hand side in
// parentheses, so the blanks around it become `(` and `)`:
// ` f(x).y = v` -> `(f(x).y) = v`. repairTree removes the parentheses from the
// tree again.
func (r *kotlinRewriter) rewriteCallChainAssignment(p, le int) {
	o := r.orig
	for i := p + 1; i < le; i++ {
		if o[i] != '=' || !r.isCode(i) {
			continue
		}
		if i+1 < le && (o[i+1] == '=' || o[i+1] == '>') {
			i++
			continue
		}
		opStart := i
		if bytes.IndexByte([]byte("+-*/%"), o[i-1]) >= 0 {
			opStart = i - 1
		} else if bytes.IndexByte([]byte("!<>="), o[i-1]) >= 0 {
			continue
		}
		if opStart < 2 || (o[opStart-1] != ' ' && o[opStart-1] != '\t') {
			continue
		}
		start, ok := r.chainStart(opStart - 2)
		if !ok || start < 1 || (o[start-1] != ' ' && o[start-1] != '\t') || !r.statementBoundary(start) {
			continue
		}
		if ls := lineStartOf(o, start); len(bytes.TrimSpace(o[ls:start])) == 0 {
			// The '(' would continue the previous line as a call.
			r.terminatePrevious(ls)
		}
		r.set(start-1, '(')
		r.set(opStart-1, ')')
		r.parenFixes = append(r.parenFixes, parenFix{open: uint32(start - 1), close: uint32(opStart - 1), start: uint32(start)})
	}
}

// statementBoundary reports whether an expression statement can start at i.
func (r *kotlinRewriter) statementBoundary(i int) bool {
	o := r.orig
	p := r.prevNonSpace(i)
	if p < 0 {
		return true
	}
	if lineStartOf(o, p) != lineStartOf(o, i) {
		if p = r.lastCodeSkippingComments(i); p < 0 {
			return true
		}
		// First token on its line: the line above must not continue.
		switch o[p] {
		case ',', '(', '[', '=', '+', '-', '*', '/', '&', '|', '.', ':', '?', '<':
			return false
		}
		return true
	}
	switch o[p] {
	case '{', ';', ')':
		return true
	case '>':
		return p > 0 && o[p-1] == '-'
	}
	return p >= 3 && r.wordAt(p-3, "else")
}

// chainScan is the backward scan state of chainStart.
type chainScan struct {
	pos               int
	groupBeforeSuffix bool
	first             bool
}

// chainStart scans backward from the last byte at end over a chain of names,
// calls and index suffixes. It reports the chain start and whether the chain
// has a call or index group followed by another suffix.
func (r *kotlinRewriter) chainStart(end int) (int, bool) {
	o := r.orig
	st := chainScan{pos: end, first: true}
	for st.pos >= 0 {
		c := o[st.pos]
		var start int
		var result, done bool
		switch {
		case c == '}':
			if !r.chainLambdaStep(&st) {
				return 0, false
			}
		case c == ')' || c == ']':
			start, result, done = r.chainGroupStep(&st, c)
		case isIdentByte(c) || c == '`':
			start, result, done = r.chainNameStep(&st, c)
		default:
			return 0, false
		}
		if done {
			return start, result
		}
	}
	return 0, false
}

// chainLambdaStep steps over a trailing lambda: `f(x) { ... }[i] = v`.
// It reports false when the lambda cannot be part of the chain.
func (r *kotlinRewriter) chainLambdaStep(st *chainScan) bool {
	o := r.orig
	if st.first {
		return false
	}
	st.groupBeforeSuffix = true
	open := r.matchBraceBackward(st.pos)
	if open < 1 {
		return false
	}
	st.pos = open - 1
	for st.pos >= 0 && (o[st.pos] == ' ' || o[st.pos] == '\t') {
		st.pos--
	}
	if st.pos < 0 || (o[st.pos] != ')' && !isIdentByte(o[st.pos])) {
		return false
	}
	return true
}

// chainGroupStep steps over the call or index group closing with c at st.pos.
// done is true when the scan ends with the given start and result.
func (r *kotlinRewriter) chainGroupStep(st *chainScan, c byte) (start int, result, done bool) {
	o := r.orig
	if !st.first {
		st.groupBeforeSuffix = true
	}
	open := r.matchBackward(st.pos)
	if open < 1 {
		return 0, false, true
	}
	st.pos = open - 1
	if c == ')' && !st.first && r.isCastReceiver(open, st.pos) {
		// `(x as T).prop`: the parenthesized receiver starts the chain.
		return open, true, true
	}
	if st.pos >= 0 && o[st.pos] == '>' && c == ')' {
		if lt := r.matchAngleBackward(st.pos); lt > 0 {
			st.pos = lt - 1
		}
	}
	if st.pos < 0 || !isChainGroupCallee(o[st.pos]) {
		return 0, false, true
	}
	st.first = false
	return 0, false, false
}

// isCastReceiver reports whether the parenthesized group opening at open, after the byte at pos,
// is a receiver of its own such as `(x as T)` rather than call arguments.
func (r *kotlinRewriter) isCastReceiver(open, pos int) bool {
	o := r.orig
	if pos >= 0 && (isIdentByte(o[pos]) || o[pos] == ')' || o[pos] == ']' || o[pos] == '>' || o[pos] == '`') {
		return false
	}
	return r.castInside(open)
}

// isChainGroupCallee reports whether b can end the expression a call or index group applies to.
func isChainGroupCallee(b byte) bool {
	return isIdentByte(b) || b == ')' || b == ']' || b == '}' || b == '`'
}

// chainNameStep steps over the name ending with c at st.pos and the member access before it.
// done is true when the scan ends with the given start and result.
func (r *kotlinRewriter) chainNameStep(st *chainScan, c byte) (start int, result, done bool) {
	o := r.orig
	idStart := st.pos
	if c == '`' {
		k := bytes.LastIndexByte(o[:st.pos], '`')
		if k < 0 {
			return 0, false, true
		}
		idStart = k
	} else {
		for idStart > 0 && isIdentByte(o[idStart-1]) {
			idStart--
		}
	}
	st.first = false
	st.pos = idStart - 1
	if st.pos < 0 || o[st.pos] != '.' {
		return idStart, st.groupBeforeSuffix, true
	}
	st.pos--
	if st.pos >= 0 && o[st.pos] == '?' {
		st.pos--
	}
	// A dot that starts its line continues the chain on the line above.
	if k := skipSpaceBackward(o, st.pos); k < st.pos && bytes.IndexByte(o[k+1:st.pos+1], '\n') >= 0 {
		st.pos = k
	}
	if st.pos >= 1 && o[st.pos] == '!' && o[st.pos-1] == '!' {
		st.pos -= 2
	}
	return 0, false, false
}

// matchBackward returns the index of the bracket that opens the one closing at
// i, or -1.
func (r *kotlinRewriter) matchBackward(i int) int {
	o := r.orig
	depth := 0
	for j := i; j >= 0 && i-j < 600; j-- {
		if !r.isCode(j) {
			continue
		}
		switch o[j] {
		case ')', ']':
			depth++
		case '(', '[':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// matchAngleBackward returns the '<' that opens the '>' at i, or -1.
func (r *kotlinRewriter) matchAngleBackward(i int) int {
	o := r.orig
	depth := 0
	for j := i; j >= 0 && i-j < 200; j-- {
		switch c := o[j]; {
		case c == '>':
			depth++
		case c == '<':
			depth--
			if depth == 0 {
				return j
			}
		case isIdentByte(c) || c == ',' || c == ' ' || c == '.' || c == '?' || c == '*' || c == '\n':
		default:
			return -1
		}
	}
	return -1
}

// rewriteCommasAndRefs blanks trailing commas the grammar rejects (in type
// arguments, index suffixes and function type parameter lists) and the `?` of
// `Type?::member`.
func (r *kotlinRewriter) rewriteCommasAndRefs() {
	o := r.orig
	for i := 0; i < len(o); i++ {
		switch o[i] {
		case ',':
			if r.isRejectedTrailingComma(i) && r.isCode(i) {
				r.set(i, ' ')
			}
		case '?':
			if r.isNullableRefQuestion(i) && r.isCode(i) {
				r.set(i, ' ')
			}
		}
	}
}

// isRejectedTrailingComma reports whether the comma at i trails a type argument list,
// an index suffix or a function type parameter list.
func (r *kotlinRewriter) isRejectedTrailingComma(i int) bool {
	o := r.orig
	j := i + 1
	for j < len(o) && isSpaceByte(o[j]) {
		j++
	}
	if j >= len(o) {
		return false
	}
	switch o[j] {
	case ']', '>':
		return true
	case ')':
		k := r.skipWS(j + 1)
		return k+1 < len(o) && o[k] == '-' && o[k+1] == '>'
	}
	return false
}

// isNullableRefQuestion reports whether the '?' at i is the `?` of `Type?::member` or `Type?.(`.
func (r *kotlinRewriter) isNullableRefQuestion(i int) bool {
	o := r.orig
	return i > 0 && isIdentByte(o[i-1]) && i+2 < len(o) && (o[i+1] == ':' && o[i+2] == ':' || o[i+1] == '.' && o[i+2] == '(')
}

// rewriteTypedDestructuringParam blanks the type of lambda parameters like
// `{ (a, b): Pair<A, B> -> }` and `{ x: X, (a, b): Y -> }`, which the grammar
// rejects.
func (r *kotlinRewriter) rewriteTypedDestructuringParam(p, le int) {
	o := r.orig
	for open := p; open < le; open++ {
		if o[open] != '(' || !r.isCode(open) {
			continue
		}
		q := r.prevNonSpace(open)
		if q < 0 || (o[q] != '{' && o[q] != ',') {
			continue
		}
		end := r.skipBalanced(open, '(', ')')
		if end < 0 || end >= le {
			continue
		}
		colon := r.skipWS(end)
		if colon >= le || o[colon] != ':' {
			continue
		}
		arrow := bytes.Index(o[colon:le], []byte("->"))
		if arrow < 0 || bytes.ContainsAny(o[colon:colon+arrow], "{=;") {
			continue
		}
		r.blank(colon, colon+arrow)
		open = end
	}
}

// castInside reports whether the parenthesized group starting at open holds a
// cast (` as ` / ` as? `), which the grammar cannot take as an assignment
// receiver.
func (r *kotlinRewriter) castInside(open int) bool {
	end := r.skipBalanced(open, '(', ')')
	if end < 0 {
		return false
	}
	seg := r.orig[open:end]
	return bytes.Contains(seg, []byte(" as ")) || bytes.Contains(seg, []byte(" as? "))
}

var modifierIdentifierWords = []string{
	"open", "inline", "data", "value", "inner", "enum", "sealed", "annotation", "lateinit",
	"const", "override", "abstract", "final", "operator", "infix", "tailrec", "external",
	"expect", "actual", "suspend",
}

// rewriteModifierIdentifiers renames identifiers that spell a modifier, such
// as `open.eventSink()`, by upper-casing the first letter in the parse buffer.
// The tree still reads the original text through its byte offsets.
func (r *kotlinRewriter) rewriteModifierIdentifiers() {
	o := r.orig
	for _, w := range modifierIdentifierWords {
		for pos := 0; pos < len(o); {
			k := bytes.Index(o[pos:], []byte(w))
			if k < 0 {
				break
			}
			at := pos + k
			pos = at + len(w)
			if at > 0 && (isIdentByte(o[at-1]) || o[at-1] == '.') {
				continue
			}
			end := at + len(w)
			if end >= len(o) {
				continue
			}
			if r.modifierUsedAsName(at, end) && r.isCode(at) {
				r.set(at, o[at]-'a'+'A')
			}
		}
	}
}

// modifierUsedAsName reports whether the modifier word at [at, end) is used as
// a variable: before a member access, or at the start of a line before a call
// or an assignment.
func (r *kotlinRewriter) modifierUsedAsName(at, end int) bool {
	o := r.orig
	next := o[end]
	if next == '.' || next == '?' && end+1 < len(o) && o[end+1] == '.' || next == '!' && end+1 < len(o) && o[end+1] == '!' {
		return true
	}
	k := end
	for k < len(o) && (o[k] == ' ' || o[k] == '\t') {
		k++
	}
	if k+1 < len(o) && o[k] == '-' && o[k+1] == '>' {
		// A lambda parameter such as `{ annotation -> ... }`.
		p := r.prevNonSpace(at)
		return p >= 0 && (o[p] == '{' || o[p] == ',')
	}
	ls := lineStartOf(o, at)
	if len(bytes.TrimSpace(o[ls:at])) != 0 || k >= len(o) {
		return false
	}
	return startsCallOrAssignment(o, k, end)
}

// startsCallOrAssignment reports whether o[k] starts a call directly after a name ending at end, or an assignment.
func startsCallOrAssignment(o []byte, k, end int) bool {
	switch o[k] {
	case '(':
		return k == end
	case '=':
		return k+1 >= len(o) || o[k+1] != '='
	case '+', '-', '*', '/', '%':
		return k+1 < len(o) && o[k+1] == '='
	}
	return false
}

// rewriteGenericCallableRefs blanks the type arguments of `Type<A>::member`.
func (r *kotlinRewriter) rewriteGenericCallableRefs() {
	o := r.orig
	for pos := 0; ; {
		k := bytes.Index(o[pos:], []byte(">::"))
		if k < 0 {
			return
		}
		gt := pos + k
		pos = gt + 3
		if !r.isCode(gt) {
			continue
		}
		lt := r.matchAngleBackward(gt)
		if lt < 1 || !isIdentByte(o[lt-1]) {
			continue
		}
		r.blank(lt, gt+1)
	}
}

// wrapNegatedIdentifier turns a when condition `!isReady && x ->` into
// `(!isReady)&& x ->`. The grammar lexes `!is` of `!isReady` as the negated
// type test operator. The blank before the line and the blank after the
// identifier become the parentheses; repairTree removes them from the tree.
func (r *kotlinRewriter) wrapNegatedIdentifier(p, le int) {
	o := r.orig
	if p < 1 || (o[p-1] != ' ' && o[p-1] != '\t') || p+3 >= le {
		return
	}
	if o[p+1] != 'i' || (o[p+2] != 's' && o[p+2] != 'n') || !isIdentByte(o[p+3]) {
		return
	}
	end := p + 1
	for end < le && isIdentByte(o[end]) {
		end++
	}
	if end < le && o[end] == '(' {
		if end = r.skipBalanced(end, '(', ')'); end < 0 {
			return
		}
	}
	if end >= le || (o[end] != ' ' && o[end] != '\t') || !r.isCode(p) {
		return
	}
	r.set(p-1, '(')
	r.set(end, ')')
	r.parenFixes = append(r.parenFixes, parenFix{open: uint32(p - 1), close: uint32(end), start: uint32(p), kind: parenFixExpression})
}

// matchBraceBackward returns the index of the '{' matching the '}' at i, or -1.
func (r *kotlinRewriter) matchBraceBackward(i int) int {
	o := r.orig
	depth := 0
	for j := i; j >= 0 && i-j < 2000; j-- {
		if !r.isCode(j) {
			continue
		}
		switch o[j] {
		case '}':
			depth++
		case '{':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// rewriteFunctionTypeReceivers joins a qualified receiver of a function type
// into one name: `Circuit.Builder.() -> Unit` reads as `Circuit_Builder.() ->
// Unit`. The grammar only accepts a single type name before `.(`. The tree
// reads the original names through its byte offsets.
func (r *kotlinRewriter) rewriteFunctionTypeReceivers() {
	o := r.orig
	for pos := 0; ; {
		k := bytes.Index(o[pos:], []byte(".("))
		if k < 0 {
			return
		}
		dot := pos + k
		pos = dot + 2
		if dot == 0 || !r.isCode(dot) {
			continue
		}
		end := r.skipBalanced(dot+1, '(', ')')
		if end < 0 {
			continue
		}
		if a := r.skipWS(end); a+1 >= len(o) || o[a] != '-' || o[a+1] != '>' {
			continue
		}
		j := dot - 1
		if o[j] == ')' {
			// `(A.B).(X) -> Y`: the parentheses go away too.
			open := r.matchBackward(j)
			if open < 0 {
				continue
			}
			r.set(open, ' ')
			r.set(j, ' ')
			j--
		}
		if j >= 0 && o[j] == '>' {
			lt := r.matchAngleBackward(j)
			if lt < 1 {
				continue
			}
			j = lt - 1
		}
		r.joinQualifiedName(j)
	}
}

// joinQualifiedName replaces the dots between the names of the qualified name ending at j with '_'.
func (r *kotlinRewriter) joinQualifiedName(j int) {
	o := r.orig
	for j >= 1 && isIdentByte(o[j]) {
		for j >= 0 && isIdentByte(o[j]) {
			j--
		}
		if j >= 1 && o[j] == '.' && isIdentByte(o[j-1]) {
			r.set(j, '_')
			j--
			continue
		}
		break
	}
}

// skipSpaceBackward returns the index of the last non-space byte at or before i, or -1.
func skipSpaceBackward(o []byte, i int) int {
	for i >= 0 && isSpaceByte(o[i]) {
		i--
	}
	return i
}

// skipLeadingBlockComments returns the offset of the first byte of the line that is neither blank nor
// part of a block comment that starts the line.
func (r *kotlinRewriter) skipLeadingBlockComments(p, le int) int {
	o := r.orig
	q := p
	for q+1 < le && o[q] == '/' && o[q+1] == '*' {
		end := bytes.Index(o[q+2:], []byte("*/"))
		if end < 0 || q+2+end+2 > le {
			return p
		}
		q += end + 4
		for q < le && (o[q] == ' ' || o[q] == '\t') {
			q++
		}
	}
	return q
}
