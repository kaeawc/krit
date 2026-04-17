package module

import (
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

// PerModuleIndex holds per-module and global CodeIndex instances for
// module-aware cross-file analysis such as dead code detection.
type PerModuleIndex struct {
	ModuleIndex map[string]*scanner.CodeIndex // module path -> its CodeIndex
	GlobalIndex *scanner.CodeIndex            // unified index (backward compat)
	Graph       *ModuleGraph
	ModuleFiles map[string][]*scanner.File // module path -> parsed files
}

// GroupFilesByModule assigns parsed files to the Gradle module that owns them.
// Files that do not belong to any discovered module are placed in the "root"
// bucket so module-aware rules can still reason about them consistently.
func GroupFilesByModule(graph *ModuleGraph, allFiles []*scanner.File) map[string][]*scanner.File {
	moduleFiles := make(map[string][]*scanner.File)
	for _, f := range allFiles {
		modPath := graph.FileToModule(f.Path)
		if modPath == "" {
			modPath = "root"
		}
		moduleFiles[modPath] = append(moduleFiles[modPath], f)
	}
	return moduleFiles
}

// BuildPerModuleIndex assigns each file to its module, builds a per-module
// CodeIndex in parallel, and also builds a global CodeIndex from all files.
// The global scan happens once; per-module indexes are sliced out of the
// collected symbols/references instead of re-parsing the AST for each module.
// Files that do not belong to any module are placed in a "root" bucket.
func BuildPerModuleIndex(graph *ModuleGraph, allFiles []*scanner.File, workers int) *PerModuleIndex {
	return BuildPerModuleIndexWithGlobal(graph, allFiles, workers, nil)
}

// BuildPerModuleIndexWithGlobal is like BuildPerModuleIndex, but lets callers
// reuse an already-built global index instead of recomputing it.
func BuildPerModuleIndexWithGlobal(graph *ModuleGraph, allFiles []*scanner.File, workers int, globalIndex *scanner.CodeIndex) *PerModuleIndex {
	if workers < 1 {
		workers = 1
	}

	pmi := &PerModuleIndex{
		ModuleIndex: make(map[string]*scanner.CodeIndex),
		Graph:       graph,
		ModuleFiles: GroupFilesByModule(graph, allFiles),
	}

	// Step 1: build the global index once, unless the caller already has one.
	if globalIndex != nil {
		pmi.GlobalIndex = globalIndex
	} else {
		pmi.GlobalIndex = scanner.BuildIndex(allFiles, workers)
	}

	// Step 2: partition the global index into module-specific buckets.
	type moduleBucket struct {
		symbols []scanner.Symbol
		refs    []scanner.Reference
	}
	buckets := make(map[string]*moduleBucket, len(pmi.ModuleFiles))
	fileToModule := make(map[string]string, len(allFiles))

	resolveModule := func(path string) string {
		if modPath, ok := fileToModule[path]; ok {
			return modPath
		}
		modPath := graph.FileToModule(path)
		if modPath == "" {
			modPath = "root"
		}
		fileToModule[path] = modPath
		return modPath
	}

	for _, f := range allFiles {
		resolveModule(f.Path)
	}

	for modPath := range pmi.ModuleFiles {
		buckets[modPath] = &moduleBucket{}
	}
	for _, sym := range pmi.GlobalIndex.Symbols {
		modPath := resolveModule(sym.File)
		bucket, ok := buckets[modPath]
		if !ok {
			bucket = &moduleBucket{}
			buckets[modPath] = bucket
		}
		bucket.symbols = append(bucket.symbols, sym)
	}
	for _, ref := range pmi.GlobalIndex.References {
		modPath := resolveModule(ref.File)
		bucket, ok := buckets[modPath]
		if !ok {
			bucket = &moduleBucket{}
			buckets[modPath] = bucket
		}
		bucket.refs = append(bucket.refs, ref)
	}

	// Step 3: build per-module CodeIndex values in parallel from the buckets.
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, workers)
	)

	for modPath, bucket := range buckets {
		wg.Add(1)
		sem <- struct{}{}
		go func(mp string, b *moduleBucket) {
			defer wg.Done()
			defer func() { <-sem }()

			idx := scanner.BuildIndexFromData(b.symbols, b.refs)
			mu.Lock()
			pmi.ModuleIndex[mp] = idx
			mu.Unlock()
		}(modPath, bucket)
	}
	wg.Wait()

	return pmi
}
