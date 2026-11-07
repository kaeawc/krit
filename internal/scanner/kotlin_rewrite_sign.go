package scanner

// A `+` or `-` that starts a line never continues the statement above: Kotlin ends the statement at the
// line break, so `a` / `+ b` is `a; +b`. The grammar joins them into one additive expression, so a ';'
// ends the previous line. Inside parentheses and brackets line breaks are not significant and the
// operator stays binary.

// signLineContext is what the rewrite needs to know about a line that starts with `+` or `-`.
type signLineContext struct {
	// enclosing maps the offset of such a sign to the innermost open bracket, or 0 outside brackets.
	enclosing map[int]byte
	// headClose holds the offsets of `)` that close an if, while, for, when or catch head.
	headClose map[int]bool
}

// signContinuationWords end a line that still expects an expression on the next line. A ';' after
// `return` on a line of its own does not stop the grammar from reading the next expression as its value.
var signContinuationWords = map[string]bool{
	"else": true, "try": true, "finally": true, "do": true, "in": true, "is": true, "as": true, "by": true,
	"where": true, "catch": true, "when": true, "if": true, "while": true, "for": true, "fun": true,
	"val": true, "var": true, "throw": true, "return": true, "break": true, "continue": true,
}

var signHeadWords = map[string]bool{"if": true, "while": true, "for": true, "when": true, "catch": true}

func (r *kotlinRewriter) signContext() *signLineContext {
	if r.signCtx != nil {
		return r.signCtx
	}
	o := r.orig
	ctx := &signLineContext{enclosing: map[int]byte{}, headClose: map[int]bool{}}
	type open struct {
		ch   byte
		head bool
	}
	var stack []open
	atLineStart := true
	for i := 0; i < len(o); i++ {
		c := o[i]
		if c == '\n' {
			atLineStart = true
			continue
		}
		if atLineStart {
			if c == ' ' || c == '\t' || c == '\r' || !r.isCode(i) && !(c == '"' || c == '\'') {
				continue
			}
			atLineStart = false
			if (c == '+' || c == '-') && r.isCode(i) {
				var top byte
				if len(stack) > 0 {
					top = stack[len(stack)-1].ch
				}
				ctx.enclosing[i] = top
			}
		}
		if !r.isCode(i) {
			// The `${` and `}` of a string template are not code but delimit code.
			switch {
			case c == '$' && i+1 < len(o) && o[i+1] == '{' && !r.isCode(i+1):
				stack = append(stack, open{ch: 'T'})
			case c == '}' && len(stack) > 0 && stack[len(stack)-1].ch == 'T':
				stack = stack[:len(stack)-1]
			}
			continue
		}
		switch c {
		case '(', '[', '{':
			stack = append(stack, open{ch: c, head: c == '(' && signHeadWords[r.wordBefore(i)]})
		case ')', ']', '}':
			if len(stack) > 0 {
				if stack[len(stack)-1].head && c == ')' {
					ctx.headClose[i] = true
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	r.signCtx = ctx
	return ctx
}

// wordBefore returns the identifier that ends just before i, skipping blanks.
func (r *kotlinRewriter) wordBefore(i int) string {
	j := i - 1
	for j >= 0 && (r.orig[j] == ' ' || r.orig[j] == '\t') {
		j--
	}
	end := j + 1
	for j >= 0 && isIdentByte(r.orig[j]) {
		j--
	}
	return string(r.orig[j+1 : end])
}

// rewriteLeadingSign ends the statement above a line that starts with a unary-looking `+` or `-`.
func (r *kotlinRewriter) rewriteLeadingSign(ls, p int) {
	o := r.orig
	if p+1 >= len(o) || !r.isCode(p) {
		return
	}
	switch next := o[p+1]; {
	case next == o[p], next == '=', o[p] == '-' && next == '>':
		return
	}
	ctx := r.signContext()
	if top, ok := ctx.enclosing[p]; !ok || top == '(' || top == '[' || top == 'T' {
		return
	}
	prev := r.lastCodeSkippingComments(ls)
	if prev < 0 || ctx.headClose[prev] {
		return
	}
	if isIdentByte(o[prev]) && signContinuationWords[r.wordBefore(prev+1)] {
		return
	}
	r.terminatePrevious(ls, true)
}
