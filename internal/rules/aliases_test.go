package rules

import (
	"reflect"
	"sort"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestAllSuppressionAliases_IncludesRegisteredRule(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	api.Registry = append(append([]*api.Rule{}, saved...), &api.Rule{
		ID:          "TestAliasRule",
		Description: "synthetic rule for AllSuppressionAliases unit test",
		Aliases:     []string{"OldName", "AncientName"},
		Check:       func(*api.Context) {},
	})

	got := AllSuppressionAliases()
	aliases, ok := got["TestAliasRule"]
	if !ok {
		t.Fatalf("AllSuppressionAliases missing entry for TestAliasRule; got keys: %v", keys(got))
	}
	want := []string{"OldName", "AncientName"}
	if !reflect.DeepEqual(aliases, want) {
		t.Errorf("aliases for TestAliasRule = %v, want %v", aliases, want)
	}
}

func TestAllSuppressionAliases_DefensivelyCopiesSlice(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	original := []string{"Old"}
	api.Registry = append(append([]*api.Rule{}, saved...), &api.Rule{
		ID:          "TestAliasCopyRule",
		Description: "synthetic rule for slice-copy unit test",
		Aliases:     original,
		Check:       func(*api.Context) {},
	})

	got := AllSuppressionAliases()["TestAliasCopyRule"]
	got[0] = "Mutated"
	if original[0] != "Old" {
		t.Errorf("AllSuppressionAliases must defensively copy; original aliases mutated to %q", original[0])
	}
}

func TestAllSuppressionAliases_ReturnsNilWhenNoRulesDeclareAliases(t *testing.T) {
	saved := api.Registry
	t.Cleanup(func() { api.Registry = saved })

	// Strip aliases from every registered rule for the duration of this
	// test so we can verify the empty-result path.
	stripped := make([]*api.Rule, 0, len(saved))
	for _, r := range saved {
		if r == nil {
			continue
		}
		clone := *r
		clone.Aliases = nil
		stripped = append(stripped, &clone)
	}
	api.Registry = stripped

	if got := AllSuppressionAliases(); got != nil {
		t.Errorf("AllSuppressionAliases with no aliases declared = %v, want nil", got)
	}
}

func TestMetaForRuleIncludesAliasesFromRuleField(t *testing.T) {
	r := &api.Rule{
		ID:          "TestAliasMergeFromField",
		Description: "synthetic rule for alias merge",
		Aliases:     []string{"OldName"},
		Check:       func(*api.Context) {},
	}
	meta, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned !ok for non-nil rule")
	}
	if len(meta.Aliases) != 1 || meta.Aliases[0] != "OldName" {
		t.Errorf("descriptor aliases = %v, want [OldName]", meta.Aliases)
	}
}

func keys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
