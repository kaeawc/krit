package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestRunProject_ShowPerfPopulatesOutputExtras asserts the #70 Step C
// wiring: when Args.ShowPerf=true and a tracker is attached, the JSON
// output carries a perf subtree and a caches subtree. When ShowPerf
// is false the JSON header omits both — the daemon's default.
func TestRunProject_ShowPerfPopulatesOutputExtras(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(file, []byte("package demo\n\nclass Foo : Any()\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	rule := findV2RuleForTest(t, "UnnecessaryInheritance")

	// --- ShowPerf=true: extras must be present.
	tracker := perf.New(true)
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
			ShowPerf:    true,
			PerfRules:   true,
		},
		Host: ProjectHostState{Tracker: tracker},
	})
	if err != nil {
		t.Fatalf("RunProject (ShowPerf=true): %v", err)
	}
	got := decodeOutput(t, res.JSON)
	for _, key := range []string{"perfTiming", "perfRuleStats", "caches", "cacheBudget"} {
		if _, ok := got[key]; !ok {
			t.Errorf("ShowPerf=true: missing %q in output keys=%v", key, mapKeys(got))
		}
	}

	// --- ShowPerf=false: extras must be absent.
	res2, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{root},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
	})
	if err != nil {
		t.Fatalf("RunProject (ShowPerf=false): %v", err)
	}
	got2 := decodeOutput(t, res2.JSON)
	for _, key := range []string{"perfTiming", "perfRuleStats", "caches", "cacheBudget", "cache"} {
		if _, ok := got2[key]; ok {
			t.Errorf("ShowPerf=false: %q must be absent from output, got keys=%v", key, mapKeys(got2))
		}
	}
}

func decodeOutput(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v\nraw=%s", err, raw)
	}
	return out
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
