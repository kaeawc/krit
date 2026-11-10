package scanner

import (
	"bytes"
	"regexp"
	"sort"
)

// tree-sitter-kotlin cannot attach a primary constructor that starts on a
// line after the class header:
//
//	class Foo
//	@Inject
//	constructor(a: A) { ... }
//
// The class loses its body and every later top-level declaration becomes an
// ERROR / prefix_expression chain. The grammar accepts the same tokens when
// they share a line, so before parsing we blank the line breaks in that gap
// with spaces. The rewrite keeps the byte length, so every offset is
// unchanged. Rows and columns are recomputed from the original source after
// flattening.

const ctorKeyword = "constructor"

// maxCtorGapLines bounds the backward walk over annotation lines.
const maxCtorGapLines = 40

var ctorClassHeaderRe = regexp.MustCompile(`(?:^|\s)class\s+[A-Za-z_]\w*(?:\s*<[^()]*>)?(?:\s+@[\w.]+(?:\([^()]*\))?)*$`)

func blankBreaks(content []byte, offsets []int) []byte {
	out := make([]byte, len(content))
	copy(out, content)
	for _, o := range offsets {
		out[o] = ' '
	}
	return out
}

// ctorGapBreaks returns the offsets of the '\n' and '\r' bytes to blank, in
// ascending order, or nil when the source has no affected class header.
func ctorGapBreaks(content []byte) []int {
	var out []int
	pos := 0
	for {
		rel := bytes.Index(content[pos:], []byte(ctorKeyword))
		if rel < 0 {
			break
		}
		hit := pos + rel
		pos = hit + len(ctorKeyword)
		if hit > 0 && isIdentByte(content[hit-1]) {
			continue
		}
		if pos < len(content) && isIdentByte(content[pos]) {
			continue
		}
		out = append(out, ctorGapForHit(content, hit)...)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Ints(out)
	return out
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func lineStartOf(content []byte, i int) int {
	if j := bytes.LastIndexByte(content[:i], '\n'); j >= 0 {
		return j + 1
	}
	return 0
}

func ctorGapForHit(content []byte, hit int) []int {
	ls := lineStartOf(content, hit)
	if !ctorGapPrefixAllowed(bytes.TrimSpace(content[ls:hit])) {
		return nil
	}
	depth := 0
	cur := ls
	var comments []int
	for n := 0; n < maxCtorGapLines && cur > 0; n++ {
		ps, line := previousLine(content, cur)
		lineEnd := ps + len(line)
		if c := lineCommentStart(line); c >= 0 {
			// Blanking the break would otherwise pull the next line into the comment.
			comments = appendRange(comments, ps+c, lineEnd)
			line = line[:c]
		} else if c == -2 {
			return nil
		}
		trimmed := bytes.TrimSpace(line)
		if bytes.HasSuffix(trimmed, []byte("*/")) {
			// A block comment between the header and the constructor.
			start, ok := blockCommentStart(content, ps)
			if !ok {
				return nil
			}
			cur = start
			continue
		}
		if bytes.Contains(trimmed, []byte("/*")) || bytes.HasPrefix(trimmed, []byte("*")) {
			return nil
		}
		depth += bytes.Count(trimmed, []byte(")")) - bytes.Count(trimmed, []byte("("))
		switch {
		case len(trimmed) == 0 || depth > 0 || trimmed[0] == '@' && depth == 0 && !ctorClassHeaderRe.Match(trimmed):
			cur = ps
			continue
		case ctorClassHeaderRe.Match(trimmed):
			return append(lineBreaksIn(content, lineEnd, ls), comments...)
		default:
			return nil
		}
	}
	return nil
}

// lineCommentStart returns the offset of a "//" comment in line, -1 when there is none, and -2 when a quote
// precedes it so the "//" may sit inside a string.
func lineCommentStart(line []byte) int {
	c := bytes.Index(line, []byte("//"))
	if c >= 0 && bytes.ContainsAny(line[:c], "\"'") {
		return -2
	}
	return c
}

func appendRange(out []int, from, to int) []int {
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

// ctorGapPrefixAllowed reports whether the text before a constructor hit on its line may
// precede a split constructor: nothing, an annotation, or a visibility modifier.
func ctorGapPrefixAllowed(prefix []byte) bool {
	if len(prefix) > 0 && prefix[0] != '@' && !isVisibilityPrefix(prefix) {
		return false
	}
	return !bytes.ContainsAny(prefix, "{}=;")
}

// previousLine returns the start and text, without a trailing '\r', of the line ending just before cur.
func previousLine(content []byte, cur int) (int, []byte) {
	// content[cur-1] is the '\n' ending the previous line.
	pe := cur - 1
	ps := lineStartOf(content, pe)
	line := content[ps:pe]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return ps, line
}

// lineBreaksIn returns the offsets of '\n' and '\r' bytes in content[from:to].
func lineBreaksIn(content []byte, from, to int) []int {
	var breaks []int
	for i := from; i < to; i++ {
		if content[i] == '\n' || content[i] == '\r' {
			breaks = append(breaks, i)
		}
	}
	return breaks
}

func isVisibilityPrefix(p []byte) bool {
	for _, w := range [][]byte{[]byte("private"), []byte("internal"), []byte("protected"), []byte("public")} {
		if bytes.Equal(p, w) {
			return true
		}
	}
	return false
}

// restoreOriginalPositions rewrites StartRows and StartCols from the original
// source after a parse of rewritten text.
func restoreOriginalPositions(t *FlatTree, original []byte) {
	if t == nil || len(ctorGapBreaks(original)) == 0 {
		return
	}
	restorePositions(t, original)
}

func restorePositions(t *FlatTree, original []byte) {
	starts := []int{0}
	for i, b := range original {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	for i, off := range t.StartBytes {
		row := sort.SearchInts(starts, int(off)+1) - 1
		t.StartRows[i] = saturateUint16(uint32(row))
		t.StartCols[i] = saturateUint16(off - uint32(starts[row]))
	}
}

// blockCommentStart returns the start of the line holding the "/*" that opens
// the block comment ending on the line that starts at ls.
func blockCommentStart(content []byte, ls int) (int, bool) {
	for n := 0; n < maxCtorGapLines; n++ {
		end := bytes.IndexByte(content[ls:], '\n')
		if end < 0 {
			end = len(content) - ls
		}
		line := content[ls : ls+end]
		if i := bytes.Index(line, []byte("/*")); i >= 0 {
			if len(bytes.TrimSpace(line[:i])) != 0 {
				return 0, false
			}
			return ls, true
		}
		if ls == 0 {
			return 0, false
		}
		ls = lineStartOf(content, ls-1)
	}
	return 0, false
}
