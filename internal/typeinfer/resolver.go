package typeinfer

import "github.com/kaeawc/krit/internal/scanner"

// TypeResolver resolves types from Kotlin source ASTs.
// It is the main interface rules use to query type information.
type TypeResolver interface {
	// ResolveFlatNode returns the inferred type at a flat-tree index.
	// This is the preferred production API.
	// Returns UnknownType if the type cannot be determined.
	ResolveFlatNode(idx uint32, file *scanner.File) *ResolvedType

	// ResolveByName looks up a variable/property name in the current scope.
	ResolveByNameFlat(name string, idx uint32, file *scanner.File) *ResolvedType

	// ResolveImport returns the FQN for an imported simple name.
	// Returns "" if not imported.
	ResolveImport(simpleName string, file *scanner.File) string

	IsNullableFlat(idx uint32, file *scanner.File) *bool

	// ClassHierarchy returns the known class hierarchy for a type.
	ClassHierarchy(typeName string) *ClassInfo

	// SealedVariants returns known variants of a sealed class/interface.
	SealedVariants(sealedTypeName string) []string

	// EnumEntries returns known entries of an enum class.
	EnumEntries(enumTypeName string) []string

	AnnotationValueFlat(idx uint32, file *scanner.File, annotationName, argName string) string

	// IsExceptionSubtype checks if exceptionA is a known subtype of exceptionB.
	IsExceptionSubtype(a, b string) bool
}

// ClassInfo holds information about a class resolved from source.
type ClassInfo struct {
	Name       string   // Simple name
	FQN        string   // Fully qualified name
	Kind       string   // "class", "interface", "object", "enum", "sealed class", "sealed interface"
	Supertypes []string // FQNs of direct supertypes
	IsSealed   bool
	IsData     bool
	IsInner    bool
	IsAbstract bool
	IsOpen     bool
	Members    []MemberInfo
	File       string // Source file path
	Line       int
}

// MemberInfo holds information about a class member.
type MemberInfo struct {
	Name       string
	Kind       string // "function", "property"
	Type       *ResolvedType
	Visibility string // "public", "private", "internal", "protected"
	IsOverride bool
	IsAbstract bool
}

// ImportTable maps simple names to FQNs for a single file.
type ImportTable struct {
	Explicit map[string]string // import com.foo.Bar → "Bar" → "com.foo.Bar"
	Wildcard []string          // import com.foo.* → ["com.foo"]
	Aliases  map[string]string // import com.foo.Bar as Baz → "Baz" → "com.foo.Bar"
}

// Resolve returns the FQN for a simple name, checking explicit imports,
// Kotlin stdlib, and primitives.
func (it *ImportTable) Resolve(simpleName string) string {
	// Check explicit imports first
	if fqn, ok := it.Explicit[simpleName]; ok {
		// Map Java types to Kotlin equivalents
		if kotlinFQN := MapJavaToKotlin(fqn); kotlinFQN != "" {
			return kotlinFQN
		}
		return fqn
	}
	// Check aliases
	if fqn, ok := it.Aliases[simpleName]; ok {
		if kotlinFQN := MapJavaToKotlin(fqn); kotlinFQN != "" {
			return kotlinFQN
		}
		return fqn
	}
	// Check primitives
	if fqn, ok := PrimitiveTypes[simpleName]; ok {
		return fqn
	}
	// Check stdlib
	if fqn, ok := KotlinStdlibTypes[simpleName]; ok {
		return fqn
	}
	// Check wildcard imports — can't resolve without classpath, but record the package
	// For partial resolution, we know it MIGHT be from one of these packages
	return ""
}

// ScopeTable tracks variable declarations and their types within a scope.
type ScopeTable struct {
	Parent         *ScopeTable
	Children       []*ScopeTable
	Entries        map[string]*ResolvedType // name → type
	SmartCasts     map[string]bool          // variable names known to be non-null in this scope
	SmartCastTypes map[string]*ResolvedType // variable names narrowed by is-checks
	StartByte      uint32                   // byte offset where this scope begins
	EndByte        uint32                   // byte offset where this scope ends
}

// NewScope creates a child scope.
func (s *ScopeTable) NewScope() *ScopeTable {
	child := &ScopeTable{Parent: s, Entries: make(map[string]*ResolvedType), SmartCasts: make(map[string]bool), SmartCastTypes: make(map[string]*ResolvedType)}
	s.Children = append(s.Children, child)
	return child
}

// NewScopeForNode creates a child scope with byte range from the given node.
func (s *ScopeTable) NewScopeForNode(node interface {
	StartByte() uint32
	EndByte() uint32
}) *ScopeTable {
	child := s.NewScope()
	child.StartByte = node.StartByte()
	child.EndByte = node.EndByte()
	return child
}

// FindScopeAtOffset finds the most specific (deepest) scope containing the given byte offset.
func (s *ScopeTable) FindScopeAtOffset(offset uint32) *ScopeTable {
	for _, child := range s.Children {
		if child.StartByte <= offset && offset <= child.EndByte {
			if deeper := child.FindScopeAtOffset(offset); deeper != nil {
				return deeper
			}
			return child
		}
	}
	// If no child contains the offset, check if this scope itself does
	if s.StartByte <= offset && offset <= s.EndByte {
		return s
	}
	// Root scope (StartByte==0, EndByte==0) always matches
	if s.StartByte == 0 && s.EndByte == 0 && s.Parent == nil {
		return s
	}
	return nil
}

// Declare adds a variable to the current scope.
func (s *ScopeTable) Declare(name string, typ *ResolvedType) {
	s.Entries[name] = typ
}

// Lookup finds a variable in the current scope or any parent.
// If the variable is smart-cast to non-null in the current scope chain,
// the returned type will have Nullable set to false.
// If the variable has been narrowed by an is-check, the narrowed type is returned.
func (s *ScopeTable) Lookup(name string) *ResolvedType {
	// Check if this scope or a parent has the entry
	typ := s.lookupRaw(name)
	if typ == nil {
		return nil
	}
	// Apply type smart cast: if the name has been narrowed by an is-check in this scope chain
	if narrowed := s.lookupSmartCastType(name); narrowed != nil {
		return narrowed
	}
	// Apply smart cast: if the name is known non-null, return a non-null copy
	if typ.Nullable && s.IsSmartCastNonNull(name) {
		nonNull := *typ
		nonNull.Nullable = false
		if nonNull.Kind == TypeNullable {
			nonNull.Kind = TypeClass
		}
		return &nonNull
	}
	return typ
}

// lookupSmartCastType checks if a variable has been narrowed by an is-check in this scope chain.
func (s *ScopeTable) lookupSmartCastType(name string) *ResolvedType {
	if s.SmartCastTypes != nil {
		if t, ok := s.SmartCastTypes[name]; ok {
			return t
		}
	}
	if s.Parent != nil {
		return s.Parent.lookupSmartCastType(name)
	}
	return nil
}

// lookupRaw finds a variable without applying smart casts.
func (s *ScopeTable) lookupRaw(name string) *ResolvedType {
	if typ, ok := s.Entries[name]; ok {
		return typ
	}
	if s.Parent != nil {
		return s.Parent.lookupRaw(name)
	}
	return nil
}

// IsSmartCastNonNull checks if a name is smart-cast to non-null in this scope or any parent.
func (s *ScopeTable) IsSmartCastNonNull(name string) bool {
	if s.SmartCasts != nil && s.SmartCasts[name] {
		return true
	}
	if s.Parent != nil {
		return s.Parent.IsSmartCastNonNull(name)
	}
	return false
}
