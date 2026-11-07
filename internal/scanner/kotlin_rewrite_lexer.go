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
			if i+1 < n && src[i+1] == '/' {
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < n && src[i+1] == '*' {
				i = lx.blockComment(i)
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
			j := i
			for j < n && src[j] == '$' {
				j++
			}
			if j-i >= 2 && j < n && src[j] == '"' {
				lx.dollarPrefixes = append(lx.dollarPrefixes, [2]int{i, j})
				i = lx.str(j, j-i)
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
	need := dollars
	if need < 1 {
		need = 1
	}
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
			j := i
			for j < n && src[j] == '"' {
				j++
			}
			if j-i >= 3 {
				lx.mask[j-1] = true
				return j
			}
			i = j
			continue
		case c == '$':
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
				i = lx.code(j+1, true)
				continue
			}
			i = j
			continue
		}
		i++
	}
	return i
}
