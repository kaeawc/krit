package pipeline

import (
	"strings"

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
// reach, so dispatch re-analyzes it, and records them on
// ir.ResolverInvalidated for the bundle replay paths. It returns how many it
// dropped.
func invalidateResolverDependents(ir *IndexResult) int {
	res := ir.CacheResult
	if res == nil || res.TotalCached == 0 || ir.Cache == nil || !rulesReadResolver(ir.ActiveRules) {
		return 0
	}
	seeds, parsed := changedDeclarationNames(ir, res)
	if len(seeds) == 0 {
		return 0
	}
	affected := resolverDependents(ir, res, seeds)
	n := dropCached(ir, res, affected, parsed)
	if n > 0 {
		ir.ResolverInvalidated = affected
	}
	return n
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

// referenceIndex answers which files mention a name.
type referenceIndex interface {
	ReferenceFiles(name string) map[string]bool
}

// resolverDependents returns the cached files a change to the seed names can
// reach. Every file that references a seed is reached, cached or not; its
// declarations whose types mention a seed (a subclass, an alias, a function
// returning it) resolve differently now, so their names join the seeds and
// whatever names them is reached in turn. Names a file merely declares
// (members such as "load" are referenced almost everywhere) are not followed.
func resolverDependents(ir *IndexResult, res *cache.Result, seeds map[string]bool) map[string]bool {
	var refs referenceIndex
	if ir.CodeIndex != nil {
		refs = ir.CodeIndex
	} else {
		refs = newTokenReferenceIndex(ir.KotlinFiles, ir.JavaFiles)
	}
	infos := resolverFileInfos(ir.Resolver)
	affected := map[string]bool{}
	frontier := declNameKeys(seeds)
	for len(frontier) > 0 {
		name := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		for ref := range refs.ReferenceFiles(name) {
			if path, ok := cachedPathFor(res, ref); ok {
				affected[path] = true
			}
			var info *typeinfer.FileTypeInfo
			if infos != nil {
				info = infos.FileTypeInfoOf(ref)
			}
			for _, n := range info.NamesDependingOn(seeds) {
				if !seeds[n] {
					seeds[n] = true
					frontier = append(frontier, n)
				}
			}
		}
	}
	return affected
}

// tokenReferenceIndex maps every identifier-shaped token of the parsed files
// to the files containing it: a coarse stand-in for the code index when no
// cross-file rule built one. Tokens in comments and strings count too, which
// only re-analyzes more.
type tokenReferenceIndex map[string]map[string]bool

func newTokenReferenceIndex(fileSets ...[]*scanner.File) tokenReferenceIndex {
	idx := tokenReferenceIndex{}
	for _, files := range fileSets {
		for _, f := range files {
			if f == nil {
				continue
			}
			forEachIdentifier(f.Content, func(tok string) {
				set := idx[tok]
				if set == nil {
					set = map[string]bool{}
					idx[tok] = set
				}
				set[f.Path] = true
			})
		}
	}
	return idx
}

func (idx tokenReferenceIndex) ReferenceFiles(name string) map[string]bool {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:] // a file names a declaration by its simple name
	}
	return idx[name]
}

// forEachIdentifier calls fn for each distinct [A-Za-z_][A-Za-z0-9_]* token.
func forEachIdentifier(content []byte, fn func(string)) {
	seen := map[string]bool{}
	start := -1
	isStart := func(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	isPart := func(c byte) bool { return isStart(c) || c >= '0' && c <= '9' }
	for i := 0; i <= len(content); i++ {
		if i < len(content) && (start >= 0 && isPart(content[i]) || start < 0 && isStart(content[i])) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			tok := string(content[start:i])
			if !seen[tok] {
				seen[tok] = true
				fn(tok)
			}
			start = -1
		}
	}
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
