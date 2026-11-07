package api

import (
	"bytes"
	"os"
	"strings"
	"sync"
)

// xmlLineCache holds the lines of XML files that have findings without a
// precise column, so repeated findings in one file read it once.
var xmlLineCache sync.Map // path -> [][]byte

// xmlLines returns the cached lines of path, or nil when it cannot be read.
func xmlLines(path string) [][]byte {
	if v, ok := xmlLineCache.Load(path); ok {
		return v.([][]byte)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := bytes.Split(data, []byte("\n"))
	xmlLineCache.Store(path, lines)
	return lines
}

// xmlSpan returns the byte span of the token at the 1-based line and column of
// a file without a syntax tree, or 0, 0 when the position is outside the file.
func xmlSpan(path string, line, col int) (start, end int) {
	lines := xmlLines(path)
	if line < 1 || line > len(lines) || col < 1 || col > len(lines[line-1]) {
		return 0, 0
	}
	offset := 0
	for _, l := range lines[:line-1] {
		offset += len(l) + 1
	}
	start = offset + col - 1
	lineEnd := offset + len(lines[line-1])
	end = start + tokenEndAt(lines[line-1], col-1) - (col - 1)
	if end > lineEnd {
		end = lineEnd
	}
	return start, end
}

// xmlIndentCol returns the 1-based column of the first non-blank byte on the
// given 1-based line of an XML file, or 0 when it cannot be determined.
func xmlIndentCol(path string, line int) int {
	if line < 1 {
		return 0
	}
	lines := xmlLines(path)
	if lines == nil {
		return 0
	}
	if line > len(lines) {
		return 0
	}
	l := lines[line-1]
	for i, b := range l {
		if b != ' ' && b != '\t' {
			if b == '\r' {
				return 0
			}
			return i + 1
		}
	}
	return 0
}

// anchorXMLColumn gives XML and Gradle findings that carry only a line the
// column of the statement start (first non-blank byte) so they do not point
// into indentation.
func anchorXMLColumn(file string, line, col int) int {
	if col > 1 || line < 1 {
		return col
	}
	if !strings.HasSuffix(file, ".xml") && !strings.HasSuffix(file, ".gradle") && !strings.HasSuffix(file, ".gradle.kts") {
		return col
	}
	if c := xmlIndentCol(file, line); c > 0 {
		return c
	}
	return col
}
