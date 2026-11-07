package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func aliasListHas(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestSuppressionAliasesCoverAndroidLintIDs(t *testing.T) {
	aliases := AllSuppressionAliases()
	checked := 0
	for _, r := range api.Registry {
		for _, suffix := range []string{"Resource", "Manifest"} {
			trimmed := strings.TrimSuffix(r.ID, suffix)
			if trimmed == r.ID || !knownLintIssueIDs[trimmed] {
				continue
			}
			checked++
			if !aliasListHas(aliases[r.ID], trimmed) {
				t.Errorf("%s: missing Android lint alias %s, got %v", r.ID, trimmed, aliases[r.ID])
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one Resource or Manifest rule mapped to a known lint ID")
	}
	for id, want := range map[string]string{
		"DisableBaselineAlignmentResource":   "DisableBaselineAlignment",
		"ClickableViewAccessibilityResource": "ClickableViewAccessibility",
		"RecyclerAdapterWithoutDiffUtil":     "NotifyDataSetChanged",
		"LayoutInflation":                    "InflateParams",
	} {
		if !aliasListHas(aliases[id], want) {
			t.Errorf("%s: want alias %s, got %v", id, want, aliases[id])
		}
	}
}

func TestResourceToolsIgnoreHonoursAndroidLintID(t *testing.T) {
	v := &android.View{Attributes: map[string]string{"tools:ignore": "DisableBaselineAlignment, ClickableViewAccessibility"}}
	for _, id := range []string{"DisableBaselineAlignmentResource", "ClickableViewAccessibilityResource"} {
		if !resourceToolsIgnoreMatches(v, id) {
			t.Errorf("tools:ignore with the lint ID must match %s", id)
		}
	}
	if resourceToolsIgnoreMatches(v, "InefficientWeightResource") {
		t.Error("an unrelated rule must not match")
	}
}
