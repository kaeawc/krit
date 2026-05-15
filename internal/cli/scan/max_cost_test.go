package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestResolveMaxCost_FlagWins(t *testing.T) {
	cfg := writeTempMaxCostConfig(t, "oracle")
	got, ok := resolveMaxCost("ast", cfg)
	if !ok {
		t.Fatalf("resolveMaxCost should return ok=true for flag value")
	}
	if got != api.CostAST {
		t.Fatalf("resolveMaxCost = %s, want ast", got)
	}
}

func TestResolveMaxCost_ConfigFallback(t *testing.T) {
	cfg := writeTempMaxCostConfig(t, "balanced")
	got, ok := resolveMaxCost("", cfg)
	if !ok {
		t.Fatalf("resolveMaxCost should fall back to config")
	}
	if got != api.CostCrossFile {
		t.Fatalf("resolveMaxCost = %s, want crossfile (balanced)", got)
	}
}

func TestResolveMaxCost_EmptyAndUnknown(t *testing.T) {
	if _, ok := resolveMaxCost("", nil); ok {
		t.Errorf("resolveMaxCost with no flag/config should return ok=false")
	}
	if _, ok := resolveMaxCost("bogus", nil); ok {
		t.Errorf("resolveMaxCost with bogus flag should return ok=false")
	}
}

func writeTempMaxCostConfig(t *testing.T, value string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "krit.yml")
	if err := os.WriteFile(path, []byte("maxCost: "+value+"\n"), 0o600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}
