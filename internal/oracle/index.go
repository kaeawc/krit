package oracle

import (
	"slices"
	"strings"
	"sync"
)

// DeclLocation describes where a declaration was defined.
//
// Line and Column are 1-based when known. The current oracle JSON does not
// emit per-declaration source positions, so Line and Column are zero until
// krit-types starts emitting them; File and Signature are always populated.
type DeclLocation struct {
	FQN       string
	Kind      string
	File      string
	JARPath   string
	Line      int
	Column    int
	Signature string
}

// ReferenceLocation is a single use of a symbol.
type ReferenceLocation struct {
	FQN           string
	File          string
	Line          int
	Column        int
	IsDeclaration bool
}

// TypeInfo is a parsed expression type, with generic arguments recovered from
// the oracle's textual type representation.
type TypeInfo struct {
	FQN       string
	Nullable  bool
	Arguments []TypeInfo
}

// Index provides O(1) reverse lookups from FQN to declaration and references,
// plus O(1) expression-id to type lookups, over an assembled Data.
//
// Index is safe for concurrent reads alongside ApplyFileUpdate / RemoveFile
// mutations from a single writer (typically the LSP didChange handler).
type Index struct {
	mu          sync.RWMutex
	decls       map[string]*DeclLocation
	refs        map[string][]ReferenceLocation
	exprs       map[string]TypeInfo
	simpleNames map[string][]*DeclLocation

	// fileFQNs[path] holds every FQN that has either a declaration or a
	// reference originating from path. Lets ApplyFileUpdate evict file
	// entries in time linear in the file's contribution rather than the
	// total index size.
	fileFQNs     map[string][]string
	fileExprKeys map[string][]string
}

// BuildIndex constructs the reverse lookup from an assembled Data. One
// linear pass over declarations and one pass over every file's expressions.
func BuildIndex(raw *Data) *Index {
	if raw == nil {
		return newEmptyIndex()
	}

	declCap := len(raw.Dependencies)
	exprCap := 0
	for _, file := range raw.Files {
		if file == nil {
			continue
		}
		declCap += len(file.Declarations) * 4
		exprCap += len(file.Expressions)
	}
	idx := &Index{
		decls:        make(map[string]*DeclLocation, declCap),
		refs:         make(map[string][]ReferenceLocation, declCap),
		exprs:        make(map[string]TypeInfo, exprCap),
		simpleNames:  make(map[string][]*DeclLocation, declCap),
		fileFQNs:     make(map[string][]string),
		fileExprKeys: make(map[string][]string),
	}

	for path, file := range raw.Files {
		if file == nil {
			continue
		}
		idx.ingestFile(path, file)
	}

	for fqn, cls := range raw.Dependencies {
		if cls == nil || fqn == "" {
			continue
		}
		if _, exists := idx.decls[fqn]; exists {
			continue
		}
		decl := &DeclLocation{
			FQN:       fqn,
			Kind:      cls.Kind,
			JARPath:   cls.JARPath,
			Signature: renderClassSignature(fqn, cls),
		}
		idx.decls[fqn] = decl
		idx.indexSimpleName(decl)
	}

	return idx
}

func newEmptyIndex() *Index {
	return &Index{
		decls:        map[string]*DeclLocation{},
		refs:         map[string][]ReferenceLocation{},
		exprs:        map[string]TypeInfo{},
		simpleNames:  map[string][]*DeclLocation{},
		fileFQNs:     map[string][]string{},
		fileExprKeys: map[string][]string{},
	}
}

func (idx *Index) indexSimpleName(decl *DeclLocation) {
	name := simpleName(decl.FQN)
	if name == "" {
		return
	}
	idx.simpleNames[name] = append(idx.simpleNames[name], decl)
}

func (idx *Index) unindexSimpleName(decl *DeclLocation) {
	name := simpleName(decl.FQN)
	if name == "" {
		return
	}
	bucket := idx.simpleNames[name]
	if len(bucket) == 0 {
		return
	}
	filtered := slices.DeleteFunc(bucket, func(d *DeclLocation) bool {
		return d == decl
	})
	if len(filtered) == 0 {
		delete(idx.simpleNames, name)
	} else {
		idx.simpleNames[name] = filtered
	}
}

func simpleName(fqn string) string {
	if i := strings.LastIndexByte(fqn, '.'); i >= 0 {
		return fqn[i+1:]
	}
	return fqn
}

// ingestFile inserts every declaration, reference, and expression entry from
// file into idx. Caller must hold the write lock when invoked from a mutation
// path; safe to call lock-free during BuildIndex.
func (idx *Index) ingestFile(path string, file *File) {
	if file == nil {
		return
	}
	seen := map[string]struct{}{}
	track := func(fqn string) {
		if _, ok := seen[fqn]; ok {
			return
		}
		seen[fqn] = struct{}{}
		idx.fileFQNs[path] = append(idx.fileFQNs[path], fqn)
	}

	for _, cls := range file.Declarations {
		if cls == nil || cls.FQN == "" {
			continue
		}
		clsDecl := &DeclLocation{
			FQN:       cls.FQN,
			Kind:      cls.Kind,
			File:      path,
			Line:      cls.Line,
			Column:    cls.Column,
			Signature: renderClassSignature(cls.FQN, cls),
		}
		idx.decls[cls.FQN] = clsDecl
		idx.indexSimpleName(clsDecl)
		idx.refs[cls.FQN] = append(idx.refs[cls.FQN], ReferenceLocation{
			FQN:           cls.FQN,
			File:          path,
			IsDeclaration: true,
		})
		track(cls.FQN)
		for _, m := range cls.Members {
			if m == nil || m.Name == "" {
				continue
			}
			memberFQN := cls.FQN + "." + m.Name
			memberDecl := &DeclLocation{
				FQN:       memberFQN,
				Kind:      m.Kind,
				File:      path,
				Line:      m.Line,
				Column:    m.Column,
				Signature: renderMemberSignature(m),
			}
			idx.decls[memberFQN] = memberDecl
			idx.indexSimpleName(memberDecl)
			idx.refs[memberFQN] = append(idx.refs[memberFQN], ReferenceLocation{
				FQN:           memberFQN,
				File:          path,
				IsDeclaration: true,
			})
			track(memberFQN)
		}
	}
	for pos, et := range file.Expressions {
		if et == nil {
			continue
		}
		key, ok := parseLineCol(pos)
		if !ok {
			continue
		}
		exprKey := path + ":" + pos
		idx.exprs[exprKey] = parseTypeInfo(et.Type, et.Nullable)
		idx.fileExprKeys[path] = append(idx.fileExprKeys[path], exprKey)
		if et.CallTarget != "" {
			idx.refs[et.CallTarget] = append(idx.refs[et.CallTarget], ReferenceLocation{
				FQN:    et.CallTarget,
				File:   path,
				Line:   int(key >> 32),
				Column: int(uint32(key)),
			})
			track(et.CallTarget)
		}
	}
}

// RemoveFile evicts every decl, ref, and expression entry that originated from
// path. Linear in the number of entries removed, not in total index size.
// Safe to call concurrently with reads.
func (idx *Index) RemoveFile(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeFileLocked(path)
}

func (idx *Index) removeFileLocked(path string) {
	for _, fqn := range idx.fileFQNs[path] {
		if d, ok := idx.decls[fqn]; ok && d.File == path {
			idx.unindexSimpleName(d)
			delete(idx.decls, fqn)
		}
		refs := idx.refs[fqn]
		if len(refs) == 0 {
			continue
		}
		// In-place filter is safe because FindReferencesByFQN returns a copy.
		filtered := slices.DeleteFunc(refs, func(r ReferenceLocation) bool {
			return r.File == path
		})
		if len(filtered) == 0 {
			delete(idx.refs, fqn)
		} else {
			idx.refs[fqn] = filtered
		}
	}
	delete(idx.fileFQNs, path)

	for _, k := range idx.fileExprKeys[path] {
		delete(idx.exprs, k)
	}
	delete(idx.fileExprKeys, path)
}

// ApplyFileUpdate replaces every entry sourced from path with entries derived
// from file. Passing a nil file is equivalent to RemoveFile(path).
// Safe to call concurrently with reads.
func (idx *Index) ApplyFileUpdate(path string, file *File) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeFileLocked(path)
	if file == nil {
		return
	}
	idx.ingestFile(path, file)
}

// FindDeclarationByFQN returns the declaration record for fqn.
func (idx *Index) FindDeclarationByFQN(fqn string) (*DeclLocation, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	d, ok := idx.decls[fqn]
	return d, ok
}

// FindDeclarationBySimpleName returns every declaration whose FQN ends in
// `.name` (or whose FQN equals name for top-level declarations without a
// package).
func (idx *Index) FindDeclarationBySimpleName(name string) []*DeclLocation {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	bucket := idx.simpleNames[name]
	if len(bucket) == 0 {
		return nil
	}
	out := make([]*DeclLocation, len(bucket))
	copy(out, bucket)
	return out
}

// FindReferencesByFQN returns every recorded use of fqn, including the
// declaration site itself. The returned slice is a copy; the caller may
// retain it across subsequent index mutations.
func (idx *Index) FindReferencesByFQN(fqn string) []ReferenceLocation {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	refs := idx.refs[fqn]
	if len(refs) == 0 {
		return nil
	}
	out := make([]ReferenceLocation, len(refs))
	copy(out, refs)
	return out
}

// TypeAtExpression returns the resolved type for an expression. The exprId is
// "filePath:line:col" using the oracle's 1-based line and column.
func (idx *Index) TypeAtExpression(exprID string) (TypeInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	t, ok := idx.exprs[exprID]
	return t, ok
}

// Index returns the FQN reverse index. Built once during Load.
func (o *Oracle) Index() *Index {
	return o.index
}

func parseTypeInfo(raw string, nullable bool) TypeInfo {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, "?") {
		nullable = true
		raw = strings.TrimSuffix(raw, "?")
	}
	lt := strings.IndexByte(raw, '<')
	if lt < 0 || !strings.HasSuffix(raw, ">") {
		return TypeInfo{FQN: raw, Nullable: nullable}
	}
	out := TypeInfo{FQN: raw[:lt], Nullable: nullable}
	for _, arg := range splitTopLevel(raw[lt+1 : len(raw)-1]) {
		out.Arguments = append(out.Arguments, parseTypeInfo(arg, false))
	}
	return out
}

func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

func renderClassSignature(fqn string, cls *Class) string {
	kind := cls.Kind
	if kind == "" {
		kind = "class"
	}
	return kind + " " + fqn
}

func renderMemberSignature(m *Member) string {
	switch m.Kind {
	case "function":
		var b strings.Builder
		b.WriteString("fun ")
		b.WriteString(m.Name)
		b.WriteByte('(')
		for i, p := range m.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			if p == nil {
				continue
			}
			b.WriteString(p.Name)
			b.WriteString(": ")
			b.WriteString(p.Type)
			if p.Nullable {
				b.WriteByte('?')
			}
		}
		b.WriteByte(')')
		if m.ReturnType != "" {
			b.WriteString(": ")
			b.WriteString(m.ReturnType)
			if m.Nullable {
				b.WriteByte('?')
			}
		}
		return b.String()
	case "property":
		s := "val " + m.Name
		if m.ReturnType != "" {
			s += ": " + m.ReturnType
			if m.Nullable {
				s += "?"
			}
		}
		return s
	default:
		return m.Kind + " " + m.Name
	}
}
