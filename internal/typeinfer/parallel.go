package typeinfer

import (
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
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
	// Phase 1: Parallel per-file analysis
	results := make([]*FileTypeInfo, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	runExtract := func() {
		for i, f := range files {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, file *scanner.File) {
				defer wg.Done()
				defer func() { <-sem }()
				results[idx] = IndexFileParallel(file)
			}(i, f)
		}
		wg.Wait()
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("perFileExtraction", func() error {
			runExtract()
			return nil
		})
	} else {
		runExtract()
	}

	// Phase 2: Sequential merge (no locking needed, single goroutine)
	runMerge := func() {
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

			for name, retType := range fi.Functions {
				r.functions[name] = retType
			}

			r.extensions = append(r.extensions, fi.Extensions...)
		}
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("mergeIndexedData", func() error {
			runMerge()
			return nil
		})
	} else {
		runMerge()
	}

	// Phase 3: Resolve supertype names to FQNs using merged class info
	runResolve := func() {
		for _, ci := range r.classes {
			for i, st := range ci.Supertypes {
				// If the supertype is a simple name, try to resolve it
				if !strings.Contains(st, ".") {
					if info, ok := r.classes[st]; ok && info.FQN != "" {
						ci.Supertypes[i] = info.FQN
					}
				}
			}
		}
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("resolveSupertypes", func() error {
			runResolve()
			return nil
		})
	} else {
		runResolve()
	}

	// Indexing complete. The resolver is now read-only for parallel rule execution.
}
