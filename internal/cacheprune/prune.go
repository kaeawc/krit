// Package cacheprune removes stale content-addressed cache entries under
// ~/.krit so the per-jar JVM caches and dev jars do not grow without bound.
package cacheprune

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

const stampName = ".prune-stamp"

// Options controls which groups survive a prune.
type Options struct {
	// Keep is the number of most recently modified groups that always survive.
	Keep int
	// MinAge is how old a group's newest entry must be before removal.
	MinAge time.Duration
	// Interval limits pruning to once per Interval per directory. Zero disables the limit.
	Interval time.Duration
	// Protect lists group keys that are never removed.
	Protect []string
	// GroupOf maps a directory entry name to its group key. ok=false leaves the entry alone.
	GroupOf func(name string) (key string, ok bool)
	// Now overrides the clock in tests.
	Now func() time.Time
}

type group struct {
	paths  []string
	newest time.Time
	locked bool
}

// Prune removes every group beyond the newest Keep that is older than MinAge.
// It is best effort: errors leave entries in place. It returns the removed paths.
func Prune(dir string, opts Options) []string {
	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	stamp := filepath.Join(dir, stampName)
	if opts.Interval > 0 {
		if info, err := os.Stat(stamp); err == nil && now().Sub(info.ModTime()) < opts.Interval {
			return nil
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	groups := map[string]*group{}
	for _, entry := range entries {
		key, ok := opts.GroupOf(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		g := groups[key]
		if g == nil {
			g = &group{}
			groups[key] = g
		}
		g.paths = append(g.paths, filepath.Join(dir, entry.Name()))
		if info.ModTime().After(g.newest) {
			g.newest = info.ModTime()
		}
		// A recent lock file means another process is building this group.
		if filepath.Ext(entry.Name()) == ".lock" && now().Sub(info.ModTime()) < 24*time.Hour {
			g.locked = true
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := groups[keys[i]], groups[keys[j]]
		if !a.newest.Equal(b.newest) {
			return a.newest.After(b.newest)
		}
		return keys[i] < keys[j]
	})
	protected := map[string]bool{}
	for _, key := range opts.Protect {
		protected[key] = true
	}
	var removed []string
	for rank, key := range keys {
		g := groups[key]
		if rank < opts.Keep || protected[key] || g.locked || now().Sub(g.newest) < opts.MinAge {
			continue
		}
		for _, path := range g.paths {
			if os.RemoveAll(path) == nil {
				removed = append(removed, path)
			}
		}
	}
	if opts.Interval > 0 {
		_ = os.WriteFile(stamp, nil, 0o600)
		_ = os.Chtimes(stamp, now(), now())
	}
	return removed
}
