package oracle

import "testing"

// krit-fir sends nullable types with their `?`; source inference keeps the
// FQN bare. A rule comparing FQNs (TrimLambda on `maybe?.trim {}`) must see
// the same type either way (#1869).
func TestMakeResolvedTypeStripsTheNullableMarker(t *testing.T) {
	for _, tc := range []struct {
		in, fqn, name string
		nullable      bool
	}{
		{"kotlin.String?", "kotlin.String", "String", true},
		{"kotlin.String", "kotlin.String", "String", false},
		{"kotlin.collections.List<kotlin.String?>?", "kotlin.collections.List<kotlin.String?>", "", true},
		{"kotlin.collections.List<kotlin.String?>", "kotlin.collections.List<kotlin.String?>", "", false},
		{"((kotlin.Int) -> kotlin.Unit)?", "((kotlin.Int) -> kotlin.Unit)", "", true},
		// The marker belongs to the function type's return type.
		{"(kotlin.Int) -> kotlin.String?", "(kotlin.Int) -> kotlin.String?", "", false},
	} {
		got := makeResolvedType(tc.in, false)
		if got.FQN != tc.fqn || got.Nullable != tc.nullable || (tc.name != "" && got.Name != tc.name) {
			t.Errorf("makeResolvedType(%q) = FQN %q Name %q Nullable %v, want FQN %q Name %q Nullable %v", tc.in, got.FQN, got.Name, got.Nullable, tc.fqn, tc.name, tc.nullable)
		}
	}
}
