package scanner

import "strings"

// HeaderImport is one import directive from a source file header.
type HeaderImport struct {
	// Path is the imported name, without a trailing `.*` and with backticks
	// removed.
	Path string
	// Alias is the `as` name, or "".
	Alias string
	// Wildcard marks `import pkg.*`; Path is then the package.
	Wildcard bool
}

// SourceHeader is the package declaration and import list of a Kotlin file.
type SourceHeader struct {
	Package string
	Imports []HeaderImport
}

// ParseSourceHeader reads the package declaration and imports from the start
// of Kotlin source. It scans only the header prefix: whitespace, line, block
// and KDoc comments, file annotations (`@file:...`), `package`, and `import`
// directives, and it stops at the first token that is none of those, so the
// cost does not grow with the file body. Comments are skipped lexically, so a
// `package` inside a block comment is not a declaration, and backticked
// segments are accepted.
func ParseSourceHeader(content []byte) SourceHeader {
	var h SourceHeader
	i := 0
	n := len(content)
	for i < n {
		i = skipHeaderTrivia(content, i)
		if i >= n {
			break
		}
		switch {
		case content[i] == '@':
			next, ok := skipFileAnnotation(content, i)
			if !ok {
				return h
			}
			i = next
		case hasKeyword(content, i, "package"):
			path, _, next := readDottedName(content, i+len("package"))
			if path == "" {
				return h
			}
			h.Package = path
			i = next
		case hasKeyword(content, i, "import"):
			path, wildcard, next := readDottedName(content, i+len("import"))
			if path == "" {
				return h
			}
			imp := HeaderImport{Path: path, Wildcard: wildcard}
			j := skipSpaces(content, next)
			if hasKeyword(content, j, "as") {
				if alias, _, after := readDottedName(content, j+len("as")); alias != "" {
					imp.Alias = alias
					next = after
				}
			}
			h.Imports = append(h.Imports, imp)
			i = next
		case content[i] == ';':
			i++
		default:
			return h
		}
	}
	return h
}

func isHeaderSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func skipSpaces(content []byte, i int) int {
	for i < len(content) && isHeaderSpace(content[i]) {
		i++
	}
	return i
}

// skipHeaderTrivia skips whitespace and comments (nested block comments
// included) starting at i.
func skipHeaderTrivia(content []byte, i int) int {
	n := len(content)
	for i < n {
		switch {
		case isHeaderSpace(content[i]):
			i++
		case content[i] == '/' && i+1 < n && content[i+1] == '/':
			for i < n && content[i] != '\n' {
				i++
			}
		case content[i] == '/' && i+1 < n && content[i+1] == '*':
			i = skipBlockComment(content, i)
		default:
			return i
		}
	}
	return i
}

// skipBlockComment returns the index after the (possibly nested) block
// comment that starts at i, or len(content) when it is unterminated.
func skipBlockComment(content []byte, i int) int {
	n := len(content)
	depth := 0
	for i < n {
		switch {
		case content[i] == '/' && i+1 < n && content[i+1] == '*':
			depth++
			i += 2
		case content[i] == '*' && i+1 < n && content[i+1] == '/':
			depth--
			i += 2
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return n
}

func isHeaderIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= 0x80 ||
		(b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// hasKeyword reports whether word starts at i as a whole token.
func hasKeyword(content []byte, i int, word string) bool {
	end := i + len(word)
	if i < 0 || end > len(content) || string(content[i:end]) != word {
		return false
	}
	return end == len(content) || !isHeaderIdentByte(content[end])
}

// skipFileAnnotation skips `@file:Name`, `@file:Name(...)`, and
// `@file:[A B]` at i. It reports false for any other annotation, which means
// the header has ended.
func skipFileAnnotation(content []byte, i int) (int, bool) {
	if !strings.HasPrefix(string(content[i:min(len(content), i+6)]), "@file:") {
		return i, false
	}
	i += len("@file:")
	if i < len(content) && content[i] == '[' {
		return skipBalanced(content, i, '[', ']'), true
	}
	for i < len(content) && (isHeaderIdentByte(content[i]) || content[i] == '.') {
		i++
	}
	if j := skipSpaces(content, i); j < len(content) && content[j] == '(' {
		i = skipBalanced(content, j, '(', ')')
	}
	return i, true
}

// skipBalanced returns the index after the bracket pair opened at i,
// stepping over string literals so brackets inside them do not count.
func skipBalanced(content []byte, i int, open, closer byte) int {
	n := len(content)
	depth := 0
	for i < n {
		switch content[i] {
		case '"':
			i++
			for i < n && content[i] != '"' && content[i] != '\n' {
				if content[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case open:
			depth++
			i++
		case closer:
			depth--
			i++
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return n
}

// readDottedName reads `a.b.c` (segments may be backticked, and a trailing
// `.*` marks a wildcard) after skipping trivia from i.
func readDottedName(content []byte, i int) (path string, wildcard bool, next int) {
	var sb strings.Builder
	n := len(content)
	i = skipHeaderTrivia(content, i)
	for i < n {
		switch {
		case content[i] == '`':
			j := i + 1
			for j < n && content[j] != '`' && content[j] != '\n' {
				j++
			}
			sb.Write(content[i+1 : j])
			i = j
			if i < n && content[i] == '`' {
				i++
			}
		case isHeaderIdentByte(content[i]):
			j := i
			for j < n && isHeaderIdentByte(content[j]) {
				j++
			}
			sb.Write(content[i:j])
			i = j
		default:
			return sb.String(), wildcard, i
		}
		if i < n && content[i] == '.' {
			if i+1 < n && content[i+1] == '*' {
				return sb.String(), true, i + 2
			}
			sb.WriteByte('.')
			i++
			continue
		}
		return sb.String(), false, i
	}
	return sb.String(), false, i
}
