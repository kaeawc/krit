package oracle

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CompositeResolver wraps a Lookup oracle with a fallback TypeResolver.
// Oracle provides dependency types; fallback provides source-level inference.
type CompositeResolver struct {
	oracle   Lookup
	fallback typeinfer.TypeResolver
	// moduleOf maps a source file to its module key; see SetModuleOf.
	moduleOf atomic.Pointer[func(path string) string]
	// moduleLog receives one verbose line per smart-cast verdict; see SetModuleLog.
	moduleLog    atomic.Pointer[func(format string, args ...any)]
	moduleLogged sync.Map
}

// parallelIndexer is an optional capability on TypeResolver implementations
// that support concurrent file indexing.
type parallelIndexer interface {
	IndexFilesParallel([]*scanner.File, int)
}

// trackedParallelIndexer is an optional capability for resolvers that also
// report progress through a perf.Tracker.
type trackedParallelIndexer interface {
	IndexFilesParallelWithTracker([]*scanner.File, int, perf.Tracker)
}

type flatExpressionLookup interface {
	LookupExpressionFlat(file *scanner.File, idx uint32) *typeinfer.ResolvedType
}

// NewCompositeResolver creates a resolver that checks the oracle first for
// dependency types and falls back to the tree-sitter resolver for source types.
func NewCompositeResolver(oracle Lookup, fallback typeinfer.TypeResolver) *CompositeResolver {
	return &CompositeResolver{oracle: oracle, fallback: fallback}
}

// Compile-time check that CompositeResolver implements TypeResolver.
var _ typeinfer.TypeResolver = (*CompositeResolver)(nil)

// IndexFilesParallel delegates to the fallback resolver's indexing.
func (c *CompositeResolver) IndexFilesParallel(files []*scanner.File, workers int) {
	if ix, ok := c.fallback.(parallelIndexer); ok {
		ix.IndexFilesParallel(files, workers)
	}
}

func (c *CompositeResolver) IndexFilesParallelWithTracker(files []*scanner.File, workers int, tracker perf.Tracker) {
	if ix, ok := c.fallback.(trackedParallelIndexer); ok {
		ix.IndexFilesParallelWithTracker(files, workers, tracker)
		return
	}
	c.IndexFilesParallel(files, workers)
}

func (c *CompositeResolver) ResolveFlatNode(idx uint32, file *scanner.File) *typeinfer.ResolvedType {
	// The line:col fact holds the same smart-cast type, so only the source-level resolver may answer.
	if c.expressionCrossesModules(file, idx) {
		return c.fallback.ResolveFlatNode(idx, file)
	}
	if flat, ok := c.oracle.(flatExpressionLookup); ok {
		// The flat lookup already falls back to line:col when the file has no byte ranges. Retrying
		// it here would return the type of a receiver that starts at the same position as the call.
		if t := flat.LookupExpressionFlat(file, idx); t != nil {
			return t
		}
		return c.fallback.ResolveFlatNode(idx, file)
	}
	if file != nil {
		line := file.FlatRow(idx) + 1
		col := file.FlatCol(idx) + 1
		if t := c.oracle.LookupExpression(file.Path, line, col); t != nil {
			return t
		}
	}
	return c.fallback.ResolveFlatNode(idx, file)
}

func (c *CompositeResolver) ResolveByNameFlat(name string, idx uint32, file *scanner.File) *typeinfer.ResolvedType {
	src := c.fallback.ResolveByNameFlat(name, idx, file)
	if src != nil && src.Kind != typeinfer.TypeUnknown && src.Resolved {
		return src
	}
	if t := c.OracleExpressionFlat(file, idx); t != nil {
		return t
	}
	if src != nil {
		return src
	}
	if info := c.oracle.LookupClass(name); info != nil {
		return &typeinfer.ResolvedType{Name: info.Name, FQN: info.FQN, Kind: typeinfer.TypeClass, Resolved: true}
	}
	return nil
}

// ResolveImport checks fallback first, then oracle dependencies.
func (c *CompositeResolver) ResolveImport(simpleName string, file *scanner.File) string {
	if fqn := c.fallback.ResolveImport(simpleName, file); fqn != "" {
		return fqn
	}
	if info := c.oracle.LookupClass(simpleName); info != nil {
		return info.FQN
	}
	return ""
}

// OracleExpressionFlat returns the oracle's type for the node, or nil when it has none or the type is a
// smart cast that rests on another module's property (see SmartCastCrossesModules): a per-module build
// keeps the declared type there, which the oracle does not record.
func (c *CompositeResolver) OracleExpressionFlat(file *scanner.File, idx uint32) *typeinfer.ResolvedType {
	if c.expressionCrossesModules(file, idx) {
		return nil
	}
	if flat, ok := c.oracle.(flatExpressionLookup); ok {
		return flat.LookupExpressionFlat(file, idx)
	}
	if file == nil {
		return nil
	}
	return c.oracle.LookupExpression(file.Path, file.FlatRow(idx)+1, file.FlatCol(idx)+1)
}

type smartCastDeclLookup interface {
	LookupExpressionSmartCastDeclsFlat(file *scanner.File, idx uint32) []string
}

// expressionCrossesModules reports whether the oracle's type for the node is a smart cast of another module's property.
func (c *CompositeResolver) expressionCrossesModules(file *scanner.File, idx uint32) bool {
	lookup, ok := c.oracle.(smartCastDeclLookup)
	if !ok || file == nil {
		return false
	}
	decls := lookup.LookupExpressionSmartCastDeclsFlat(file, idx)
	if len(decls) == 0 {
		return false
	}
	crosses, _ := c.smartCastCrossesModules(file.Path, decls)
	return crosses
}

// Oracle returns the underlying oracle Lookup, allowing rules to call
// LookupAnnotations, LookupCallTarget, etc. directly.
func (c *CompositeResolver) Oracle() Lookup { return c.oracle }

// Fallback returns the source-level TypeResolver wrapped by this
// composite. The dispatcher hands this out as ctx.Resolver for rules
// that declare TypeInfo.PreferBackend = PreferResolver so they avoid
// the oracle IPC for every lookup.
func (c *CompositeResolver) Fallback() typeinfer.TypeResolver { return c.fallback }

func (c *CompositeResolver) IsNullableFlat(idx uint32, file *scanner.File) *bool {
	return c.fallback.IsNullableFlat(idx, file)
}

// ClassHierarchy checks oracle first (has dependency types), then fallback.
func (c *CompositeResolver) ClassHierarchy(typeName string) *typeinfer.ClassInfo {
	if info := c.oracle.LookupClass(typeName); info != nil {
		return info
	}
	return c.fallback.ClassHierarchy(typeName)
}

// HasKnownSubtypes delegates to the source-level fallback resolver.
func (c *CompositeResolver) HasKnownSubtypes(typeName string) bool {
	finder, ok := c.fallback.(typeinfer.SubtypeFinder)
	return ok && finder.HasKnownSubtypes(typeName)
}

// SealedVariants checks oracle first, then fallback.
func (c *CompositeResolver) SealedVariants(sealedTypeName string) []string {
	sealedTypeName = stripTypeArguments(sealedTypeName)
	if variants := c.oracle.LookupSealedVariants(sealedTypeName); len(variants) > 0 {
		return variants
	}
	return c.fallback.SealedVariants(sealedTypeName)
}

// EnumEntries checks oracle first, then fallback.
func (c *CompositeResolver) EnumEntries(enumTypeName string) []string {
	if entries := c.oracle.LookupEnumEntries(enumTypeName); len(entries) > 0 {
		return entries
	}
	return c.fallback.EnumEntries(enumTypeName)
}

func (c *CompositeResolver) AnnotationValueFlat(idx uint32, file *scanner.File, annotationName, argName string) string {
	return c.fallback.AnnotationValueFlat(idx, file, annotationName, argName)
}

// IsExceptionSubtype checks oracle first (full hierarchy), then fallback.
func (c *CompositeResolver) IsExceptionSubtype(a, b string) bool {
	if c.oracle.IsSubtype(a, b) {
		return true
	}
	return c.fallback.IsExceptionSubtype(a, b)
}

// SetModuleOf installs the mapping from a source file to the key of the
// module that compiles it, "" when unknown. Source sets that see each other's
// internals (a project's main and test) must share a key. Without a mapping
// SmartCastCrossesModules is always false and every verdict stands.
func (c *CompositeResolver) SetModuleOf(moduleOf func(path string) string) {
	if moduleOf == nil {
		c.moduleOf.Store(nil)
		return
	}
	c.moduleOf.Store(&moduleOf)
}

// SetModuleLog installs a sink for one line per smart-cast diagnostic
// explaining whether SmartCastCrossesModules kept or dropped it; nil disables it.
func (c *CompositeResolver) SetModuleLog(logf func(format string, args ...any)) {
	c.moduleLogged.Clear()
	if logf == nil {
		c.moduleLog.Store(nil)
		return
	}
	c.moduleLog.Store(&logf)
}

// UnknownSmartCastDeclaration is the SmartCastDecls entry for a property whose declaring file the compiler
// helper could not find. It never maps to a module.
const UnknownSmartCastDeclaration = "<unknown>"

// SmartCastCrossesModules reports whether d, reported in filePath, rests on a
// smart cast of a property declared in another module. The oracle compiles the
// whole repo as one module, where such a property is stable; a per-module
// build rejects the smart cast, so the verdict does not hold there.
func (c *CompositeResolver) SmartCastCrossesModules(filePath string, d Diagnostic) bool {
	crosses, reason := c.smartCastCrossesModules(filePath, d.SmartCastDecls)
	if reason != "" {
		c.logSmartCast(filePath, d, crosses, reason)
	}
	return crosses
}

func (c *CompositeResolver) smartCastCrossesModules(filePath string, decls []string) (bool, string) {
	if len(decls) == 0 {
		return false, ""
	}
	fn := c.moduleOf.Load()
	if fn == nil {
		return false, "kept: no module map"
	}
	own := (*fn)(filePath)
	if own == "" {
		return false, "kept: file has no module"
	}
	unknown := 0
	for _, decl := range decls {
		var m string
		if decl != UnknownSmartCastDeclaration {
			m = (*fn)(decl)
		}
		if m == "" {
			unknown++
			continue
		}
		if m != own {
			return true, fmt.Sprintf("dropped: %s declared in %s, used in %s", decl, m, own)
		}
	}
	// Stability needs the property in the same module. A declaration of unknown module may be another one,
	// so the verdict is unproven and does not stand.
	if unknown > 0 {
		return true, fmt.Sprintf("dropped: %d declaring file(s) have no module", unknown)
	}
	return false, "kept: declared in the same module " + own
}

// logSmartCast writes each diagnostic's verdict once, though several rules may ask.
func (c *CompositeResolver) logSmartCast(filePath string, d Diagnostic, crosses bool, reason string) {
	logf := c.moduleLog.Load()
	if logf == nil {
		return
	}
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%d:%d", filePath, d.FactoryName, d.StartByte, d.Line, d.Col)
	if _, seen := c.moduleLogged.LoadOrStore(key, crosses); seen {
		return
	}
	(*logf)("verbose: oracle smart cast %s %s:%d:%d: %s\n", d.FactoryName, filePath, d.Line, d.Col, reason)
}
