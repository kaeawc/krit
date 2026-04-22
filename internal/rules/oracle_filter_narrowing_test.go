package rules

import (
	"slices"
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestOracleFilterNarrowingForAuditedRules locks in the oracle filters
// for the three rules migrated from AllFiles: true to identifier-based
// narrowing in issue #306. A regression to AllFiles: true (or to an
// unexpected identifier set) would silently re-expand the oracle input
// corpus, undoing the Scenario A wall-time win; this test guards against
// that.
func TestOracleFilterNarrowingForAuditedRules(t *testing.T) {
	cases := []struct {
		id          string
		identifiers []string
		callTargets []string
	}{
		{"Deprecation", []string{"Deprecated"}, []string{"Deprecated"}},
		{"IgnoredReturnValue", []string{"CheckReturnValue", "CheckResult"}, []string{"CheckReturnValue", "CheckResult"}},
		{"UnreachableCode", []string{"return", "throw", "break", "continue"}, nil},
		{"UseIsNullOrEmpty", []string{"isEmpty", "count", ".size", ".length", "\"\""}, nil},
		{"CastNullableToNonNullableType", []string{" as "}, nil},
	}

	byID := map[string]*v2.Rule{}
	for _, r := range v2.Registry {
		byID[r.ID] = r
	}

	for _, tc := range cases {
		r, ok := byID[tc.id]
		if !ok {
			t.Errorf("%s: rule not found in v2.Registry", tc.id)
			continue
		}
		if !r.Needs.Has(v2.NeedsOracle) {
			t.Errorf("%s: expected NeedsOracle capability, got Needs=%b", tc.id, r.Needs)
		}
		if r.Oracle == nil {
			t.Errorf("%s: Oracle filter is nil (would default to AllFiles); expected identifier-based narrowing", tc.id)
			continue
		}
		if r.Oracle.AllFiles {
			t.Errorf("%s: Oracle.AllFiles=true; expected identifier-based narrowing (issue #306)", tc.id)
		}
		if !slices.Equal(r.Oracle.Identifiers, tc.identifiers) {
			t.Errorf("%s: Oracle.Identifiers = %v, want %v", tc.id, r.Oracle.Identifiers, tc.identifiers)
		}
		if len(tc.callTargets) > 0 {
			if r.OracleCallTargets == nil {
				t.Errorf("%s: OracleCallTargets is nil; expected annotated declaration narrowing", tc.id)
			} else if r.OracleCallTargets.AllCalls {
				t.Errorf("%s: OracleCallTargets.AllCalls=true; expected annotated declaration narrowing", tc.id)
			} else if !slices.Equal(r.OracleCallTargets.AnnotatedIdentifiers, tc.callTargets) {
				t.Errorf("%s: OracleCallTargets.AnnotatedIdentifiers = %v, want %v", tc.id, r.OracleCallTargets.AnnotatedIdentifiers, tc.callTargets)
			}
		}
	}
}

func TestOracleCallTargetFilterDefaultRulesEnabled(t *testing.T) {
	var active []*v2.Rule
	for _, r := range v2.Registry {
		if IsDefaultActive(r.ID) {
			active = append(active, r)
		}
	}

	summary := BuildOracleCallTargetFilterV2ForFiles(active, []*scanner.File{{
		Path:    "Empty.kt",
		Content: []byte("package test\nclass Empty\n"),
	}})
	if !summary.Enabled {
		t.Fatalf("default rules disabled oracle call filtering: disabledBy=%v", summary.DisabledBy)
	}
	if slices.Contains(summary.DisabledBy, "IgnoredReturnValue") {
		t.Fatalf("IgnoredReturnValue should not disable default call filtering: %+v", summary)
	}
}
