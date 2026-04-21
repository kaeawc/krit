package oracle

import (
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// CompositeResolver wraps a Lookup oracle with a fallback TypeResolver.
// Oracle provides dependency types; fallback provides source-level inference.
type CompositeResolver struct {
	oracle   Lookup
	fallback typeinfer.TypeResolver
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
	if t := c.fallback.ResolveByNameFlat(name, idx, file); t != nil {
		return t
	}
	if info := c.oracle.LookupClass(name); info != nil {
		return &typeinfer.ResolvedType{Name: info.Name, FQN: info.FQN, Kind: typeinfer.TypeClass}
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

// SealedVariants checks oracle first, then fallback.
func (c *CompositeResolver) SealedVariants(sealedTypeName string) []string {
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
