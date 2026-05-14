package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestV2RulePrecision_Override verifies that an explicit Rule.Precision
// wins over the derived classification.
func TestV2RulePrecision_Override(t *testing.T) {
	// Derived precision for a Needs-Resolver rule is PrecisionTypeAware;
	// the explicit override should flip it to PrecisionPolicy.
	r := &api.Rule{
		ID:          "OverrideRule",
		Category:    "test",
		Description: "override test",
		Sev:         api.SeverityWarning,
		Needs:       api.NeedsResolver,
		Check:       func(*api.Context) {},
		Precision:   api.PrecisionPolicy,
	}
	if got := V2RulePrecision(r); got != api.PrecisionPolicy {
		t.Fatalf("override ignored: V2RulePrecision = %q, want %q", got, api.PrecisionPolicy)
	}

	// Sanity: without the override, the same rule shape derives type-aware.
	r.Precision = api.PrecisionUnset
	if got := V2RulePrecision(r); got != api.PrecisionTypeAware {
		t.Fatalf("derivation regressed: V2RulePrecision = %q, want %q", got, api.PrecisionTypeAware)
	}
}

// stubPrecisionProvider exercises the PrecisionProvider fallback path.
type stubPrecisionProvider struct{ p api.Precision }

func (s stubPrecisionProvider) Precision() api.Precision { return s.p }

// TestV2RulePrecision_ProviderFallback verifies a PrecisionProvider
// implementation on Rule.Implementation is consulted before the
// Needs/NodeTypes derivation.
func TestV2RulePrecision_ProviderFallback(t *testing.T) {
	r := &api.Rule{
		ID:             "ProviderRule",
		Category:       "test",
		Description:    "provider test",
		Sev:            api.SeverityWarning,
		NodeTypes:      []string{"call_expression"},
		Check:          func(*api.Context) {},
		Implementation: stubPrecisionProvider{p: api.PrecisionPolicy},
	}
	if got := V2RulePrecision(r); got != api.PrecisionPolicy {
		t.Fatalf("provider ignored: V2RulePrecision = %q, want %q", got, api.PrecisionPolicy)
	}
}

// TestMetaForRule_PrecisionFallback verifies that MetaForRule emits the
// derived tier when the rule has no explicit Precision set.
func TestMetaForRule_PrecisionFallback(t *testing.T) {
	r := &api.Rule{
		ID:          "FallbackRule",
		Category:    "test",
		Description: "fallback test",
		Sev:         api.SeverityWarning,
		NodeTypes:   []string{"call_expression"},
		Check:       func(*api.Context) {},
	}
	desc, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned ok=false")
	}
	if desc.Precision != api.PrecisionASTBacked {
		t.Fatalf("fallback descriptor precision = %q, want %q",
			desc.Precision, api.PrecisionASTBacked)
	}
}

// TestMetaForRule_PrecisionExplicit verifies that an explicit
// Rule.Precision is mirrored on the descriptor without re-deriving.
func TestMetaForRule_PrecisionExplicit(t *testing.T) {
	r := &api.Rule{
		ID:          "ExplicitRule",
		Category:    "test",
		Description: "explicit test",
		Sev:         api.SeverityWarning,
		NodeTypes:   []string{"call_expression"},
		Check:       func(*api.Context) {},
		Precision:   api.PrecisionPolicy,
	}
	desc, ok := MetaForRule(r)
	if !ok {
		t.Fatal("MetaForRule returned ok=false")
	}
	if desc.Precision != api.PrecisionPolicy {
		t.Fatalf("explicit descriptor precision = %q, want %q",
			desc.Precision, api.PrecisionPolicy)
	}
}

// TestMetaForRule_PrecisionAlwaysSet verifies that every registered
// rule has a non-zero precision once MetaForRule has resolved it. This
// is the no-zero-leakage invariant from the issue's evaluation criteria.
func TestMetaForRule_PrecisionAlwaysSet(t *testing.T) {
	for _, r := range api.Registry {
		desc, ok := MetaForRule(r)
		if !ok {
			t.Fatalf("MetaForRule(%s) returned ok=false", r.ID)
		}
		if desc.Precision == api.PrecisionUnset {
			t.Fatalf("rule %s: descriptor precision is PrecisionUnset", r.ID)
		}
	}
}
