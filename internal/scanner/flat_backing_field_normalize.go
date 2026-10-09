package scanner

// ExplicitBackingFieldNodeType is the krit-specific node type given to a
// Kotlin 2.4 explicit backing field (`field = MutableStateFlow(0)` under a
// property). The pinned tree-sitter grammar predates the syntax and recovers
// that line as a separate property_declaration with a MISSING `val`, which
// declaration rules would report as a public property named `field`.
const ExplicitBackingFieldNodeType = "explicit_backing_field"

// normalizeExplicitBackingFields re-types recovered explicit backing fields so
// property rules skip them. Only that exact recovered shape is touched: a
// property_declaration named `field`, whose binding keyword is a zero-width
// MISSING node, directly after another property_declaration.
func normalizeExplicitBackingFields(t *FlatTree, content []byte) *FlatTree {
	if t == nil || len(t.Types) == 0 {
		return t
	}
	changed := false
	for i := range t.Types {
		idx := uint32(i)
		if t.Flags[idx]&flatNodeFlagError == 0 || nodeTypeName(t.Types[idx]) != "property_declaration" {
			continue
		}
		if isRecoveredBackingField(t, idx, content) {
			t.Types[idx] = internNodeType(ExplicitBackingFieldNodeType)
			changed = true
		}
	}
	if changed {
		t.buildNodesByType()
	}
	return t
}

func isRecoveredBackingField(t *FlatTree, idx uint32, content []byte) bool {
	prev := t.PrevSibs[idx]
	if prev == 0 || nodeTypeName(t.Types[prev]) != "property_declaration" {
		return false
	}
	kind := t.FirstChildren[idx]
	if kind == 0 || nodeTypeName(t.Types[kind]) != "binding_pattern_kind" {
		return false
	}
	keyword := t.FirstChildren[kind]
	if keyword == 0 || t.Flags[keyword]&flatNodeFlagIsError == 0 || t.StartBytes[keyword] != t.EndBytes[keyword] {
		return false
	}
	decl := t.NextSibs[kind]
	if decl == 0 || nodeTypeName(t.Types[decl]) != "variable_declaration" {
		return false
	}
	name := t.FirstChildren[decl]
	if name == 0 || nodeTypeName(t.Types[name]) != "simple_identifier" {
		return false
	}
	start, end := t.StartBytes[name], t.EndBytes[name]
	return int(end) <= len(content) && string(content[start:end]) == "field"
}
