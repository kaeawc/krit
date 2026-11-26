package rules

import (
	"bytes"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/scanner"
)

// slotComposeLintsCommentsByEnd is the per-file fact mapping a comment's end byte to its flat node.
const slotComposeLintsCommentsByEnd = "composeLintsCommentsByEnd"

// composeLintsDeclarationStart returns the line and column (1-based) where Lint's location for a Kotlin declaration
// starts. Kotlin PSI binds leading comments into the declaration: the nearest KDoc in the run of comments directly
// before it (even across blank lines), otherwise the own-line comments not separated from it by a blank line.
func composeLintsDeclarationStart(file *scanner.File, decl uint32) (int, int) {
	start := decl
	if file.FlatType(decl) == "multiline_comment" || file.FlatType(decl) == "line_comment" {
		return file.FlatRow(decl) + 1, file.FlatCol(decl) + 1
	}
	byEnd := filefacts.FileFact(fileFactsCache(), file, slotComposeLintsCommentsByEnd, func() map[uint32]uint32 {
		out := map[uint32]uint32{}
		file.FlatWalkAllNodes(0, func(n uint32) {
			if t := file.FlatType(n); t == "line_comment" || t == "multiline_comment" {
				out[file.FlatEndByte(n)] = n
			}
		})
		return out
	})
	if len(byEnd) == 0 {
		return file.FlatRow(decl) + 1, file.FlatCol(decl) + 1
	}
	content := file.Content
	// Collect the comments directly before decl, nearest first.
	type leading struct {
		comment    uint32
		ownLine    bool
		blankAfter bool
	}
	var run []leading
	pos := file.FlatStartByte(decl)
	for {
		ws := pos
		for ws > 0 && isComposeLintsSpace(content[ws-1]) {
			ws--
		}
		comment, ok := byEnd[ws]
		if !ok {
			break
		}
		gap := content[ws:pos]
		cStart := file.FlatStartByte(comment)
		before := cStart
		for before > 0 && (content[before-1] == ' ' || content[before-1] == '\t') {
			before--
		}
		run = append(run, leading{comment: comment,
			ownLine:    before == 0 || content[before-1] == '\n' || content[before-1] == '\r',
			blankAfter: bytes.Count(gap, []byte{'\n'}) > 1})
		pos = cStart
	}
	for _, l := range run {
		if file.FlatType(l.comment) == "multiline_comment" && strings.HasPrefix(file.FlatNodeText(l.comment), "/**") {
			start = l.comment
			return file.FlatRow(start) + 1, file.FlatCol(start) + 1
		}
	}
	for _, l := range run {
		if l.blankAfter || !l.ownLine {
			break
		}
		start = l.comment
	}
	return file.FlatRow(start) + 1, file.FlatCol(start) + 1
}

func isComposeLintsSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func composeLintsIsComment(file *scanner.File, node uint32) bool {
	t := file.FlatType(node)
	return t == "line_comment" || t == "multiline_comment"
}
