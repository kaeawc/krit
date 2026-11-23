package android

import (
	"bytes"
	"html"
	"strings"
	"unicode/utf8"
)

// parseXMLFast builds the XMLNode tree for documents written in a strict,
// well-formed subset of XML without going through tree-sitter. For every
// document it accepts, the result is the tree buildXMLNode produces from
// the tree-sitter parse (same tags, attributes, text, positions and DOM
// indices). ok is false for anything outside the subset, and the caller
// then parses with tree-sitter, so the subset only has to be accepted
// correctly, not be complete.
//
// The subset: valid UTF-8 with no control characters other than tab, LF
// and CR and no byte-order mark; an optional plain XML declaration; comments
// and whitespace around one root element; ASCII names; quoted attribute
// values without '<'; character data; entity and character references;
// comments and CDATA sections in content; matching end tags. A DOCTYPE,
// processing instruction, malformed reference or anything unexpected
// declines the fast path.
func parseXMLFast(data []byte) (root *XMLNode, ok bool) {
	if !xmlFastBytesOK(data) {
		return nil, false
	}
	p := xmlFastParser{src: data}
	if bytes.HasPrefix(data, []byte("<?xml")) {
		if !p.xmlDecl() {
			return nil, false
		}
	}
	if !p.misc() || p.pos >= len(data) || data[p.pos] != '<' || !xmlFastNameStart(p.peek(1)) {
		return nil, false
	}
	root, ok = p.element(0)
	if !ok || !p.misc() || p.pos != len(data) {
		return nil, false
	}
	return root, true
}

// xmlFastMaxDepth bounds element nesting; deeper documents take the
// tree-sitter path.
const xmlFastMaxDepth = 256

// xmlFastBytesOK rejects invalid UTF-8, control characters other than tab,
// LF and CR, DEL, and byte-order marks.
func xmlFastBytesOK(data []byte) bool {
	for _, b := range data {
		if (b < 0x20 && b != '\t' && b != '\n' && b != '\r') || b == 0x7f {
			return false
		}
	}
	return utf8.Valid(data) && !bytes.Contains(data, []byte("\xef\xbb\xbf"))
}

type xmlFastParser struct {
	src []byte
	pos int
	// row and rowStart track the 0-based row and its start offset for the
	// last position converted by point; conversions move forward only.
	pointPos int
	row      int
	rowStart int
}

func (p *xmlFastParser) peek(off int) byte {
	if p.pos+off < len(p.src) {
		return p.src[p.pos+off]
	}
	return 0
}

func (p *xmlFastParser) hasPrefix(s string) bool {
	return bytes.HasPrefix(p.src[p.pos:], []byte(s))
}

// point returns tree-sitter's 1-based line and byte column of offset off.
// Rows advance only at '\n', as in tree-sitter.
func (p *xmlFastParser) point(off int) (line, col int) {
	if off < p.pointPos {
		p.pointPos, p.row, p.rowStart = 0, 0, 0
	}
	seg := p.src[p.pointPos:off]
	if n := bytes.Count(seg, []byte{'\n'}); n > 0 {
		p.row += n
		p.rowStart = p.pointPos + bytes.LastIndexByte(seg, '\n') + 1
	}
	p.pointPos = off
	return p.row + 1, off - p.rowStart + 1
}

func xmlFastSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func xmlFastNameStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func xmlFastNameByte(b byte) bool {
	return xmlFastNameStart(b) || (b >= '0' && b <= '9') || b == '.' || b == '-' || b == ':'
}

func (p *xmlFastParser) skipSpace() int {
	start := p.pos
	for p.pos < len(p.src) && xmlFastSpace(p.src[p.pos]) {
		p.pos++
	}
	return p.pos - start
}

func (p *xmlFastParser) name() (string, bool) {
	start := p.pos
	if p.pos >= len(p.src) || !xmlFastNameStart(p.src[p.pos]) {
		return "", false
	}
	for p.pos < len(p.src) && xmlFastNameByte(p.src[p.pos]) {
		p.pos++
	}
	return string(p.src[start:p.pos]), true
}

// xmlDecl accepts `<?xml version="1.x"( encoding="name")?( standalone="yes|no")? ?>`.
func (p *xmlFastParser) xmlDecl() bool {
	p.pos += len("<?xml")
	if p.skipSpace() == 0 || !p.declAttr("version", func(v string) bool {
		return strings.HasPrefix(v, "1.") && len(v) > 2 && strings.Trim(v[2:], "0123456789") == ""
	}) {
		return false
	}
	for _, opt := range []struct {
		name  string
		valid func(string) bool
	}{
		{"encoding", func(v string) bool {
			return v != "" && xmlFastNameStart(v[0]) && v[0] != '_' && strings.Trim(v, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") == ""
		}},
		{"standalone", func(v string) bool { return v == "yes" || v == "no" }},
	} {
		save := p.pos
		if p.skipSpace() > 0 && p.hasPrefix(opt.name) {
			if !p.declAttr(opt.name, opt.valid) {
				return false
			}
			continue
		}
		p.pos = save
	}
	p.skipSpace()
	if !p.hasPrefix("?>") {
		return false
	}
	p.pos += 2
	return true
}

func (p *xmlFastParser) declAttr(name string, valid func(string) bool) bool {
	if !p.hasPrefix(name + "=") {
		return false
	}
	p.pos += len(name) + 1
	q := p.peek(0)
	if q != '"' && q != '\'' {
		return false
	}
	end := bytes.IndexByte(p.src[p.pos+1:], q)
	if end < 0 {
		return false
	}
	value := string(p.src[p.pos+1 : p.pos+1+end])
	p.pos += end + 2
	return valid(value)
}

// misc skips whitespace and comments outside the root element.
func (p *xmlFastParser) misc() bool {
	for {
		p.skipSpace()
		if !p.hasPrefix("<!--") {
			return true
		}
		if !p.comment() {
			return false
		}
	}
}

// comment skips `<!-- ... -->`, rejecting "--" inside the body.
func (p *xmlFastParser) comment() bool {
	body := p.pos + len("<!--")
	end := bytes.Index(p.src[body:], []byte("--"))
	if end < 0 || body+end+2 >= len(p.src) || p.src[body+end+2] != '>' {
		return false
	}
	p.pos = body + end + 3
	return true
}

// reference reads an entity or character reference at p.pos and returns its
// raw text and node kind.
func (p *xmlFastParser) reference() (raw, kind string, ok bool) {
	start := p.pos
	i := p.pos + 1
	switch {
	case i+1 < len(p.src) && p.src[i] == '#' && p.src[i+1] == 'x':
		i, ok = p.scanRun(i+2, isXMLFastHexDigit)
		kind = "CharRef"
	case i < len(p.src) && p.src[i] == '#':
		i, ok = p.scanRun(i+1, isXMLFastDigit)
		kind = "CharRef"
	case i < len(p.src) && xmlFastNameStart(p.src[i]):
		i, ok = p.scanRun(i, xmlFastNameByte)
		kind = "EntityRef"
	}
	if !ok || i >= len(p.src) || p.src[i] != ';' {
		return "", "", false
	}
	p.pos = i + 1
	return string(p.src[start:p.pos]), kind, true
}

// scanRun returns the end of the run of bytes matching in starting at i, and
// whether the run is non-empty.
func (p *xmlFastParser) scanRun(i int, in func(byte) bool) (int, bool) {
	start := i
	for i < len(p.src) && in(p.src[i]) {
		i++
	}
	return i, i > start
}

func isXMLFastDigit(b byte) bool { return b >= '0' && b <= '9' }

func isXMLFastHexDigit(b byte) bool {
	return isXMLFastDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func (p *xmlFastParser) attribute() (XMLAttribute, bool) {
	start := p.pos
	name, ok := p.name()
	if !ok {
		return XMLAttribute{}, false
	}
	p.skipSpace()
	if p.peek(0) != '=' {
		return XMLAttribute{}, false
	}
	p.pos++
	p.skipSpace()
	q := p.peek(0)
	if q != '"' && q != '\'' {
		return XMLAttribute{}, false
	}
	valueStart := p.pos
	p.pos++
	for {
		if p.pos >= len(p.src) {
			return XMLAttribute{}, false
		}
		switch p.src[p.pos] {
		case q:
			p.pos++
			line, col := p.point(start)
			return XMLAttribute{
				Name:      name,
				Value:     strings.Trim(string(p.src[valueStart:p.pos]), "\"'"),
				Line:      line,
				Col:       col,
				StartByte: uint(start),
				EndByte:   uint(p.pos),
			}, true
		case '<':
			return XMLAttribute{}, false
		case '&':
			if _, _, ok := p.reference(); !ok {
				return XMLAttribute{}, false
			}
		default:
			p.pos++
		}
	}
}

// element parses the element starting at p.pos ('<' followed by a name).
func (p *xmlFastParser) element(depth int) (*XMLNode, bool) {
	if depth >= xmlFastMaxDepth {
		return nil, false
	}
	start := p.pos
	p.pos++
	tag, ok := p.name()
	if !ok {
		return nil, false
	}
	line, col := p.point(start)
	out := &XMLNode{Tag: tag, Line: line, Col: col, StartByte: uint(start)}
	for {
		spaced := p.skipSpace() > 0
		switch {
		case p.peek(0) == '>':
			p.pos++
			return p.content(out, depth)
		case p.hasPrefix("/>"):
			p.pos += 2
			out.EndByte = uint(p.pos)
			return out, true
		case !spaced:
			return nil, false
		}
		attr, ok := p.attribute()
		if !ok {
			return nil, false
		}
		out.Attrs = append(out.Attrs, attr)
	}
}

// endTag consumes the end tag of out at p.pos ("</").
func (p *xmlFastParser) endTag(out *XMLNode) bool {
	p.pos += 2
	tag, ok := p.name()
	if !ok || tag != out.Tag {
		return false
	}
	p.skipSpace()
	if p.peek(0) != '>' {
		return false
	}
	p.pos++
	out.EndByte = uint(p.pos)
	return true
}

// markup consumes a comment, CDATA section or child element at p.pos ('<'),
// flushing the pending text run before a child element.
func (p *xmlFastParser) markup(out *XMLNode, dom *domCounter, flush func(), depth int) bool {
	switch {
	case p.hasPrefix("<!--"):
		if !p.comment() {
			return false
		}
		dom.advance("Comment")
	case p.hasPrefix("<![CDATA["):
		end := bytes.Index(p.src[p.pos:], []byte("]]>"))
		if end < 0 {
			return false
		}
		p.pos += end + 3
		dom.advance("CDSect")
	case xmlFastNameStart(p.peek(1)):
		flush()
		index := dom.advance("element")
		child, ok := p.element(depth + 1)
		if !ok {
			return false
		}
		child.Index = index
		out.Children = append(out.Children, child)
	default:
		return false
	}
	return true
}

// content reads out's children up to and including its end tag. The text
// handling mirrors buildXMLNode: a run of character data and references
// lasts until the next child element, and comments and CDATA sections only
// advance the DOM index.
func (p *xmlFastParser) content(out *XMLNode, depth int) (*XMLNode, bool) {
	var run, charData strings.Builder
	runLine, runCol := 0, 0
	flush := func() {
		raw := run.String()
		run.Reset()
		trimmed := strings.TrimLeft(raw, " \t\r\n")
		if strings.TrimSpace(trimmed) == "" {
			return
		}
		line, col := runLine, runCol
		for _, r := range raw[:len(raw)-len(trimmed)] {
			if r == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		out.TextNodes = append(out.TextNodes, XMLTextNode{Text: trimmed, Line: line, Col: col})
	}
	addText := func(at int, text string) {
		if run.Len() == 0 {
			runLine, runCol = p.point(at)
		}
		run.WriteString(text)
	}
	var dom domCounter
	for {
		if p.pos >= len(p.src) {
			return nil, false
		}
		switch p.src[p.pos] {
		case '<':
			if p.hasPrefix("</") {
				if !p.endTag(out) {
					return nil, false
				}
				flush()
				out.Text = charData.String()
				return out, true
			}
			if !p.markup(out, &dom, flush, depth) {
				return nil, false
			}
		case '&':
			at := p.pos
			raw, kind, ok := p.reference()
			if !ok {
				return nil, false
			}
			dom.advance(kind)
			addText(at, html.UnescapeString(raw))
		default:
			at := p.pos
			end := bytes.IndexAny(p.src[at:], "<&")
			if end < 0 {
				return nil, false
			}
			text := string(p.src[at : at+end])
			if strings.Contains(text, "]]>") {
				return nil, false
			}
			p.pos = at + end
			dom.advance("CharData")
			charData.WriteString(text)
			addText(at, text)
		}
	}
}
