package typeinfer

import (
	"context"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
	"golang.org/x/sync/errgroup"
)

// FileTypeInfo holds all type information extracted from a single file.
// It is produced independently per file with no shared state,
// enabling embarrassingly parallel indexing.
type FileTypeInfo struct {
	Path        string
	ImportTable *ImportTable
	RootScope   *ScopeTable
	Classes     []*ClassInfo
	SealedSubs  map[string][]string      // supertype name → subclass names
	EnumEntries map[string][]string      // enum class name → entry names
	TypeAliases map[string]*ResolvedType // alias name or FQN → target type
	Functions   map[string]*ResolvedType // function name → return type
	Extensions  []*ExtensionFuncInfo     // extension functions
}

// IndexFileParallel extracts type info from a single file without
// touching any shared state. Returns a FileTypeInfo that can be
// merged later.
func IndexFileParallel(file *scanner.File) *FileTypeInfo {
	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 || file.FlatType(0) != "source_file" {
		return nil
	}

	// Use a temporary resolver for per-file indexing.
	// It writes to its own local maps, not shared state.
	tmp := &defaultResolver{
		imports:        make(map[string]*ImportTable),
		scopes:         make(map[string]*ScopeTable),
		classes:        make(map[string]*ClassInfo),
		classFQN:       make(map[string]*ClassInfo),
		sealedVariants: make(map[string][]string),
		enumEntries:    make(map[string][]string),
		typeAliases:    make(map[string]*ResolvedType),
		functions:      make(map[string]*ResolvedType),
	}

	headers := scanFileHeadersFlat(0, file)
	it := headers.it
	rootScope := &ScopeTable{Entries: make(map[string]*ResolvedType), SmartCasts: make(map[string]bool), SmartCastTypes: make(map[string]*ResolvedType)}
	pkg := headers.pkg

	tmp.indexDeclarationsFlat(0, file, rootScope, it, pkg)
	tmp.buildScopesFlat(0, file, rootScope, it)
	tmp.buildFlatSmartCastScopes(file, rootScope)

	// Collect classes from the temporary resolver. Sort by FQN so the
	// returned slice has a stable, content-defined order — otherwise
	// `for _, ci := range tmp.classes` would expose Go map iteration
	// randomness to the merge step. Two declarations with the same
	// short Name but different FQNs (cross-package collisions) then
	// have a deterministic winner under last-write-wins. See #35.
	classes := make([]*ClassInfo, 0, len(tmp.classes))
	for _, ci := range tmp.classes {
		classes = append(classes, ci)
	}
	sortClassInfos(classes)

	return &FileTypeInfo{
		Path:        file.Path,
		ImportTable: it,
		RootScope:   rootScope,
		Classes:     classes,
		SealedSubs:  tmp.sealedVariants,
		EnumEntries: tmp.enumEntries,
		TypeAliases: tmp.typeAliases,
		Functions:   tmp.functions,
		Extensions:  tmp.extensions,
	}
}

// IndexFilesParallel indexes all files in parallel and merges results
// into the resolver. This is the fast path: O(n/cores) for indexing,
// O(n) for merge.
func (r *defaultResolver) IndexFilesParallel(files []*scanner.File, workers int) {
	r.IndexFilesParallelWithTracker(files, workers, nil)
}

func (r *defaultResolver) IndexFilesParallelWithTracker(files []*scanner.File, workers int, tracker perf.Tracker) {
	results := make([]*FileTypeInfo, len(files))
	r.runTracked(tracker, "perFileExtraction", func() { r.extractFilesParallel(files, workers, results) })
	r.runTracked(tracker, "mergeIndexedData", func() { r.mergeFileResults(results) })
	r.runTracked(tracker, "resolveSupertypes", r.resolveSupertypeNames)
}

// IndexFilesParallelCachedWithTracker indexes files through the per-file
// on-disk cache. A changed file invalidates only its own cached FileTypeInfo;
// merge and supertype resolution still run over the current file set so the
// resolver remains deterministic.
func (r *defaultResolver) IndexFilesParallelCachedWithTracker(files []*scanner.File, workers int, cacheDir string, tracker perf.Tracker) (hits, misses int) {
	results := make([]*FileTypeInfo, len(files))
	r.runTracked(tracker, "perFileExtraction", func() {
		hits, misses = r.extractFilesParallelCached(files, workers, cacheDir, results)
	})
	r.runTracked(tracker, "mergeIndexedData", func() { r.mergeFileResults(results) })
	r.runTracked(tracker, "resolveSupertypes", r.resolveSupertypeNames)
	return hits, misses
}

func (r *defaultResolver) runTracked(tracker perf.Tracker, name string, fn func()) {
	if tracker != nil && tracker.IsEnabled() {
		tracker.TrackVoid(name, fn)
	} else {
		fn()
	}
}

func (r *defaultResolver) extractFilesParallel(files []*scanner.File, workers int, results []*FileTypeInfo) {
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	for i, f := range files {
		idx, file := i, f
		g.Go(func() error {
			results[idx] = IndexFileParallel(file)
			return nil
		})
	}
	_ = g.Wait()
}

func (r *defaultResolver) extractFilesParallelCached(files []*scanner.File, workers int, cacheDir string, results []*FileTypeInfo) (int, int) {
	var hitCount, missCount atomic.Int64
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(workers)
	resident := r.residentCache
	for i, f := range files {
		idx, file := i, f
		g.Go(func() error {
			if file == nil {
				return nil
			}
			// Resident cache (daemon-resident, path-keyed,
			// watcher-invalidated) is hottest path: a map lookup
			// instead of a disk read + zstd-gob decode. Trades
			// content-hash verification for the watcher's
			// best-effort invalidation contract — same trade the
			// resident parsed-trees cache already makes for files.
			if resident != nil {
				if info, ok := resident.LookupFileTypeInfo(file.Path); ok {
					results[idx] = info
					hitCount.Add(1)
					return nil
				}
			}
			if info, ok := loadFileTypeInfoCached(cacheDir, file); ok {
				results[idx] = info
				if resident != nil {
					resident.StoreFileTypeInfo(file.Path, info)
				}
				hitCount.Add(1)
				return nil
			}
			info := IndexFileParallel(file)
			results[idx] = info
			if info != nil {
				_ = saveFileTypeInfoCached(cacheDir, file, info)
				if resident != nil {
					resident.StoreFileTypeInfo(file.Path, info)
				}
			}
			missCount.Add(1)
			return nil
		})
	}
	_ = g.Wait()
	return int(hitCount.Load()), int(missCount.Load())
}

func (r *defaultResolver) mergeFileResults(results []*FileTypeInfo) {
	// `results` is slot-indexed by the parallel extraction step, so its
	// ordering matches the caller's `files` argument. To make the merge
	// independent of that contract — and immune to short-name
	// collisions across files where the previous code's "last in
	// `results` wins" was effectively non-deterministic — process
	// results in canonical Path-ascending order with content-sorted
	// inner data. See #35.
	ordered := make([]*FileTypeInfo, 0, len(results))
	for _, fi := range results {
		if fi != nil {
			ordered = append(ordered, fi)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })

	for _, fi := range ordered {
		r.imports[fi.Path] = fi.ImportTable
		r.scopes[fi.Path] = fi.RootScope

		// Iterate classes in canonical FQN order. Last-write-wins on
		// short-name collisions is preserved, but the "winner" is now
		// the (path-largest, FQN-largest) class — a deterministic
		// function of the input set rather than goroutine scheduling.
		for _, ci := range orderedClassInfos(fi.Classes) {
			r.classes[ci.Name] = ci
			if ci.FQN != "" {
				r.classFQN[ci.FQN] = ci
			}
		}

		for _, typeName := range sortedKeys(fi.SealedSubs) {
			variants := append([]string(nil), fi.SealedSubs[typeName]...)
			sort.Strings(variants)
			r.sealedVariants[typeName] = append(r.sealedVariants[typeName], variants...)
		}

		for _, typeName := range sortedKeys(fi.EnumEntries) {
			entries := append([]string(nil), fi.EnumEntries[typeName]...)
			sort.Strings(entries)
			r.enumEntries[typeName] = entries
		}

		for _, name := range sortedKeysResolved(fi.TypeAliases) {
			r.typeAliases[name] = fi.TypeAliases[name]
		}

		for _, name := range sortedKeysResolved(fi.Functions) {
			r.functions[name] = fi.Functions[name]
		}

		r.extensions = append(r.extensions, fi.Extensions...)
	}
}

// sortClassInfos / orderedClassInfos / sortedKeys helpers keep the
// merge step's iteration order a deterministic function of input
// content rather than goroutine scheduling or map randomization.

func sortClassInfos(classes []*ClassInfo) {
	sort.SliceStable(classes, func(i, j int) bool {
		a, b := classes[i], classes[j]
		if a.FQN != b.FQN {
			return a.FQN < b.FQN
		}
		return a.Name < b.Name
	})
}

func orderedClassInfos(in []*ClassInfo) []*ClassInfo {
	if len(in) <= 1 {
		return in
	}
	out := append([]*ClassInfo(nil), in...)
	sortClassInfos(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysResolved(m map[string]*ResolvedType) []string {
	return sortedKeys(m)
}

func (r *defaultResolver) resolveSupertypeNames() {
	for _, ci := range r.classes {
		for i, st := range ci.Supertypes {
			if !strings.Contains(st, ".") {
				if info, ok := r.classes[st]; ok && info.FQN != "" {
					ci.Supertypes[i] = info.FQN
				}
			}
		}
	}
}
