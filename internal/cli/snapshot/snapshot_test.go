package snapshot

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
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
