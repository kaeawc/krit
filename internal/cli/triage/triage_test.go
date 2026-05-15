package triage

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/output"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestTriageMaxEffort_filtersByEffort(t *testing.T) {
	classify := func(rule string) api.Effort {
		switch rule {
		case "Trivial":
			return api.EffortTrivial
		case "Local":
			return api.EffortLocal
		case "Refactor":
			return api.EffortRefactor
		case "Architectural":
			return api.EffortArchitectural
		}
		return api.EffortUnset
	}
	in := []output.JSONFinding{
		{Rule: "Trivial", RuleSet: "a"},
		{Rule: "Local", RuleSet: "a"},
		{Rule: "Refactor", RuleSet: "b"},
		{Rule: "Architectural", RuleSet: "b"},
	}

	cases := []struct {
		cap  api.Effort
		want []string
	}{
		{api.EffortTrivial, []string{"Trivial"}},
		{api.EffortLocal, []string{"Trivial", "Local"}},
		{api.EffortRefactor, []string{"Trivial", "Local", "Refactor"}},
		{api.EffortArchitectural, []string{"Trivial", "Local", "Refactor", "Architectural"}},
	}
	for _, c := range cases {
		got := FilterFindings(in, c.cap, classify)
		names := make([]string, len(got))
		for i, f := range got {
			names[i] = f.Rule
		}
		if !equal(names, c.want) {
			t.Errorf("cap=%v: got %v, want %v", c.cap, names, c.want)
		}
	}
}

// Findings carrying an explicit Effort field should be honored without
// consulting the classifier — this is how the field flows through from
// the JSON producer when teams archive reports for later triage.
func TestTriageMaxEffort_honorsExplicitEffort(t *testing.T) {
	classify := func(string) api.Effort { return api.EffortUnset }
	in := []output.JSONFinding{
		{Rule: "A", Effort: "trivial"},
		{Rule: "B", Effort: "refactor"},
		{Rule: "C"}, // no effort, unknown rule -> kept (conservative)
	}
	got := FilterFindings(in, api.EffortLocal, classify)
	if len(got) != 2 || got[0].Rule != "A" || got[1].Rule != "C" {
		t.Errorf("got %+v", got)
	}
}

// Smoke-test Run end-to-end via stdin/stdout to confirm the command
// rebuilds the summary after filtering.
func TestRun_filtersAndRebuildsSummary(t *testing.T) {
	report := output.JSONReport{
		Success: true, Version: "test",
		Findings: []output.JSONFinding{
			{Rule: "A", RuleSet: "x", Effort: "trivial", Fixable: true},
			{Rule: "B", RuleSet: "x", Effort: "refactor"},
		},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--max-effort=local"}, bytes.NewReader(body), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}

	var out output.JSONReport
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\nstdout=%s", err, stdout.String())
	}
	if len(out.Findings) != 1 || out.Findings[0].Rule != "A" {
		t.Fatalf("findings = %+v", out.Findings)
	}
	if out.Summary.Total != 1 || out.Summary.Fixable != 1 {
		t.Errorf("summary = %+v", out.Summary)
	}
}

func TestRun_rejectsUnknownEffort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--max-effort=bogus"}, strings.NewReader("{}"), &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for bogus effort")
	}
	if !strings.Contains(stderr.String(), "invalid --max-effort") {
		t.Errorf("stderr lacked diagnostic: %q", stderr.String())
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
