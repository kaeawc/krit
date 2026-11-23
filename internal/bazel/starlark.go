// Package bazel reads the parts of a Bazel workspace krit's project model
// needs: JVM and Android targets with their sources, dependencies and
// resource directories from BUILD files, and Maven artifacts from
// MODULE.bazel, WORKSPACE and rules_jvm_external lock files.
//
// BUILD files are Starlark, but the facts krit needs sit in top-level rule
// calls with literal arguments, so this is a small tolerant parser rather
// than an interpreter: it evaluates string and list literals, `+`
// concatenation, top-level list variables, glob(include) and the branches
// of select(), and treats everything else as opaque.
package bazel

import (
	"strings"
)

// Value is an evaluated Starlark expression. Only strings, lists, globs and
// the union of select() branches carry data; other expressions are opaque.
type Value struct {
	Str    string
	IsStr  bool
	List   []Value
	IsList bool
	// Glob holds the include patterns of a glob() call.
	Glob   []string
	IsGlob bool
	// Bool is set for the True literal.
	Bool bool
}

// Strings returns the string literals in v, flattening lists.
func (v Value) Strings() []string {
	switch {
	case v.IsStr:
		return []string{v.Str}
	case v.IsList:
		var out []string
		for _, e := range v.List {
			out = append(out, e.Strings()...)
		}
		return out
	}
	return nil
}

// Globs returns the glob include patterns in v, flattening lists.
func (v Value) Globs() []string {
	switch {
	case v.IsGlob:
		return v.Glob
	case v.IsList:
		var out []string
		for _, e := range v.List {
			out = append(out, e.Globs()...)
		}
		return out
	}
	return nil
}

// Call is a top-level rule or macro invocation such as
// `kt_jvm_library(name = "lib", srcs = glob(["*.kt"]))` or
// `maven.install(artifacts = [...])`.
type Call struct {
	Kind  string
	Attrs map[string]Value
	// Args holds positional arguments.
	Args []Value
	Line int
}

// Attr returns the named attribute, or an opaque zero Value.
func (c Call) Attr(name string) Value { return c.Attrs[name] }

type tokenKind uint8

const (
	tokIdent tokenKind = iota
	tokString
	tokNumber
	tokPunct
)

type token struct {
	kind tokenKind
	text string
	line int
	col  int
}

// tokenize splits Starlark source into tokens, skipping comments and
// whitespace. Malformed input never fails: an unterminated string runs to
// the end of the file.
func tokenize(src string) []token {
	lx := &lexer{src: src, line: 1}
	for lx.i < len(src) {
		lx.next()
	}
	return lx.toks
}

type lexer struct {
	src             string
	i, line, lineAt int
	toks            []token
}

func (lx *lexer) emit(kind tokenKind, text string, line, start int) {
	lx.toks = append(lx.toks, token{kind: kind, text: text, line: line, col: start - lx.lineAt})
}

func (lx *lexer) next() {
	src, c := lx.src, lx.src[lx.i]
	switch {
	case c == '\n':
		lx.line++
		lx.i++
		lx.lineAt = lx.i
	case c == ' ' || c == '\t' || c == '\r' || c == '\\':
		lx.i++
	case c == '#':
		for lx.i < len(src) && src[lx.i] != '\n' {
			lx.i++
		}
	case isStringStart(src, lx.i):
		lx.str()
	case isIdentStart(c):
		lx.word(tokIdent, isIdentPart)
	case c >= '0' && c <= '9':
		lx.word(tokNumber, func(b byte) bool { return isIdentPart(b) || b == '.' })
	default:
		lx.emit(tokPunct, string(c), lx.line, lx.i)
		lx.i++
	}
}

func isStringStart(src string, i int) bool {
	c := src[i]
	if c == '"' || c == '\'' {
		return true
	}
	return (c == 'r' || c == 'b' || c == 'u') && i+1 < len(src) && (src[i+1] == '"' || src[i+1] == '\'')
}

func (lx *lexer) word(kind tokenKind, part func(byte) bool) {
	j := lx.i + 1
	for j < len(lx.src) && part(lx.src[j]) {
		j++
	}
	lx.emit(kind, lx.src[lx.i:j], lx.line, lx.i)
	lx.i = j
}

func (lx *lexer) str() {
	start, line := lx.i, lx.line
	raw := false
	if c := lx.src[lx.i]; c != '"' && c != '\'' {
		raw = c == 'r'
		lx.i++
	}
	text, next, newlines := scanString(lx.src, lx.i, raw)
	lx.emit(tokString, text, line, start)
	if newlines > 0 {
		lx.line += newlines
		lx.lineAt = strings.LastIndexByte(lx.src[:next], '\n') + 1
	}
	lx.i = next
}

// scanString reads a quoted string starting at the quote at i. It returns
// the unescaped contents, the index after the closing quote, and the number
// of newlines inside.
func scanString(src string, i int, raw bool) (string, int, int) {
	q := src[i]
	triple := i+2 < len(src) && src[i+1] == q && src[i+2] == q
	if triple {
		i += 3
	} else {
		i++
	}
	var b strings.Builder
	newlines := 0
	for i < len(src) {
		c := src[i]
		if c == '\n' {
			newlines++
			if !triple {
				return b.String(), i, newlines
			}
		}
		if c == '\\' && !raw && i+1 < len(src) {
			b.WriteByte(src[i+1])
			if src[i+1] == '\n' {
				newlines++
			}
			i += 2
			continue
		}
		if c == q {
			if !triple {
				return b.String(), i + 1, newlines
			}
			if i+2 < len(src) && src[i+1] == q && src[i+2] == q {
				return b.String(), i + 3, newlines
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String(), i, newlines
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool { return isIdentStart(c) || (c >= '0' && c <= '9') }

type parser struct {
	toks []token
	pos  int
	vars map[string]Value
}

// ParseFile returns the top-level calls in a BUILD, MODULE.bazel or
// WORKSPACE file. Top-level list and string assignments are remembered so
// later calls can refer to them (`deps = COMMON_DEPS + [...]`); def, if and
// for blocks are skipped.
func ParseFile(src []byte) []Call {
	p := &parser{toks: tokenize(string(src)), vars: map[string]Value{}}
	var calls []Call
	for p.pos < len(p.toks) {
		start := p.pos
		t := p.toks[p.pos]
		if t.kind != tokIdent || t.col != 0 {
			p.pos++
			continue
		}
		switch t.text {
		case "def", "if", "for", "while", "with", "class":
			p.skipBlock(t.line)
			continue
		case "load":
			p.pos++
			if p.peekPunct("(") {
				p.skipBracketed()
			}
			continue
		}
		name := p.dottedName()
		switch {
		case p.peekPunct("("):
			call := p.parseCall(name, t.line)
			calls = append(calls, call)
		case p.peekPunct("=") && !strings.Contains(name, "."):
			p.pos++
			p.vars[name] = p.parseExpr()
		}
		if p.pos == start {
			p.pos++
		}
	}
	return calls
}

// skipBlock skips a compound statement: its header line and every
// following token indented past column 0.
func (p *parser) skipBlock(line int) {
	for p.pos < len(p.toks) && p.toks[p.pos].line == line {
		p.pos++
	}
	for p.pos < len(p.toks) && p.toks[p.pos].col > 0 {
		p.pos++
	}
}

func (p *parser) peekPunct(s string) bool {
	return p.pos < len(p.toks) && p.toks[p.pos].kind == tokPunct && p.toks[p.pos].text == s
}

func (p *parser) dottedName() string {
	name := p.toks[p.pos].text
	p.pos++
	for p.peekPunct(".") && p.pos+1 < len(p.toks) && p.toks[p.pos+1].kind == tokIdent {
		name += "." + p.toks[p.pos+1].text
		p.pos += 2
	}
	return name
}

// skipBracketed skips from an opening bracket to its match.
func (p *parser) skipBracketed() {
	depth := 0
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		p.pos++
		if t.kind != tokPunct {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth <= 0 {
				return
			}
		}
	}
}

func (p *parser) parseCall(kind string, line int) Call {
	call := Call{Kind: kind, Attrs: map[string]Value{}, Line: line}
	p.pos++ // (
	for p.pos < len(p.toks) && !p.peekPunct(")") {
		before := p.pos
		if p.toks[p.pos].kind == tokIdent && p.pos+1 < len(p.toks) &&
			p.toks[p.pos+1].kind == tokPunct && p.toks[p.pos+1].text == "=" &&
			(p.pos+2 >= len(p.toks) || p.toks[p.pos+2].text != "=") {
			key := p.toks[p.pos].text
			p.pos += 2
			call.Attrs[key] = p.parseExpr()
		} else if p.peekPunct("*") {
			p.pos++
			if p.peekPunct("*") {
				p.pos++
			}
			p.parseExpr()
		} else {
			call.Args = append(call.Args, p.parseExpr())
		}
		if p.peekPunct(",") {
			p.pos++
		}
		if p.pos == before {
			p.pos++
		}
	}
	if p.peekPunct(")") {
		p.pos++
	}
	return call
}

// parseExpr evaluates `primary (+ primary)*`, skipping any other operator
// or trailing method call as opaque.
func (p *parser) parseExpr() Value {
	v := p.parsePrimary()
	for p.pos < len(p.toks) {
		switch {
		case p.peekPunct("+"):
			p.pos++
			v = concat(v, p.parsePrimary())
		case p.peekPunct(".") || p.peekPunct("%") || p.peekPunct("["):
			// Method calls, formatting and indexing produce values this
			// parser does not model.
			p.pos++
			if p.pos < len(p.toks) && p.toks[p.pos].kind == tokIdent {
				p.pos++
			}
			if p.peekPunct("(") || p.peekPunct("[") {
				p.skipBracketed()
			}
			v = Value{}
		case p.pos < len(p.toks) && p.toks[p.pos].kind == tokIdent && (p.toks[p.pos].text == "if" || p.toks[p.pos].text == "for"):
			// Conditional expressions and comprehensions end the value.
			return Value{}
		default:
			return v
		}
	}
	return v
}

func concat(a, b Value) Value {
	switch {
	case a.IsStr && b.IsStr:
		return Value{Str: a.Str + b.Str, IsStr: true}
	case (a.IsList || a.IsGlob) && (b.IsList || b.IsGlob):
		return Value{List: append(asList(a), asList(b)...), IsList: true}
	case a.IsList || a.IsGlob:
		return Value{List: asList(a), IsList: true}
	case b.IsList || b.IsGlob:
		return Value{List: asList(b), IsList: true}
	}
	return Value{}
}

func asList(v Value) []Value {
	if v.IsList {
		return append([]Value(nil), v.List...)
	}
	return []Value{v}
}

func (p *parser) parsePrimary() Value {
	if p.pos >= len(p.toks) {
		return Value{}
	}
	t := p.toks[p.pos]
	switch t.kind {
	case tokString:
		p.pos++
		s := t.text
		// Adjacent string literals concatenate.
		for p.pos < len(p.toks) && p.toks[p.pos].kind == tokString {
			s += p.toks[p.pos].text
			p.pos++
		}
		return Value{Str: s, IsStr: true}
	case tokNumber:
		p.pos++
		return Value{}
	case tokIdent:
		return p.parseIdentPrimary(t)
	}
	switch t.text {
	case "[":
		return p.parseList()
	case "{":
		return p.parseDict()
	case "(":
		return p.parseParenthesized()
	case "-":
		p.pos++
		p.parsePrimary()
		return Value{}
	}
	p.pos++
	return Value{}
}

func (p *parser) parseIdentPrimary(t token) Value {
	switch t.text {
	case "True":
		p.pos++
		return Value{Bool: true}
	case "False", "None":
		p.pos++
		return Value{}
	case "lambda":
		p.pos++
		for p.pos < len(p.toks) && !p.peekPunct(":") {
			p.pos++
		}
		p.pos++
		return p.parseExpr()
	}
	name := p.dottedName()
	if p.peekPunct("(") {
		return p.evalCall(p.parseCall(name, t.line))
	}
	return p.vars[name]
}

// parseParenthesized reads a parenthesized expression or a tuple, whose
// elements are kept as a list.
func (p *parser) parseParenthesized() Value {
	p.pos++ // (
	v := p.parseExpr()
	if p.peekPunct(",") {
		items := []Value{v}
		for p.peekPunct(",") {
			p.pos++
			if p.peekPunct(")") {
				break
			}
			items = append(items, p.parseExpr())
		}
		v = Value{List: items, IsList: true}
	}
	if p.peekPunct(")") {
		p.pos++
	}
	return v
}

func (p *parser) parseList() Value {
	start := p.pos
	p.pos++ // [
	var items []Value
	for p.pos < len(p.toks) && !p.peekPunct("]") {
		before := p.pos
		items = append(items, p.parseExpr())
		if p.pos < len(p.toks) && p.toks[p.pos].kind == tokIdent && p.toks[p.pos].text == "for" {
			// List comprehension: opaque.
			p.pos = start
			p.skipBracketed()
			return Value{}
		}
		if p.peekPunct(",") {
			p.pos++
		}
		if p.pos == before {
			p.pos++
		}
	}
	if p.peekPunct("]") {
		p.pos++
	}
	return Value{List: items, IsList: true}
}

// parseDict returns the union of the dict's values as a list, which is what
// a select() over lists can contribute.
func (p *parser) parseDict() Value {
	start := p.pos
	p.pos++ // {
	var items []Value
	for p.pos < len(p.toks) && !p.peekPunct("}") {
		before := p.pos
		p.parseExpr()
		if p.pos < len(p.toks) && p.toks[p.pos].kind == tokIdent && p.toks[p.pos].text == "for" {
			p.pos = start
			p.skipBracketed()
			return Value{}
		}
		if p.peekPunct(":") {
			p.pos++
			v := p.parseExpr()
			if v.IsList {
				items = append(items, v.List...)
			} else if v.IsStr || v.IsGlob {
				items = append(items, v)
			}
		}
		if p.peekPunct(",") {
			p.pos++
		}
		if p.pos == before {
			p.pos++
		}
	}
	if p.peekPunct("}") {
		p.pos++
	}
	return Value{List: items, IsList: true}
}

// evalCall evaluates the builtins whose results carry project facts.
func (p *parser) evalCall(c Call) Value {
	switch c.Kind {
	case "glob":
		include := c.Attr("include")
		if len(c.Args) > 0 {
			include = c.Args[0]
		}
		return Value{Glob: include.Strings(), IsGlob: true}
	case "select":
		if len(c.Args) > 0 {
			return c.Args[0]
		}
	}
	return Value{}
}
