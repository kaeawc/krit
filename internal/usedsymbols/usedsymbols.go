// Package usedsymbols extracts the set of external (out-of-module)
// fully-qualified symbols a Kotlin compilation unit references.
//
// Build tools key compile actions on hash(used-symbols ∩ upstream-ABI)
// instead of hash(upstream-ABI), avoiding rebuilds when an upstream change
// touches a symbol the consumer never references.
//
// The extractor is import-table driven: it walks `import_header`,
// `type_identifier`, `user_type`, `navigation_expression`,
// `delegation_specifier`, and `annotation` nodes, resolves simple names
// against the file's import table, and records the resulting FQNs.
// Same-package and same-module references are filtered out by the caller.
package usedsymbols

import (
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Symbol is one external reference. Kind is one of "class", "function",
// "annotation", "import"; arity is set for callables when known.
type Symbol struct {
	FQN   string `json:"fqn"`
	Kind  string `json:"kind"`
	Arity int    `json:"arity,omitempty"`
}

// FileResult is the per-file extraction output.
type FileResult struct {
	File    string   `json:"file"`
	Symbols []Symbol `json:"symbols"`
}

type importTable struct {
	Explicit map[string]string // simple name → FQN
	Aliases  map[string]string // alias → FQN
	Wildcard []string          // package prefixes
}

func newImportTable() *importTable {
	return &importTable{
		Explicit: make(map[string]string),
		Aliases:  make(map[string]string),
	}
}

// resolve returns the FQN for a simple name, "" if unknown.
func (it *importTable) resolve(name string) string {
	if fqn, ok := it.Explicit[name]; ok {
		return fqn
	}
	if fqn, ok := it.Aliases[name]; ok {
		return fqn
	}
	return ""
}

// Package returns the file's declared package, or "" if none is declared.
func Package(file *scanner.File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	pkg, _ := scanHeaders(file)
	return pkg
}

// Extract returns the deduplicated, sorted set of external symbols
// referenced by file. The pkg/module filter is applied by the caller.
func Extract(file *scanner.File) []Symbol {
	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		return nil
	}
	_, it := scanHeaders(file)
	seen := make(map[string]Symbol)

	add := func(fqn, kind string, arity int) {
		if fqn == "" {
			return
		}
		key := kind + "|" + fqn
		if existing, ok := seen[key]; ok {
			if arity > existing.Arity {
				existing.Arity = arity
				seen[key] = existing
			}
			return
		}
		seen[key] = Symbol{FQN: fqn, Kind: kind, Arity: arity}
	}

	for _, fqn := range it.Explicit {
		add(fqn, importKind(fqn), 0)
	}
	for _, fqn := range it.Aliases {
		add(fqn, importKind(fqn), 0)
	}
	walkBody(file, it, add)

	out := make([]Symbol, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FQN != out[j].FQN {
			return out[i].FQN < out[j].FQN
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// importKind heuristically classifies an imported FQN. The last segment
// starting with an uppercase letter is treated as a class; lowercase is a
// top-level function or property.
func importKind(fqn string) string {
	idx := strings.LastIndex(fqn, ".")
	last := fqn
	if idx >= 0 {
		last = fqn[idx+1:]
	}
	if last == "" {
		return "import"
	}
	r := last[0]
	if r >= 'A' && r <= 'Z' {
		return "class"
	}
	return "function"
}

func scanHeaders(file *scanner.File) (string, *importTable) {
	it := newImportTable()
	pkg := ""
	rootChildren := file.FlatNamedChildCount(0)
	for i := 0; i < rootChildren; i++ {
		child := file.FlatNamedChild(0, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "package_header":
			text := strings.TrimSpace(file.FlatNodeText(child))
			text = strings.TrimPrefix(text, "package ")
			pkg = strings.TrimSpace(text)
		case "import_header":
			parseImportHeader(file, child, it)
		case "import_list":
			n := file.FlatNamedChildCount(child)
			for j := 0; j < n; j++ {
				sub := file.FlatNamedChild(child, j)
				if sub == 0 {
					continue
				}
				if file.FlatType(sub) == "import_header" {
					parseImportHeader(file, sub, it)
				}
			}
		}
	}
	return pkg, it
}

func parseImportHeader(file *scanner.File, idx uint32, it *importTable) {
	text := strings.TrimSpace(file.FlatNodeText(idx))
	text = strings.TrimPrefix(text, "import ")
	text = strings.TrimSpace(text)

	if i := strings.Index(text, " as "); i >= 0 {
		fqn := strings.TrimSpace(text[:i])
		alias := strings.TrimSpace(text[i+4:])
		it.Aliases[alias] = fqn
		return
	}
	if strings.HasSuffix(text, ".*") {
		it.Wildcard = append(it.Wildcard, strings.TrimSuffix(text, ".*"))
		return
	}
	parts := strings.Split(text, ".")
	if len(parts) == 0 {
		return
	}
	it.Explicit[parts[len(parts)-1]] = text
}

// walkBody finds Type.Member chains and annotation uses, emitting symbols
// the import table alone wouldn't capture.
func walkBody(file *scanner.File, it *importTable, add func(fqn, kind string, arity int)) {
	tree := file.FlatTree
	for i := uint32(1); i < uint32(len(tree.Nodes)); i++ {
		switch file.FlatType(i) {
		case "navigation_expression":
			handleNavigation(file, i, it, add)
		case "user_type":
			handleUserType(file, i, it, add)
		case "annotation":
			handleAnnotation(file, i, it, add)
		case "call_expression":
			handleCallExpression(file, i, it, add)
		}
	}
}

// handleNavigation emits Class.member for chains rooted at an imported
// type identifier. e.g. UserRepository.Result → com.acme.core.UserRepository.Result.
func handleNavigation(file *scanner.File, idx uint32, it *importTable, add func(fqn, kind string, arity int)) {
	// Collect the chain of identifier text segments left-to-right.
	parts := flattenNavigation(file, idx)
	if len(parts) < 2 {
		return
	}
	root := parts[0]
	rootFQN := it.resolve(root)
	if rootFQN == "" {
		return
	}
	// rootFQN is already added by the import sweep; emit nested members.
	full := rootFQN
	for i := 1; i < len(parts); i++ {
		full = full + "." + parts[i]
		kind := "function"
		if parts[i] != "" {
			c := parts[i][0]
			if c >= 'A' && c <= 'Z' {
				kind = "class"
			}
		}
		add(full, kind, 0)
	}
}

// flattenNavigation walks a navigation_expression and returns its identifier
// segments in source order. Non-identifier receivers (this, calls, literals)
// abort with a nil result.
func flattenNavigation(file *scanner.File, idx uint32) []string {
	var parts []string
	var collect func(n uint32) bool
	collect = func(n uint32) bool {
		switch file.FlatType(n) {
		case "navigation_expression":
			n0 := file.FlatChildCount(n)
			for i := 0; i < n0; i++ {
				c := file.FlatChild(n, i)
				ct := file.FlatType(c)
				if ct == "." || ct == "?." || ct == "::" {
					continue
				}
				if ct == "navigation_suffix" {
					// navigation_suffix wraps "." + identifier
					nn := file.FlatChildCount(c)
					for j := 0; j < nn; j++ {
						gc := file.FlatChild(c, j)
						gct := file.FlatType(gc)
						if gct == "simple_identifier" || gct == "type_identifier" {
							parts = append(parts, file.FlatNodeText(gc))
						}
					}
					continue
				}
				if !collect(c) {
					return false
				}
			}
			return true
		case "simple_identifier", "type_identifier":
			parts = append(parts, file.FlatNodeText(n))
			return true
		default:
			return false
		}
	}
	if !collect(idx) {
		return nil
	}
	return parts
}

func handleUserType(file *scanner.File, idx uint32, it *importTable, add func(fqn, kind string, arity int)) {
	// user_type is one or more type_identifier segments separated by '.'.
	segs := userTypeSegments(file, idx)
	if len(segs) == 0 {
		return
	}
	rootFQN := it.resolve(segs[0])
	if rootFQN == "" {
		return
	}
	full := rootFQN
	add(full, "class", 0)
	for i := 1; i < len(segs); i++ {
		full = full + "." + segs[i]
		add(full, "class", 0)
	}
}

func userTypeSegments(file *scanner.File, idx uint32) []string {
	var segs []string
	n := file.FlatChildCount(idx)
	for i := 0; i < n; i++ {
		c := file.FlatChild(idx, i)
		switch file.FlatType(c) {
		case "type_identifier", "simple_identifier":
			segs = append(segs, file.FlatNodeText(c))
		case "user_type":
			segs = append(segs, userTypeSegments(file, c)...)
		}
	}
	return segs
}

func handleAnnotation(file *scanner.File, idx uint32, it *importTable, add func(fqn, kind string, arity int)) {
	// Find the annotation's user_type or constructor_invocation; resolve its name.
	name := firstAnnotationName(file, idx)
	if name == "" {
		return
	}
	if fqn := it.resolve(name); fqn != "" {
		add(fqn, "annotation", 0)
	}
}

func firstAnnotationName(file *scanner.File, idx uint32) string {
	n := file.FlatChildCount(idx)
	for i := 0; i < n; i++ {
		c := file.FlatChild(idx, i)
		switch file.FlatType(c) {
		case "user_type":
			segs := userTypeSegments(file, c)
			if len(segs) > 0 {
				return segs[0]
			}
		case "constructor_invocation":
			if id, ok := file.FlatFindChild(c, "user_type"); ok {
				segs := userTypeSegments(file, id)
				if len(segs) > 0 {
					return segs[0]
				}
			}
		case "type_identifier", "simple_identifier":
			return file.FlatNodeText(c)
		}
	}
	return ""
}

func handleCallExpression(file *scanner.File, idx uint32, it *importTable, add func(fqn, kind string, arity int)) {
	// Direct call to an imported function: first child is a simple_identifier.
	n := file.FlatChildCount(idx)
	if n == 0 {
		return
	}
	first := file.FlatChild(idx, 0)
	if file.FlatType(first) != "simple_identifier" {
		return
	}
	name := file.FlatNodeText(first)
	fqn := it.resolve(name)
	if fqn == "" {
		return
	}
	arity := callArity(file, idx)
	add(fqn, "function", arity)
}

func callArity(file *scanner.File, callIdx uint32) int {
	if vargs, ok := file.FlatFindChild(callIdx, "value_arguments"); ok {
		count := 0
		n := file.FlatNamedChildCount(vargs)
		for i := 0; i < n; i++ {
			c := file.FlatNamedChild(vargs, i)
			if file.FlatType(c) == "value_argument" {
				count++
			}
		}
		return count
	}
	return 0
}
