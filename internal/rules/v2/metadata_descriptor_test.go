package v2

import "testing"

// fakeMetaRule is the minimal MetaProvider used to exercise descriptor-
// level helpers that don't need the full ApplyConfig runtime.
type fakeMetaRule struct {
	id       string
	ruleSet  string
	active   bool
	severity string
}

func (f *fakeMetaRule) Meta() RuleDescriptor {
	return RuleDescriptor{
		ID:            f.id,
		RuleSet:       f.ruleSet,
		Severity:      f.severity,
		Description:   "fake meta rule for tests",
		DefaultActive: f.active,
	}
}

// Test 8: MetaProvider smoke test — a fake rule type implements Meta()
// and DefaultInactiveSet returns the expected set for a single-rule
// slice. Kept in descriptor_test.go because it exercises the descriptor
// layer without touching the ApplyConfig runtime.
func TestDescriptor_MetaProviderSmoke(t *testing.T) {
	active := &fakeMetaRule{id: "ActiveRule", ruleSet: "fakes", severity: "warning", active: true}
	inactive := &fakeMetaRule{id: "InactiveRule", ruleSet: "fakes", severity: "warning", active: false}

	// Smoke-check MetaProvider interface satisfaction via a type assertion.
	var _ MetaProvider = active
	var _ MetaProvider = inactive

	got := DefaultInactiveSet([]RuleDescriptor{active.Meta(), inactive.Meta()})
	if len(got) != 1 {
		t.Fatalf("expected 1 inactive rule, got %d: %v", len(got), got)
	}
	if !got["InactiveRule"] {
		t.Fatalf("expected InactiveRule in the inactive set, got %v", got)
	}
	if got["ActiveRule"] {
		t.Fatalf("ActiveRule should not be in the inactive set, got %v", got)
	}
}

// Test 4: DefaultInactive derivation — given 10 descriptors where 3 have
// DefaultActive: false, DefaultInactiveSet returns exactly those 3 names.
func TestDescriptor_DefaultInactiveDerivation(t *testing.T) {
	descs := make([]RuleDescriptor, 0, 10)
	for i := 0; i < 10; i++ {
		d := RuleDescriptor{
			ID:            ruleName(i),
			RuleSet:       "bulk",
			Severity:      "warning",
			Description:   "bulk test rule",
			DefaultActive: true,
		}
		// Mark 3 of them inactive.
		if i == 2 || i == 5 || i == 7 {
			d.DefaultActive = false
		}
		descs = append(descs, d)
	}

	got := DefaultInactiveSet(descs)
	if len(got) != 3 {
		t.Fatalf("expected 3 inactive rules, got %d: %v", len(got), got)
	}
	for _, want := range []string{"rule2", "rule5", "rule7"} {
		if !got[want] {
			t.Errorf("expected %q in inactive set, got %v", want, got)
		}
	}
}

func TestOptionType_String(t *testing.T) {
	cases := []struct {
		t    OptionType
		want string
	}{
		{OptInt, "int"},
		{OptBool, "bool"},
		{OptString, "string"},
		{OptStringList, "string[]"},
		{OptRegex, "regex"},
		{OptionType(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("OptionType(%d).String() = %q, want %q", tc.t, got, tc.want)
		}
	}
}

func ruleName(i int) string {
	// Keep names short and predictable so the test assertions stay readable.
	switch i {
	case 0:
		return "rule0"
	case 1:
		return "rule1"
	case 2:
		return "rule2"
	case 3:
		return "rule3"
	case 4:
		return "rule4"
	case 5:
		return "rule5"
	case 6:
		return "rule6"
	case 7:
		return "rule7"
	case 8:
		return "rule8"
	case 9:
		return "rule9"
	}
	return "ruleX"
}
