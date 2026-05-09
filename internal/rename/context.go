package rename

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// FileContext captures the package and imports of a single source file so
// references in that file can be resolved to fully qualified names.
type FileContext struct {
	File      string
	Package   string
	Language  scanner.Language
	Imports   map[string]string // simple name -> FQN
	Aliases   map[string]string // alias -> FQN (Kotlin only)
	Wildcards []string          // package names imported with `.*`
}

// BuildFileContext walks file's flat AST to extract package, imports, aliases
// and wildcard imports. Works for both Kotlin and Java files.
func BuildFileContext(file *scanner.File) FileContext {
	ctx := FileContext{
		Imports: make(map[string]string),
		Aliases: make(map[string]string),
	}
	if file == nil {
		return ctx
	}
	ctx.File = file.Path
	ctx.Language = file.Language

	if file.FlatTree == nil {
		return ctx
	}

	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "package_header", "package_declaration":
			if ctx.Package == "" {
				ctx.Package = firstHeaderLine(file.FlatNodeText(idx), "package")
			}
		case "import_header":
			parseKotlinImport(firstSourceLine(file.FlatNodeText(idx)), &ctx)
		case "import_declaration":
			parseJavaImport(firstSourceLine(file.FlatNodeText(idx)), &ctx)
		}
	})

	return ctx
}

// firstSourceLine returns the first non-empty, non-comment line of raw,
// trimmed. Used to ignore trailing trivia that tree-sitter sometimes
// attaches to header nodes.
func firstSourceLine(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}
		return line
	}
	return ""
}

// firstHeaderLine returns the first non-empty, non-comment line of raw with
// the given keyword stripped. Tree-sitter Kotlin/Java sometimes attach
// trailing comments and whitespace to header nodes, so a naïve TrimSpace
// of FlatNodeText would include them.
func firstHeaderLine(raw, keyword string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}
		line = strings.TrimPrefix(line, keyword)
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, ";")
		return strings.TrimSpace(line)
	}
	return ""
}

func parseKotlinImport(raw string, ctx *FileContext) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if i := strings.Index(text, " as "); i >= 0 {
		fqn := strings.TrimSpace(text[:i])
		alias := strings.TrimSpace(text[i+len(" as "):])
		if fqn != "" && alias != "" {
			ctx.Aliases[alias] = fqn
		}
		return
	}
	if strings.HasSuffix(text, ".*") {
		pkg := strings.TrimSuffix(text, ".*")
		if pkg != "" {
			ctx.Wildcards = append(ctx.Wildcards, pkg)
		}
		return
	}
	if name, ok := simpleName(text); ok {
		ctx.Imports[name] = text
	}
}

func parseJavaImport(raw string, ctx *FileContext) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "static")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if strings.HasSuffix(text, ".*") {
		pkg := strings.TrimSuffix(text, ".*")
		if pkg != "" {
			ctx.Wildcards = append(ctx.Wildcards, pkg)
		}
		return
	}
	if name, ok := simpleName(text); ok {
		ctx.Imports[name] = text
	}
}

// ResolveSimpleName returns the FQN that `name` refers to in this file, or
// the empty string if it cannot be resolved deterministically. The
// resolution order is: alias > explicit import > same-package > single
// wildcard match.
func (c FileContext) ResolveSimpleName(name string) string {
	if name == "" {
		return ""
	}
	if fqn, ok := c.Aliases[name]; ok {
		return fqn
	}
	if fqn, ok := c.Imports[name]; ok {
		return fqn
	}
	return ""
}

// MatchesFQN reports whether a reference of the given simple `name` in this
// file resolves to fqn. Same-package references and wildcard imports are
// considered alongside explicit imports/aliases. A wildcard match counts
// only when no explicit import for `name` exists in this file.
func (c FileContext) MatchesFQN(name, fqn string) bool {
	if name == "" || fqn == "" {
		return false
	}
	if resolved := c.ResolveSimpleName(name); resolved != "" {
		return resolved == fqn
	}
	parent, simple, ok := splitFQN(fqn)
	if !ok || simple != name {
		return false
	}
	if c.Package != "" && c.Package == parent {
		return true
	}
	for _, wp := range c.Wildcards {
		if wp == parent {
			return true
		}
	}
	return false
}

// HeaderRanges holds byte ranges for package and import statement nodes in
// a single file. Used by Apply to rewrite or replace those statements
// when a rename moves a symbol across packages.
type HeaderRanges struct {
	Package        [2]int            // byte range of package_header / package_declaration; zero means no package
	Imports        map[string][2]int // FQN -> import_header range
	Aliases        map[string][2]int // alias -> import_header range
	Wildcards      map[string][2]int // package -> wildcard import range
	FirstImportPos int               // byte offset where a new import would be inserted; 0 if no anchor
}

// CollectHeaderRanges walks file's flat AST to extract byte ranges for the
// package declaration and every import statement. Works for both Kotlin
// and Java files.
func CollectHeaderRanges(file *scanner.File) HeaderRanges {
	hr := HeaderRanges{
		Imports:   make(map[string][2]int),
		Aliases:   make(map[string][2]int),
		Wildcards: make(map[string][2]int),
	}
	if file == nil || file.FlatTree == nil {
		return hr
	}
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "package_header", "package_declaration":
			hr.Package = clampToFirstLine(file, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
		case "import_header":
			recordImportRange(file, idx, &hr, parseKotlinImportTarget)
		case "import_declaration":
			recordImportRange(file, idx, &hr, parseJavaImportTarget)
		}
	})
	hr.FirstImportPos = computeFirstImportPos(file, hr)
	return hr
}

type importTarget struct {
	fqn      string
	alias    string
	wildcard bool
	pkg      string
}

func recordImportRange(file *scanner.File, idx uint32, hr *HeaderRanges, parse func(string) importTarget) {
	rng := clampToFirstLine(file, int(file.FlatStartByte(idx)), int(file.FlatEndByte(idx)))
	tgt := parse(firstSourceLine(file.FlatNodeText(idx)))
	switch {
	case tgt.alias != "" && tgt.fqn != "":
		hr.Aliases[tgt.alias] = rng
		hr.Imports[tgt.fqn] = rng
	case tgt.wildcard && tgt.pkg != "":
		hr.Wildcards[tgt.pkg] = rng
	case tgt.fqn != "":
		hr.Imports[tgt.fqn] = rng
	}
}

func parseKotlinImportTarget(raw string) importTarget {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)
	if text == "" {
		return importTarget{}
	}
	if i := strings.Index(text, " as "); i >= 0 {
		fqn := strings.TrimSpace(text[:i])
		alias := strings.TrimSpace(text[i+len(" as "):])
		return importTarget{fqn: fqn, alias: alias}
	}
	if strings.HasSuffix(text, ".*") {
		return importTarget{wildcard: true, pkg: strings.TrimSuffix(text, ".*")}
	}
	return importTarget{fqn: text}
}

func parseJavaImportTarget(raw string) importTarget {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "import")
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "static")
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ";")
	text = strings.TrimSpace(text)
	if text == "" {
		return importTarget{}
	}
	if strings.HasSuffix(text, ".*") {
		return importTarget{wildcard: true, pkg: strings.TrimSuffix(text, ".*")}
	}
	return importTarget{fqn: text}
}

// computeFirstImportPos returns the byte offset where a freshly-inserted
// import line should go. Prefers the byte immediately before the first
// existing import; otherwise the byte after the package declaration; else 0.
func computeFirstImportPos(_ *scanner.File, hr HeaderRanges) int {
	first := -1
	consider := func(rng [2]int) {
		if rng[1] == 0 && rng[0] == 0 {
			return
		}
		if first == -1 || rng[0] < first {
			first = rng[0]
		}
	}
	for _, r := range hr.Imports {
		consider(r)
	}
	for _, r := range hr.Wildcards {
		consider(r)
	}
	if first >= 0 {
		return first
	}
	if hr.Package != [2]int{} {
		return hr.Package[1]
	}
	return 0
}

// clampToFirstLine narrows a byte range to its first line. If the range
// starts at the beginning of a header but extends into trailing
// comments/whitespace (a tree-sitter quirk), the rewriter would otherwise
// erase those when replacing the line.
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
