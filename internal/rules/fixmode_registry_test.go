package rules

import (
	"errors"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRegistryFixModeIsValid is the CI gate for the mutual-exclusion
// invariant between autofix and suggested fixes across the entire
// built-in registry. api.Register already panics on invalid
// declarations, but a fresh registration error in an init() only
// surfaces when a downstream importer loads the package; walking
// api.Registry here makes the check explicit and gives a clearer
// failure mode than a panic at first import.
//
// Companion to unit-level coverage in internal/rules/api/fixmode_test.go.
// See docs/suggested-fixes.md for the rule-author contract.
func TestRegistryFixModeIsValid(t *testing.T) {
	if len(api.Registry) == 0 {
		t.Fatal("api.Registry is empty; rule packages not loaded")
	}
	for _, r := range api.Registry {
		err := r.ValidateFixMode()
		if err == nil {
			continue
		}
		var fme *api.FixModeError
		if !errors.As(err, &fme) {
			t.Errorf("rule %s: ValidateFixMode returned non-FixModeError %T: %v",
				r.ID, err, err)
			continue
		}
		t.Errorf("rule %s: %v", r.ID, err)
	}
}

// TestRegistryFixModeIsObservable exercises FixMode() on every
// registered rule so we have one place that fails loudly if FixMode
// ever drifts away from the Fix / SuggestedFixes invariants — e.g. a
// refactor that changes the resolution order between the two slots.
func TestRegistryFixModeIsObservable(t *testing.T) {
	if len(api.Registry) == 0 {
		t.Fatal("api.Registry is empty; rule packages not loaded")
	}
	var autofix, suggested, none int
	for _, r := range api.Registry {
		got := r.FixMode()
		switch {
		case len(r.SuggestedFixes) > 0:
			if got != api.FixModeSuggested {
				t.Errorf("rule %s: FixMode() = %v, want FixModeSuggested", r.ID, got)
			}
			suggested++
		case r.Fix != api.FixNone:
			if got != api.FixModeAutofix {
				t.Errorf("rule %s: FixMode() = %v, want FixModeAutofix", r.ID, got)
			}
			autofix++
		default:
			if got != api.FixModeNone {
				t.Errorf("rule %s: FixMode() = %v, want FixModeNone", r.ID, got)
			}
			none++
		}
	}
	t.Logf("fix-mode distribution: none=%d autofix=%d suggested=%d (total=%d)",
		none, autofix, suggested, len(api.Registry))
}
