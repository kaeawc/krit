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
		{"IgnoredReturnValue", []string{"Sequence", "Flow", "Stream", "Function", "->", "CheckReturnValue", "CheckResult", "CanIgnoreReturnValue"}, []string{"CheckReturnValue", "CheckResult", "CanIgnoreReturnValue"}},
		{"NullableToStringCall", []string{"toString", "$"}, []string{"toString"}},
		{"ObjectAnimatorBinding", []string{"ObjectAnimator", "ofFloat", "ofInt", "ofObject"}, nil},
		{"SwallowedException", []string{"catch", "Log", "Timber", "Logger", "println", "trace", "debug", "info", "warn", "warning", "severe", "error", "log", "makeText", "Snackbar", "AlertDialog", "showDialog", "showError", "handleError", "reportError", "recoverFrom", "onError", "fallback", "notifyError"}, swallowedExceptionCallTargetCallees()},
		{"UnreachableCode", []string{"return", "throw", "break", "continue"}, nil},
		{"UseIsNullOrEmpty", []string{"isEmpty", "count", ".size", ".length", "\"\""}, nil},
		{"UnsafeCast", []string{" as ", " as?"}, nil},
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
		if !RuleNeedsKotlinOracle(r) {
			t.Errorf("%s: expected oracle consumer, got Needs=%b", tc.id, r.Needs)
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
				t.Errorf("%s: OracleCallTargets is nil; expected call-target narrowing", tc.id)
			} else if r.OracleCallTargets.AllCalls {
				t.Errorf("%s: OracleCallTargets.AllCalls=true; expected call-target narrowing", tc.id)
			} else if !slices.Equal(oracleCallTargetIdentifiers(r.OracleCallTargets), tc.callTargets) {
				t.Errorf("%s: OracleCallTargets identifiers = %v, want %v", tc.id, oracleCallTargetIdentifiers(r.OracleCallTargets), tc.callTargets)
			}
		}
	}
}

func oracleCallTargetIdentifiers(filter *v2.OracleCallTargetFilter) []string {
	if len(filter.AnnotatedIdentifiers) > 0 {
		return filter.AnnotatedIdentifiers
	}
	return filter.CalleeNames
}

func TestMissingPermissionOracleFiltersAreNarrowed(t *testing.T) {
	var rule *v2.Rule
	for _, r := range v2.Registry {
		if r.ID == "MissingPermission" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("MissingPermission rule not found in v2.Registry")
	}
	if rule.Oracle == nil || rule.Oracle.AllFiles {
		t.Fatalf("MissingPermission Oracle filter = %+v, want identifier narrowing", rule.Oracle)
	}
	wantIdentifiers := []string{"RequiresPermission", "getCellLocation", "getLastKnownLocation", "open", "requestLocationUpdates", "setAudioSource"}
	if !slices.Equal(rule.Oracle.Identifiers, wantIdentifiers) {
		t.Fatalf("MissingPermission Oracle.Identifiers = %v, want %v", rule.Oracle.Identifiers, wantIdentifiers)
	}
	if rule.OracleCallTargets == nil || rule.OracleCallTargets.AllCalls {
		t.Fatalf("MissingPermission OracleCallTargets = %+v, want bounded call filtering", rule.OracleCallTargets)
	}
	wantCallees := []string{"getCellLocation", "getLastKnownLocation", "open", "requestLocationUpdates", "setAudioSource"}
	if !slices.Equal(rule.OracleCallTargets.CalleeNames, wantCallees) {
		t.Fatalf("MissingPermission OracleCallTargets.CalleeNames = %v, want %v", rule.OracleCallTargets.CalleeNames, wantCallees)
	}
	if !slices.Equal(rule.OracleCallTargets.AnnotatedIdentifiers, []string{"RequiresPermission"}) {
		t.Fatalf("MissingPermission OracleCallTargets.AnnotatedIdentifiers = %v, want [RequiresPermission]", rule.OracleCallTargets.AnnotatedIdentifiers)
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
	if slices.Contains(summary.DisabledBy, "IgnoredReturnValue") {
		t.Fatalf("IgnoredReturnValue should not disable default call filtering: %+v", summary)
	}
	for range summary.DisabledBy {
		t.Fatalf("unexpected default rule disabled oracle call filtering: disabledBy=%v", summary.DisabledBy)
	}
}

func TestResolverOnlyRulesDoNotContributeToOracle(t *testing.T) {
	for _, id := range []string{
		"CastNullableToNonNullableType",
		"ComposeClickableWithoutMinTouchTarget",
		"InjectDispatcher",
		"LogOfSharedPreferenceRead",
		"PlainFileWriteOfSensitive",
		"SharedPreferencesForSensitiveKey",
		"SpreadOperator",
		"UnnecessaryNotNullOperator",
	} {
		rule := findRegisteredRule(t, id)
		if RuleNeedsKotlinOracle(rule) {
			t.Fatalf("%s should be resolver-only, got Needs=%b Oracle=%+v OracleCallTargets=%+v OracleDeclarationNeeds=%+v",
				id, rule.Needs, rule.Oracle, rule.OracleCallTargets, rule.OracleDeclarationNeeds)
		}
	}
}

func TestTimberTreeNotPlantedUsesLexicalHintsForLoggerCallees(t *testing.T) {
	rule := findRegisteredRule(t, "TimberTreeNotPlanted")
	if rule.OracleCallTargets == nil {
		t.Fatal("TimberTreeNotPlanted OracleCallTargets is nil")
	}
	for _, callee := range []string{"v", "d", "i", "w", "e", "wtf", "plant"} {
		hints := rule.OracleCallTargets.LexicalHintsByCallee[callee]
		if !slices.Contains(hints, "timber.log.Timber") || !slices.Contains(hints, "Timber") {
			t.Fatalf("TimberTreeNotPlanted hints for %q = %v, want Timber hints", callee, hints)
		}
	}
}

func findRegisteredRule(t *testing.T, id string) *v2.Rule {
	t.Helper()
	for _, r := range v2.Registry {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("%s rule not found in v2.Registry", id)
	return nil
}
