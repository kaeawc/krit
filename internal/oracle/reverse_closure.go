package oracle

import (
	"sort"

	"github.com/kaeawc/krit/internal/store"
)

// ExpandStaleOraclePaths returns changed plus every cached file whose stored
// dependency closure contains a changed path. It builds a reverse index from
// the existing CacheEntry.Closure.DepPaths data, then performs a sorted BFS.
// The seen set is both the duplicate filter and the cycle guard for mutually
// recursive source files.
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
	for dependency := range reverse {
		sort.Strings(reverse[dependency])
	}

	queue := append([]string(nil), changed...)
	queue = append(queue, unresolved...)
	sort.Strings(queue)
	seen := make(map[string]bool, len(queue))
	result := make([]string, 0, len(queue))
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
		queue = append(queue, reverse[path]...)
	}
	sort.Strings(result)
	return result
}
