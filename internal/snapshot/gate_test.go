package snapshot

import (
	"path/filepath"
	"testing"
)

func TestGatePassesWhenAllThresholdsHold(t *testing.T) {
	root := writeGateFixtures(t, 100, 105)
	limit := 10.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{{Metric: "loc", MaxIncreasePct: &limit}},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 0 {
		t.Fatalf("expected pass, got violations: %+v", res.Violations)
	}
}

func TestGateFlagsPercentBreach(t *testing.T) {
	root := writeGateFixtures(t, 100, 110)
	limit := 5.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{{Metric: "loc", MaxIncreasePct: &limit}},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 1 {
		t.Fatalf("expected one violation, got %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.Constraint != "max_increase_pct" || v.Limit != 5 {
		t.Fatalf("violation shape: %+v", v)
	}
}

func TestGateFlagsAllConstraintsIndependently(t *testing.T) {
	root := writeGateFixtures(t, 50, 100)
	abs, delta, pct := 80.0, 10.0, 25.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{{
			Metric:         "loc",
			MaxAbsolute:    &abs,
			MaxIncrease:    &delta,
			MaxIncreasePct: &pct,
		}},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 3 {
		t.Fatalf("expected 3 violations (one per constraint), got %d: %+v", len(res.Violations), res.Violations)
	}
	seen := map[GateConstraint]bool{}
	for _, v := range res.Violations {
		seen[v.Constraint] = true
	}
	for _, want := range []GateConstraint{ConstraintMaxAbsolute, ConstraintMaxIncrease, ConstraintMaxIncreasePct} {
		if !seen[want] {
			t.Fatalf("missing %s violation: %+v", want, res.Violations)
		}
	}
}

func TestGateFlagsInfinitePctOnFromZero(t *testing.T) {
	root := writeGateFixtures(t, 0, 25)
	limit := 10.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{{Metric: "loc", MaxIncreasePct: &limit}},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 1 || res.Violations[0].Constraint != ConstraintMaxIncreasePct {
		t.Fatalf("expected one max_increase_pct violation, got %+v", res.Violations)
	}
}

func TestGateRequiresThresholds(t *testing.T) {
	if _, err := Gate(GateOptions{}); err == nil {
		t.Fatal("expected error with no thresholds")
	}
}

func TestGate_ModuleScopeFlagsBreach(t *testing.T) {
	root := writeGateModuleFixtures(t, 30, 40, 0, 1)
	deltaCap := 5.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{
			// Repo-scope LOC moved 30 -> 40 (delta=10) — fails.
			{Metric: "loc", MaxIncrease: &deltaCap},
			// Module :app fan_in moved 0 -> 1 (delta=1) — passes.
			{Module: ":app", Metric: "fan_in", MaxIncrease: &deltaCap},
		},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 1 {
		t.Fatalf("expected exactly the repo-scope LOC violation, got %+v", res.Violations)
	}
	if res.Violations[0].Module != "" || res.Violations[0].Metric != "loc" {
		t.Errorf("wrong violation: %+v", res.Violations[0])
	}
}

func TestGate_ModuleScopeAbsoluteCap(t *testing.T) {
	root := writeGateModuleFixtures(t, 30, 30, 5, 12)
	cap := 10.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{
			{Module: ":app", Metric: "fan_in", MaxAbsolute: &cap},
		},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 1 {
		t.Fatalf("expected one module-scope violation, got %+v", res.Violations)
	}
	v := res.Violations[0]
	if v.Module != ":app" || v.Metric != "fan_in" || v.Constraint != ConstraintMaxAbsolute {
		t.Errorf("wrong violation shape: %+v", v)
	}
	if v.From != 5 || v.To != 12 {
		t.Errorf("violation should carry module-scope readings; got %+v", v)
	}
}

func TestGate_UnknownModuleSkipsThreshold(t *testing.T) {
	root := writeGateModuleFixtures(t, 30, 40, 0, 1)
	deltaCap := 0.0
	res, err := Gate(GateOptions{
		Root: root, FromSHA: gateFromSHA, ToSHA: gateToSHA,
		Thresholds: []GateThreshold{
			{Module: ":does-not-exist", Metric: "fan_in", MaxIncrease: &deltaCap},
		},
	})
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if len(res.Violations) != 0 {
		t.Fatalf("missing module should silently skip; got %+v", res.Violations)
	}
}

func writeGateModuleFixtures(t *testing.T, fromLOC, toLOC, fromFanIn, toFanIn int) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	for _, pair := range []struct {
		sha   string
		loc   int
		fanIn int
	}{{gateFromSHA, fromLOC, fromFanIn}, {gateToSHA, toLOC, toFanIn}} {
		if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: pair.sha, CapturedAt: 1}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		m := &Metrics{
			SchemaVersion: MetricsSchemaVersion,
			CommitSHA:     pair.sha,
			Files:         []FileMetrics{{Path: "app/a.kt", Module: ":app", LOC: pair.loc}},
			Modules:       []ModuleMetrics{{Path: ":app", LOC: pair.loc, FanIn: pair.fanIn}},
		}
		if _, err := SaveMetrics(root, m); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}
	return root
}

const (
	gateFromSHA = "11111111111111111111111111111111aaaaaaaa"
	gateToSHA   = "22222222222222222222222222222222bbbbbbbb"
)

func writeGateFixtures(t *testing.T, fromLOC, toLOC int) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	for _, pair := range []struct {
		sha string
		loc int
	}{{gateFromSHA, fromLOC}, {gateToSHA, toLOC}} {
		if _, err := Save(root, &Blob{SchemaVersion: SchemaVersion, CommitSHA: pair.sha, CapturedAt: 1}); err != nil {
			t.Fatalf("Save: %v", err)
		}
		m := &Metrics{
			SchemaVersion: MetricsSchemaVersion,
			CommitSHA:     pair.sha,
			Files:         []FileMetrics{{Path: "a.kt", LOC: pair.loc}},
		}
		if _, err := SaveMetrics(root, m); err != nil {
			t.Fatalf("SaveMetrics: %v", err)
		}
	}
	return root
}
