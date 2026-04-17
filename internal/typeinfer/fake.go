package typeinfer

import (
	"github.com/kaeawc/krit/internal/scanner"
)

// FakeResolver is a configurable test double for TypeResolver.
// Set up responses before using in tests.
type FakeResolver struct {
	NodeTypes   map[string]*ResolvedType // node text → type
	NameTypes   map[string]*ResolvedType // variable name → type
	Imports     map[string]string        // simple name → FQN
	Nullability map[string]*bool         // node text → nullable
	Classes     map[string]*ClassInfo    // type name → class info
	SealedMap   map[string][]string      // sealed class → variants
	EnumMap     map[string][]string      // enum class → entries
	Annotations map[string]string        // "AnnotationName.argName" → value
}

// NewFakeResolver creates a FakeResolver with all maps initialized.
func NewFakeResolver() *FakeResolver {
	return &FakeResolver{
		NodeTypes:   make(map[string]*ResolvedType),
		NameTypes:   make(map[string]*ResolvedType),
		Imports:     make(map[string]string),
		Nullability: make(map[string]*bool),
		Classes:     make(map[string]*ClassInfo),
		SealedMap:   make(map[string][]string),
		EnumMap:     make(map[string][]string),
		Annotations: make(map[string]string),
	}
}

func (f *FakeResolver) ResolveFlatNode(idx uint32, file *scanner.File) *ResolvedType {
	if file == nil || idx == 0 {
		return UnknownType()
	}
	if t, ok := f.NodeTypes[file.FlatNodeText(idx)]; ok {
		return t
	}
	return UnknownType()
}

func (f *FakeResolver) ResolveByNameFlat(name string, idx uint32, file *scanner.File) *ResolvedType {
	if t, ok := f.NameTypes[name]; ok {
		return t
	}
	return nil
}

// ResolveImport returns the configured FQN for the simple name, or "".
func (f *FakeResolver) ResolveImport(simpleName string, file *scanner.File) string {
	if fqn, ok := f.Imports[simpleName]; ok {
		return fqn
	}
	return ""
}

func (f *FakeResolver) IsNullableFlat(idx uint32, file *scanner.File) *bool {
	if file == nil || idx == 0 {
		return nil
	}
	if v, ok := f.Nullability[file.FlatNodeText(idx)]; ok {
		return v
	}
	return nil
}

// ClassHierarchy returns the configured ClassInfo for the type name, or nil.
func (f *FakeResolver) ClassHierarchy(typeName string) *ClassInfo {
	if ci, ok := f.Classes[typeName]; ok {
		return ci
	}
	return nil
}

// SealedVariants returns the configured variants for the sealed class, or nil.
func (f *FakeResolver) SealedVariants(sealedTypeName string) []string {
	if v, ok := f.SealedMap[sealedTypeName]; ok {
		return v
	}
	return nil
}

// EnumEntries returns the configured entries for the enum class, or nil.
func (f *FakeResolver) EnumEntries(enumTypeName string) []string {
	if v, ok := f.EnumMap[enumTypeName]; ok {
		return v
	}
	return nil
}

func (f *FakeResolver) AnnotationValueFlat(idx uint32, file *scanner.File, annotationName, argName string) string {
	key := annotationName + "." + argName
	if v, ok := f.Annotations[key]; ok {
		return v
	}
	return ""
}

// IsExceptionSubtype delegates to the global ExceptionAncestors table.
func (f *FakeResolver) IsExceptionSubtype(a, b string) bool {
	return IsSubtypeOfException(a, b)
}

// Compile-time check that FakeResolver implements TypeResolver.
var _ TypeResolver = (*FakeResolver)(nil)
