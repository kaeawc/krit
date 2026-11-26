package api

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/hashutil"
)

// xmlLineCache holds the lines of XML and Gradle files that have findings
// without a precise column, so repeated findings in one file read it once.
// It lives as long as the process, which in the daemon, LSP and MCP spans
// many scans, so each entry records the size and mtime it was read at and is
// reused only while the file still has them (#1852).
var xmlLineCache sync.Map // path -> *xmlLinesEntry

type xmlLinesEntry struct {
	size  int64
	mtime time.Time
	lines [][]byte
}

// xmlLines returns the lines of path, or nil when it cannot be read.
func xmlLines(path string) [][]byte {
	info, err := os.Stat(path)
	if err != nil {
		xmlLineCache.Delete(path)
		return nil
	}
	if v, ok := xmlLineCache.Load(path); ok {
		if e := v.(*xmlLinesEntry); e.size == info.Size() && e.mtime.Equal(info.ModTime()) {
			return e.lines
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		xmlLineCache.Delete(path)
		return nil
	}
	lines := bytes.Split(data, []byte("\n"))
	// A file written within the filesystem's mtime granularity can change
	// again without its size or mtime moving, so only a file that was
	// already older than that when it was stat'd is cached. The stat comes
	// before the read, so the cached lines are never older than it.
	if hashutil.StatTrustedNow(info) {
		xmlLineCache.Store(path, &xmlLinesEntry{size: info.Size(), mtime: info.ModTime(), lines: lines})
	} else {
		xmlLineCache.Delete(path)
	}
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
