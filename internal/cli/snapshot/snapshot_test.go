package snapshot

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	snap "github.com/kaeawc/krit/internal/snapshot"
)

func TestFormatInfoError_FriendlyOnMissing(t *testing.T) {
	got := formatInfoError("abc1234", fs.ErrNotExist)
	if !strings.Contains(got, "abc1234") {
		t.Errorf("missing arg in message: %q", got)
	}
	if !strings.Contains(got, "is not a captured snapshot") {
		t.Errorf("missing friendly hint: %q", got)
	}
	if !strings.Contains(got, "krit snapshot status") {
		t.Errorf("missing status hint: %q", got)
	}
}

func TestFormatInfoError_PassthroughOnOtherError(t *testing.T) {
	custom := errors.New("boom")
	got := formatInfoError("abc1234", custom)
	if !strings.Contains(got, "boom") {
		t.Errorf("expected non-ENOENT error to pass through: %q", got)
	}
	if strings.Contains(got, "is not a captured snapshot") {
		t.Errorf("non-ENOENT error should not get the friendly hint: %q", got)
	}
}

func TestSplitModuleMetric(t *testing.T) {
	cases := []struct {
		spec   string
		module string
		metric string
	}{
		{"loc", "", "loc"},
		{"cyclomatic", "", "cyclomatic"},
		{":app/loc", ":app", "loc"},
		{":feature:checkout/fan_in", ":feature:checkout", "fan_in"},
		{"a/b/loc", "a/b", "loc"},
	}
	for _, tc := range cases {
		gotMod, gotMetric := splitModuleMetric(tc.spec)
		if gotMod != tc.module || gotMetric != tc.metric {
			t.Errorf("splitModuleMetric(%q) = (%q, %q); want (%q, %q)",
				tc.spec, gotMod, gotMetric, tc.module, tc.metric)
		}
	}
}

func TestParseGateThresholds_RepoAndModuleScope(t *testing.T) {
	out, err := parseGateThresholds(
		[]string{":app/fan_in=30"},
		[]string{},
		[]string{"loc=5"},
	)
	if err != nil {
		t.Fatalf("parseGateThresholds: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 thresholds, got %+v", out)
	}
	var sawApp, sawRepo bool
	for _, th := range out {
		switch {
		case th.Module == ":app" && th.Metric == "fan_in":
			sawApp = true
			if th.MaxAbsolute == nil || *th.MaxAbsolute != 30 {
				t.Errorf("module-scope MaxAbsolute = %v; want 30", th.MaxAbsolute)
			}
		case th.Module == "" && th.Metric == "loc":
			sawRepo = true
			if th.MaxIncreasePct == nil || *th.MaxIncreasePct != 5 {
				t.Errorf("repo-scope MaxIncreasePct = %v; want 5", th.MaxIncreasePct)
			}
		default:
			t.Errorf("unexpected threshold: %+v", th)
		}
	}
	if !sawApp || !sawRepo {
		t.Errorf("expected both repo and module thresholds: app=%v repo=%v", sawApp, sawRepo)
	}
}

func TestParseGateConfigSection_RepoEntries(t *testing.T) {
	data := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"gate": map[string]interface{}{
				"repo": []interface{}{
					map[string]interface{}{"metric": "loc", "max_increase_pct": 5},
					map[string]interface{}{"metric": "cyclomatic", "max_absolute": 100},
				},
			},
		},
	}
	got := parseGateConfigSection(data)
	if len(got) != 2 {
		t.Fatalf("want 2 thresholds; got %+v", got)
	}
	for _, th := range got {
		if th.Module != "" {
			t.Errorf("repo entries should have empty Module; got %q", th.Module)
		}
		switch th.Metric {
		case "loc":
			if th.MaxIncreasePct == nil || *th.MaxIncreasePct != 5 {
				t.Errorf("loc.MaxIncreasePct = %v; want 5", th.MaxIncreasePct)
			}
		case "cyclomatic":
			if th.MaxAbsolute == nil || *th.MaxAbsolute != 100 {
				t.Errorf("cyclomatic.MaxAbsolute = %v; want 100", th.MaxAbsolute)
			}
		default:
			t.Errorf("unexpected metric %q", th.Metric)
		}
	}
}

func TestParseGateConfigSection_ModuleEntries(t *testing.T) {
	data := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"gate": map[string]interface{}{
				"module": map[string]interface{}{
					":app": []interface{}{
						map[string]interface{}{"metric": "fan_in", "max_absolute": 30},
					},
				},
			},
		},
	}
	got := parseGateConfigSection(data)
	if len(got) != 1 {
		t.Fatalf("want 1 threshold; got %+v", got)
	}
	if got[0].Module != ":app" || got[0].Metric != "fan_in" {
		t.Errorf("wrong (module, metric): %+v", got[0])
	}
	if got[0].MaxAbsolute == nil || *got[0].MaxAbsolute != 30 {
		t.Errorf("MaxAbsolute = %v; want 30", got[0].MaxAbsolute)
	}
}

func TestParseGateConfigSection_RejectsEmptyMetricAndMissingConstraints(t *testing.T) {
	data := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"gate": map[string]interface{}{
				"repo": []interface{}{
					map[string]interface{}{"metric": "", "max_absolute": 10},
					map[string]interface{}{"metric": "loc"}, // no constraint
					map[string]interface{}{"metric": "cyclomatic", "max_increase_pct": 5},
				},
			},
		},
	}
	got := parseGateConfigSection(data)
	if len(got) != 1 || got[0].Metric != "cyclomatic" {
		t.Fatalf("only cyclomatic should survive: %+v", got)
	}
}

func TestParseGateConfigSection_TolerantOfNumericTypes(t *testing.T) {
	data := map[string]interface{}{
		"snapshot": map[string]interface{}{
			"gate": map[string]interface{}{
				"repo": []interface{}{
					map[string]interface{}{"metric": "a", "max_absolute": int(10)},
					map[string]interface{}{"metric": "b", "max_increase_pct": float64(5.5)},
					map[string]interface{}{"metric": "c", "max_increase": "20"},
				},
			},
		},
	}
	got := parseGateConfigSection(data)
	if len(got) != 3 {
		t.Fatalf("want 3 thresholds; got %+v", got)
	}
	have := map[string]float64{}
	for _, th := range got {
		switch {
		case th.MaxAbsolute != nil:
			have[th.Metric+":abs"] = *th.MaxAbsolute
		case th.MaxIncreasePct != nil:
			have[th.Metric+":pct"] = *th.MaxIncreasePct
		case th.MaxIncrease != nil:
			have[th.Metric+":delta"] = *th.MaxIncrease
		}
	}
	if have["a:abs"] != 10 || have["b:pct"] != 5.5 || have["c:delta"] != 20 {
		t.Errorf("type coercion failed: %+v", have)
	}
}

func TestMergeGateThresholds_CLIWinsOnSameKey(t *testing.T) {
	cfgPct := 5.0
	cliPct := 10.0
	configThresholds := []snap.GateThreshold{
		{Metric: "loc", MaxIncreasePct: &cfgPct},
	}
	cliThresholds := []snap.GateThreshold{
		{Metric: "loc", MaxIncreasePct: &cliPct},
	}
	got := mergeGateThresholds(configThresholds, cliThresholds)
	if len(got) != 1 {
		t.Fatalf("want 1 merged threshold; got %+v", got)
	}
	if got[0].MaxIncreasePct == nil || *got[0].MaxIncreasePct != 10 {
		t.Errorf("CLI should win; got %v", got[0].MaxIncreasePct)
	}
}

func TestMergeGateThresholds_MergesDisjointConstraints(t *testing.T) {
	pct := 5.0
	abs := 100.0
	configThresholds := []snap.GateThreshold{
		{Metric: "loc", MaxIncreasePct: &pct},
	}
	cliThresholds := []snap.GateThreshold{
		{Metric: "loc", MaxAbsolute: &abs},
	}
	got := mergeGateThresholds(configThresholds, cliThresholds)
	if len(got) != 1 {
		t.Fatalf("want 1 merged threshold; got %+v", got)
	}
	if got[0].MaxIncreasePct == nil || *got[0].MaxIncreasePct != 5 {
		t.Errorf("config-only constraint dropped: %+v", got[0])
	}
	if got[0].MaxAbsolute == nil || *got[0].MaxAbsolute != 100 {
		t.Errorf("CLI-only constraint dropped: %+v", got[0])
	}
}

func TestMergeGateThresholds_KeepsDistinctKeysSeparate(t *testing.T) {
	a := 5.0
	b := 10.0
	got := mergeGateThresholds(
		[]snap.GateThreshold{{Metric: "loc", MaxIncreasePct: &a}},
		[]snap.GateThreshold{{Module: ":app", Metric: "fan_in", MaxAbsolute: &b}},
	)
	if len(got) != 2 {
		t.Fatalf("repo + module keys should not collapse; got %+v", got)
	}
}
