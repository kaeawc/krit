package scan

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/rules/api"
)

func TestFirFallbackRulesListsDefaultOnTwins(t *testing.T) {
	ids := preflightFallbackRules(freshScanFlags(t), nil)
	if len(ids) == 0 {
		t.Fatal("expected default-on rules with FIR twins")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("not sorted/unique: %v", ids)
		}
	}
}

// The fallback list must name only rules the scan would run, so the CLI
// rule-selection flags and the config both narrow it.
func TestPreflightFallbackRulesHonorsRuleSelection(t *testing.T) {
	all := preflightFallbackRules(freshScanFlags(t), nil)
	if len(all) == 0 {
		t.Fatal("expected default-on rules with FIR twins")
	}
	victim := all[0]
	var category string
	for _, r := range api.Registry {
		if r.ID == victim {
			category = r.Category
		}
	}

	disabled := freshScanFlags(t)
	*disabled.DisableRules = victim
	if slices.Contains(preflightFallbackRules(disabled, nil), victim) {
		t.Fatalf("--disable-rules=%s still listed", victim)
	}

	cfg := config.NewConfigFromData(map[string]interface{}{
		category: map[string]interface{}{victim: map[string]interface{}{"active": false}},
	})
	if slices.Contains(preflightFallbackRules(freshScanFlags(t), cfg), victim) {
		t.Fatalf("config-disabled %s still listed", victim)
	}
	enabled := freshScanFlags(t)
	*enabled.EnableRules = victim
	if !slices.Contains(preflightFallbackRules(enabled, cfg), victim) {
		t.Fatalf("--enable-rules=%s must override the config", victim)
	}

	cheap := freshScanFlags(t)
	*cheap.MaxCost = "trivial"
	if got := preflightFallbackRules(cheap, nil); len(got) >= len(all) {
		t.Fatalf("--max-cost=trivial kept %d of %d rules", len(got), len(all))
	}
}

func TestHandleFirPreflightFailure(t *testing.T) {
	cause := errors.New("no classpath")
	newFlags := func(fail bool) *scanFlags {
		f := freshScanFlags(t)
		*f.NoFir = false
		*f.FailOnDegraded = fail
		return f
	}
	t.Run("warns and falls back", func(t *testing.T) {
		f := newFlags(false)
		var warn bytes.Buffer
		if err := handleFirPreflightFailure(f, nil, cause, &warn); err != nil {
			t.Fatal(err)
		}
		if !*f.NoFir || f.firDegraded == nil || !strings.Contains(warn.String(), "warning: FIR rules were skipped (no classpath)") {
			t.Fatalf("noFir=%v degraded=%v warn=%q", *f.NoFir, f.firDegraded, warn.String())
		}
	})
	t.Run("fail-on-degraded errors", func(t *testing.T) {
		f := newFlags(true)
		var warn bytes.Buffer
		err := handleFirPreflightFailure(f, nil, cause, &warn)
		if err == nil || !strings.Contains(err.Error(), "FIR rules were skipped") || warn.Len() != 0 {
			t.Fatalf("err=%v warn=%q", err, warn.String())
		}
	})
}

// A mid-scan checker failure lists the FIR twins among the rules this scan ran.
func TestMarkFirDegradedUsesActiveRules(t *testing.T) {
	var twin, other *api.Rule
	for _, r := range api.Registry {
		if twin == nil && firchecks.IsKnownFIRRule(r.ID) {
			twin = r
		}
	}
	for _, r := range api.Registry {
		if firchecks.IsKnownFIRRule(r.ID) && r.ID != twin.ID {
			other = r
			break
		}
	}
	if twin == nil || other == nil {
		t.Fatal("expected at least two FIR-twin rules")
	}
	r := &runner{activeRules: []*api.Rule{twin}}
	r.markFirDegraded(errors.New("boom"))
	if got := r.firDegraded.FallbackRules; !slices.Equal(got, []string{twin.ID}) {
		t.Fatalf("FallbackRules = %v, want [%s]", got, twin.ID)
	}
}
