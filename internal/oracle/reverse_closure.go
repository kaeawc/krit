package oracle

import (
	"sort"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/store"
)

// ExpansionScope is the compile scope of the run whose cached entries
// ExpandStaleOraclePaths reads: the same Kotlin settings, classpath and JVM
// target the oracle invocation was keyed on. The zero value is a run without
// any of them.
type ExpansionScope struct {
	Kotlin    gradlemodel.KotlinSettings
	Classpath []string
	JvmTarget string
}

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
func ExpandStaleOraclePaths(s *store.FileStore, cacheDir string, allPaths, changed []string, runScope ...ExpansionScope) []string {
	var es ExpansionScope
	if len(runScope) > 0 {
		es = runScope[0]
	}
	// Read exactly what InvokeCached wrote: the legacy entries live in the
	// directory for the run's Kotlin settings, and store entries under the scope
	// (jar, classpath, JVM target, settings) the run was bound to.
	legacyDir := LegacyEntryDir(cacheDir, es.Kotlin, es.Classpath)
	var scopes map[string]StoreScope
	if s != nil {
		scanPaths := storeScanPaths(cacheDir, "")
		scopes = map[string]StoreScope{
			ApproximationFIRWholeCompilation: NewStoreScopeWithSettings(BackendFIR, FindBackendJar(BackendFIR, scanPaths), es.Classpath, es.JvmTarget, es.Kotlin),
			ApproximationKAATaggedReferences: NewStoreScopeWithSettings(BackendKAA, FindBackendJar(BackendKAA, scanPaths), es.Classpath, es.JvmTarget, es.Kotlin),
		}
	}
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
		var entries []*CacheEntry
		if s != nil {
			// The freshness gate has no selected backend yet. Union both
			// namespaces so neither backend's dependents are lost.
			for _, approximation := range []string{ApproximationFIRWholeCompilation, ApproximationKAATaggedReferences} {
				release := BindStoreScope(s, scopes[approximation])
				entry, loadErr := LoadEntryFromStore(s, hash, cacheDir, approximation)
				release()
				if loadErr != nil {
					err = loadErr
					break
				}
				if entry != nil {
					entries = append(entries, entry)
				}
			}
		} else {
			entry, loadErr := LoadEntry(legacyDir, hash)
			err = loadErr
			if entry != nil {
				entries = append(entries, entry)
			}
		}
		if err != nil || len(entries) == 0 {
			unresolved = append(unresolved, path)
			continue
		}
		for _, entry := range entries {
			if entry.Crashed {
				unresolved = append(unresolved, path)
				continue
			}
			for _, dependency := range entry.Closure.DepPaths {
				if dependency != "" {
					reverse[dependency] = append(reverse[dependency], path)
				}
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
