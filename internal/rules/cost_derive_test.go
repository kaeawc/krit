package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestDeriveCost(t *testing.T) {
	cases := []struct {
		name string
		rule *api.Rule
		want api.Cost
	}{
		{
			name: "java facts beats every other bit",
			rule: &api.Rule{
				Needs:     api.NeedsCrossFile | api.NeedsOracleCallTargets,
				JavaFacts: &api.JavaFactProfile{Annotations: []string{"java.lang.Deprecated"}},
			},
			want: api.CostFIR,
		},
		{
			name: "narrow oracle bit promotes to oracle",
			rule: &api.Rule{Needs: api.NeedsOracleSupertypes},
			want: api.CostOracle,
		},
		{
			name: "umbrella oracle bits promote to oracle",
			rule: &api.Rule{Needs: api.NeedsOracle},
			want: api.CostOracle,
		},
		{
			name: "type info required promotes to oracle",
			rule: &api.Rule{TypeInfo: api.TypeInfoHint{Required: true}},
			want: api.CostOracle,
		},
		{
			name: "cross-file index requires project phase",
			rule: &api.Rule{Needs: api.NeedsCrossFile},
			want: api.CostCrossFile,
		},
		{
			name: "module index requires project phase",
			rule: &api.Rule{Needs: api.NeedsModuleIndex},
			want: api.CostCrossFile,
		},
		{
			name: "parsed files counts as cross-file",
			rule: &api.Rule{Needs: api.NeedsParsedFiles},
			want: api.CostCrossFile,
		},
		{
			name: "aggregate scope counts as cross-file",
			rule: &api.Rule{Needs: api.NeedsAggregate},
			want: api.CostCrossFile,
		},
		{
			name: "line pass tier",
			rule: &api.Rule{Needs: api.NeedsLinePass},
			want: api.CostLine,
		},
		{
			name: "manifest rule treated as AST tier",
			rule: &api.Rule{Needs: api.NeedsManifest},
			want: api.CostAST,
		},
		{
			name: "resource rule treated as AST tier",
			rule: &api.Rule{Needs: api.NeedsResources},
			want: api.CostAST,
		},
		{
			name: "gradle rule treated as AST tier",
			rule: &api.Rule{Needs: api.NeedsGradle},
			want: api.CostAST,
		},
		{
			name: "ast-only rule with NodeTypes",
			rule: &api.Rule{NodeTypes: []string{"call_expression"}},
			want: api.CostAST,
		},
		{
			name: "rule with no bits defaults to AST",
			rule: &api.Rule{},
			want: api.CostAST,
		},
		{
			name: "resolver aspect does not promote past AST",
			rule: &api.Rule{Needs: api.NeedsResolver},
			want: api.CostAST,
		},
		{
			name: "concurrent aspect does not promote past AST",
			rule: &api.Rule{Needs: api.NeedsConcurrent},
			want: api.CostAST,
		},
		{
			name: "cross-file plus oracle picks the higher tier",
			rule: &api.Rule{Needs: api.NeedsCrossFile | api.NeedsOracleSupertypes},
			want: api.CostOracle,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveCost(tc.rule)
			if got != tc.want {
				t.Fatalf("DeriveCost = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestCostFor_ExplicitCostWins(t *testing.T) {
	r := &api.Rule{
		Cost:      api.CostTrivial,
		Needs:     api.NeedsCrossFile,
		JavaFacts: &api.JavaFactProfile{},
	}
	if got := CostFor(r); got != api.CostTrivial {
		t.Fatalf("CostFor = %s, want %s (explicit Cost should win)", got, api.CostTrivial)
	}
}

func TestCostFor_NilRule(t *testing.T) {
	if got := CostFor(nil); got != api.CostUnset {
		t.Fatalf("CostFor(nil) = %s, want unset", got)
	}
	if got := DeriveCost(nil); got != api.CostUnset {
		t.Fatalf("DeriveCost(nil) = %s, want unset", got)
	}
}

func TestParseCostAndPresets(t *testing.T) {
	cases := map[string]api.Cost{
		"trivial":    api.CostTrivial,
		"line":       api.CostLine,
		"ast":        api.CostAST,
		"AST":        api.CostAST,
		"fast":       api.CostAST,
		"crossfile":  api.CostCrossFile,
		"cross-file": api.CostCrossFile,
		"cross_file": api.CostCrossFile,
		"balanced":   api.CostCrossFile,
		"oracle":     api.CostOracle,
		"fir":        api.CostFIR,
		"thorough":   api.CostFIR,
	}
	for in, want := range cases {
		got, ok := api.ParseCost(in)
		if !ok {
			t.Errorf("ParseCost(%q) returned !ok", in)
			continue
		}
		if got != want {
			t.Errorf("ParseCost(%q) = %s, want %s", in, got, want)
		}
	}
	if _, ok := api.ParseCost("nonsense"); ok {
		t.Errorf("ParseCost(nonsense) should not parse")
	}
}

func TestFilterByMaxCost(t *testing.T) {
	rules := []*api.Rule{
		{ID: "line", Needs: api.NeedsLinePass},
		{ID: "ast", NodeTypes: []string{"call_expression"}},
		{ID: "crossfile", Needs: api.NeedsCrossFile},
		{ID: "oracle", Needs: api.NeedsOracleSupertypes},
		{ID: "fir", JavaFacts: &api.JavaFactProfile{}},
		{ID: "explicit-trivial", Cost: api.CostTrivial, Needs: api.NeedsCrossFile},
	}

	cases := []struct {
		max  api.Cost
		want []string
	}{
		{api.CostUnset, []string{"line", "ast", "crossfile", "oracle", "fir", "explicit-trivial"}},
		{api.CostTrivial, []string{"explicit-trivial"}},
		{api.CostLine, []string{"line", "explicit-trivial"}},
		{api.CostAST, []string{"line", "ast", "explicit-trivial"}},
		{api.CostCrossFile, []string{"line", "ast", "crossfile", "explicit-trivial"}},
		{api.CostOracle, []string{"line", "ast", "crossfile", "oracle", "explicit-trivial"}},
		{api.CostFIR, []string{"line", "ast", "crossfile", "oracle", "fir", "explicit-trivial"}},
	}
	for _, tc := range cases {
		t.Run(tc.max.String(), func(t *testing.T) {
			got := FilterByMaxCost(rules, tc.max)
			ids := make([]string, 0, len(got))
			for _, r := range got {
				ids = append(ids, r.ID)
			}
			if !sameStringSet(ids, tc.want) {
				t.Fatalf("FilterByMaxCost(%s) = %v, want %v", tc.max, ids, tc.want)
			}
		})
	}
}

func TestSortedRuleExecutionStatsByCost(t *testing.T) {
	stats := RunStats{
		RuleStatsByRule: map[string]RuleExecutionStat{
			"oracleRule":    {Rule: "oracleRule", Cost: "oracle", DurationNs: 10},
			"astFast":       {Rule: "astFast", Cost: "ast", DurationNs: 5},
			"astSlow":       {Rule: "astSlow", Cost: "ast", DurationNs: 100},
			"lineRule":      {Rule: "lineRule", Cost: "line", DurationNs: 7},
			"crossFileRule": {Rule: "crossFileRule", Cost: "crossfile", DurationNs: 9},
		},
	}
	rows := SortedRuleExecutionStatsByCost(stats)
	wantOrder := []string{"lineRule", "astSlow", "astFast", "crossFileRule", "oracleRule"}
	if len(rows) != len(wantOrder) {
		t.Fatalf("got %d rows, want %d", len(rows), len(wantOrder))
	}
	for i, want := range wantOrder {
		if rows[i].Rule != want {
			ids := make([]string, len(rows))
			for j, r := range rows {
				ids[j] = r.Rule
			}
			t.Errorf("rows[%d].Rule = %s, want %s (full order: %v)", i, rows[i].Rule, want, ids)
		}
	}
}

func TestRegisteredRulesAllHaveResolvedCost(t *testing.T) {
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		if got := CostFor(r); got == api.CostUnset {
			t.Errorf("rule %s has unresolved cost", r.ID)
		}
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}
