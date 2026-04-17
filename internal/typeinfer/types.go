// Package typeinfer provides lightweight type inference for Kotlin source code.
// It builds a partial type system from AST analysis — no JVM, no classpath,
// no compilation required. Runs at tree-sitter speed.
//
// What it resolves:
//   - Import-based type resolution (java.util.Random → knows Random is java.util.Random)
//   - Declaration-site types (val x: String = ... → knows x is String)
//   - Nullable vs non-null tracking (String vs String?)
//   - Partial class hierarchy from source (class Foo : Bar → knows Foo extends Bar)
//   - Sealed class/enum variants (sealed class Result → knows Success, Failure)
//   - Annotation arguments (@RequiresApi(26) → knows value is 26)
//
// What it does NOT resolve:
//   - Types from compiled dependencies (JARs)
//   - Generic type argument inference
//   - Complex expression type inference (result of chained calls)
//   - Overload resolution
package typeinfer

// TypeKind represents the category of a resolved type.
type TypeKind int

const (
	TypeUnknown   TypeKind = iota
	TypeClass              // class, interface, object, enum
	TypePrimitive          // Int, Long, Boolean, etc.
	TypeNullable           // T? wrapper
	TypeFunction           // (A, B) -> C
	TypeGeneric            // T, with bounds
	TypeArray              // Array<T>, IntArray, etc.
	TypeUnit               // Unit (void equivalent)
	TypeNothing            // Nothing (bottom type)
)

// ResolvedType represents a type resolved from source analysis.
type ResolvedType struct {
	Name       string     // Simple name: "String", "MutableList"
	FQN        string     // Fully qualified: "kotlin.String", "kotlin.collections.MutableList"
	Kind       TypeKind   // What category of type
	Nullable   bool       // Is this T?
	TypeArgs   []ResolvedType // Generic type arguments
	Supertypes []string   // Known supertypes from source (FQN)
}

// IsNullable returns whether this type allows null values.
func (t *ResolvedType) IsNullable() bool {
	return t.Nullable || t.Kind == TypeNullable
}

// IsMutable returns whether this is a known mutable collection type.
func (t *ResolvedType) IsMutable() bool {
	mutable := map[string]bool{
		"MutableList": true, "MutableSet": true, "MutableMap": true,
		"ArrayList": true, "HashSet": true, "HashMap": true,
		"LinkedHashSet": true, "LinkedHashMap": true,
	}
	return mutable[t.Name]
}

// IsPrimitive returns whether this is a Kotlin primitive type.
func (t *ResolvedType) IsPrimitive() bool {
	return t.Kind == TypePrimitive
}

// IsSubtypeOf checks if this type is a known subtype of the given type name.
func (t *ResolvedType) IsSubtypeOf(typeName string) bool {
	if t.Name == typeName || t.FQN == typeName {
		return true
	}
	for _, st := range t.Supertypes {
		if st == typeName {
			return true
		}
	}
	return false
}

// UnknownType returns a type with no resolution.
func UnknownType() *ResolvedType {
	return &ResolvedType{Kind: TypeUnknown}
}

// PrimitiveTypes maps Kotlin primitive type names to their FQNs.
var PrimitiveTypes = map[string]string{
	"Int": "kotlin.Int", "Long": "kotlin.Long", "Short": "kotlin.Short",
	"Byte": "kotlin.Byte", "Float": "kotlin.Float", "Double": "kotlin.Double",
	"Boolean": "kotlin.Boolean", "Char": "kotlin.Char", "String": "kotlin.String",
	"Unit": "kotlin.Unit", "Nothing": "kotlin.Nothing", "Any": "kotlin.Any",
}

// KotlinStdlibTypes maps common stdlib type simple names to FQNs.
var KotlinStdlibTypes = map[string]string{
	"List":       "kotlin.collections.List",
	"MutableList": "kotlin.collections.MutableList",
	"Set":        "kotlin.collections.Set",
	"MutableSet": "kotlin.collections.MutableSet",
	"Map":        "kotlin.collections.Map",
	"MutableMap": "kotlin.collections.MutableMap",
	"Sequence":   "kotlin.sequences.Sequence",
	"Flow":             "kotlinx.coroutines.flow.Flow",
	"StateFlow":        "kotlinx.coroutines.flow.StateFlow",
	"SharedFlow":       "kotlinx.coroutines.flow.SharedFlow",
	"MutableStateFlow": "kotlinx.coroutines.flow.MutableStateFlow",
	"MutableSharedFlow":"kotlinx.coroutines.flow.MutableSharedFlow",
	"Job":              "kotlinx.coroutines.Job",
	"Deferred":         "kotlinx.coroutines.Deferred",
	"Pair":       "kotlin.Pair",
	"Triple":     "kotlin.Triple",
	"Result":     "kotlin.Result",
	"Lazy":       "kotlin.Lazy",
	"Comparable": "kotlin.Comparable",
	"Iterable":   "kotlin.collections.Iterable",
	"Iterator":   "kotlin.collections.Iterator",
	"Collection": "kotlin.collections.Collection",
	"Array":      "kotlin.Array",
	"IntArray":   "kotlin.IntArray",
	"LongArray":  "kotlin.LongArray",
	"ByteArray":  "kotlin.ByteArray",
	"CharArray":  "kotlin.CharArray",
	"BooleanArray": "kotlin.BooleanArray",
	"FloatArray":  "kotlin.FloatArray",
	"DoubleArray": "kotlin.DoubleArray",
	"ShortArray":  "kotlin.ShortArray",
}
