package traces

import (
	"os"
	"path/filepath"
	"testing"

	tr "github.com/kaeawc/krit/internal/traces"
)

// TestIngestOTelFixtureProducesStates runs the ingest path against
// the checked-in OTel fixture and verifies the on-disk store has the
// expected shape.
func TestIngestOTelFixtureProducesStates(t *testing.T) {
	repoRoot := t.TempDir()
	// Locate the fixture relative to this test file's working dir.
	// `go test ./internal/cli/traces` runs with cwd = package dir.
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	fixture := filepath.Join(pkgDir, "..", "..", "..", "tests", "fixtures", "traces", "otel_sample.json")
	if _, err := os.Stat(fixture); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	code := runIngest([]string{"--otel", fixture, "--repo", repoRoot, "--commit", "abc1234"})
	if code != 0 {
		t.Fatalf("ingest exit %d", code)
	}
	store, err := tr.Load(repoRoot)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	if len(store.States) == 0 {
		t.Fatalf("expected states from ingest")
	}
	if len(store.Sources) != 1 {
		t.Fatalf("want 1 source, got %d", len(store.Sources))
	}
	if store.Sources[0].CommitSHA != "abc1234" {
		t.Fatalf("commit not stamped: %s", store.Sources[0].CommitSHA)
	}
	// The fixture has a 3-span chain; the reducer should observe 3 states.
	if len(store.States) != 3 {
		t.Fatalf("want 3 states, got %d", len(store.States))
	}
	// And 2 transitions (call1->call2, call2->call3).
	if len(store.Transitions) != 2 {
		t.Fatalf("want 2 transitions, got %d", len(store.Transitions))
	}
}

func TestIngestRejectsMultipleInputs(t *testing.T) {
	repoRoot := t.TempDir()
	code := runIngest([]string{"--otel", "a.json", "--jfr", "b.json", "--repo", repoRoot})
	if code == 0 {
		t.Fatalf("expected non-zero exit when two inputs given")
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	code := Run([]string{"nope"})
	if code == 0 {
		t.Fatalf("expected non-zero exit on unknown subcommand")
	}
}
