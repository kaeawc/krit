package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// bufferRuleIDs runs one analyze-buffers call on a buffer with an empty
// function and returns the rule IDs it found and the rule IDs the daemon
// reports evaluating.
func bufferRuleIDs(t *testing.T, root string) (found, evaluated []string) {
	t.Helper()
	// Buffer scans write rule config into package globals; re-apply the
	// default config so later tests in this package see default rules.
	t.Cleanup(func() {
		cfg, _ := loadDaemonConfig(t.TempDir())
		rules.ApplyConfig(cfg)
	})
	socket, _ := startServerWith(t, root, time.Second)
	args := daemon.AnalyzeBuffersArgs{Buffers: []daemon.AnalyzeBufferArgs{
		{Path: filepath.Join(root, "Bad.kt"), Content: "fun main() {   \n}\n"},
	}}
	var got daemon.AnalyzeBuffersResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers, args, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Results) != 1 || got.Results[0].Error != "" {
		t.Fatalf("unexpected results: %+v", got.Results)
	}
	var cols struct {
		Rules []string `json:"rules"`
	}
	if err := json.Unmarshal(got.Results[0].Findings, &cols); err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	return cols.Rules, got.Rules
}

func TestAnalyzeBuffers_HonorsProjectConfig(t *testing.T) {
	found, evaluated := bufferRuleIDs(t, t.TempDir())
	if !slices.Contains(found, "EmptyFunctionBlock") || !slices.Contains(evaluated, "EmptyFunctionBlock") {
		t.Fatalf("premise: default config should run and report EmptyFunctionBlock; found=%v", found)
	}

	root := t.TempDir()
	cfg := "empty-blocks:\n  EmptyFunctionBlock:\n    active: false\n"
	if err := os.WriteFile(filepath.Join(root, "krit.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	found, evaluated = bufferRuleIDs(t, root)
	if slices.Contains(found, "EmptyFunctionBlock") {
		t.Errorf("EmptyFunctionBlock is disabled in krit.yml but the buffer scan reported it")
	}
	if slices.Contains(evaluated, "EmptyFunctionBlock") {
		t.Errorf("EmptyFunctionBlock is disabled in krit.yml but the daemon lists it as evaluated")
	}
}

func TestAnalyzeBuffers_ReportsOnlyPerFileRules(t *testing.T) {
	_, evaluated := bufferRuleIDs(t, t.TempDir())
	if len(evaluated) == 0 {
		t.Fatal("daemon reported no evaluated rules")
	}
	byID := make(map[string]*api.Rule, len(api.Registry))
	for _, r := range api.Registry {
		byID[r.ID] = r
	}
	projectScope := api.NeedsCrossFile | api.NeedsModuleIndex | api.NeedsManifest | api.NeedsResources | api.NeedsGradle
	for _, id := range evaluated {
		r := byID[id]
		if r == nil {
			t.Errorf("evaluated rule %q is not registered", id)
			continue
		}
		if r.Needs&projectScope != 0 {
			t.Errorf("rule %s needs project context (%v) but is listed as evaluated on buffers", id, r.Needs)
		}
	}
}
