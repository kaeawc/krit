package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pruneScenario captures the inputs needed to drive Prune through a
// fake git reach implementation. The actual snapshot fixtures live on
// disk — Prune still walks .krit/snapshots/graphs/ via List and
// LoadManifests.
type pruneScenario struct {
	root              string
	repoRoot          string
	permanentSet      map[string]bool
	anyRefSet         map[string]bool
	now               time.Time
	keepFeatureAge    time.Duration
	keepOrphanAge     time.Duration
	permanentBranches []string
}

func (s pruneScenario) reach(_ string, _ []string) (map[string]bool, error) {
	out := make(map[string]bool, len(s.permanentSet))
	for k, v := range s.permanentSet {
		out[k] = v
	}
	return out, nil
}

func (s pruneScenario) all(_ string) (map[string]bool, error) {
	out := make(map[string]bool, len(s.anyRefSet))
	for k, v := range s.anyRefSet {
		out[k] = v
	}
	return out, nil
}

func (s pruneScenario) opts(dryRun bool) PruneOptions {
	return PruneOptions{
		Root:              s.root,
		RepoRoot:          s.repoRoot,
		PermanentBranches: s.permanentBranches,
		KeepFeatureAge:    s.keepFeatureAge,
		KeepOrphanAge:     s.keepOrphanAge,
		DryRun:            dryRun,
		Now:               s.now,
		ReachableSHAs:     s.reach,
		AllRefSHAs:        s.all,
	}
}

func writePruneSnapshot(t *testing.T, root, sha string, capturedAt time.Time) {
	t.Helper()
	if _, err := Save(root, &Blob{
		SchemaVersion: SchemaVersion, CommitSHA: sha, CapturedAt: capturedAt.Unix(),
	}); err != nil {
		t.Fatalf("Save %s: %v", sha, err)
	}
	if _, err := SaveManifest(root, &Manifest{
		SchemaVersion: ManifestSchemaVersion, CommitSHA: sha, CapturedAt: capturedAt.Unix(),
	}); err != nil {
		t.Fatalf("SaveManifest %s: %v", sha, err)
	}
}

func snapshotExists(root, sha string) bool {
	dir, err := shaDir(root, sha)
	if err != nil {
		return false
	}
	_, err = os.Stat(dir)
	return err == nil
}

func TestPrune_KeepsPermanentReachable(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old := now.Add(-365 * 24 * time.Hour)

	mainSHA := "aaaa1111aaaa1111aaaa1111aaaa1111aaaa1111"
	writePruneSnapshot(t, root, mainSHA, old)

	res, err := Prune(pruneScenario{
		root:         root,
		repoRoot:     dir,
		permanentSet: map[string]bool{mainSHA: true},
		anyRefSet:    map[string]bool{mainSHA: true},
		now:          now,
	}.opts(false))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Pruned != 0 {
		t.Errorf("expected 0 pruned for permanent-reachable old snapshot; got %+v", res.Entries)
	}
	if !snapshotExists(root, mainSHA) {
		t.Errorf("permanent-reachable snapshot was deleted")
	}
}

func TestPrune_FeatureBranchEvictedAfterAge(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	freshSHA := "bbbb1111bbbb1111bbbb1111bbbb1111bbbb1111"
	staleSHA := "cccc2222cccc2222cccc2222cccc2222cccc2222"
	writePruneSnapshot(t, root, freshSHA, now.Add(-7*24*time.Hour))
	writePruneSnapshot(t, root, staleSHA, now.Add(-90*24*time.Hour))

	res, err := Prune(pruneScenario{
		root:           root,
		repoRoot:       dir,
		anyRefSet:      map[string]bool{freshSHA: true, staleSHA: true},
		now:            now,
		keepFeatureAge: 30 * 24 * time.Hour,
	}.opts(false))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Pruned != 1 {
		t.Errorf("expected exactly the stale feature snapshot pruned; got %+v", res.Entries)
	}
	if snapshotExists(root, staleSHA) {
		t.Errorf("stale feature snapshot should have been removed")
	}
	if !snapshotExists(root, freshSHA) {
		t.Errorf("fresh feature snapshot was wrongly removed")
	}
}

func TestPrune_OrphansEvictedAfterShorterAge(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	freshOrphan := "dddd3333dddd3333dddd3333dddd3333dddd3333"
	staleOrphan := "eeee4444eeee4444eeee4444eeee4444eeee4444"
	writePruneSnapshot(t, root, freshOrphan, now.Add(-3*24*time.Hour))
	writePruneSnapshot(t, root, staleOrphan, now.Add(-15*24*time.Hour))

	res, err := Prune(pruneScenario{
		root:          root,
		repoRoot:      dir,
		anyRefSet:     map[string]bool{}, // both are orphans
		now:           now,
		keepOrphanAge: 7 * 24 * time.Hour,
	}.opts(false))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Pruned != 1 {
		t.Errorf("expected exactly the stale orphan pruned; got %+v", res.Entries)
	}
	if snapshotExists(root, staleOrphan) {
		t.Errorf("stale orphan should have been removed")
	}
	if !snapshotExists(root, freshOrphan) {
		t.Errorf("fresh orphan was wrongly removed")
	}
}

func TestPrune_DryRunDoesNotDelete(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	staleSHA := "ffff5555ffff5555ffff5555ffff5555ffff5555"
	writePruneSnapshot(t, root, staleSHA, now.Add(-365*24*time.Hour))

	res, err := Prune(pruneScenario{
		root:          root,
		repoRoot:      dir,
		anyRefSet:     map[string]bool{},
		now:           now,
		keepOrphanAge: 7 * 24 * time.Hour,
	}.opts(true)) // DryRun
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Pruned != 1 {
		t.Errorf("dry-run should still report what would be pruned; got %+v", res.Entries)
	}
	if !snapshotExists(root, staleSHA) {
		t.Fatalf("dry-run must not delete; snapshot was removed anyway")
	}
}

func TestPrune_RemovesFromIndex(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	staleSHA := "1111aaaa1111aaaa1111aaaa1111aaaa1111aaaa"
	writePruneSnapshot(t, root, staleSHA, now.Add(-365*24*time.Hour))

	idx, err := LoadIndex(root)
	if err != nil || idx == nil || len(idx.Entries) != 1 {
		t.Fatalf("setup: index should contain the seeded sha; got %v err=%v", idx, err)
	}

	if _, err := Prune(pruneScenario{
		root:          root,
		repoRoot:      dir,
		anyRefSet:     map[string]bool{},
		now:           now,
		keepOrphanAge: 1 * time.Hour,
	}.opts(false)); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	idx, err = LoadIndex(root)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx != nil && len(idx.Entries) != 0 {
		t.Errorf("rolled-up index should drop pruned shas; got %+v", idx.Entries)
	}
}

func TestPrune_ContinuesPastRemovalErrors(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	a := "aaaaffff5555aaaaffff5555aaaaffff5555aaaa"
	b := "bbbb6666bbbb6666bbbb6666bbbb6666bbbb6666"
	writePruneSnapshot(t, root, a, now.Add(-365*24*time.Hour))
	writePruneSnapshot(t, root, b, now.Add(-365*24*time.Hour))

	// Remove a's directory under the test's feet to simulate a
	// concurrent rmdir. RemoveAll is fine with missing files but a
	// bad shaDir (e.g. permission error) would not be — this test
	// at least proves the loop continues when one removal silently
	// no-ops.
	dirA, _ := shaDir(root, a)
	if err := os.RemoveAll(dirA); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := Prune(pruneScenario{
		root:          root,
		repoRoot:      dir,
		anyRefSet:     map[string]bool{},
		now:           now,
		keepOrphanAge: 1 * time.Hour,
	}.opts(false))
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if snapshotExists(root, b) {
		t.Errorf("snapshot b should still have been pruned despite a's missing dir")
	}
	_ = res
}
