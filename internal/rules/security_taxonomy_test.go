package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestSecurityRulesHaveCWE enforces the issue #190 evaluation
// criterion: every existing security-category rule carries at least
// one CWE identifier on its Security taxonomy.
func TestSecurityRulesHaveCWE(t *testing.T) {
	var missing []string
	for _, r := range api.Registry {
		if r.Category != "security" {
			continue
		}
		if r.Security == nil || len(r.Security.CWE) == 0 {
			missing = append(missing, r.ID)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("security-category rules missing CWE taxonomy: %v", missing)
	}
}

func TestTaxonomyMatcher(t *testing.T) {
	tax := &api.SecurityTaxonomy{
		CWE:   []string{"CWE-89"},
		OWASP: []string{"A03:2021-Injection"},
	}
	if !tax.HasCWE("cwe-89") {
		t.Fatal("HasCWE should be case-insensitive")
	}
	m := api.TaxonomyMatcher{IDs: []string{"CWE-89"}}
	if !m.Matches(tax) {
		t.Fatal("TaxonomyMatcher should match CWE-89")
	}
	if m.Matches(nil) {
		t.Fatal("TaxonomyMatcher should not match nil taxonomy")
	}
	empty := api.TaxonomyMatcher{}
	if empty.Matches(tax) {
		t.Fatal("empty matcher should match nothing")
	}
}

func TestMetaForRuleSurfacesSecurity(t *testing.T) {
	for _, r := range api.Registry {
		if r.ID != "SqlInjectionRawQuery" {
			continue
		}
		desc, ok := MetaForRule(r)
		if !ok {
			t.Fatal("MetaForRule returned !ok for SqlInjectionRawQuery")
		}
		if desc.Security == nil || !desc.Security.HasCWE("CWE-89") {
			t.Fatalf("expected SqlInjectionRawQuery descriptor to carry CWE-89, got %+v", desc.Security)
		}
		return
	}
	t.Fatal("SqlInjectionRawQuery rule not found in registry")
}
