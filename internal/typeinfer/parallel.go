package typeinfer

import (
	"context"
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

	// Collect classes from the temporary resolver
	var classes []*ClassInfo
	for _, ci := range tmp.classes {
		classes = append(classes, ci)
	}

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
	for i, f := range files {
		idx, file := i, f
		g.Go(func() error {
			if info, ok := loadFileTypeInfoCached(cacheDir, file); ok {
				results[idx] = info
				hitCount.Add(1)
				return nil
			}
			info := IndexFileParallel(file)
			results[idx] = info
			if info != nil {
				_ = saveFileTypeInfoCached(cacheDir, file, info)
			}
			missCount.Add(1)
			return nil
		})
	}
	_ = g.Wait()
	return int(hitCount.Load()), int(missCount.Load())
}

func (r *defaultResolver) mergeFileResults(results []*FileTypeInfo) {
	for _, fi := range results {
		if fi == nil {
			continue
		}
		r.imports[fi.Path] = fi.ImportTable
		r.scopes[fi.Path] = fi.RootScope

		for _, ci := range fi.Classes {
			r.classes[ci.Name] = ci
			if ci.FQN != "" {
				r.classFQN[ci.FQN] = ci
			}
		}

		for typeName, variants := range fi.SealedSubs {
			r.sealedVariants[typeName] = append(r.sealedVariants[typeName], variants...)
		}

		for typeName, entries := range fi.EnumEntries {
			r.enumEntries[typeName] = entries
		}

		for name, targetType := range fi.TypeAliases {
			r.typeAliases[name] = targetType
		}

		for name, retType := range fi.Functions {
			r.functions[name] = retType
		}

		r.extensions = append(r.extensions, fi.Extensions...)
	}
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
