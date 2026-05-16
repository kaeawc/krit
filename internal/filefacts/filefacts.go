// Package filefacts is the shared per-run memoization layer for
// per-file derived facts (imports, references, declaration summaries,
// and arbitrary rule-local computations).
//
// One Cache is created per analysis run and attached to v2.Context so
// every rule sees the same memoized facts for a given file. Cache
// methods accept a nil receiver and fall back to recomputing without
// caching, so mini-contexts constructed outside the dispatcher
// (helpers, tests) work unchanged.
package filefacts

import (
	"fmt"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

// slotTypeMismatch panics with the offending slot key when a cache slot
// is reused with a different generic instantiation. The slot is a
// programmer-controlled constant, so a mismatch is a bug at the call
// site, not a runtime data condition. Failing loud here surfaces the
// offending slot instead of the opaque "interface conversion" panic the
// runtime would otherwise produce.
func slotTypeMismatch(slot string, want, got any) {
	panic(fmt.Sprintf("filefacts: slot %q type mismatch: want %T, got %T", slot, want, got))
}

// loadOrStoreTyped is the shared cache-miss/cache-hit body for the
// StringFact/FileFact/NodeFact accessors. Both assertions are
// necessary: the first guards the cache-hit fast path, the second
// guards against a concurrent writer storing a wrong-typed value
// under the same key+slot (the LoadOrStore returns whichever value
// the race produced, which is not guaranteed to be the one we just
// computed).
func loadOrStoreTyped[T any](sm *sync.Map, key any, slot string, compute func() T) T {
	if cached, ok := sm.Load(key); ok {
		typed, tok := cached.(T)
		if !tok {
			var zero T
			slotTypeMismatch(slot, zero, cached)
		}
		return typed
	}
	v := compute()
	actual, _ := sm.LoadOrStore(key, v)
	typed, tok := actual.(T)
	if !tok {
		var zero T
		slotTypeMismatch(slot, zero, actual)
	}
	return typed
}

// Cache memoizes per-file facts for a single analysis run. The zero
// value is unusable; obtain one with NewCache. A nil *Cache is a valid
// receiver and forces every accessor to recompute.
type Cache struct {
	imports      sync.Map // *scanner.File -> *ImportFacts
	references   sync.Map // *scanner.File -> *ReferenceFacts
	declarations sync.Map // declarationKey -> *FunctionDeclFact
	fileSlots    sync.Map // fileSlotKey  -> any
	nodeSlots    sync.Map // nodeSlotKey  -> any
	stringSlots  sync.Map // stringSlotKey -> any
}

type stringSlotKey struct {
	key  string
	slot string
}

// StringFact returns a derived value keyed by an arbitrary string,
// computed on first access. Useful for memoizing facts whose natural
// key is not a *scanner.File or AST index — for example "gradle module
// info for directory X". Slot disambiguates fact types stored under
// the same string key. A nil *Cache or empty key recomputes.
func StringFact[T any](c *Cache, key, slot string, compute func() T) T {
	if c == nil || key == "" {
		return compute()
	}
	return loadOrStoreTyped(&c.stringSlots, stringSlotKey{key: key, slot: slot}, slot, compute)
}

type fileSlotKey struct {
	file *scanner.File
	slot string
}

type nodeSlotKey struct {
	file *scanner.File
	idx  uint32
	slot string
}

// FileFact returns a per-file derived value, computing it on first
// access and caching it for the rest of the run. Slot disambiguates
// concurrently-cached facts of different types on the same file
// (e.g. "complexity", "jumpmetrics"). A nil *Cache recomputes on every
// call. Each (file, slot) pair must always be used with the same T.
func FileFact[T any](c *Cache, file *scanner.File, slot string, compute func() T) T {
	if c == nil || file == nil {
		return compute()
	}
	return loadOrStoreTyped(&c.fileSlots, fileSlotKey{file: file, slot: slot}, slot, compute)
}

// NodeFact returns a per-(file, node) derived value, computing on
// first access. Slot disambiguates types stored on the same node by
// different consumers. A nil *Cache or zero idx recomputes.
func NodeFact[T any](c *Cache, file *scanner.File, idx uint32, slot string, compute func() T) T {
	if c == nil || file == nil || idx == 0 {
		return compute()
	}
	return loadOrStoreTyped(&c.nodeSlots, nodeSlotKey{file: file, idx: idx, slot: slot}, slot, compute)
}

// NewCache returns a fresh, empty Cache. Use one Cache per analysis
// run; lifetimes are bounded by the run, so no eviction is needed.
func NewCache() *Cache { return &Cache{} }

// ImportFacts summarizes the import set of a single Kotlin or Java file.
// Construction strips the `import` prefix, trailing `;`, and any `as`
// alias; alias-stripped paths land in FQNs and Wildcards.
type ImportFacts struct {
	// FQNs is the alias-stripped set of fully-qualified import paths in
	// the file. Wildcard imports (`foo.bar.*`) appear in Wildcards
	// instead.
	FQNs map[string]struct{}
	// Wildcards holds the wildcard-import bases observed in the file.
	// Each entry includes the trailing `.*` for direct comparison.
	Wildcards map[string]struct{}
	// Aliases maps the simple name (or explicit `as`-alias) of an
	// imported symbol to its alias-stripped FQN. Useful for resolving
	// receiver expressions to their import.
	Aliases map[string]string
}

// HasFQN reports whether the file imports exactly fqn or a wildcard
// covering it (`<package-of-fqn>.*`).
func (f *ImportFacts) HasFQN(fqn string) bool {
	if f == nil || fqn == "" {
		return false
	}
	if _, ok := f.FQNs[fqn]; ok {
		return true
	}
	if dot := strings.LastIndex(fqn, "."); dot > 0 {
		if _, ok := f.Wildcards[fqn[:dot]+".*"]; ok {
			return true
		}
	}
	return false
}

// HasAnyPrefix reports whether any imported FQN starts with one of the
// supplied prefixes.
func (f *ImportFacts) HasAnyPrefix(prefixes ...string) bool {
	if f == nil {
		return false
	}
	for fqn := range f.FQNs {
		for _, p := range prefixes {
			if strings.HasPrefix(fqn, p) {
				return true
			}
		}
	}
	for w := range f.Wildcards {
		for _, p := range prefixes {
			if strings.HasPrefix(w, p) {
				return true
			}
		}
	}
	return false
}

// Imports returns the import facts for file, computing and caching them
// on first access. A nil *Cache recomputes without caching.
func (c *Cache) Imports(file *scanner.File) *ImportFacts {
	if file == nil {
		return emptyImportFacts()
	}
	if c == nil {
		return computeImportFacts(file)
	}
	if cached, ok := c.imports.Load(file); ok {
		return cached.(*ImportFacts)
	}
	facts := computeImportFacts(file)
	if existing, loaded := c.imports.LoadOrStore(file, facts); loaded {
		return existing.(*ImportFacts)
	}
	return facts
}

func emptyImportFacts() *ImportFacts {
	return &ImportFacts{
		FQNs:      map[string]struct{}{},
		Wildcards: map[string]struct{}{},
		Aliases:   map[string]string{},
	}
}

func computeImportFacts(file *scanner.File) *ImportFacts {
	facts := emptyImportFacts()
	if file == nil {
		return facts
	}
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		raw := strings.TrimSpace(file.FlatNodeText(node))
		if nl := strings.IndexAny(raw, "\r\n"); nl >= 0 {
			raw = strings.TrimSpace(raw[:nl])
		}
		raw = strings.TrimPrefix(raw, "import ")
		raw = strings.TrimSuffix(raw, ";")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		path := raw
		alias := ""
		if idx := strings.Index(raw, " as "); idx >= 0 {
			path = strings.TrimSpace(raw[:idx])
			alias = strings.TrimSpace(raw[idx+4:])
		}
		if strings.HasSuffix(path, ".*") {
			facts.Wildcards[path] = struct{}{}
			return
		}
		facts.FQNs[path] = struct{}{}
		key := alias
		if key == "" {
			if dot := strings.LastIndex(path, "."); dot >= 0 {
				key = path[dot+1:]
			} else {
				key = path
			}
		}
		if key != "" {
			facts.Aliases[key] = path
		}
	})
	return facts
}

// ReferenceFacts is a per-file index of identifier references useful
// for unused-symbol and dead-code analysis. It excludes references that
// occur inside import_header or package_header nodes.
type ReferenceFacts struct {
	// ByName indexes ranges where each identifier name is referenced.
	ByName map[string][]ReferenceRange
}

// ReferenceRange is a half-open byte range in the source file.
type ReferenceRange struct {
	Start uint32
	End   uint32
}

// NameReferenced reports whether the file contains at least one
// reference to name outside of import/package headers.
func (f *ReferenceFacts) NameReferenced(name string) bool {
	if f == nil || name == "" {
		return false
	}
	_, ok := f.ByName[name]
	return ok
}

// References returns the cached reference index for file, computing it
// on first access. The supplied collect callback is invoked once per
// node and should return the reference name (or "" to skip). A nil
// *Cache recomputes without caching.
//
// The callback shape lets the caller drive node-type filtering and
// name extraction without forcing filefacts to import semantics.
func (c *Cache) References(file *scanner.File, collect func(file *scanner.File, idx uint32) string) *ReferenceFacts {
	if file == nil || collect == nil {
		return &ReferenceFacts{ByName: map[string][]ReferenceRange{}}
	}
	if c == nil {
		return computeReferenceFacts(file, collect)
	}
	if cached, ok := c.references.Load(file); ok {
		return cached.(*ReferenceFacts)
	}
	facts := computeReferenceFacts(file, collect)
	if existing, loaded := c.references.LoadOrStore(file, facts); loaded {
		return existing.(*ReferenceFacts)
	}
	return facts
}

func computeReferenceFacts(file *scanner.File, collect func(file *scanner.File, idx uint32) string) *ReferenceFacts {
	facts := &ReferenceFacts{ByName: map[string][]ReferenceRange{}}
	file.FlatWalkAllNodes(0, func(n uint32) {
		name := collect(file, n)
		if name == "" {
			return
		}
		if hasAncestorInHeader(file, n) {
			return
		}
		facts.ByName[name] = append(facts.ByName[name], ReferenceRange{
			Start: file.FlatStartByte(n),
			End:   file.FlatEndByte(n),
		})
	})
	return facts
}

func hasAncestorInHeader(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "import_header", "package_header":
			return true
		}
	}
	return false
}

// FunctionDeclFact is the canonical summary of a single function or
// method declaration. Rules read these instead of re-scanning the
// declaration's modifiers and parameters.
type FunctionDeclFact struct {
	Name        string
	HasOverride bool
	HasOpen     bool
	HasAbstract bool
	HasOperator bool
	Body        uint32
	ParamsNode  uint32
	// Annotations records the simple-names of annotations attached to
	// the declaration. Use AnnotationPresent for queries.
	Annotations map[string]struct{}
}

// AnnotationPresent reports whether the declaration is annotated with
// the given simple-name.
func (f *FunctionDeclFact) AnnotationPresent(name string) bool {
	if f == nil {
		return false
	}
	_, ok := f.Annotations[name]
	return ok
}

type declarationKey struct {
	file *scanner.File
	idx  uint32
}

// FunctionDecl returns the cached function-declaration summary for the
// node at idx. The compute callback is invoked on miss. A nil *Cache
// recomputes on every call.
func (c *Cache) FunctionDecl(file *scanner.File, idx uint32, compute func() *FunctionDeclFact) *FunctionDeclFact {
	if file == nil || idx == 0 {
		return nil
	}
	if c == nil {
		return compute()
	}
	key := declarationKey{file: file, idx: idx}
	if cached, ok := c.declarations.Load(key); ok {
		return cached.(*FunctionDeclFact)
	}
	fact := compute()
	if existing, loaded := c.declarations.LoadOrStore(key, fact); loaded {
		return existing.(*FunctionDeclFact)
	}
	return fact
}
