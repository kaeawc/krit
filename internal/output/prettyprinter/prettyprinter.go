// Package prettyprinter implements a Wadler-Lindig style pretty printer.
//
// A document (Doc) is an algebraic value composed from Text, SoftLine,
// HardLine, Nest, Group, and Concat. Rendering chooses between flat and
// broken layouts for each Group so that the result fits within a target
// column width when possible.
//
// The algorithm is the iterative variant of Wadler's "A prettier
// printer" due to Lindig: Render walks the document with an explicit
// stack, and Group decisions are made by a bounded fits() check that
// stops as soon as a break-mode line is reached.
package prettyprinter

import (
	"io"
	"strings"
)

// Doc is a pretty-printable document.
type Doc interface {
	isDoc()
}

type docNil struct{}
type docText struct{ s string }
type docLine struct {
	// hard forces a line break even inside a flat group.
	hard bool
}
type docConcat struct{ left, right Doc }
type docNest struct {
	n int
	d Doc
}
type docGroup struct{ d Doc }

func (docNil) isDoc()    {}
func (docText) isDoc()   {}
func (docLine) isDoc()   {}
func (docConcat) isDoc() {}
func (docNest) isDoc()   {}
func (docGroup) isDoc()  {}

var nilDoc Doc = docNil{}

// Nil is the empty document.
func Nil() Doc { return nilDoc }

// Text is a literal string. The string must not contain newlines; use
// HardLine or SoftLine for line breaks.
func Text(s string) Doc {
	if s == "" {
		return nilDoc
	}
	return docText{s: s}
}

// SoftLine is a line break that renders as a single space when the
// enclosing Group fits on one line, and as a newline otherwise.
func SoftLine() Doc { return docLine{hard: false} }

// HardLine is a line break that always renders as a newline and forces
// its enclosing Group to use the broken layout.
func HardLine() Doc { return docLine{hard: true} }

// Nest increases the indentation level of d by n columns. Indentation
// is applied after newlines inside d.
func Nest(n int, d Doc) Doc {
	if n == 0 {
		return d
	}
	if _, ok := d.(docNil); ok {
		return nilDoc
	}
	return docNest{n: n, d: d}
}

// Group marks d as a unit that should be rendered flat when it fits on
// the current line, or broken otherwise.
func Group(d Doc) Doc {
	if _, ok := d.(docNil); ok {
		return nilDoc
	}
	return docGroup{d: d}
}

// Concat returns the left-to-right concatenation of docs. Nil entries
// are skipped.
func Concat(docs ...Doc) Doc {
	var result Doc = nilDoc
	for i := len(docs) - 1; i >= 0; i-- {
		d := docs[i]
		if d == nil {
			continue
		}
		if _, ok := d.(docNil); ok {
			continue
		}
		if _, ok := result.(docNil); ok {
			result = d
			continue
		}
		result = docConcat{left: d, right: result}
	}
	return result
}

// Join concatenates docs interleaved with sep.
func Join(sep Doc, docs ...Doc) Doc {
	if len(docs) == 0 {
		return nilDoc
	}
	parts := make([]Doc, 0, 2*len(docs)-1)
	for i, d := range docs {
		if i > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, d)
	}
	return Concat(parts...)
}

type mode uint8

const (
	modeBreak mode = 0
	modeFlat  mode = 1
)

type frame struct {
	indent int
	mode   mode
	doc    Doc
}

// Render writes the best layout for d to w, targeting the given column
// width. width <= 0 forces every Group to break.
func Render(w io.Writer, width int, d Doc) error {
	r := newRenderer(w)
	stack := []frame{{indent: 0, mode: modeBreak, doc: d}}
	for len(stack) > 0 && r.err == nil {
		n := len(stack) - 1
		f := stack[n]
		stack = stack[:n]
		switch v := f.doc.(type) {
		case docNil:
		case docText:
			r.writeString(v.s)
			r.col += len(v.s)
		case docLine:
			if f.mode == modeFlat && !v.hard {
				r.writeByte(' ')
				r.col++
			} else {
				r.writeByte('\n')
				r.writeIndent(f.indent)
				r.col = f.indent
			}
		case docConcat:
			stack = append(stack,
				frame{indent: f.indent, mode: f.mode, doc: v.right},
				frame{indent: f.indent, mode: f.mode, doc: v.left},
			)
		case docNest:
			stack = append(stack, frame{indent: f.indent + v.n, mode: f.mode, doc: v.d})
		case docGroup:
			if f.mode == modeFlat {
				stack = append(stack, frame{indent: f.indent, mode: modeFlat, doc: v.d})
				continue
			}
			if width > 0 && fits(width-r.col, v.d, f.indent, stack) {
				stack = append(stack, frame{indent: f.indent, mode: modeFlat, doc: v.d})
			} else {
				stack = append(stack, frame{indent: f.indent, mode: modeBreak, doc: v.d})
			}
		}
	}
	return r.err
}

// RenderString renders d and returns the resulting string.
func RenderString(width int, d Doc) string {
	var sb strings.Builder
	_ = Render(&sb, width, d)
	return sb.String()
}

// fits reports whether rendering gdoc in flat mode followed by the
// remaining frames on stack keeps the column count within remaining,
// stopping at the first break-mode line (which is considered a fit).
//
// The check costs O(k) where k is the number of nodes before the
// earliest break-mode line, giving WL's bounded lookahead.
func fits(remaining int, gdoc Doc, gindent int, outer []frame) bool {
	if remaining < 0 {
		return false
	}
	// Local work stack for substructure of gdoc; outer is walked in
	// place (top-of-stack is the highest index) once work drains.
	var work []frame
	work = append(work, frame{indent: gindent, mode: modeFlat, doc: gdoc})
	outerTop := len(outer) - 1

	for {
		var f frame
		if n := len(work); n > 0 {
			f = work[n-1]
			work = work[:n-1]
		} else if outerTop >= 0 {
			f = outer[outerTop]
			outerTop--
		} else {
			break
		}
		switch v := f.doc.(type) {
		case docNil:
		case docText:
			remaining -= len(v.s)
			if remaining < 0 {
				return false
			}
		case docLine:
			if v.hard && f.mode == modeFlat {
				return false
			}
			if f.mode == modeFlat {
				remaining--
				if remaining < 0 {
					return false
				}
			} else {
				return true
			}
		case docConcat:
			work = append(work,
				frame{indent: f.indent, mode: f.mode, doc: v.right},
				frame{indent: f.indent, mode: f.mode, doc: v.left},
			)
		case docNest:
			work = append(work, frame{indent: f.indent + v.n, mode: f.mode, doc: v.d})
		case docGroup:
			// Nested groups inherit the outer mode during fits: once
			// flat is committed, inner groups must also fit flat.
			work = append(work, frame{indent: f.indent, mode: f.mode, doc: v.d})
		}
	}
	return remaining >= 0
}

// renderer is a tiny buffered writer that tracks the current column and
// the first error encountered. Writes become no-ops once err is set.
type renderer struct {
	w   io.Writer
	sw  io.StringWriter // non-nil when w implements StringWriter
	bw  io.ByteWriter   // non-nil when w implements ByteWriter
	col int
	err error
}

func newRenderer(w io.Writer) renderer {
	r := renderer{w: w}
	if sw, ok := w.(io.StringWriter); ok {
		r.sw = sw
	}
	if bw, ok := w.(io.ByteWriter); ok {
		r.bw = bw
	}
	return r
}

func (r *renderer) writeString(s string) {
	if r.err != nil || s == "" {
		return
	}
	if r.sw != nil {
		_, r.err = r.sw.WriteString(s)
		return
	}
	_, r.err = r.w.Write([]byte(s))
}

func (r *renderer) writeByte(b byte) {
	if r.err != nil {
		return
	}
	if r.bw != nil {
		r.err = r.bw.WriteByte(b)
		return
	}
	_, r.err = r.w.Write([]byte{b})
}

// indentChunk is reused to amortize allocations when emitting
// indentation runs of typical widths (up to 64 columns).
var indentChunk = strings.Repeat(" ", 64)

func (r *renderer) writeIndent(n int) {
	for n >= len(indentChunk) {
		r.writeString(indentChunk)
		n -= len(indentChunk)
	}
	if n > 0 {
		r.writeString(indentChunk[:n])
	}
}
