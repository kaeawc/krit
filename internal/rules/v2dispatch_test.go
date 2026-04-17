package rules

import (
	"testing"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func TestBuildV2Index_Classification(t *testing.T) {
	flat := &stubFlatRule{BaseRule: BaseRule{
		RuleName: "FlatRule", RuleSetName: "test", Sev: "warning", Desc: "flat",
	}}
	line := &stubLineRule{BaseRule: BaseRule{
		RuleName: "line rule", RuleSetName: "test", Sev: "info", Desc: "line",
	}}
	cross := &stubCrossFileRule{BaseRule: BaseRule{
		RuleName: "CrossRule", RuleSetName: "test", Sev: "warning", Desc: "cross",
	}}
	legacy := &stubLegacyRule{BaseRule: BaseRule{
		RuleName: "LegacyRule", RuleSetName: "test", Sev: "warning", Desc: "legacy",
	}}

	rules := []Rule{flat, line, cross, legacy}
	idx := BuildV2Index(rules)

	if len(idx.All) != 4 {
		t.Fatalf("All: got %d, want 4", len(idx.All))
	}
	if len(idx.NodeRules) != 1 {
		t.Errorf("NodeRules: got %d, want 1", len(idx.NodeRules))
	}
	if len(idx.LineRules) != 1 {
		t.Errorf("LineRules: got %d, want 1", len(idx.LineRules))
	}
	if len(idx.CrossFile) != 1 {
		t.Errorf("CrossFile: got %d, want 1", len(idx.CrossFile))
	}
	if len(idx.Legacy) != 1 {
		t.Errorf("Legacy: got %d, want 1", len(idx.Legacy))
	}
	if len(idx.ModuleAware) != 0 {
		t.Errorf("ModuleAware: got %d, want 0", len(idx.ModuleAware))
	}
}

func TestBuildV2Index_ByID(t *testing.T) {
	flat := &stubFlatRule{BaseRule: BaseRule{
		RuleName: "AlphaRule", RuleSetName: "test", Sev: "warning", Desc: "alpha",
	}}
	line := &stubLineRule{BaseRule: BaseRule{
		RuleName: "BetaRule", RuleSetName: "test", Sev: "info", Desc: "beta",
	}}

	idx := BuildV2Index([]Rule{flat, line})

	if r, ok := idx.ByID["AlphaRule"]; !ok {
		t.Error("ByID missing AlphaRule")
	} else if r.ID != "AlphaRule" {
		t.Errorf("ByID[AlphaRule].ID = %q", r.ID)
	}

	if r, ok := idx.ByID["BetaRule"]; !ok {
		t.Error("ByID missing BetaRule")
	} else if r.ID != "BetaRule" {
		t.Errorf("ByID[BetaRule].ID = %q", r.ID)
	}

	if _, ok := idx.ByID["NoSuchRule"]; ok {
		t.Error("ByID should not contain NoSuchRule")
	}
}

func TestBuildV2Index_Capabilities(t *testing.T) {
	flat := &stubFlatRule{BaseRule: BaseRule{
		RuleName: "F", RuleSetName: "t", Sev: "w", Desc: "f",
	}}
	line := &stubLineRule{BaseRule: BaseRule{
		RuleName: "L", RuleSetName: "t", Sev: "w", Desc: "l",
	}}
	cross := &stubCrossFileRule{BaseRule: BaseRule{
		RuleName: "C", RuleSetName: "t", Sev: "w", Desc: "c",
	}}

	idx := BuildV2Index([]Rule{flat, line, cross})

	// Flat dispatch: per-file, no NeedsLinePass
	fr := idx.ByID["F"]
	if fr.Needs.Has(v2.NeedsLinePass) {
		t.Error("flat rule should not have NeedsLinePass")
	}
	if !fr.Needs.IsPerFile() {
		t.Error("flat rule should be per-file")
	}

	// Line rule: per-file, NeedsLinePass
	lr := idx.ByID["L"]
	if !lr.Needs.Has(v2.NeedsLinePass) {
		t.Error("line rule should have NeedsLinePass")
	}
	if !lr.Needs.IsPerFile() {
		t.Error("line rule should be per-file")
	}

	// Cross-file: NeedsCrossFile, not per-file
	cr := idx.ByID["C"]
	if !cr.Needs.Has(v2.NeedsCrossFile) {
		t.Error("cross-file rule should have NeedsCrossFile")
	}
	if cr.Needs.IsPerFile() {
		t.Error("cross-file rule should not be per-file")
	}
}

func TestBuildV2Index_Empty(t *testing.T) {
	idx := BuildV2Index(nil)

	if len(idx.All) != 0 {
		t.Errorf("All: got %d, want 0", len(idx.All))
	}
	if len(idx.ByID) != 0 {
		t.Errorf("ByID: got %d entries, want 0", len(idx.ByID))
	}
}

func TestDispatcher_V2Rules(t *testing.T) {
	flat := &stubFlatRule{BaseRule: BaseRule{
		RuleName: "DispFlat", RuleSetName: "test", Sev: "warning", Desc: "flat",
	}}
	line := &stubLineRule{BaseRule: BaseRule{
		RuleName: "DispLine", RuleSetName: "test", Sev: "info", Desc: "line",
	}}

	d := NewDispatcher([]Rule{flat, line})

	// First call builds the index
	idx := d.V2Rules()
	if idx == nil {
		t.Fatal("V2Rules returned nil")
	}
	if len(idx.All) != 2 {
		t.Errorf("All: got %d, want 2", len(idx.All))
	}
	if len(idx.NodeRules) != 1 {
		t.Errorf("NodeRules: got %d, want 1", len(idx.NodeRules))
	}
	if len(idx.LineRules) != 1 {
		t.Errorf("LineRules: got %d, want 1", len(idx.LineRules))
	}

	// Second call returns the same cached instance
	idx2 := d.V2Rules()
	if idx != idx2 {
		t.Error("V2Rules should return the cached instance on subsequent calls")
	}

	// Clean up the global cache to avoid leaking into other tests
	v2CacheMu.Lock()
	delete(v2Cache, d)
	v2CacheMu.Unlock()
}
