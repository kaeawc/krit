package rules

import (
	"reflect"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestLanguageSupportFilterMatches(t *testing.T) {
	supported := mustV2RuleByID(t, "AddJavascriptInterface") // explicit supported
	longMethod := mustV2RuleByID(t, "LongMethod")            // ruleset default = pending
	composeRule := findRuleForCategory(t, "compose")         // ruleset default = not-applicable

	cases := []struct {
		name   string
		filter LanguageSupportFilter
		rule   *api.Rule
		want   bool
	}{
		{
			name:   "zero filter matches every rule",
			filter: LanguageSupportFilter{},
			rule:   supported,
			want:   true,
		},
		{
			name:   "language only matches when classification exists",
			filter: LanguageSupportFilter{Language: "java"},
			rule:   supported,
			want:   true,
		},
		{
			name:   "matching status passes",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportSupported}},
			rule:   supported,
			want:   true,
		},
		{
			name:   "non-matching status drops",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportPartial}},
			rule:   supported,
			want:   false,
		},
		{
			name:   "ruleset default classifies pending rules",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportPending}},
			rule:   longMethod,
			want:   true,
		},
		{
			name:   "negation flips match",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportSupported}, Negate: true},
			rule:   supported,
			want:   false,
		},
		{
			name:   "negation passes when status doesn't match",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportSupported}, Negate: true},
			rule:   longMethod,
			want:   true,
		},
		{
			name:   "not-applicable status reachable via ruleset default",
			filter: LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportNotApplicable}},
			rule:   composeRule,
			want:   true,
		},
		{
			name:   "unknown language returns false unless negated",
			filter: LanguageSupportFilter{Language: "scala"},
			rule:   supported,
			want:   false,
		},
		{
			name:   "unknown language with negate passes through",
			filter: LanguageSupportFilter{Language: "scala", Negate: true},
			rule:   supported,
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.filter.Matches(tc.rule)
			if got != tc.want {
				t.Fatalf("Matches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLanguageSupportFilterValidate(t *testing.T) {
	if err := (LanguageSupportFilter{}).Validate(); err != nil {
		t.Fatalf("zero filter should validate, got %v", err)
	}
	if err := (LanguageSupportFilter{Status: []api.LanguageSupportStatus{api.LanguageSupportSupported}}).Validate(); err == nil {
		t.Fatal("status without language should error")
	}
	if err := (LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{"bogus"}}).Validate(); err == nil {
		t.Fatal("unknown status should error")
	}
	if err := (LanguageSupportFilter{Language: "java", Status: []api.LanguageSupportStatus{api.LanguageSupportSupported, api.LanguageSupportPartial}}).Validate(); err != nil {
		t.Fatalf("valid filter should pass, got %v", err)
	}
}

func TestParseStatusFilter(t *testing.T) {
	cases := []struct {
		in         string
		want       []api.LanguageSupportStatus
		wantNegate bool
		wantErr    bool
	}{
		{in: "", want: nil},
		{in: "supported", want: []api.LanguageSupportStatus{api.LanguageSupportSupported}},
		{in: "supported,partial", want: []api.LanguageSupportStatus{api.LanguageSupportPartial, api.LanguageSupportSupported}},
		{in: "!supported", want: []api.LanguageSupportStatus{api.LanguageSupportSupported}, wantNegate: true},
		{in: " ! supported , partial ", want: []api.LanguageSupportStatus{api.LanguageSupportPartial, api.LanguageSupportSupported}, wantNegate: true},
		{in: "!", wantErr: true},
		{in: "bogus", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, neg, err := ParseStatusFilter(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if neg != tc.wantNegate {
				t.Fatalf("negate = %v, want %v", neg, tc.wantNegate)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("statuses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterRegistry(t *testing.T) {
	all := FilterRegistry(api.Registry, LanguageSupportFilter{})
	if len(all) == 0 {
		t.Fatal("zero filter should return the full registry")
	}
	supported := FilterRegistry(api.Registry, LanguageSupportFilter{
		Language: "java",
		Status:   []api.LanguageSupportStatus{api.LanguageSupportSupported},
	})
	if len(supported) == 0 {
		t.Fatal("expected at least one rule with java=supported")
	}
	for _, r := range supported {
		s, _ := JavaSupportForRule(r)
		if s.Status != api.LanguageSupportSupported {
			t.Fatalf("rule %s leaked through java=supported filter (status=%s)", r.ID, s.Status)
		}
	}
}

func findRuleForCategory(t *testing.T, category string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r != nil && r.Category == category {
			return r
		}
	}
	t.Fatalf("no rule found in category %s", category)
	return nil
}
