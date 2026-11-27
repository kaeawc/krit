package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// IndexPhase derives the RuleHash that keys the cross-file findings cache
// from CacheRuleNames and CacheConfig. Without an analysis-cache lookup they
// were filled only under --no-type-oracle, so a run that saves cross findings
// keyed them blind to the rule config (#1893).
func TestCrossFindingsRuleHashCoversConfigWithoutAnalysisCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "A.kt"), []byte("package p\n\nclass A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("CrossRule", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(*api.Context) {}))
	var seen []IndexInput
	testHookIndexInput = func(in IndexInput) { seen = append(seen, in) }
	t.Cleanup(func() { testHookIndexInput = nil })
	cfg := config.NewConfig()
	_, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{Config: cfg, Paths: []string{dir}, ActiveRules: []*api.Rule{rule}, Format: "json", Version: "test"},
		Host: ProjectHostState{CrossFindingsCacheDir: scanner.CrossFindingsCacheDir(filepath.Join(dir, ".krit"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 {
		t.Fatalf("saw %d IndexInputs, want 1", len(seen))
	}
	if seen[0].CacheConfig != cfg || seen[0].CacheRuleNames == nil {
		t.Fatalf("cross findings wired without an analysis cache: CacheConfig=%p (want %p), CacheRuleNames=%v", seen[0].CacheConfig, cfg, seen[0].CacheRuleNames)
	}
}
