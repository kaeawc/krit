package scan

import (
	"bytes"
	"encoding/json"
	"flag"
	"strings"
	"testing"

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
