package rules

// A per-pass view of the cross-file index for the slack-lints ports that walk project class hierarchies
// (MustUseNamedParams, RestrictCallsTo, JavaOnlyDetector, DoNotCallProviders, BindsTypeMismatch). It memoizes the
// path → file map, each class's direct supertypes and the project's packages, so a pass costs one lookup per class
// instead of a scan of every file per lookup.

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type slackHier struct {
	index    *scanner.CodeIndex
	byPath   map[string]*scanner.File
	supers   map[string][]string
	complete map[string]bool // whether every direct supertype of a class resolved (see slackDirectSupertypesOf)
	packages map[string]bool
}

func newSlackHier(index *scanner.CodeIndex) *slackHier {
	h := &slackHier{index: index, byPath: map[string]*scanner.File{}, supers: map[string][]string{},
		complete: map[string]bool{}, packages: map[string]bool{}}
	for _, f := range index.Files {
		if f == nil {
			continue
		}
		h.byPath[f.Path] = f
		if pkg := moshiFilePackage(f); pkg != "" {
			h.packages[pkg] = true
		}
	}
	return h
}

// declOf returns the file and declaration node of an indexed symbol.
func (h *slackHier) declOf(sym scanner.Symbol) (*scanner.File, uint32, bool) {
	file := h.byPath[sym.File]
	if file == nil || file.FlatTree == nil {
		return nil, 0, false
	}
	decl, ok := file.FlatNamedDescendantForByteRange(uint32(sym.StartByte), uint32(sym.EndByte))
	return file, decl, ok
}

// classDecl returns the declaration of the indexed class fqn.
func (h *slackHier) classDecl(fqn string) (*scanner.File, uint32, bool) {
	sym, ok := h.index.SymbolByFQN(fqn)
	if !ok {
		return nil, 0, false
	}
	return h.declOf(sym)
}

// isProjectPackage reports whether some project file is in pkg.
func (h *slackHier) isProjectPackage(pkg string) bool { return h.packages[pkg] }

// candidateFiles returns the files whose text contains one of names as an identifier. Each file is scanned once,
// so the cost is linear in the project's size however many names there are.
func (h *slackHier) candidateFiles(names []string) []*scanner.File {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	var out []*scanner.File
	for _, f := range h.index.Files {
		if f != nil && slackMentionsIdentifier(f.Content, set) {
			out = append(out, f)
		}
	}
	return out
}

// slackMentionsIdentifier reports whether content contains one of names as a whole identifier.
func slackMentionsIdentifier(content []byte, names map[string]bool) bool {
	isIdent := func(b byte) bool {
		return b == '_' || b == '$' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b >= 0x80
	}
	for i := 0; i < len(content); {
		if !isIdent(content[i]) {
			i++
			continue
		}
		j := i
		for j < len(content) && isIdent(content[j]) {
			j++
		}
		if names[string(content[i:j])] {
			return true
		}
		i = j
	}
	return false
}

// slackEnclosingClassFQNs returns the FQNs of the classes enclosing decl (not decl itself), innermost first.
func slackEnclosingClassFQNs(file *scanner.File, decl uint32) []string {
	pkg := moshiFilePackage(file)
	var out []string
	for p, ok := file.FlatParent(decl); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_declaration", "object_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			owner, _ := slackDeclOwner(file, p, pkg)
			name := extractIdentifierFlat(file, p)
			if file.Language == scanner.LangJava {
				if id, ok := file.FlatFindChild(p, "identifier"); ok {
					name = file.FlatNodeText(id)
				}
			}
			switch {
			case owner != "":
				out = append(out, owner+"."+name)
			case pkg != "":
				out = append(out, pkg+"."+name)
			default:
				out = append(out, name)
			}
		}
	}
	return out
}

// slackTypeNameKind classifies a type name written at decl in file: the indexed project class it names (resolved
// in the enclosing classes first, then the file's imports and package), "" with library true for a name the source
// shows is a library type (a built-in, or imported from a package no project file is in), or "" with library false
// when it cannot be told (an unindexed name of a project package, such as a typealias or generated class).
func (h *slackHier) slackTypeNameKind(file *scanner.File, decl uint32, text string) (fqn string, library bool) {
	text = strings.Join(strings.Fields(text), "")
	if i := strings.IndexByte(text, '<'); i >= 0 {
		text = text[:i]
	}
	text = strings.TrimSuffix(text, "?")
	if text == "" {
		return "", false
	}
	for _, encl := range slackEnclosingClassFQNs(file, decl) {
		if _, ok := h.index.SymbolByFQN(encl + "." + text); ok {
			return encl + "." + text, false
		}
	}
	if fqn := slackClassFQN(h.index, file, text); fqn != "" {
		return fqn, false
	}
	head, _, _ := strings.Cut(text, ".")
	if full, ok := lintImportsOf(file).bySimple[head]; ok {
		pkg := full[:max(strings.LastIndexByte(full, '.'), 0)]
		return "", !h.isProjectPackage(pkg)
	}
	if strings.Contains(text, ".") && text[0] >= 'a' && text[0] <= 'z' {
		pkg := text[:strings.LastIndexByte(text, '.')]
		return "", !h.isProjectPackage(pkg)
	}
	return "", daggerLibraryTypes[text]
}
