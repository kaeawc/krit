package rules

import (
	"strings"
	"testing"
)

// TestComposeLintsComposableTablesUnmangled checks the generated library tables: names come without the JVM
// inline-class mangling (whose hash may itself contain '-') and without compiler-synthetic entries, so real
// composables such as FloatingActionButton are present under their source names.
func TestComposeLintsComposableTablesUnmangled(t *testing.T) {
	for _, fqn := range []string{
		"androidx.compose.material3.FloatingActionButton",
		"androidx.compose.material3.SmallFloatingActionButton",
		"androidx.compose.material3.LargeFloatingActionButton",
		"androidx.compose.material3.WideNavigationRailItem",
		"androidx.compose.foundation.text.input.rememberTextFieldState",
		"androidx.compose.material.LeadingIconTab",
		"androidx.compose.foundation.layout.Column",
	} {
		if composeLintsLibraryComposableFunctions[fqn] == 0 {
			t.Errorf("composeLintsLibraryComposableFunctions lacks %s", fqn)
		}
	}
	for _, name := range []string{"elevation", "loweredElevation", "filterChipColors", "ExposedDropdownMenu",
		"rememberSharedContentState"} {
		if composeLintsLibraryComposableMembers[name] == 0 {
			t.Errorf("composeLintsLibraryComposableMembers lacks %s", name)
		}
	}
	for table, entries := range map[string]map[string]int8{
		"composeLintsLibraryComposableFunctions":      composeLintsLibraryComposableFunctions,
		"composeLintsLibraryComposableMembers":        composeLintsLibraryComposableMembers,
		"composeLintsLibraryComposableMembersByClass": composeLintsLibraryComposableMembersByClass,
	} {
		for key := range entries {
			if strings.ContainsAny(key, "-$") || strings.Contains(key, "ComposableSingletons") {
				t.Errorf("%s holds a mangled or synthetic name %q", table, key)
			}
		}
	}
}
