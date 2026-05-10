package snapshot

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Reachability classifies a captured snapshot for retention purposes.
type Reachability string

const (
	// ReachPermanent — sha is reachable from a permanent branch
	// (default main / master). Always kept.
	ReachPermanent Reachability = "permanent"
	// ReachFeature — sha is reachable from at least one ref but none
	// of the permanent branches. Kept while younger than KeepFeatureAge.
	ReachFeature Reachability = "feature"
	// ReachOrphan — sha is not reachable from any current ref. Kept
	// while younger than KeepOrphanAge.
	ReachOrphan Reachability = "orphan"
)

// PruneOptions configures Prune. Reachability is resolved via the
// ReachableSHAs / AllRefSHAs hooks, which are normally backed by git
// in production and stubbed in tests.
type PruneOptions struct {
	// Root is the snapshots directory (i.e. SnapshotsDir(repoRoot)).
	Root string
	// RepoRoot is the working-tree root passed to git.
	RepoRoot string
	// PermanentBranches lists the branch names treated as
	// always-keep. Empty means use DefaultPermanentBranches.
	PermanentBranches []string
	// KeepFeatureAge is the retention window for feature-branch-only
	// snapshots. Zero means use DefaultKeepFeatureAge.
	KeepFeatureAge time.Duration
	// KeepOrphanAge is the retention window for unreachable
	// snapshots. Zero means use DefaultKeepOrphanAge.
	KeepOrphanAge time.Duration
	// DryRun reports what would be pruned without removing anything.
	DryRun bool
	// Now is the reference clock for age comparisons. Zero means
	// time.Now(). Test seam.
	Now time.Time
	// ReachableSHAs returns the set of commit shas reachable from any
	// of the given refs. Nil means "use the production git-backed
	// implementation". Test seam.
	ReachableSHAs func(repoRoot string, refs []string) (map[string]bool, error)
	// AllRefSHAs returns the set of commit shas reachable from any
	// ref in the repository. Nil means "use the production git-backed
	// implementation". Test seam.
	AllRefSHAs func(repoRoot string) (map[string]bool, error)
}

// DefaultPermanentBranches is the fallback set of always-keep
// branches when PruneOptions.PermanentBranches is empty.
var DefaultPermanentBranches = []string{"main", "master"}

// DefaultKeepFeatureAge is the retention window for snapshots only
// reachable from feature branches.
const DefaultKeepFeatureAge = 30 * 24 * time.Hour

// DefaultKeepOrphanAge is the retention window for unreachable
// snapshots — shorter than feature retention because force-pushes /
// rebases produce these and they're typically the user wanting them
// gone.
const DefaultKeepOrphanAge = 7 * 24 * time.Hour

// PruneEntry is one captured snapshot's prune classification.
type PruneEntry struct {
	CommitSHA  string
	CapturedAt time.Time
	Reach      Reachability
	// Pruned is true when Prune deleted (or, in DryRun, would have
	// deleted) this snapshot.
	Pruned bool
	// Reason is a short human-readable note: "kept (permanent)",
	// "kept (feature, age 12d < 30d)", "pruned (orphan, age 9d > 7d)".
	Reason string
}

// PruneResult summarises a Prune run.
type PruneResult struct {
	Entries []PruneEntry
	// Pruned counts entries actually removed (or, in DryRun, that
	// would have been removed).
	Pruned int
	// Errors collects per-sha removal failures so the caller can
	// surface them; the run continues past individual errors.
	Errors []error
}

// Prune walks the snapshots tree and applies retention rules.
// Errors are accumulated per-sha rather than aborting the run so a
// single broken entry doesn't prevent cleanup of the rest.
func Prune(opts PruneOptions) (*PruneResult, error) {
	if opts.Root == "" {
		return nil, errors.New("snapshot: Prune requires Root")
	}
	if opts.RepoRoot == "" {
		return nil, errors.New("snapshot: Prune requires RepoRoot")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	branches := opts.PermanentBranches
	if len(branches) == 0 {
		branches = DefaultPermanentBranches
	}
	keepFeature := opts.KeepFeatureAge
	if keepFeature == 0 {
		keepFeature = DefaultKeepFeatureAge
	}
	keepOrphan := opts.KeepOrphanAge
	if keepOrphan == 0 {
		keepOrphan = DefaultKeepOrphanAge
	}
	reach := opts.ReachableSHAs
	if reach == nil {
		reach = gitReachableSHAs
	}
	allRefs := opts.AllRefSHAs
	if allRefs == nil {
		allRefs = gitAllRefSHAs
	}

	manifests, err := LoadManifests(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("snapshot: load manifests: %w", err)
	}
	if len(manifests) == 0 {
		return &PruneResult{}, nil
	}

	permanentSet, err := reach(opts.RepoRoot, branches)
	if err != nil {
		return nil, fmt.Errorf("snapshot: resolve permanent reachability: %w", err)
	}
	anyRefSet, err := allRefs(opts.RepoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot: resolve all-ref reachability: %w", err)
	}

	res := &PruneResult{Entries: make([]PruneEntry, 0, len(manifests))}
	var toRemove []string
	for _, m := range manifests {
		entry := classifyForPrune(m, now, permanentSet, anyRefSet, keepFeature, keepOrphan)
		res.Entries = append(res.Entries, entry)
		if entry.Pruned {
			res.Pruned++
			toRemove = append(toRemove, m.CommitSHA)
		}
	}
	sort.Slice(res.Entries, func(i, j int) bool {
		return res.Entries[i].CommitSHA < res.Entries[j].CommitSHA
	})

	if opts.DryRun {
		return res, nil
	}
	for _, sha := range toRemove {
		dir, err := shaDir(opts.Root, sha)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("shaDir %s: %w", sha, err))
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("remove %s: %w", sha, err))
		}
	}
	if err := removeFromIndex(opts.Root, toRemove...); err != nil {
		res.Errors = append(res.Errors, err)
	}
	return res, nil
}

// classifyForPrune maps a captured manifest to its retention
// outcome. Pulled out of Prune so the main function stays under the
// gocyclo threshold and the policy is easier to test in isolation.
func classifyForPrune(m Manifest, now time.Time, permanent, anyRef map[string]bool, keepFeature, keepOrphan time.Duration) PruneEntry {
	entry := PruneEntry{
		CommitSHA:  m.CommitSHA,
		CapturedAt: time.Unix(m.CapturedAt, 0),
	}
	if permanent[m.CommitSHA] {
		entry.Reach = ReachPermanent
		entry.Reason = "kept (permanent)"
		return entry
	}
	if anyRef[m.CommitSHA] {
		entry.Reach = ReachFeature
		entry.Pruned, entry.Reason = pruneByAge(now.Sub(entry.CapturedAt), keepFeature, "feature")
		return entry
	}
	entry.Reach = ReachOrphan
	entry.Pruned, entry.Reason = pruneByAge(now.Sub(entry.CapturedAt), keepOrphan, "orphan")
	return entry
}

// pruneByAge returns (prune, reason) for an age vs. window comparison.
func pruneByAge(age, keep time.Duration, kind string) (bool, string) {
	if age > keep {
		return true, fmt.Sprintf("pruned (%s, age %s > %s)", kind, roundAge(age), roundAge(keep))
	}
	return false, fmt.Sprintf("kept (%s, age %s ≤ %s)", kind, roundAge(age), roundAge(keep))
}

// gitReachableSHAs runs `git rev-list <branches...>` and returns the
// set of commit shas reachable from any of the named branches.
// Branches that don't exist locally are silently skipped so a repo
// without "main" but with "master" still works.
func gitReachableSHAs(repoRoot string, branches []string) (map[string]bool, error) {
	out := make(map[string]bool)
	for _, branch := range branches {
		// Resolve to an actual commit first so non-existent branches
		// don't fail the whole call.
		if _, err := runGitLine(repoRoot, "rev-parse", "--verify", branch); err != nil {
			continue
		}
		raw, err := runGitLine(repoRoot, "rev-list", branch)
		if err != nil {
			return nil, err
		}
		for _, sha := range strings.Fields(raw) {
			out[sha] = true
		}
	}
	return out, nil
}

// gitAllRefSHAs runs `git rev-list --all` and returns the set of
// commit shas reachable from any ref in the repository.
func gitAllRefSHAs(repoRoot string) (map[string]bool, error) {
	raw, err := runGitLine(repoRoot, "rev-list", "--all")
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, sha := range strings.Fields(raw) {
		out[sha] = true
	}
	return out, nil
}

// roundAge formats a duration to a single unit suitable for
// human-readable prune reasons (e.g. "12d", "5h"). Sub-hour
// durations fall back to the default Duration.String shape.
func roundAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return d.Round(time.Second).String()
}
