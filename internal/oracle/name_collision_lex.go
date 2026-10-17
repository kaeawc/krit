package oracle

// identifiersUsed returns which of the wanted names appear as identifiers in
// Kotlin source outside comments. Line, block (nested), and KDoc comments are
// skipped. Plain string text is skipped too, but string templates (`$name`
// and `${...}`) are scanned as code, so a name used only in a template still
// counts. Character literals are skipped. The scan is lexical, not a parse:
// it errs toward reporting a name rather than missing one.
func identifiersUsed(content []byte, wanted map[string]bool) map[string]bool {
	l := &kotlinLexer{content: content, wanted: wanted, found: map[string]bool{}, stack: []lexFrame{{mode: lexCode}}}
	for l.i < len(content) && len(l.found) < len(wanted) {
		if l.stack[len(l.stack)-1].mode == lexCode {
			l.codeStep()
		} else {
			l.textStep()
		}
	}
	return l.found
}

type lexMode byte

const (
	lexCode lexMode = iota
	lexString
	lexRawString
)

type lexFrame struct {
	mode lexMode
	// braces counts the braces opened inside a template's code frame, so
	// the `}` that closes `${` is told apart from nested blocks.
	braces int
}

type kotlinLexer struct {
	content []byte
	i       int
	stack   []lexFrame
	wanted  map[string]bool
	found   map[string]bool
}

func (l *kotlinLexer) record(id []byte) {
	if l.wanted[string(id)] {
		l.found[string(id)] = true
	}
}

func (l *kotlinLexer) push(mode lexMode) { l.stack = append(l.stack, lexFrame{mode: mode}) }
func (l *kotlinLexer) pop()              { l.stack = l.stack[:len(l.stack)-1] }

func (l *kotlinLexer) peek(offset int) byte {
	if l.i+offset < len(l.content) {
		return l.content[l.i+offset]
	}
	return 0
}

func (l *kotlinLexer) readIdent() {
	j := l.i + 1
	for j < len(l.content) && isIdentByte(l.content[j]) {
		j++
	}
	l.record(l.content[l.i:j])
	l.i = j
}

// readTemplateIdent reads the identifier after a `$` in string text. Unlike
// code identifiers it cannot contain `$`, so "$a$foo" is two references.
func (l *kotlinLexer) readTemplateIdent() {
	j := l.i + 1
	for j < len(l.content) && l.content[j] != '$' && isIdentByte(l.content[j]) {
		j++
	}
	l.record(l.content[l.i:j])
	l.i = j
}

func (l *kotlinLexer) skipCharLiteral() {
	l.i++
	for l.i < len(l.content) && l.content[l.i] != '\'' && l.content[l.i] != '\n' {
		if l.content[l.i] == '\\' {
			l.i++
		}
		l.i++
	}
	l.i++
}

func (l *kotlinLexer) readBacktick() {
	j := l.i + 1
	for j < len(l.content) && l.content[j] != '`' && l.content[j] != '\n' {
		j++
	}
	l.record(l.content[l.i+1 : j])
	l.i = j + 1
}

// codeStep consumes one token of code.
func (l *kotlinLexer) codeStep() {
	c := l.content[l.i]
	switch {
	case c == '/' && l.peek(1) == '/':
		for l.i < len(l.content) && l.content[l.i] != '\n' {
			l.i++
		}
	case c == '/' && l.peek(1) == '*':
		l.i = skipBlockCommentBytes(l.content, l.i)
	case c == '"':
		if l.peek(1) == '"' && l.peek(2) == '"' {
			l.push(lexRawString)
			l.i += 3
		} else {
			l.push(lexString)
			l.i++
		}
	case c == '-' && l.peek(1) == '>':
		l.record([]byte("->"))
		l.i += 2
	case c == '\'':
		l.skipCharLiteral()
	case c == '{', c == '}':
		l.braceStep(c)
	case c == '`':
		l.readBacktick()
	case isIdentStart(c):
		l.readIdent()
	case c >= '0' && c <= '9':
		for l.i < len(l.content) && isIdentByte(l.content[l.i]) {
			l.i++
		}
	default:
		l.i++
	}
}

// braceStep tracks braces only inside a template's code frame, where the
// matching `}` returns to the string.
func (l *kotlinLexer) braceStep(c byte) {
	l.i++
	if len(l.stack) == 1 {
		return
	}
	top := &l.stack[len(l.stack)-1]
	switch {
	case c == '{':
		top.braces++
	case top.braces == 0:
		l.pop()
	default:
		top.braces--
	}
}

// textStep consumes one token of string or raw string text.
func (l *kotlinLexer) textStep() {
	mode := l.stack[len(l.stack)-1].mode
	c := l.content[l.i]
	switch {
	case c == '\\' && mode == lexString:
		l.i += 2
	case c == '\n' && mode == lexString:
		l.pop() // unterminated: recover at the line end
		l.i++
	case c == '"' && mode == lexString:
		l.pop()
		l.i++
	case c == '"' && mode == lexRawString && l.peek(1) == '"' && l.peek(2) == '"':
		// The closer is the last three quotes of a run of quotes; the
		// earlier ones are content.
		l.pop()
		for l.i < len(l.content) && l.content[l.i] == '"' {
			l.i++
		}
	case c == '$' && l.peek(1) == '{':
		l.push(lexCode)
		l.i += 2
	case c == '$' && l.peek(1) != '$' && isIdentStart(l.peek(1)):
		l.i++
		l.readTemplateIdent()
	default:
		l.i++
	}
}

// skipBlockCommentBytes returns the index after the possibly nested block
// comment starting at i, or len(content) when it is unterminated.
func skipBlockCommentBytes(content []byte, i int) int {
	n := len(content)
	depth := 0
	for i < n {
		switch {
		case content[i] == '/' && i+1 < n && content[i+1] == '*':
			depth++
			i += 2
		case content[i] == '*' && i+1 < n && content[i+1] == '/':
			depth--
			i += 2
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return n
}
