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
		case "package_header":
			text := strings.TrimSpace(file.FlatNodeText(idx))
			text = strings.TrimPrefix(text, "package")
			text = strings.TrimSpace(text)
			ctx.Package = strings.TrimSuffix(text, ";")
		case "package_declaration":
			text := strings.TrimSpace(file.FlatNodeText(idx))
			text = strings.TrimPrefix(text, "package")
			text = strings.TrimSpace(text)
			ctx.Package = strings.TrimSuffix(text, ";")
		case "import_header":
			parseKotlinImport(file.FlatNodeText(idx), &ctx)
		case "import_declaration":
			parseJavaImport(file.FlatNodeText(idx), &ctx)
		}
	})

	return ctx
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

func splitFQN(fqn string) (parent, simple string, ok bool) {
	idx := strings.LastIndex(fqn, ".")
	if idx <= 0 || idx == len(fqn)-1 {
		return "", "", false
	}
	return fqn[:idx], fqn[idx+1:], true
}
