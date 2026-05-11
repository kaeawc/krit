package mcp

import (
	"slices"
	"testing"
)

func TestFormatList(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"nil", nil, ""},
		{"empty", []string{}, ""},
		{"single", []string{"foo"}, "foo"},
		{"multi", []string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatList(tc.in); got != tc.want {
				t.Fatalf("formatList(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestDiscriminatorSlicesIncludeAllConstants guards against the common bug
// where a new constant is added but the corresponding slice isn't updated.
// The slices are the source of truth for tool schemas; a missing entry
// silently breaks the dispatcher's enum validation.
func TestDiscriminatorSlicesIncludeAllConstants(t *testing.T) {
	cases := []struct {
		name  string
		slice []string
		want  []string
	}{
		{"analyzeModes", analyzeModes, []string{modeCode, modeProject, modeAndroid, modeImpact}},
		{"fixOperations", fixOperations, []string{opFixSuggest, opFixSuppress}},
		{"rulesOperations", rulesOperations, []string{opRulesExplain, opRulesSearch, opRulesCategories, opRulesConfigure}},
		{"symbolsOperations", symbolsOperations, []string{opSymbolsOutline, opSymbolsReferences}},
		{"typesQueries", typesQueries, []string{queryTypesClasses, queryTypesHierarchy, queryTypesImports, queryTypesSealedVariants, queryTypesEnumEntries, queryTypesFunctionSigs}},
		{"structureOperations", structureOperations, []string{opStructureModules, opStructureProfile, opStructureHotspots, opStructureBreadth, opStructurePkgDrift}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.slice) != len(tc.want) {
				t.Fatalf("%s has %d entries, want %d", tc.name, len(tc.slice), len(tc.want))
			}
			for _, w := range tc.want {
				if !slices.Contains(tc.slice, w) {
					t.Errorf("%s missing %q", tc.name, w)
				}
			}
		})
	}
}

// TestDiscriminatorConstantsDoNotCollide guards against a copy-paste mistake
// where two operations within the same tool accidentally share a string value.
func TestDiscriminatorConstantsDoNotCollide(t *testing.T) {
	cases := []struct {
		name  string
		slice []string
	}{
		{"analyzeModes", analyzeModes},
		{"fixOperations", fixOperations},
		{"rulesOperations", rulesOperations},
		{"symbolsOperations", symbolsOperations},
		{"typesQueries", typesQueries},
		{"structureOperations", structureOperations},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[string]struct{}, len(tc.slice))
			for _, v := range tc.slice {
				if v == "" {
					t.Errorf("%s has empty string entry", tc.name)
				}
				if _, dup := seen[v]; dup {
					t.Errorf("%s has duplicate value %q", tc.name, v)
				}
				seen[v] = struct{}{}
			}
		})
	}
}
