package extension

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegister_AddsToRegistry confirms the public Register entrypoint
// joins api.Registry on equal footing with built-in rules.
func TestRegister_AddsToRegistry(t *testing.T) {
	before := len(api.Registry)
	defer restoreRegistry(before)

	r := &Rule{
		ID:          "ExtensionTestRule",
		Category:    "test",
		Description: "test rule registered via the public extension package",
		Sev:         SeverityWarning,
		Maturity:    MaturityExperimental,
		Check:       func(*Context) {},
	}
	Register(r)

	if len(api.Registry) != before+1 {
		t.Fatalf("registry length = %d, want %d", len(api.Registry), before+1)
	}
	got := api.Registry[len(api.Registry)-1]
	if got.ID != "ExtensionTestRule" {
		t.Errorf("registered rule ID = %q, want ExtensionTestRule", got.ID)
	}
	if got.Maturity != MaturityExperimental {
		t.Errorf("registered rule Maturity = %v, want experimental", got.Maturity)
	}
}

// TestRegisterAll_SkipsNil confirms RegisterAll accepts a slice with
// nil entries without panicking and registers only the real ones.
func TestRegisterAll_SkipsNil(t *testing.T) {
	before := len(api.Registry)
	defer restoreRegistry(before)

	RegisterAll([]*Rule{
		nil,
		{
			ID:          "ExtensionBatchRuleA",
			Description: "batch a",
			Check:       func(*Context) {},
		},
		nil,
		{
			ID:          "ExtensionBatchRuleB",
			Description: "batch b",
			Check:       func(*Context) {},
		},
	})

	if len(api.Registry) != before+2 {
		t.Fatalf("registry length = %d, want %d", len(api.Registry), before+2)
	}
}

// TestSeverityAndFixLevelAliases confirms the type aliases pass
// through to api so external users can compare values directly.
func TestSeverityAndFixLevelAliases(t *testing.T) {
	if SeverityError != api.SeverityError {
		t.Error("SeverityError alias drift")
	}
	if FixIdiomatic != api.FixIdiomatic {
		t.Error("FixIdiomatic alias drift")
	}
	if MaturityDeprecated != api.MaturityDeprecated {
		t.Error("MaturityDeprecated alias drift")
	}
}

// restoreRegistry trims api.Registry back to the supplied length so
// tests do not pollute later runs that count rules.
func restoreRegistry(n int) {
	if len(api.Registry) > n {
		api.Registry = api.Registry[:n]
	}
}
