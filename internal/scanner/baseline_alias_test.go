package scanner

import "testing"

func TestFilterByBaselineMatchesFormerRuleIDs(t *testing.T) {
	saved := BaselineRuleAliases
	t.Cleanup(func() { BaselineRuleAliases = saved })
	BaselineRuleAliases = func(rule string) []string {
		if rule == "NewRule" {
			return []string{"OldRule"}
		}
		return nil
	}
	finding := Finding{File: "/project/app/src/main/java/A.kt", Line: 3, Col: 5, Rule: "NewRule", Message: "msg"}
	old := finding
	old.Rule = "OldRule"
	baseline := &Baseline{ManuallySuppressed: map[string]bool{}, CurrentIssues: map[string]bool{BaselineID(old, "", ""): true}}
	if got := FilterByBaseline([]Finding{finding}, baseline, "/project"); len(got) != 0 {
		t.Fatalf("a baseline entry under the former rule ID did not silence the finding: %v", got)
	}
	other := finding
	other.Message = "different"
	if got := FilterByBaseline([]Finding{other}, baseline, "/project"); len(got) != 1 {
		t.Fatalf("the former-ID entry silenced a finding with another message: %v", got)
	}
	BaselineRuleAliases = nil
	if got := FilterByBaseline([]Finding{finding}, baseline, "/project"); len(got) != 1 {
		t.Fatalf("without aliases the former-ID entry must not match: %v", got)
	}
}
