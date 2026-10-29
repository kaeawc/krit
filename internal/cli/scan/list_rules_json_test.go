package scan

import (
	"bytes"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestListRulesJSONMatchesRegistry checks the --list-rules --format json
// document: one entry per registered rule, with the fields editor
// integrations read (the IntelliJ plugin builds per-rule inspections
// from it).
func TestListRulesJSONMatchesRegistry(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code, ok := PrintListRules(&stdout, &stderr, false, "", "", "json", nil, nil); !ok {
		t.Fatalf("PrintListRules failed: code=%d stderr=%q", code, stderr.String())
	}
	var doc ListedRules
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if len(doc.Rules) != len(api.Registry) {
		t.Fatalf("got %d rules, registry has %d", len(doc.Rules), len(api.Registry))
	}
	byID := make(map[string]ListedRule, len(doc.Rules))
	for _, r := range doc.Rules {
		if r.ID == "" || r.Category == "" || r.Severity == "" {
			t.Fatalf("incomplete entry: %+v", r)
		}
		byID[r.ID] = r
	}
	for _, r := range api.Registry {
		got, ok := byID[r.ID]
		if !ok {
			t.Fatalf("rule %s missing from JSON list", r.ID)
		}
		if got.DefaultActive != rules.IsDefaultActive(r.ID) {
			t.Errorf("%s: defaultActive=%v, want %v", r.ID, got.DefaultActive, rules.IsDefaultActive(r.ID))
		}
		lvl, fixable := rules.GetV2FixLevel(r)
		if got.Fixable != fixable || (fixable && got.FixLevel != lvl.String()) {
			t.Errorf("%s: fixable=%v fixLevel=%q, want %v %q", r.ID, got.Fixable, got.FixLevel, fixable, lvl.String())
		}
	}
}

// TestListRulesJSONHonorsTaxonomyFilter: --cwe narrows the JSON list the
// same way it narrows the text table.
func TestListRulesJSONHonorsTaxonomyFilter(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if _, ok := PrintListRules(&stdout, &stderr, false, "", "CWE-89", "json", nil, nil); !ok {
		t.Fatalf("PrintListRules failed: %q", stderr.String())
	}
	var doc ListedRules
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if len(doc.Rules) >= len(api.Registry) {
		t.Fatalf("expected the CWE filter to narrow the list, got %d rules", len(doc.Rules))
	}
}

// TestListRulesFormatNeedsExplicitFlag: --format defaults to json for
// scans, so only an explicit -f/--format json switches --list-rules to
// JSON; the bare flag keeps the text table.
func TestListRulesFormatNeedsExplicitFlag(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--list-rules"}, ""},
		{[]string{"--list-rules", "--format", "json"}, "json"},
		{[]string{"--list-rules", "-f", "json"}, "json"},
		{[]string{"--list-rules", "--format", "plain"}, ""},
	}
	for _, tc := range cases {
		fs := flag.NewFlagSet("krit", flag.ContinueOnError)
		f := registerScanFlags(fs)
		if err := fs.Parse(tc.args); err != nil {
			t.Fatalf("parse %v: %v", tc.args, err)
		}
		if got := listRulesFormat(f, fs); got != tc.want {
			t.Errorf("%s: listRulesFormat=%q, want %q", strings.Join(tc.args, " "), got, tc.want)
		}
	}
}

// TestListRulesJSONReportsDeclaredDefault: a krit.yml that activates an
// opt-in rule must not change the defaultActive the JSON list reports.
// Editor integrations cache the list per binary, so it has to describe
// the binary, not the project config loaded before the listing.
func TestListRulesJSONReportsDeclaredDefault(t *testing.T) {
	var optIn *api.Rule
	for _, r := range api.Registry {
		if r.Maturity == api.MaturityStable && !rules.IsDefaultActive(r.ID) && r.Category != "" {
			optIn = r
			break
		}
	}
	if optIn == nil {
		t.Skip("no stable opt-in rule registered")
	}
	cfg := config.NewConfigFromData(map[string]interface{}{
		optIn.Category: map[string]interface{}{
			optIn.ID: map[string]interface{}{"active": true},
		},
	})
	rules.ApplyConfig(cfg)
	t.Cleanup(func() { rules.ApplyConfig(config.NewConfig()) })
	if !rules.IsDefaultActive(optIn.ID) {
		t.Fatalf("test setup: config did not activate %s", optIn.ID)
	}

	var stdout, stderr bytes.Buffer
	if _, ok := PrintListRules(&stdout, &stderr, false, "", "", "json", nil, nil); !ok {
		t.Fatalf("PrintListRules failed: %q", stderr.String())
	}
	var doc ListedRules
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	for _, r := range doc.Rules {
		if r.ID == optIn.ID {
			if r.DefaultActive {
				t.Fatalf("%s: defaultActive=true after a config activation; want the declared default (false)", r.ID)
			}
			return
		}
	}
	t.Fatalf("%s missing from the JSON list", optIn.ID)
}

// TestFilterPluginRules: plugin descriptors have no taxonomy data, so a
// --cwe filter drops them; --maturity keeps the matching ones.
func TestFilterPluginRules(t *testing.T) {
	in := []oracle.PluginRuleDescriptor{
		{RuleID: "Stable", Maturity: "stable"},
		{RuleID: "Exp", Maturity: "experimental"},
		{RuleID: "Unset"},
	}
	if got := filterPluginRules(in, false, nil); len(got) != 3 {
		t.Fatalf("no filter: got %d rules, want 3", len(got))
	}
	if got := filterPluginRules(in, true, nil); len(got) != 0 {
		t.Fatalf("taxonomy filter: got %v, want none", got)
	}
	exp := api.MaturityExperimental
	if got := filterPluginRules(in, false, &exp); len(got) != 1 || got[0].RuleID != "Exp" {
		t.Fatalf("maturity=experimental: got %v", got)
	}
	stable := api.MaturityStable
	if got := filterPluginRules(in, false, &stable); len(got) != 2 {
		t.Fatalf("maturity=stable: got %v, want Stable and Unset", got)
	}
}
