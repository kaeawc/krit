package oracle

import (
	"bytes"
	"context"
	"os"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// kotlinDefaultImports are the packages Kotlin imports into every file. A new
// top-level declaration in one of them is visible without an import.
var kotlinDefaultImports = map[string]bool{
	"kotlin": true, "kotlin.annotation": true, "kotlin.collections": true,
	"kotlin.comparisons": true, "kotlin.io": true, "kotlin.ranges": true,
	"kotlin.sequences": true, "kotlin.text": true, "kotlin.jvm": true,
	"java.lang": true,
}

// demoteNameCollidingHits moves cached KAA hits back to the miss list when a
// file being re-analyzed this run declares a top-level name the hit mentions
// from a scope that can see it.
//
// A krit-types cache entry's dependency closure records only the
// declarations its file already resolved to. A new file that adds an overload
// (or a same-named function, extension, property, or type) in the same
// package, or in one the file imports, can change what a cached caller
// resolves to without touching anything in that caller's closure, so
// closure verification alone keeps the caller's stale facts. The files
// analyzed this run are exactly the ones that are new or changed (everything
// else verified as a hit), so their top-level declared names are the names
// that may have appeared. A hit is demoted when it
//
//   - mentions such a name as a whole identifier, and
//   - is in the declaring package, imports it (explicitly, by wildcard, or
//     through a default import), or spells it package-qualified.
//
// The test is deliberately conservative on the hit side (a lexical identifier
// match, so a comment or an unrelated receiver's member can demote a hit) and
// precise on the declaring side (flat-AST top-level, non-private
// declarations only). Over-invalidation costs one re-analysis of that file;
// an unrelated added file demotes nothing. After re-analysis the new file is
// in the caller's closure, so later edits of it invalidate through the
// ordinary closure check.
//
// Whole-compilation backends already refresh every file's facts together and
// are not affected.
func demoteNameCollidingHits(opts InvocationOptions, hits []*CacheEntry, misses []string, tracker perf.Tracker, verbose bool) ([]*CacheEntry, []string) {
	if opts.Backend.ReturnsWholeCompilation() || len(hits) == 0 || len(misses) == 0 {
		return hits, misses
	}
	declared := declaredNamesByPackage(misses)
	if len(declared) == 0 {
		return hits, misses
	}
	kept := make([]*CacheEntry, 0, len(hits))
	var demoted []string
	for _, hit := range hits {
		if !hit.Crashed && hitSeesDeclaredName(hit.FilePath, declared) {
			demoted = append(demoted, hit.FilePath)
			continue
		}
		kept = append(kept, hit)
	}
	if len(demoted) == 0 {
		return hits, misses
	}
	sort.Strings(demoted)
	addOracleInstant(tracker, "nameCollisionInvalidated", map[string]int64{"files": int64(len(demoted))}, nil)
	if verbose {
		reporter().Verbosef("verbose: cache classify: %d hit(s) mention a name declared by a changed file; re-analyzing\n", len(demoted))
	}
	return kept, append(append([]string(nil), misses...), demoted...)
}

// declaredNamesByPackage maps a package to the visible top-level names the
// given Kotlin files declare in it. Files that cannot be parsed contribute
// nothing; they are already misses and the closure check covers their
// callers.
func declaredNamesByPackage(paths []string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, path := range paths {
		if !strings.HasSuffix(path, ".kt") {
			continue
		}
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			continue
		}
		pkg, names := scanner.TopLevelDeclaredNames(file)
		if len(names) == 0 {
			continue
		}
		set := out[pkg]
		if set == nil {
			set = map[string]bool{}
			out[pkg] = set
		}
		for _, name := range names {
			set[name] = true
		}
	}
	return out
}

func hitSeesDeclaredName(path string, declared map[string]map[string]bool) bool {
	var content []byte
	var hitPkg string
	var imports, wildcards []string
	headerRead := false
	for pkg, names := range declared {
		for name := range names {
			if content == nil {
				data, err := os.ReadFile(path)
				if err != nil {
					return false
				}
				content = data
			}
			if !mentionsIdentifier(content, name) {
				continue
			}
			if !headerRead {
				hitPkg, imports, wildcards = scanner.SourceHeaderFacts(content)
				headerRead = true
			}
			if hitPkg == pkg || kotlinDefaultImports[pkg] || containsString(wildcards, pkg) {
				return true
			}
			qualified := pkg + "." + name
			if pkg == "" {
				continue
			}
			if containsString(imports, qualified) || bytes.Contains(content, []byte(qualified)) {
				return true
			}
		}
	}
	return false
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// mentionsIdentifier reports whether content contains name as a whole
// identifier (not as part of a longer one).
func mentionsIdentifier(content []byte, name string) bool {
	needle := []byte(name)
	for from := 0; from < len(content); {
		i := bytes.Index(content[from:], needle)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(needle)
		if (start == 0 || !isIdentByte(content[start-1])) && (end == len(content) || !isIdentByte(content[end])) {
			return true
		}
		from = start + 1
	}
	return false
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b >= 0x80 ||
		(b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
