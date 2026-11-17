package scan

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

func TestSelectScanRulesAppliesScopeFlags(t *testing.T) {
	all, _, _ := selectScanRules(freshScanFlags(t), nil)

	fileOnly := freshScanFlags(t)
	*fileOnly.OnlyScope = "file"
	fileRules, _, _ := selectScanRules(fileOnly, nil)

	projectOnly := freshScanFlags(t)
	*projectOnly.OnlyScope = "project"
	projectRules, _, _ := selectScanRules(projectOnly, nil)

	if len(fileRules) == 0 || len(projectRules) == 0 {
		t.Fatalf("file=%d project=%d rules, want both non-empty", len(fileRules), len(projectRules))
	}
	if len(fileRules)+len(projectRules) != len(all) {
		t.Errorf("file %d + project %d != default %d", len(fileRules), len(projectRules), len(all))
	}
	for _, r := range fileRules {
		if rules.ScopeGroup(r) != rules.ScopeGroupFile {
			t.Errorf("--only-scope=file kept %s (%s)", r.ID, rules.ScopeGroup(r))
		}
	}

	excluded := freshScanFlags(t)
	*excluded.ExcludeScope = "project"
	if got, _, _ := selectScanRules(excluded, nil); len(got) != len(fileRules) {
		t.Errorf("--exclude-scope=project kept %d rules, want %d", len(got), len(fileRules))
	}
}

func TestParseScopeFlagsRejectsUnknownScope(t *testing.T) {
	f := freshScanFlags(t)
	*f.ExcludeScope = "module"
	if _, _, err := parseScopeFlags(f); err == nil || !strings.Contains(err.Error(), "--exclude-scope") {
		t.Fatalf("parseScopeFlags error = %v, want an --exclude-scope error", err)
	}
}

// The analyze-project wire carries no rule filters, so a scope- or
// cost-filtered scan must stay in-process.
func TestRuleFiltersForceInProcess(t *testing.T) {
	for _, set := range []func(*scanFlags){
		func(f *scanFlags) { *f.OnlyScope = "file" },
		func(f *scanFlags) { *f.ExcludeScope = "project" },
		func(f *scanFlags) { *f.MaxCost = "ast" },
	} {
		f := freshScanFlags(t)
		if !daemonCompatibleFlags(f) {
			t.Fatal("baseline flags should be daemon compatible")
		}
		set(f)
		if daemonCompatibleFlags(f) {
			t.Errorf("rule filter flags routed to the daemon")
		}
	}
}
