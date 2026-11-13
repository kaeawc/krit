package scanner

import "bytes"

// kotlinLexer classifies each source byte as code or not. Comments, string
// contents, character literals and backtick names are not code. Quote marks
// of strings count as code so the end of a line can be told from the end of a
// comment. Code inside `${...}` templates is code.
type kotlinLexer struct {
	src  []byte
	mask []bool

	// dollarPrefixes are the [start, end) spans of the `$$` before a
	// multi-dollar string.
	dollarPrefixes [][2]int
	// edits neutralize `$` bytes inside multi-dollar strings that the grammar
	// would read as templates.
	edits []byteEdit
}

type byteEdit struct {
	pos int
	b   byte
}

func lexKotlin(src []byte) *kotlinLexer {
	lx := &kotlinLexer{src: src, mask: make([]bool, len(src))}
	lx.code(0, false)
	return lx
}

func isIdentStartByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// code scans code from i. In a template it returns after the closing '}'.
func (lx *kotlinLexer) code(i int, inTemplate bool) int {
	src := lx.src
	n := len(src)
	depth := 0
	for i < n {
		c := src[i]
		switch c {
		case '/':
			if j, ok := lx.comment(i); ok {
				i = j
				continue
			}
		case '"':
			i = lx.str(i, 0)
			continue
		case '\'':
			if j := lx.charLiteral(i); j > i {
				i = j
				continue
			}
		case '`':
			k := bytes.IndexByte(src[i+1:], '`')
			if k < 0 {
				return n
			}
			i += k + 2
			continue
		case '$':
			if j, ok := lx.multiDollarString(i); ok {
				i = j
				continue
			}
		case '{':
			depth++
		case '}':
			if inTemplate && depth == 0 {
				return i + 1
			}
			depth--
		}
		lx.mask[i] = true
		i++
	}
	return i
}

// comment skips a line or block comment starting at the '/' at i. ok is false when none starts there.
func (lx *kotlinLexer) comment(i int) (int, bool) {
	src := lx.src
	n := len(src)
	if i+1 < n && src[i+1] == '/' {
		for i < n && src[i] != '\n' {
			i++
		}
		return i, true
	}
	if i+1 < n && src[i+1] == '*' {
		return lx.blockComment(i), true
	}
	return i, false
}

// multiDollarString scans a `$$"..."` string whose dollar run starts at i. ok is false when the run does not prefix a string.
func (lx *kotlinLexer) multiDollarString(i int) (int, bool) {
	src := lx.src
	n := len(src)
	j := i
	for j < n && src[j] == '$' {
		j++
	}
	if j-i >= 2 && j < n && src[j] == '"' {
		lx.dollarPrefixes = append(lx.dollarPrefixes, [2]int{i, j})
		return lx.str(j, j-i), true
	}
	return i, false
}

func (lx *kotlinLexer) blockComment(i int) int {
	src := lx.src
	depth := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			depth++
			i += 2
			continue
		}
		if i+1 < len(src) && src[i] == '*' && src[i+1] == '/' {
			depth--
			i += 2
			if depth == 0 {
				return i
			}
			continue
		}
		i++
	}
	return i
}

// charLiteral returns the index after a character literal at i, or i when
// the quote does not start one.
func (lx *kotlinLexer) charLiteral(i int) int {
	src := lx.src
	if i+2 >= len(src) {
		return i
	}
	if src[i+1] == '\\' {
		for j := i + 3; j < len(src) && j < i+12; j++ {
			if src[j] == '\'' {
				return j + 1
			}
			if src[j] == '\n' {
				break
			}
		}
		return i
	}
	// One UTF-8 encoded character.
	j := i + 2
	for j < len(src) && src[j]&0xC0 == 0x80 {
		j++
	}
	if j < len(src) && src[j] == '\'' {
		return j + 1
	}
	return i
}

// str scans a string starting at the quote at i. dollars is the number of
// `$` before the quote of a multi-dollar string, else 0.
func (lx *kotlinLexer) str(i, dollars int) int {
	src := lx.src
	n := len(src)
	raw := i+2 < n && src[i+1] == '"' && src[i+2] == '"'
	lx.mask[i] = true
	if raw {
		i += 3
	} else {
		i++
	}
	need := max(dollars, 1)
	for i < n {
		c := src[i]
		switch {
		case !raw && c == '\\':
			i += 2
			continue
		case !raw && c == '"':
			lx.mask[i] = true
			return i + 1
		case !raw && c == '\n':
			return i
		case raw && c == '"':
			j, closed := lx.rawQuotes(i)
			if closed {
				return j
			}
			i = j
			continue
		case c == '$':
			i = lx.strDollars(i, dollars, need)
			continue
		}
		i++
	}
	return i
}

// rawQuotes skips the run of quotes at i inside a raw string. closed reports whether the run ends the string.
func (lx *kotlinLexer) rawQuotes(i int) (int, bool) {
	src := lx.src
	j := i
	for j < len(src) && src[j] == '"' {
		j++
	}
	if j-i >= 3 {
		lx.mask[j-1] = true
		// The grammar reads `\"` in a raw string as an escape, so a backslash right before
		// a closing `"""` would keep the string open. Raw strings have no escapes.
		if j-i == 3 && i > 0 && src[i-1] == '\\' {
			lx.edits = append(lx.edits, byteEdit{i - 1, '_'})
		}
		return j, true
	}
	return j, false
}

// strDollars handles the dollar run at i inside a string whose template prefix needs need dollars,
// and returns the index to resume scanning from.
func (lx *kotlinLexer) strDollars(i, dollars, need int) int {
	src := lx.src
	n := len(src)
	j := i
	for j < n && src[j] == '$' {
		j++
	}
	var next byte
	if j < n {
		next = src[j]
	}
	interp := j-i >= need && (next == '{' || isIdentStartByte(next))
	if dollars >= 2 {
		keep := j
		if interp {
			keep = j - 1
		}
		for p := i; p < keep; p++ {
			lx.edits = append(lx.edits, byteEdit{p, '_'})
		}
	}
	if interp && next == '{' {
		return lx.code(j+1, true)
	}
	return j
}
