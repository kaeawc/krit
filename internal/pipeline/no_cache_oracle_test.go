package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// A project run hands --no-cache-oracle (which --no-cache implies) to
// IndexPhase, whose oracle would otherwise reuse a cached types.json or run
// through its incremental cache. The daemon runs the oracle inside
// IndexPhase, so without this a delegated --no-cache scan used that cache
// (#1831).
func TestRunProjectPassesNoCacheOracleToIndexPhase(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "A.kt"), []byte("package p\n\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("ClassDecl", api.WithNodeTypes("class_declaration"), api.WithCheck(func(*api.Context) {}))
	var seen []bool
	testHookIndexInput = func(in IndexInput) { seen = append(seen, in.NoCacheOracle) }
	t.Cleanup(func() { testHookIndexInput = nil })
	for _, noCacheOracle := range []bool{true, false} {
		_, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:        config.NewConfig(),
				Paths:         []string{dir},
				ActiveRules:   []*api.Rule{rule},
				Format:        "json",
				Version:       "test",
				NoCacheOracle: noCacheOracle,
			},
			Host: ProjectHostState{},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 2 || !seen[0] || seen[1] {
		t.Fatalf("IndexInput.NoCacheOracle per run = %v, want [true false]", seen)
	}
}
