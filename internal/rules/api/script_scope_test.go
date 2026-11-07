package api

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestScriptScopeApplies(t *testing.T) {
	tests := []struct {
		scope ScriptScope
		kind  scanner.ScriptKind
		want  bool
	}{
		{ScriptsAll, scanner.ScriptGradle, true},
		{ScriptsAll, scanner.ScriptStandalone, true},
		{ScriptsNone, scanner.ScriptGradle, false},
		{ScriptsNone, scanner.ScriptStandalone, false},
		{ScriptsStandalone, scanner.ScriptGradle, false},
		{ScriptsStandalone, scanner.ScriptStandalone, true},
		{ScriptsGradle, scanner.ScriptGradle, true},
		{ScriptsGradle, scanner.ScriptStandalone, false},
		// Regular Kotlin files are never gated.
		{ScriptsNone, scanner.ScriptNone, true},
		{ScriptsGradle, scanner.ScriptNone, true},
	}
	for _, tc := range tests {
		if got := tc.scope.Applies(tc.kind); got != tc.want {
			t.Errorf("%v.Applies(%v) = %v, want %v", tc.scope, tc.kind, got, tc.want)
		}
	}
}

func TestRuleScriptScopeResolution(t *testing.T) {
	tests := []struct {
		name string
		rule *Rule
		want ScriptScope
	}{
		{"unknown category runs", &Rule{ID: "X", Category: "style"}, ScriptsAll},
		{"nil rule runs", nil, ScriptsAll},
		{"none category", &Rule{ID: "X", Category: "compose"}, ScriptsNone},
		{"override beats category", &Rule{ID: "MagicNumber", Category: "style"}, ScriptsNone},
		{"explicit field beats override", &Rule{ID: "MagicNumber", Category: "style", Scripts: ScriptsAll}, ScriptsAll},
		{"explicit field beats category", &Rule{ID: "X", Category: "compose", Scripts: ScriptsStandalone}, ScriptsStandalone},
	}
	for _, tc := range tests {
		if got := RuleScriptScope(tc.rule); got != tc.want {
			t.Errorf("%s: RuleScriptScope = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestScriptScopeStringRoundTrip(t *testing.T) {
	for scope, want := range map[ScriptScope]string{
		ScriptsDefault: "default", ScriptsAll: "all", ScriptsStandalone: "standalone", ScriptsGradle: "gradle", ScriptsNone: "none",
	} {
		if got := scope.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", scope, got, want)
		}
	}
}
