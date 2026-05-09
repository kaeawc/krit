package rename

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// fileContext captures the package, imports, and their byte ranges for a
// single source file. References in that file resolve to FQNs through
// Imports/Aliases/Wildcards; Apply rewrites import and package lines via
// the byte ranges. A single AST walk in buildFileContext produces both.
type fileContext struct {
	File         string
	Package      string
	PackageRange [2]int
	Language     scanner.Language

	// Imports is keyed by simple name → {FQN, byte range}.
	// Aliases is keyed by alias → {FQN, byte range}.
	// Wildcards is keyed by package → {FQN: "", byte range}.
	Imports   map[string]importInfo
	Aliases   map[string]importInfo
	Wildcards map[string]importInfo
}

type importInfo struct {
	FQN   string
	Range [2]int
}

func buildFileContext(file *scanner.File) fileContext {
	ctx := fileContext{
		Imports:   make(map[string]importInfo),
		Aliases:   make(map[string]importInfo),
		Wildcards: make(map[string]importInfo),
	}
	if file == nil || file.FlatTree == nil {
		return ctx
	}
	ctx.File = file.Path
	ctx.Language = file.Language

	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "package_header", "package_declaration":
			if ctx.Package == "" {
				ctx.Package = firstHeaderLine(file.FlatNodeText(idx), "package")
				ctx.PackageRange = clampToFirstLine(file, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
			}
		case "import_header":
			recordImport(file, idx, &ctx, parseKotlinImport)
		case "import_declaration":
			recordImport(file, idx, &ctx, parseJavaImport)
		}
	})
	return ctx
}

// resolveSimpleName returns the FQN that name refers to in this file via
// alias or explicit import, or "" if the file did not import it.
func (c fileContext) resolveSimpleName(name string) string {
	if name == "" {
		return ""
	}
	if e, ok := c.Aliases[name]; ok {
		return e.FQN
	}
	if e, ok := c.Imports[name]; ok {
		return e.FQN
	}
	return ""
}

// matchesFQN reports whether a reference of the given simple name in this
// file resolves to fqn. Resolution order: alias > explicit import >
// same-package > wildcard import. A wildcard match counts only when no
// explicit alias/import for name exists.
func (c fileContext) matchesFQN(name, fqn string) bool {
	if name == "" || fqn == "" {
		return false
	}
	if resolved := c.resolveSimpleName(name); resolved != "" {
		return resolved == fqn
	}
	parent, simple, ok := splitFQN(fqn)
	if !ok || simple != name {
		return false
	}
	if c.Package != "" && c.Package == parent {
		return true
	}
	for wp := range c.Wildcards {
		if wp == parent {
			return true
		}
	}
	return false
}

// findImportByFQN returns the byte range of the import_header that
// imports fqn, if any. Aliased imports also count.
func (c fileContext) findImportByFQN(fqn string) ([2]int, bool) {
	for _, e := range c.Aliases {
		if e.FQN == fqn {
			return e.Range, true
		}
	}
	for _, e := range c.Imports {
		if e.FQN == fqn {
			return e.Range, true
		}
	}
	return [2]int{}, false
}

// firstImportAnchor returns the byte offset where a freshly-inserted import
// line should go. Prefers the position before the earliest existing
// import; falls back to the end of the package line; else 0.
func (c fileContext) firstImportAnchor() (int, bool) {
	first := -1
	for _, e := range c.Imports {
		if first == -1 || e.Range[0] < first {
			first = e.Range[0]
		}
	}
	for _, e := range c.Wildcards {
		if first == -1 || e.Range[0] < first {
			first = e.Range[0]
		}
	}
	if first >= 0 {
		return first, true
	}
	if c.PackageRange != ([2]int{}) {
		return c.PackageRange[1], false
	}
	return 0, false
}

type parsedImport struct {
	fqn      string
	alias    string
	wildcard bool
	pkg      string
}

func recordImport(file *scanner.File, idx uint32, ctx *fileContext, parse func(string) parsedImport) {
	rng := clampToFirstLine(file, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	tgt := parse(firstSourceLine(file.FlatNodeText(idx)))
	switch {
	case tgt.alias != "" && tgt.fqn != "":
		entry := importInfo{FQN: tgt.fqn, Range: rng}
		ctx.Aliases[tgt.alias] = entry
		ctx.Imports[tgt.fqn] = entry
	case tgt.wildcard && tgt.pkg != "":
		ctx.Wildcards[tgt.pkg] = importInfo{Range: rng}
	case tgt.fqn != "":
		entry := importInfo{FQN: tgt.fqn, Range: rng}
		if name, ok := simpleName(tgt.fqn); ok {
			ctx.Imports[name] = entry
		}
	}
}

func parseKotlinImport(raw string) parsedImport {
	text := trimImportLine(raw)
	if text == "" {
		return parsedImport{}
	}
	if i := strings.Index(text, " as "); i >= 0 {
		return parsedImport{
			fqn:   strings.TrimSpace(text[:i]),
			alias: strings.TrimSpace(text[i+len(" as "):]),
		}
	}
	if strings.HasSuffix(text, ".*") {
		return parsedImport{wildcard: true, pkg: strings.TrimSuffix(text, ".*")}
	}
	return parsedImport{fqn: text}
}

func parseJavaImport(raw string) parsedImport {
	text := trimImportLine(raw)
	text = strings.TrimPrefix(text, "static")
	text = strings.TrimSpace(text)
	if text == "" {
		return parsedImport{}
	}
	if strings.HasSuffix(text, ".*") {
		return parsedImport{wildcard: true, pkg: strings.TrimSuffix(text, ".*")}
	}
	return parsedImport{fqn: text}
}

func trimImportLine(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	return strings.TrimSpace(text)
}

// firstSourceLine returns the first non-empty, non-comment line of raw,
// trimmed. Used to ignore trailing trivia that tree-sitter sometimes
// attaches to header nodes.
func firstSourceLine(raw string) string {
	for s := raw; ; {
		i := strings.IndexByte(s, '\n')
		var line string
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

// firstHeaderLine returns the first non-empty, non-comment line of raw with
// the given keyword stripped.
func firstHeaderLine(raw, keyword string) string {
	line := firstSourceLine(raw)
	line = strings.TrimPrefix(line, keyword)
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, ";")
	return strings.TrimSpace(line)
}

// clampToFirstLine narrows a byte range to its first line so a header
// rewriter doesn't erase trailing comments that tree-sitter sometimes
// attaches to a header node.
func clampToFirstLine(file *scanner.File, start, end int) [2]int {
	if file.Content == nil || start < 0 || end > len(file.Content) || start >= end {
		return [2]int{start, end}
	}
	for i := start; i < end; i++ {
		if file.Content[i] == '\n' {
			return [2]int{start, i}
		}
	}
	return [2]int{start, end}
}

func splitFQN(fqn string) (parent, simple string, ok bool) {
	idx := strings.LastIndex(fqn, ".")
	if idx <= 0 || idx == len(fqn)-1 {
		return "", "", false
	}
	return fqn[:idx], fqn[idx+1:], true
}
