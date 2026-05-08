package lsp

import (
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// PositionToByteOffset converts an LSP position to a byte offset in content.
// LSP character offsets are UTF-16 code units, not UTF-8 bytes.
func PositionToByteOffset(content []byte, pos Position) int {
	lineStart := lineStartOffset(content, int(pos.Line))
	if lineStart >= len(content) {
		return len(content)
	}
	lineEnd := lineEndOffset(content, lineStart)
	wantUnits := int(pos.Character)
	seenUnits := 0
	for off := lineStart; off < lineEnd; {
		r, size := utf8.DecodeRune(content[off:lineEnd])
		if r == utf8.RuneError && size == 0 {
			break
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		if seenUnits+width > wantUnits {
			return off
		}
		seenUnits += width
		off += size
	}
	return lineEnd
}

func filePositionToByteOffset(file *scanner.File, pos Position) int {
	if file == nil {
		return 0
	}
	return PositionToByteOffset(file.Content, pos)
}

func lineStartOffset(content []byte, line int) int {
	if line <= 0 {
		return 0
	}
	current := 0
	for i, b := range content {
		if b == '\n' {
			current++
			if current == line {
				return i + 1
			}
		}
	}
	return len(content)
}

func lineEndOffset(content []byte, start int) int {
	for i := start; i < len(content); i++ {
		if content[i] == '\n' {
			if i > start && content[i-1] == '\r' {
				return i - 1
			}
			return i
		}
	}
	return len(content)
}
