package mcp

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestResolveRelatedRules_NilRule(t *testing.T) {
	if got := resolveRelatedRules(nil); got != nil {
		t.Fatalf("resolveRelatedRules(nil) = %v; want nil", got)
	}
}

func TestResolveRelatedRules_EmptyList(t *testing.T) {
	r := &api.Rule{ID: "Solo"}
	if got := resolveRelatedRules(r); got != nil {
		t.Fatalf("resolveRelatedRules with empty RelatedRules = %v; want nil", got)
	}
}

func TestResolveRelatedRules_ResolvesAgainstRegistry(t *testing.T) {
	// Pick two real registered rules so findRule can return them.
	if len(api.Registry) < 2 {
		t.Skip("registry has fewer than 2 rules; cannot exercise resolveRelatedRules cross-link")
	}
	first := api.Registry[0].ID
	second := api.Registry[1].ID

	r := &api.Rule{ID: "synthetic", RelatedRules: []string{first, "Ghost-Rule-Should-Not-Exist", second}}
	got := resolveRelatedRules(r)
	want := []string{first, second}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("resolveRelatedRules = %v; want %v (unknown IDs filtered)", got, want)
	}
}
