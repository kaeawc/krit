// Package sourceheader extracts package and import header text from
// tree-sitter node strings. Tree-sitter Kotlin/Java sometimes attaches
// trailing comments and whitespace to header nodes; the helpers here
// strip that trivia so callers see only the dotted name.
//
// The package was extracted from internal/rename and internal/scanner,
// which previously kept byte-identical copies of FirstSourceLine and
// FirstHeaderLine. See issue #44.
package sourceheader

import "strings"

// FirstSourceLine returns the first non-empty, non-comment line of raw,
// trimmed. Used to ignore trailing trivia tree-sitter sometimes attaches
// to header nodes (line/block comments, blank lines).
//
// Single-pass over raw: walks the line-delimited string in place so we
// don't allocate a slice covering every line for the typical
// one-or-two-line header.
func FirstSourceLine(raw string) string {
	for s := raw; ; {
		var line string
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			line = strings.TrimSpace(s)
		} else {
			line = strings.TrimSpace(s[:i])
		}
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "/*") {
			return line
		}
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
}

// FirstHeaderLine returns the first source line of raw with the given
// keyword (e.g. "package", "import") and any trailing semicolon stripped.
// Returns the empty string when raw has no source line.
func FirstHeaderLine(raw, keyword string) string {
	line := FirstSourceLine(raw)
	line = strings.TrimPrefix(line, keyword)
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ";")
	return strings.TrimSpace(line)
}
