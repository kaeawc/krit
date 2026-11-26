package pipeline

import (
	"github.com/kaeawc/krit/internal/cache"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// The per-file analysis cache keys a file's findings by that file's own
// content, but rules that read cross-file facts through the resolver
// (supertypes, sealed variants, declarations in other files) can change
// verdict when another file's declarations change (#1778). Each cache entry
// therefore records the file's contribution to cross-file resolution
// (typeinfer.CrossFileFacts). On a warm run, a re-analyzed file whose
// contribution changed, a new file, or a deleted one seeds the names it
// declared; every cached file that references one of them, transitively
// through the cached files it pulls in, is re-analyzed instead of replayed.
// Body-only edits keep the contribution and invalidate nothing.

// fileTypeInfoProvider is the resolver capability recording needs.
type fileTypeInfoProvider interface {
	FileTypeInfoOf(path string) *typeinfer.FileTypeInfo
}

// resolverFileInfos unwraps r (an oracle composite delegates to its source
// resolver) to the per-file type information it merged, or nil.
func resolverFileInfos(r typeinfer.TypeResolver) fileTypeInfoProvider {
	if c, ok := r.(interface{ Fallback() typeinfer.TypeResolver }); ok {
		r = c.Fallback()
	}
	p, _ := r.(fileTypeInfoProvider)
	return p
}

// declFactsFor returns path's contribution to cross-file resolution in this
// run, the zero value when no resolver indexed it.
func declFactsFor(infos fileTypeInfoProvider, path string) cache.DeclFacts {
	if infos == nil {
		return cache.DeclFacts{}
	}
	f := infos.FileTypeInfoOf(path).CrossFileFacts()
	return cache.DeclFacts{Hash: f.Hash, Names: f.ByName}
}

// rulesReadResolver reports whether any active rule reads the resolver.
func rulesReadResolver(rules []*api.Rule) bool {
	for _, r := range rules {
		if r != nil && (r.Needs.Has(api.NeedsResolver) || r.OptionalResolver) {
			return true
		}
	}
	return false
}

// invalidateResolverDependents drops from ir.CacheResult every cached file
// whose resolver-backed findings a declaration change in another file can
// reach, so dispatch re-analyzes it. It returns how many it dropped.
func invalidateResolverDependents(ir *IndexResult) int {
	res := ir.CacheResult
	if res == nil || res.TotalCached == 0 || ir.Cache == nil || !rulesReadResolver(ir.ActiveRules) {
		return 0
	}
	seeds, parsed := changedDeclarationNames(ir, res)
	if len(seeds) == 0 {
		return 0
	}
	return dropCached(ir, res, resolverDependents(ir, res, seeds), parsed)
}

// changedDeclarationNames returns the lookup names whose declarations changed
// since the cache was written (in re-analyzed, new and deleted files), and
// the set of parsed paths, spelled as parsed and absolute.
func changedDeclarationNames(ir *IndexResult, res *cache.Result) (seeds, parsed map[string]bool) {
	infos := resolverFileInfos(ir.Resolver)
	seeds, parsed = map[string]bool{}, map[string]bool{}
	add := func(names []string) {
		for _, n := range names {
			seeds[n] = true
		}
	}
	for _, files := range [][]*scanner.File{ir.KotlinFiles, ir.JavaFiles} {
		for _, f := range files {
			if f == nil {
				continue
			}
			parsed[f.Path] = true
			parsed[absOrSelf(f.Path)] = true
			if !isCachedPath(res, f.Path) {
				add(changedNamesOf(ir.Cache, infos, f.Path))
			}
		}
	}
	for _, gone := range ir.Cache.VanishedDecls(parsed) {
		add(declNameKeys(gone.Names))
	}
	return seeds, parsed
}

// changedNamesOf compares a re-analyzed file's declarations with those its
// cache entry recorded.
func changedNamesOf(c *cache.Cache, infos fileTypeInfoProvider, path string) []string {
	cur := declFactsFor(infos, path)
	prior, ok := c.PriorDecls(path)
	switch {
	case ok && prior.Hash != "" && prior.Hash == cur.Hash:
		return nil // a body-only edit: other files resolve it the same way
	case !ok || prior.Hash == "":
		return declNameKeys(cur.Names) // a new file, or an entry without the facts
	default:
		return typeinfer.ChangedNames(prior.Names, cur.Names)
	}
}

// resolverDependents returns the cached files that reference a seed name,
// transitively: an affected file's own declarations resolve differently
// (their supertypes moved), so whatever names them is reached too. Without
// a reference index every cached file is.
func resolverDependents(ir *IndexResult, res *cache.Result, seeds map[string]bool) map[string]bool {
	affected := map[string]bool{}
	if ir.CodeIndex == nil {
		for path := range res.CachedPaths {
			affected[path] = true
		}
		return affected
	}
	frontier := declNameKeys(seeds)
	for len(frontier) > 0 {
		name := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		for ref := range ir.CodeIndex.ReferenceFiles(name) {
			path, ok := cachedPathFor(res, ref)
			if !ok || affected[path] {
				continue
			}
			affected[path] = true
			prior, _ := ir.Cache.PriorDecls(path)
			for n := range prior.Names {
				if !seeds[n] {
					seeds[n] = true
					frontier = append(frontier, n)
				}
			}
		}
	}
	return affected
}

// dropCached removes affected files from the cache hits so dispatch
// re-analyzes them, and returns how many it removed.
func dropCached(ir *IndexResult, res *cache.Result, affected, parsed map[string]bool) int {
	dropped := 0
	for path := range affected {
		if !parsed[path] && !parsed[absOrSelf(path)] {
			// Dispatch cannot re-analyze an unparsed file; keeping its entry
			// is the pre-#1778 behavior. Resolver rules force a full parse
			// (RulesNeedParsedSource), so this is not expected.
			continue
		}
		delete(res.CachedPaths, path)
		delete(res.CachedHashes, path)
		res.TotalCached--
		dropped++
	}
	if dropped > 0 {
		res.CachedColumns = res.CachedColumns.FilterRows(func(row int) bool {
			file := res.CachedColumns.FileAt(row)
			return !affected[file] && !affected[absOrSelf(file)]
		})
		if ir.CacheStats != nil {
			ir.CacheStats.Cached = res.TotalCached
		}
	}
	return dropped
}

func declNameKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// isCachedPath reports whether path is a cache hit, as spelled or absolute.
func isCachedPath(res *cache.Result, path string) bool {
	_, ok := cachedPathFor(res, path)
	return ok
}

// cachedPathFor returns the CachedPaths key for path, which the cache check
// may have spelled differently (relative vs absolute).
func cachedPathFor(res *cache.Result, path string) (string, bool) {
	if res.CachedPaths[path] {
		return path, true
	}
	if abs := absOrSelf(path); res.CachedPaths[abs] {
		return abs, true
	}
	return "", false
}
