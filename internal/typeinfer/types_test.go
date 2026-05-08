package typeinfer

import "testing"

// --- ResolvedType.IsNullable ---

func TestIsNullable_NullableFlag(t *testing.T) {
	rt := &ResolvedType{Name: "String", Kind: TypeClass, Nullable: true}
	if !rt.IsNullable() {
		t.Error("expected nullable type to report IsNullable() == true")
	}
}

func TestIsNullable_NullableKind(t *testing.T) {
	rt := &ResolvedType{Name: "String", Kind: TypeNullable}
	if !rt.IsNullable() {
		t.Error("expected TypeNullable kind to report IsNullable() == true")
	}
}

func TestIsNullable_NonNullable(t *testing.T) {
	rt := &ResolvedType{Name: "String", Kind: TypeClass}
	if rt.IsNullable() {
		t.Error("expected non-nullable type to report IsNullable() == false")
	}
}

// --- ResolvedType.IsMutable ---

func TestIsMutable_MutableList(t *testing.T) {
	rt := &ResolvedType{Name: "MutableList", Kind: TypeClass}
	if !rt.IsMutable() {
		t.Error("expected MutableList to be mutable")
	}
}

func TestIsMutable_ArrayList(t *testing.T) {
	rt := &ResolvedType{Name: "ArrayList", Kind: TypeClass}
	if !rt.IsMutable() {
		t.Error("expected ArrayList to be mutable")
	}
}

func TestIsMutable_HashMap(t *testing.T) {
	rt := &ResolvedType{Name: "HashMap", Kind: TypeClass}
	if !rt.IsMutable() {
		t.Error("expected HashMap to be mutable")
	}
}

func TestIsMutable_ImmutableList(t *testing.T) {
	rt := &ResolvedType{Name: "List", Kind: TypeClass}
	if rt.IsMutable() {
		t.Error("expected List to not be mutable")
	}
}

func TestIsMutable_String(t *testing.T) {
	rt := &ResolvedType{Name: "String", Kind: TypeClass}
	if rt.IsMutable() {
		t.Error("expected String to not be mutable")
	}
}

// --- ResolvedType.IsPrimitive ---

func TestIsPrimitive_Int(t *testing.T) {
	rt := &ResolvedType{Name: "Int", Kind: TypePrimitive}
	if !rt.IsPrimitive() {
		t.Error("expected Int with TypePrimitive kind to be primitive")
	}
}

func TestIsPrimitive_Class(t *testing.T) {
	rt := &ResolvedType{Name: "Foo", Kind: TypeClass}
	if rt.IsPrimitive() {
		t.Error("expected class type to not be primitive")
	}
}

func TestIsPrimitive_Unknown(t *testing.T) {
	rt := UnknownType()
	if rt.IsPrimitive() {
		t.Error("expected unknown type to not be primitive")
	}
}

// --- ResolvedType.IsSubtypeOf ---

func TestIsSubtypeOf_DirectNameMatch(t *testing.T) {
	rt := &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}
	if !rt.IsSubtypeOf("String") {
		t.Error("expected direct name match to return true")
	}
}

func TestIsSubtypeOf_FQNMatch(t *testing.T) {
	rt := &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}
	if !rt.IsSubtypeOf("kotlin.String") {
		t.Error("expected FQN match to return true")
	}
}

func TestIsSubtypeOf_SupertypeMatch(t *testing.T) {
	rt := &ResolvedType{
		Name:       "ArrayList",
		FQN:        "java.util.ArrayList",
		Kind:       TypeClass,
		Supertypes: []string{"java.util.List", "kotlin.collections.MutableList"},
	}
	if !rt.IsSubtypeOf("java.util.List") {
		t.Error("expected supertype match to return true")
	}
	if !rt.IsSubtypeOf("kotlin.collections.MutableList") {
		t.Error("expected second supertype match to return true")
	}
}

func TestIsSubtypeOf_NoMatch(t *testing.T) {
	rt := &ResolvedType{
		Name: "String",
		FQN:  "kotlin.String",
		Kind: TypePrimitive,
	}
	if rt.IsSubtypeOf("Int") {
		t.Error("expected no match to return false")
	}
	if rt.IsSubtypeOf("kotlin.Int") {
		t.Error("expected FQN mismatch to return false")
	}
}

// --- UnknownType ---

func TestUnknownType(t *testing.T) {
	u := UnknownType()
	if u.Kind != TypeUnknown {
		t.Errorf("expected TypeUnknown, got %v", u.Kind)
	}
	if u.Name != "" {
		t.Errorf("expected empty name, got %q", u.Name)
	}
}

// --- ImportTable.Resolve ---

func TestImportTable_Resolve_ExplicitImport(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{"Date": "java.util.Date"},
		Aliases:  map[string]string{},
	}
	got := it.Resolve("Date")
	if got != "java.util.Date" {
		t.Errorf("expected java.util.Date, got %q", got)
	}
}

func TestImportTable_Resolve_Alias(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{},
		Aliases:  map[string]string{"ML": "kotlin.collections.MutableList"},
	}
	got := it.Resolve("ML")
	if got != "kotlin.collections.MutableList" {
		t.Errorf("expected kotlin.collections.MutableList, got %q", got)
	}
}

func TestImportTable_Resolve_Primitive(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{},
		Aliases:  map[string]string{},
	}
	got := it.Resolve("Int")
	if got != "kotlin.Int" {
		t.Errorf("expected kotlin.Int, got %q", got)
	}
}

func TestImportTable_Resolve_Stdlib(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{},
		Aliases:  map[string]string{},
	}
	got := it.Resolve("List")
	if got != "kotlin.collections.List" {
		t.Errorf("expected kotlin.collections.List, got %q", got)
	}
}

func TestImportTable_Resolve_ExplicitOverridesPrimitive(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{"Int": "com.custom.Int"},
		Aliases:  map[string]string{},
	}
	got := it.Resolve("Int")
	if got != "com.custom.Int" {
		t.Errorf("expected explicit to override primitive, got %q", got)
	}
}

func TestImportTable_Resolve_Unknown(t *testing.T) {
	it := &ImportTable{
		Explicit: map[string]string{},
		Aliases:  map[string]string{},
	}
	got := it.Resolve("NonExistent")
	if got != "" {
		t.Errorf("expected empty string for unknown, got %q", got)
	}
}

// --- ScopeTable ---

func TestScopeTable_DeclareAndLookup(t *testing.T) {
	scope := &ScopeTable{Entries: make(map[string]*ResolvedType)}
	strType := &ResolvedType{Name: "String", FQN: "kotlin.String", Kind: TypePrimitive}
	scope.Declare("name", strType)

	got := scope.Lookup("name")
	if got != strType {
		t.Errorf("expected to find declared type, got %v", got)
	}
}

func TestScopeTable_LookupParent(t *testing.T) {
	parent := &ScopeTable{Entries: make(map[string]*ResolvedType)}
	intType := &ResolvedType{Name: "Int", FQN: "kotlin.Int", Kind: TypePrimitive}
	parent.Declare("count", intType)

	child := parent.NewScope()
	got := child.Lookup("count")
	if got != intType {
		t.Errorf("expected to find parent scope variable, got %v", got)
	}
}

func TestScopeTable_ChildShadowsParent(t *testing.T) {
	parent := &ScopeTable{Entries: make(map[string]*ResolvedType)}
	parentType := &ResolvedType{Name: "String", Kind: TypePrimitive}
	parent.Declare("x", parentType)

	child := parent.NewScope()
	childType := &ResolvedType{Name: "Int", Kind: TypePrimitive}
	child.Declare("x", childType)

	got := child.Lookup("x")
	if got != childType {
		t.Errorf("expected child to shadow parent, got %v", got)
	}
}

func TestScopeTable_LookupNotFound(t *testing.T) {
	scope := &ScopeTable{Entries: make(map[string]*ResolvedType)}
	got := scope.Lookup("missing")
	if got != nil {
		t.Errorf("expected nil for missing variable, got %v", got)
	}
}
