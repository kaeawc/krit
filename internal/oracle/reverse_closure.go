package oracle

import (
	"sort"

	"github.com/kaeawc/krit/internal/store"
)

// ExpandStaleOraclePaths returns changed plus every cached file whose stored
// dependency closure contains a changed path. It builds a reverse index from
// the existing CacheEntry.Closure.DepPaths data and takes one hop through it.
// Stored closures are already transitive through propagating dependencies;
// walking farther would incorrectly cross a tagged non-propagating edge.
//
// A file whose cache entry cannot be loaded (read error, missing, or Crashed)
// is seeded into the work set directly: its stored dependency closure is
// unknown, so it can neither contribute a reverse edge nor be trusted to have
// fresh facts. Skipping it would silently leave a dependent of a changed file
// unanalyzed, keeping its stale facts in types.json. Directly changed paths
// always remain in the result.
func ExpandStaleOraclePaths(s *store.FileStore, cacheDir string, allPaths, changed []string) []string {
	paths := append([]string(nil), allPaths...)
	sort.Strings(paths)
	reverse := make(map[string][]string)
	var unresolved []string
	for _, path := range paths {
		hash, err := ContentHash(path)
		if err != nil {
			unresolved = append(unresolved, path)
			continue
		}
		var entry *CacheEntry
		if s != nil {
			entry, err = LoadEntryFromStore(s, hash)
		} else {
			entry, err = LoadEntry(cacheDir, hash)
		}
		if err != nil || entry == nil || entry.Crashed {
			unresolved = append(unresolved, path)
			continue
		}
		for _, dependency := range entry.Closure.DepPaths {
			if dependency != "" {
				reverse[dependency] = append(reverse[dependency], path)
			}
		}
	}
	seeds := append([]string(nil), changed...)
	seeds = append(seeds, unresolved...)
	seen := make(map[string]bool, len(seeds))
	result := make([]string, 0, len(seeds))
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	for _, path := range seeds {
		add(path)
		for _, dependent := range reverse[path] {
			add(dependent)
		}
	}
	sort.Strings(result)
	return result
}
