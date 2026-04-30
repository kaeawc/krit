package scanner

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/kaeawc/krit/internal/perf"
)

// classRefPatterns matches class references in XML files.
// Compiled once at package level to avoid recompilation per XML file walk.
var classRefPatterns = []*regexp.Regexp{
	regexp.MustCompile(`android:name="([^"]+)"`),
	regexp.MustCompile(`class="([^"]+)"`),
	regexp.MustCompile(`app:argType="([^"]+)"`),
	regexp.MustCompile(`app:destination="@id/([^"]+)"`),
	regexp.MustCompile(`tools:context="([^"]+)"`),
	regexp.MustCompile(`<([a-z][a-zA-Z0-9_.]+\.[A-Z][a-zA-Z0-9]*)`), // FQN as XML tag
}

var sourceImportRe = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_$][A-Za-z0-9_$.*]*)(?:\s+as\s+([A-Za-z_][A-Za-z0-9_]*))?\s*;?`)

// Symbol represents a declared symbol in the codebase.
type Symbol struct {
	Name       string
	Kind       string // "function", "class", "property", "object", "interface"
	Visibility string // "public", "private", "internal", "protected"
	File       string
	Line       int
	StartByte  int
	EndByte    int
	Language   Language
	Package    string
	FQN        string
	Owner      string
	Signature  string
	Arity      int
	IsOverride bool
	IsTest     bool
	IsMain     bool
	IsStatic   bool
	IsFinal    bool
}

// Reference represents a usage of a name in the codebase.
type Reference struct {
	Name      string
	File      string
	Line      int
	InComment bool // true if this reference is inside a comment node
}

// ResolvedSymbol is a language-tagged source declaration resolved from the
// mixed Kotlin/Java source index.
type ResolvedSymbol struct {
	FQN      string
	Language Language
	Owner    string
	Kind     string
	Symbol   Symbol
}

// CodeIndex holds the cross-file symbol table.
type CodeIndex struct {
	Symbols    []Symbol
	References []Reference
	Files      []*File

	// Fingerprint is the cache fingerprint computed from the input file
	// set's content hashes. Populated by BuildIndexCached on both hit
	// and miss paths so downstream callers (e.g. cross-file findings
	// cache) can reuse it as part of their own cache keys without
	// rehashing every file.
	Fingerprint string

	// Lookup maps
	symbolsByName                map[string][]Symbol
	symbolsByFQN                 map[string]Symbol
	refCountByName               map[string]int
	refFilesByName               map[string]map[string]bool // name -> set of files referencing it
	nonCommentRefFilesByName     map[string]map[string]bool // name -> set of files with non-comment references
	nonCommentRefCountByNameFile map[string]map[string]int  // name -> file -> non-comment ref count

	// Bloom filter for fast "is this name referenced?" checks.
	// False positives are OK (we fall back to exact check), false negatives are not.
	refBloom *bloom.BloomFilter
}

// BuildIndex constructs a cross-file index from parsed Kotlin files,
// optionally including Java files for reference-only indexing.
func BuildIndex(files []*File, workers int, javaFiles ...*File) *CodeIndex {
	return BuildIndexWithTracker(files, workers, nil, javaFiles...)
}

// BuildIndexCached behaves like BuildIndexWithTracker but tries the
// on-disk cross-file index cache first. When cacheDir is empty, the
// cache is bypassed entirely and this reduces to BuildIndexWithTracker.
// On a miss (or when persistence fails) the full build path runs and
// the result is written back. Returns the index and a bool reporting
// whether the cache was hit.
func BuildIndexCached(cacheDir string, files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) (*CodeIndex, bool) {
	if cacheDir == "" {
		return BuildIndexWithTracker(files, workers, tracker, javaFiles...), false
	}

	// Sweep any pre-v3 shards/ directory before touching the cache.
	// Safe to call unconditionally: no-op when the legacy dir is
	// absent. Runs even on monolithic cache hits so upgraders don't
	// carry ~750 MB of dead data forever.
	(&packStore{cacheDir: cacheDir}).sweepLegacyShardsDir()

	// Pre-load XML files so fingerprint and reference extraction share
	// one disk walk. Also gives the cache a complete file-set snapshot.
	xmlFiles := loadXMLFilesForCache(files)
	fingerprint, _ := computeCrossFileFingerprint(files, javaFiles, xmlFiles)

	if cachedIdx, ok := LoadCrossFileCacheIndex(cacheDir, fingerprint); ok {
		cachedIdx.Files = append(cachedIdx.Files, files...)
		cachedIdx.Fingerprint = fingerprint
		return cachedIdx, true
	}

	// Monolithic miss → shard-backed partial rebuild so unchanged files
	// reuse their cached contribution; only new/edited files run fresh.
	// The unioned bloom produced by the sharded path lets the lookup
	// build skip the per-reference AddString loop.
	symbols, refs, prebuiltBloom := collectIndexDataSharded(cacheDir, files, javaFiles, xmlFiles, workers, tracker)
	idx := BuildIndexFromDataWithBloom(symbols, refs, prebuiltBloom, tracker)
	idx.Files = append(idx.Files, files...)
	idx.Fingerprint = fingerprint

	meta := CrossFileCacheMeta{
		KotlinFiles: len(files),
		JavaFiles:   len(javaFiles),
		XMLFiles:    len(xmlFiles),
	}
	// Best-effort persistence; any error just means the next run rebuilds.
	_ = SaveCrossFileCacheIndex(cacheDir, fingerprint, meta, idx)
	return idx, false
}

// BuildIndexWithTracker constructs a cross-file index and records sub-phase timings when tracker is enabled.
func BuildIndexWithTracker(files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) *CodeIndex {
	symbols, refs := collectIndexDataWithTracker(files, workers, tracker, javaFiles...)
	idx := BuildIndexFromDataWithTracker(symbols, refs, tracker)
	idx.Files = append(idx.Files, files...)
	return idx
}

// BuildIndexFromData constructs a CodeIndex from pre-collected symbols and
// references. This lets callers reuse indexing work instead of rescanning ASTs.
func BuildIndexFromData(symbols []Symbol, refs []Reference) *CodeIndex {
	return BuildIndexFromDataWithTracker(symbols, refs, nil)
}

// BuildIndexFromDataWithTracker constructs a CodeIndex from pre-collected symbols and
// references and records sub-phase timings when tracker is enabled.
func BuildIndexFromDataWithTracker(symbols []Symbol, refs []Reference, tracker perf.Tracker) *CodeIndex {
	return BuildIndexFromDataWithBloom(symbols, refs, nil, tracker)
}

// BuildIndexFromDataWithBloom is like BuildIndexFromDataWithTracker but
// accepts a pre-built bloom filter. When prebuilt is non-nil it replaces
// the AddString loop in lookup-map construction, so warm-load paths that
// already unioned per-shard blooms don't pay the per-reference hash
// cost again. prebuilt must cover at least every ref's Name; extra
// items are fine (bloom false positives are already tolerated by
// callers) but missing items would produce false negatives and are
// considered a bug.
func BuildIndexFromDataWithBloom(symbols []Symbol, refs []Reference, prebuilt *bloom.BloomFilter, tracker perf.Tracker) *CodeIndex {
	build := func() *CodeIndex { return buildCodeIndexWithBloom(symbols, refs, prebuilt) }
	if tracker != nil && tracker.IsEnabled() {
		var idx *CodeIndex
		_ = tracker.Track("lookupMapBuild", func() error {
			idx = build()
			return nil
		})
		return idx
	}
	return build()
}

func collectIndexData(files []*File, workers int, javaFiles ...*File) ([]Symbol, []Reference) {
	return collectIndexDataWithTracker(files, workers, nil, javaFiles...)
}

func collectIndexDataWithTracker(files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) ([]Symbol, []Reference) {
	return collectIndexDataInternal(files, workers, tracker, nil, javaFiles...)
}

// shardJob is one file's contribution task, uniform across Kotlin,
// Java, and XML phases. fresh runs the fresh-index work when the
// shard is absent; its outputs are then persisted as a shard.
type shardJob struct {
	Path        string
	ContentHash string
	Fresh       func() (syms []Symbol, refs []Reference)
}

const (
	localBufferMinCap             = 64
	localBufferKotlinSymbolPerJob = 2
	localBufferRefPerJob          = 16
)

type indexDataBuffer struct {
	symbols []Symbol
	refs    []Reference
}

func workerCountForJobs(workers, jobs int) int {
	if jobs == 0 {
		return 0
	}
	if workers < 1 {
		workers = 1
	}
	if workers > jobs {
		return jobs
	}
	return workers
}

func newIndexDataBuffers(workers, jobs, symbolsPerJob, refsPerJob int) []indexDataBuffer {
	if workers == 0 {
		return nil
	}
	jobsPerWorker := (jobs + workers - 1) / workers
	symbolCap := localBufferCap(jobsPerWorker, symbolsPerJob)
	refCap := localBufferCap(jobsPerWorker, refsPerJob)

	buffers := make([]indexDataBuffer, workers)
	for i := range buffers {
		if symbolCap > 0 {
			buffers[i].symbols = make([]Symbol, 0, symbolCap)
		}
		if refCap > 0 {
			buffers[i].refs = make([]Reference, 0, refCap)
		}
	}
	return buffers
}

func localBufferCap(jobsPerWorker, itemsPerJob int) int {
	if itemsPerJob == 0 {
		return 0
	}
	capacity := jobsPerWorker * itemsPerJob
	if capacity < localBufferMinCap {
		return localBufferMinCap
	}
	return capacity
}

func appendIndexDataBuffers(symbols []Symbol, refs []Reference, buffers []indexDataBuffer) ([]Symbol, []Reference) {
	var symbolCount, refCount int
	for _, buf := range buffers {
		symbolCount += len(buf.symbols)
		refCount += len(buf.refs)
	}
	if symbolCount > 0 {
		needed := len(symbols) + symbolCount
		if cap(symbols) < needed {
			merged := make([]Symbol, 0, needed)
			merged = append(merged, symbols...)
			symbols = merged
		}
		for _, buf := range buffers {
			symbols = append(symbols, buf.symbols...)
		}
	}
	if refCount > 0 {
		needed := len(refs) + refCount
		if cap(refs) < needed {
			merged := make([]Reference, 0, needed)
			merged = append(merged, refs...)
			refs = merged
		}
		for _, buf := range buffers {
			refs = append(refs, buf.refs...)
		}
	}
	return symbols, refs
}

// collectIndexDataSharded threads the per-file shard cache through
// the same bounded worker-pool shape that collectIndexDataInternal uses.
// cacheDir must be non-empty; callers that want a pure-rebuild path
// should call collectIndexDataInternal directly.
//
// Returns the aggregated symbols, references, and a bloom filter
// unioned from every shard's per-shard bloom — both cache hits (decoded
// from the shard payload) and cache misses (built fresh from the
// file's references and persisted back). A nil bloom means no shard
// contributed any references, and callers should treat it as "no
// prebuilt filter"; the rebuild path will create one.
func collectIndexDataSharded(cacheDir string, files []*File, javaFiles []*File, xmlFiles []*xmlCacheFile, workers int, tracker perf.Tracker) ([]Symbol, []Reference, *bloom.BloomFilter) {
	if workers < 1 {
		workers = 1
	}
	// One packStore per scan. Nil when cacheDir == ""; store methods
	// tolerate nil receivers to match the pre-pack fs-backend shape.
	store := newPackStore(cacheDir)
	var (
		symbols       []Symbol
		refs          []Reference
		aggBloom      *bloom.BloomFilter
		bloomMu       sync.Mutex
		pendingWrites []encodedShardWrite
		pendingMu     sync.Mutex
	)

	mergeBloom := func(bf *bloom.BloomFilter) {
		if bf == nil {
			return
		}
		bloomMu.Lock()
		if aggBloom == nil {
			aggBloom = newShardBloom()
		}
		_ = aggBloom.Merge(bf)
		bloomMu.Unlock()
	}

	collectShardJob := func(job shardJob) (syms []Symbol, refs []Reference, shardBf *bloom.BloomFilter) {
		if s, ok := store.LoadShard(job.Path, job.ContentHash); ok {
			syms, refs = s.Symbols, s.References
			// Cache hit: decode the persisted bloom. A decode
			// failure falls back to rebuilding from refs so the
			// aggregate is never missing a name — correctness
			// beats a single-shard perf win.
			if bf, err := decodeShardBloom(s.Bloom); err == nil && bf != nil {
				shardBf = bf
			} else if len(refs) > 0 {
				shardBf = buildShardBloomFromRefs(refs)
			}
		} else {
			syms, refs = job.Fresh()
			shardBf = buildShardBloomFromRefs(refs)
			encoded, _ := encodeShardBloom(shardBf)
			blob, err := encodeShardBlob(&fileShard{
				Version:     crossFileShardVersion,
				Path:        job.Path,
				ContentHash: job.ContentHash,
				Symbols:     syms,
				References:  refs,
				Bloom:       encoded,
			})
			if err == nil {
				pendingMu.Lock()
				pendingWrites = append(pendingWrites, encodedShardWrite{
					key:  shardKey(job.Path, job.ContentHash),
					blob: blob,
				})
				pendingMu.Unlock()
			}
		}
		return syms, refs, shardBf
	}

	collectShardJobsByWorker := func(jobs []shardJob, symbolsPerJob, refsPerJob int) []indexDataBuffer {
		workerCount := workerCountForJobs(workers, len(jobs))
		if workerCount == 0 {
			return nil
		}
		buffers := newIndexDataBuffers(workerCount, len(jobs), symbolsPerJob, refsPerJob)
		jobCh := make(chan shardJob)

		var wg sync.WaitGroup
		for workerID := 0; workerID < workerCount; workerID++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				buf := &buffers[workerID]
				for job := range jobCh {
					syms, fileRefs, shardBf := collectShardJob(job)
					mergeBloom(shardBf)
					if len(syms) > 0 {
						buf.symbols = append(buf.symbols, syms...)
					}
					if len(fileRefs) > 0 {
						buf.refs = append(buf.refs, fileRefs...)
					}
				}
			}(workerID)
		}
		for _, job := range jobs {
			jobCh <- job
		}
		close(jobCh)
		wg.Wait()
		return buffers
	}

	runPhase := func(label string, jobs []shardJob, symbolsPerJob, refsPerJob int) {
		run := func() {
			buffers := collectShardJobsByWorker(jobs, symbolsPerJob, refsPerJob)
			symbols, refs = appendIndexDataBuffers(symbols, refs, buffers)
		}
		if tracker != nil && tracker.IsEnabled() {
			_ = tracker.Track(label, func() error { run(); return nil })
		} else {
			run()
		}
	}

	kotlinJobs := make([]shardJob, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		file := f
		kotlinJobs = append(kotlinJobs, shardJob{
			Path:        file.Path,
			ContentHash: contentHashForFile(file.Path, file.Content),
			Fresh:       func() ([]Symbol, []Reference) { return indexFile(file) },
		})
	}
	runPhase("kotlinIndexCollection", kotlinJobs, localBufferKotlinSymbolPerJob, localBufferRefPerJob)

	javaJobs := make([]shardJob, 0, len(javaFiles))
	for _, f := range javaFiles {
		if f == nil {
			continue
		}
		file := f
		javaJobs = append(javaJobs, shardJob{
			Path:        file.Path,
			ContentHash: contentHashForFile(file.Path, file.Content),
			Fresh: func() ([]Symbol, []Reference) {
				var syms []Symbol
				var r []Reference
				collectJavaDeclarationsFlat(file, &syms)
				collectJavaReferencesFlat(file, &r)
				return syms, r
			},
		})
	}
	runPhase("javaReferenceCollection", javaJobs, localBufferKotlinSymbolPerJob, localBufferRefPerJob)

	xmlJobs := make([]shardJob, 0, len(xmlFiles))
	for _, f := range xmlFiles {
		if f == nil {
			continue
		}
		file := f
		xmlJobs = append(xmlJobs, shardJob{
			Path:        file.Path,
			ContentHash: file.Hash,
			Fresh: func() ([]Symbol, []Reference) {
				var r []Reference
				appendXMLReferences(&r, file.Path, file.Content)
				return nil, r
			},
		})
	}
	runPhase("xmlReferenceCollection", xmlJobs, 0, localBufferRefPerJob)

	if len(pendingWrites) > 0 {
		writeShards := func() {
			_ = store.SaveEncodedShards(pendingWrites)
		}
		if tracker != nil && tracker.IsEnabled() {
			_ = tracker.Track("shardWrite", func() error {
				writeShards()
				return nil
			})
		} else {
			writeShards()
		}
	}

	return symbols, refs, aggBloom
}

// collectIndexDataInternal is the shared body. A non-nil preloadedXML
// skips the per-run XML disk walk and reuses the caller's read bytes;
// nil falls back to a fresh walk.
func collectIndexDataInternal(files []*File, workers int, tracker perf.Tracker, preloadedXML []*xmlCacheFile, javaFiles ...*File) ([]Symbol, []Reference) {
	var (
		symbols []Symbol
		refs    []Reference
	)
	if workers < 1 {
		workers = 1
	}

	collectKotlinByWorker := func(files []*File) []indexDataBuffer {
		workerCount := workerCountForJobs(workers, len(files))
		if workerCount == 0 {
			return nil
		}
		buffers := newIndexDataBuffers(workerCount, len(files), localBufferKotlinSymbolPerJob, localBufferRefPerJob)
		fileCh := make(chan *File)

		var wg sync.WaitGroup
		for workerID := 0; workerID < workerCount; workerID++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				buf := &buffers[workerID]
				for file := range fileCh {
					if file == nil {
						continue
					}
					syms, fileRefs := indexFile(file)
					buf.symbols = append(buf.symbols, syms...)
					buf.refs = append(buf.refs, fileRefs...)
				}
			}(workerID)
		}
		for _, file := range files {
			fileCh <- file
		}
		close(fileCh)
		wg.Wait()
		return buffers
	}

	collectJavaByWorker := func(files []*File) []indexDataBuffer {
		workerCount := workerCountForJobs(workers, len(files))
		if workerCount == 0 {
			return nil
		}
		buffers := newIndexDataBuffers(workerCount, len(files), localBufferKotlinSymbolPerJob, localBufferRefPerJob)
		fileCh := make(chan *File)

		var wg sync.WaitGroup
		for workerID := 0; workerID < workerCount; workerID++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				buf := &buffers[workerID]
				for file := range fileCh {
					if file == nil {
						continue
					}
					var javaSymbols []Symbol
					var javaRefs []Reference
					collectJavaDeclarationsFlat(file, &javaSymbols)
					collectJavaReferencesFlat(file, &javaRefs)
					buf.symbols = append(buf.symbols, javaSymbols...)
					buf.refs = append(buf.refs, javaRefs...)
				}
			}(workerID)
		}
		for _, file := range files {
			fileCh <- file
		}
		close(fileCh)
		wg.Wait()
		return buffers
	}

	runKotlin := func() {
		buffers := collectKotlinByWorker(files)
		symbols, refs = appendIndexDataBuffers(symbols, refs, buffers)
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("kotlinIndexCollection", func() error {
			runKotlin()
			return nil
		})
	} else {
		runKotlin()
	}

	// Index Java files for declarations and references.
	runJava := func() {
		buffers := collectJavaByWorker(javaFiles)
		symbols, refs = appendIndexDataBuffers(symbols, refs, buffers)
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("javaReferenceCollection", func() error {
			runJava()
			return nil
		})
	} else {
		runJava()
	}

	// Index XML files for class/name references (Android layouts, navigation, manifest).
	runXML := func() {
		if preloadedXML != nil {
			refs = append(refs, collectXmlReferencesFromLoaded(preloadedXML)...)
		} else {
			refs = append(refs, collectXmlReferences(files)...)
		}
	}
	if tracker != nil && tracker.IsEnabled() {
		_ = tracker.Track("xmlReferenceCollection", func() error {
			runXML()
			return nil
		})
	} else {
		runXML()
	}
	return symbols, refs
}

func buildCodeIndex(symbols []Symbol, refs []Reference) *CodeIndex {
	return buildCodeIndexWithBloom(symbols, refs, nil)
}

// buildCodeIndexWithBloom assembles the lookup maps and bloom filter. A
// non-nil prebuilt bloom replaces the per-reference AddString loop —
// prebuilt is adopted as-is and the loop only mutates the maps. The
// prebuilt filter must be a superset of the ref names; extra bits are
// fine (existing MayHaveReference callers already tolerate false
// positives) but missing bits would break IsReferencedOutsideFile
// short-circuits.
func buildCodeIndexWithBloom(symbols []Symbol, refs []Reference, prebuilt *bloom.BloomFilter) *CodeIndex {
	idx := &CodeIndex{
		Symbols:       symbols,
		References:    refs,
		symbolsByName: make(map[string][]Symbol),
		symbolsByFQN:  make(map[string]Symbol),
	}

	if prebuilt != nil {
		idx.refBloom = prebuilt
	} else {
		// Estimate bloom filter size: number of unique name+file pairs.
		estimatedRefs := uint(len(idx.References))
		if estimatedRefs < 1000 {
			estimatedRefs = 1000
		}
		idx.refBloom = bloom.NewWithEstimates(estimatedRefs, 0.01) // 1% false positive
	}

	for _, sym := range idx.Symbols {
		idx.symbolsByName[sym.Name] = append(idx.symbolsByName[sym.Name], sym)
		if sym.FQN != "" {
			idx.symbolsByFQN[sym.FQN] = sym
			if sym.FQN != sym.Name {
				idx.symbolsByName[sym.FQN] = append(idx.symbolsByName[sym.FQN], sym)
			}
		}
	}
	idx.buildReferenceLookups(prebuilt == nil)

	return idx
}

// SymbolsNamed returns declarations indexed under a simple or fully-qualified
// name. A nil result means no matching declaration is known.
func (idx *CodeIndex) SymbolsNamed(name string) []Symbol {
	if idx == nil || name == "" {
		return nil
	}
	return idx.symbolsByName[name]
}

// SymbolByFQN returns the declaration with the exact fully-qualified name.
func (idx *CodeIndex) SymbolByFQN(fqn string) (Symbol, bool) {
	if idx == nil || fqn == "" {
		return Symbol{}, false
	}
	sym, ok := idx.symbolsByFQN[fqn]
	return sym, ok
}

// ResolveType resolves a type name from a Kotlin or Java source file using
// source-visible package and import information plus declarations in the
// mixed-language CodeIndex.
func (idx *CodeIndex) ResolveType(file *File, name string) []ResolvedSymbol {
	if idx == nil || name == "" {
		return nil
	}
	imports, wildcards := sourceImports(file)
	if imported := imports[name]; imported != "" {
		return idx.resolveSymbols([]string{imported}, isTypeSymbol)
	}
	if strings.Contains(name, ".") {
		return idx.resolveSymbols([]string{name}, isTypeSymbol)
	}
	var candidates []string
	if !strings.Contains(name, ".") {
		if pkg := packageNameForFile(file); pkg != "" {
			candidates = append(candidates, pkg+"."+name)
		}
		for _, wildcard := range wildcards {
			candidates = append(candidates, wildcard+"."+name)
		}
	}
	resolved := idx.resolveSymbols(candidates, isTypeSymbol)
	if len(resolved) > 0 {
		return resolved
	}
	return idx.resolveSymbols([]string{name}, isTypeSymbol)
}

// ResolveCallable resolves a function/method/property callable from the mixed
// source index. arity < 0 disables arity filtering.
func (idx *CodeIndex) ResolveCallable(file *File, receiver, name string, arity int) []ResolvedSymbol {
	if idx == nil || name == "" {
		return nil
	}
	var ownerCandidates map[string]bool
	if receiver != "" {
		ownerCandidates = make(map[string]bool)
		for _, sym := range idx.ResolveType(file, receiver) {
			ownerCandidates[sym.FQN] = true
			ownerCandidates[sym.Symbol.Name] = true
		}
		if len(ownerCandidates) == 0 {
			ownerCandidates[receiver] = true
		}
	}
	imports, _ := sourceImports(file)
	if imported := imports[name]; imported != "" {
		return idx.resolveSymbols([]string{imported}, func(sym Symbol) bool {
			return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
		})
	}
	if strings.Contains(name, ".") {
		return idx.resolveSymbols([]string{name}, func(sym Symbol) bool {
			return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
		})
	}
	var candidates []string
	if pkg := packageNameForFile(file); pkg != "" {
		candidates = append(candidates, pkg+"."+name)
	}
	accept := func(sym Symbol) bool {
		return isCallableSymbol(sym) && callableMatches(sym, ownerCandidates, arity)
	}
	resolved := idx.resolveSymbols(candidates, accept)
	if len(resolved) > 0 {
		return resolved
	}
	return idx.resolveSymbols([]string{name}, accept)
}

func isTypeSymbol(sym Symbol) bool {
	switch sym.Kind {
	case "class", "interface", "object", "enum", "record", "annotation":
		return true
	default:
		return false
	}
}

func isCallableSymbol(sym Symbol) bool {
	switch sym.Kind {
	case "function", "method", "property", "field", "constructor":
		return true
	default:
		return false
	}
}

func callableMatches(sym Symbol, ownerCandidates map[string]bool, arity int) bool {
	if arity >= 0 && sym.Arity != arity {
		return false
	}
	if len(ownerCandidates) > 0 && !ownerCandidates[sym.Owner] {
		return false
	}
	return true
}

func (idx *CodeIndex) resolveSymbols(names []string, accept func(Symbol) bool) []ResolvedSymbol {
	seen := map[string]bool{}
	var out []ResolvedSymbol
	for _, name := range names {
		for _, sym := range idx.SymbolsNamed(name) {
			key := sym.FQN + "|" + sym.Signature + "|" + sym.File
			if seen[key] || !accept(sym) {
				continue
			}
			seen[key] = true
			out = append(out, ResolvedSymbol{
				FQN:      sym.FQN,
				Language: sym.Language,
				Owner:    sym.Owner,
				Kind:     sym.Kind,
				Symbol:   sym,
			})
		}
	}
	return out
}

func sourceImports(file *File) (map[string]string, []string) {
	explicit := map[string]string{}
	if file == nil {
		return explicit, nil
	}
	var wildcards []string
	for _, match := range sourceImportRe.FindAllStringSubmatch(string(file.Content), -1) {
		target := strings.TrimSpace(match[1])
		if target == "" {
			continue
		}
		if strings.HasSuffix(target, ".*") {
			wildcards = append(wildcards, strings.TrimSuffix(target, ".*"))
			continue
		}
		simple := target
		if alias := strings.TrimSpace(match[2]); alias != "" {
			simple = alias
		} else if dot := strings.LastIndex(target, "."); dot >= 0 {
			simple = target[dot+1:]
		}
		explicit[simple] = target
	}
	return explicit, wildcards
}

func (idx *CodeIndex) buildReferenceLookups(addToBloom bool) {
	if len(idx.References) == 0 {
		idx.refCountByName = make(map[string]int)
		idx.refFilesByName = make(map[string]map[string]bool)
		idx.nonCommentRefFilesByName = make(map[string]map[string]bool)
		idx.nonCommentRefCountByNameFile = make(map[string]map[string]int)
		return
	}

	estimatedNames := estimateUniqueReferenceNames(len(idx.References))
	nameToAgg := make(map[string]int, estimatedNames)
	aggs := make([]referenceAggregate, 0, estimatedNames)

	for _, ref := range idx.References {
		aggIdx, ok := nameToAgg[ref.Name]
		if !ok {
			aggIdx = len(aggs)
			nameToAgg[ref.Name] = aggIdx
			aggs = append(aggs, referenceAggregate{name: ref.Name})
		}
		aggs[aggIdx].add(ref)
	}

	idx.refCountByName = make(map[string]int, len(aggs))
	idx.refFilesByName = make(map[string]map[string]bool, len(aggs))
	idx.nonCommentRefFilesByName = make(map[string]map[string]bool, len(aggs))
	idx.nonCommentRefCountByNameFile = make(map[string]map[string]int, len(aggs))

	for i := range aggs {
		agg := &aggs[i]
		idx.refCountByName[agg.name] = agg.count
		idx.refFilesByName[agg.name] = agg.files
		if len(agg.nonCommentFiles) > 0 {
			idx.nonCommentRefFilesByName[agg.name] = agg.nonCommentFiles
			idx.nonCommentRefCountByNameFile[agg.name] = agg.nonCommentCounts
		}
		if addToBloom {
			idx.refBloom.AddString(agg.name)
		}
	}
}

type referenceAggregate struct {
	name             string
	count            int
	files            map[string]bool
	nonCommentFiles  map[string]bool
	nonCommentCounts map[string]int
}

func (a *referenceAggregate) add(ref Reference) {
	a.count++
	if a.files == nil {
		a.files = make(map[string]bool, 1)
	}
	a.files[ref.File] = true

	if ref.InComment {
		return
	}
	if a.nonCommentFiles == nil {
		a.nonCommentFiles = make(map[string]bool, 1)
		a.nonCommentCounts = make(map[string]int, 1)
	}
	a.nonCommentFiles[ref.File] = true
	a.nonCommentCounts[ref.File]++
}

func estimateUniqueReferenceNames(refCount int) int {
	if refCount <= 1024 {
		return refCount
	}
	estimated := refCount / 16
	if estimated < 1024 {
		return 1024
	}
	if estimated > 262144 {
		return 262144
	}
	return estimated
}

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) MayHaveReference(name string) bool {
	if idx == nil || idx.refBloom == nil {
		return false
	}
	return idx.refBloom.TestString(name)
}

// ReferenceCount returns how many times a name is referenced across all files.
func (idx *CodeIndex) ReferenceCount(name string) int {
	return idx.refCountByName[name]
}

// ReferenceFiles returns the set of files that reference a name.
func (idx *CodeIndex) ReferenceFiles(name string) map[string]bool {
	return idx.refFilesByName[name]
}

// SymbolReferenceCount returns the total number of references that can identify
// sym by either simple name or fully-qualified name.
func (idx *CodeIndex) SymbolReferenceCount(sym Symbol) int {
	if idx == nil {
		return 0
	}
	count := 0
	for _, name := range symbolReferenceNames(sym) {
		count += idx.ReferenceCount(name)
	}
	return count
}

// IsReferencedOutsideFile checks if a name is referenced in any file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFile(name, file string) bool {
	// Fast path: bloom filter says name not referenced at all
	if !idx.refBloom.TestString(name) {
		return false
	}
	files := idx.refFilesByName[name]
	if files == nil {
		return false
	}
	for f := range files {
		if f != file {
			return true
		}
	}
	return false
}

// IsSymbolReferencedOutsideFile checks whether sym is referenced from another
// file by either simple name or fully-qualified name.
func (idx *CodeIndex) IsSymbolReferencedOutsideFile(sym Symbol, ignoreCommentRefs bool) bool {
	if idx == nil {
		return false
	}
	for _, name := range symbolReferenceNames(sym) {
		if ignoreCommentRefs {
			if idx.IsReferencedOutsideFileExcludingComments(name, sym.File) {
				return true
			}
		} else if idx.IsReferencedOutsideFile(name, sym.File) {
			return true
		}
	}
	return false
}

// IsReferencedOutsideFileExcludingComments checks if a name has any non-comment
// reference in a file other than the given one.
func (idx *CodeIndex) IsReferencedOutsideFileExcludingComments(name, file string) bool {
	// Fast path: bloom filter says name not referenced at all
	if !idx.refBloom.TestString(name) {
		return false
	}
	files := idx.nonCommentRefFilesByName[name]
	if files == nil {
		return false
	}
	for f := range files {
		if f != file {
			return true
		}
	}
	return false
}

// CountNonCommentRefsInFile counts references to a name in a file that are NOT inside comments.
func (idx *CodeIndex) CountNonCommentRefsInFile(name, file string) int {
	files := idx.nonCommentRefCountByNameFile[name]
	if files == nil {
		return 0
	}
	return files[file]
}

func (idx *CodeIndex) countRefsInFileForSymbol(sym Symbol, file string, ignoreCommentRefs bool) int {
	if idx == nil {
		return 0
	}
	count := 0
	for _, name := range symbolReferenceNames(sym) {
		if ignoreCommentRefs {
			count += idx.CountNonCommentRefsInFile(name, file)
			continue
		}
		for _, ref := range idx.References {
			if ref.Name == name && ref.File == file {
				count++
			}
		}
	}
	return count
}

// UnusedSymbols returns symbols that are never referenced from any other file.
// If ignoreCommentRefs is true, references inside comments don't count as usage.
func (idx *CodeIndex) UnusedSymbols(ignoreCommentRefs bool) []Symbol {
	var unused []Symbol
	for _, sym := range idx.Symbols {
		if sym.IsOverride || sym.IsMain || sym.IsTest {
			continue
		}
		if sym.Visibility == "private" {
			continue // handled by single-file rules
		}

		// Check for references outside the declaring file
		hasExternalRef := idx.IsSymbolReferencedOutsideFile(sym, ignoreCommentRefs)

		if !hasExternalRef {
			// Check if referenced within its own file beyond the declaration itself
			localRefs := idx.countRefsInFileForSymbol(sym, sym.File, ignoreCommentRefs)
			// The declaration itself counts as 1 non-comment ref. If there are more, it's used locally.
			if localRefs > 1 {
				continue
			}
			unused = append(unused, sym)
		}
	}
	return unused
}

func symbolReferenceNames(sym Symbol) []string {
	if sym.Name == "" {
		return nil
	}
	if sym.FQN == "" || sym.FQN == sym.Name {
		return []string{sym.Name}
	}
	return []string{sym.Name, sym.FQN}
}

// BloomStats returns the bloom filter memory usage in bytes.
func (idx *CodeIndex) BloomStats() (refBits, crossBits uint) {
	if idx.refBloom != nil {
		refBits = idx.refBloom.Cap()
	}
	return
}

func indexFile(file *File) ([]Symbol, []Reference) {
	var symbols []Symbol
	var references []Reference

	if file == nil || file.FlatTree == nil || len(file.FlatTree.Nodes) == 0 {
		return symbols, references
	}

	collectDeclarationsFlat(file, &symbols)
	collectReferencesFlat(file, &references)

	return symbols, references
}

func collectDeclarationsFlat(file *File, symbols *[]Symbol) {
	pkg := packageNameForFile(file)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		switch nodeType {
		case "function_declaration":
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			sym := Symbol{
				Name:       name,
				Kind:       "function",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  symbolSignature(owner, name, kotlinFunctionArity(file, idx)),
				Arity:      kotlinFunctionArity(file, idx),
				IsOverride: file.FlatHasModifier(idx, "override"),
				IsMain:     name == "main",
			}
			sym.IsTest = strings.Contains(file.FlatNodeText(idx), "@Test")
			*symbols = append(*symbols, sym)
		case "class_declaration":
			name := file.FlatChildTextOrEmpty(idx, "type_identifier")
			if name == "" {
				name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
			}
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			kind := "class"
			text := file.FlatNodeText(idx)
			if strings.Contains(text, "interface ") {
				kind = "interface"
			}
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       kind,
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
			})
		case "object_declaration":
			name := file.FlatChildTextOrEmpty(idx, "type_identifier")
			if name == "" {
				name = file.FlatChildTextOrEmpty(idx, "simple_identifier")
			}
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "object",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
			})
		case "property_declaration":
			parent, ok := file.FlatParent(idx)
			if !ok {
				return
			}
			parentType := file.FlatType(parent)
			if parentType != "source_file" && parentType != "class_body" &&
				!(parentType == "statements" && hasFlatAncestorType(file, parent, "class_body")) {
				return
			}
			name := file.FlatChildTextOrEmpty(idx, "simple_identifier")
			if name == "" {
				if varDecl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
					name = file.FlatChildTextOrEmpty(varDecl, "simple_identifier")
				}
			}
			if name == "" || name == "_" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "property",
				Visibility: flatVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   file.Language,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  symbolSignature(owner, name, 0),
				Arity:      0,
				IsOverride: file.FlatHasModifier(idx, "override"),
			})
		}
	})
}

func collectJavaDeclarationsFlat(file *File, symbols *[]Symbol) {
	if file == nil || file.FlatTree == nil {
		return
	}
	pkg := javaPackageName(file)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		switch file.FlatType(idx) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			fqn := symbolFQN(pkg, owner, name)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       javaClassKind(file.FlatType(idx)),
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        fqn,
				Owner:      owner,
				Signature:  fqn,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
				IsStatic:   file.FlatHasModifier(idx, "static"),
				IsFinal:    file.FlatHasModifier(idx, "final"),
			})
		case "method_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			arity := javaFormalParameterArity(file, idx)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "method",
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        symbolFQN(pkg, owner, name),
				Owner:      owner,
				Signature:  symbolSignature(owner, name, arity),
				Arity:      arity,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
				IsStatic:   file.FlatHasModifier(idx, "static"),
				IsFinal:    file.FlatHasModifier(idx, "final"),
			})
		case "constructor_declaration", "compact_constructor_declaration":
			name := file.FlatChildTextOrEmpty(idx, "identifier")
			if name == "" {
				return
			}
			name = internString(name)
			owner := symbolOwner(file, idx, pkg)
			arity := javaFormalParameterArity(file, idx)
			*symbols = append(*symbols, Symbol{
				Name:       name,
				Kind:       "constructor",
				Visibility: javaVisibility(file, idx),
				File:       file.Path,
				Line:       file.FlatRow(idx) + 1,
				StartByte:  int(file.FlatStartByte(idx)),
				EndByte:    int(file.FlatEndByte(idx)),
				Language:   LangJava,
				Package:    pkg,
				FQN:        symbolFQN(pkg, owner, name),
				Owner:      owner,
				Signature:  symbolSignature(owner, name, arity),
				Arity:      arity,
				IsTest:     javaDeclarationHasTestAnnotation(file, idx),
			})
		case "field_declaration":
			collectJavaFieldDeclarations(file, idx, pkg, symbols)
		}
	})
}

func collectReferencesFlat(file *File, refs *[]Reference) {
	simpleID, hasSimpleID := lookupNodeType("simple_identifier")
	typeID, hasTypeID := lookupNodeType("type_identifier")
	if !hasSimpleID && !hasTypeID {
		return
	}
	commentTypes := make([]uint16, 0, 2)
	if lineCommentID, ok := lookupNodeType("line_comment"); ok {
		commentTypes = append(commentTypes, lineCommentID)
	}
	if multilineCommentID, ok := lookupNodeType("multiline_comment"); ok {
		commentTypes = append(commentTypes, multilineCommentID)
	}

	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatTree.Nodes[idx].Type
		if (!hasSimpleID || nodeType != simpleID) && (!hasTypeID || nodeType != typeID) {
			return
		}
		name := file.FlatNodeText(idx)
		if name == "" {
			return
		}
		*refs = append(*refs, Reference{
			Name:      name,
			File:      file.Path,
			Line:      file.FlatRow(idx) + 1,
			InComment: file.FlatHasAnyAncestorOfType(idx, commentTypes...),
		})
	})
}

func flatVisibility(file *File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "internal"):
		return "internal"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	default:
		return "public"
	}
}

func javaVisibility(file *File, idx uint32) string {
	switch {
	case file.FlatHasModifier(idx, "private"):
		return "private"
	case file.FlatHasModifier(idx, "protected"):
		return "protected"
	case file.FlatHasModifier(idx, "public"):
		return "public"
	default:
		return "package"
	}
}

func javaClassKind(nodeType string) string {
	switch nodeType {
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "record_declaration":
		return "record"
	case "annotation_type_declaration":
		return "annotation"
	default:
		return "class"
	}
}

func javaDeclarationHasTestAnnotation(file *File, idx uint32) bool {
	return strings.Contains(file.FlatNodeText(idx), "@Test")
}

func packageNameForFile(file *File) string {
	if file == nil {
		return ""
	}
	if file.Language == LangJava {
		return javaPackageName(file)
	}
	return kotlinPackageName(file)
}

func kotlinPackageName(file *File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var pkg string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if pkg != "" || file.FlatType(idx) != "package_header" {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "package")
		text = strings.TrimSpace(text)
		pkg = strings.TrimSuffix(text, ";")
	})
	return internString(strings.TrimSpace(pkg))
}

func javaPackageName(file *File) string {
	if file == nil || file.FlatTree == nil {
		return ""
	}
	var pkg string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if pkg != "" || file.FlatType(idx) != "package_declaration" {
			return
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "package")
		text = strings.TrimSuffix(text, ";")
		pkg = strings.TrimSpace(text)
	})
	return internString(pkg)
}

func symbolOwner(file *File, idx uint32, pkg string) string {
	var names []string
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration", "annotation_type_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "type_identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "simple_identifier"); name != "" {
				names = append(names, name)
			}
		case "object_declaration":
			if name := file.FlatChildTextOrEmpty(parent, "type_identifier"); name != "" {
				names = append(names, name)
			} else if name := file.FlatChildTextOrEmpty(parent, "simple_identifier"); name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	owner := strings.Join(names, ".")
	if pkg != "" {
		owner = pkg + "." + owner
	}
	return internString(owner)
}

func symbolFQN(pkg, owner, name string) string {
	if name == "" {
		return ""
	}
	if owner != "" {
		return internString(owner + "." + name)
	}
	if pkg != "" {
		return internString(pkg + "." + name)
	}
	return name
}

func symbolSignature(owner, name string, arity int) string {
	prefix := owner
	if prefix == "" {
		prefix = "<package>"
	}
	return internString(prefix + "#" + name + "/" + strconv.Itoa(arity))
}

func kotlinFunctionArity(file *File, idx uint32) int {
	params, ok := file.FlatFindChild(idx, "function_value_parameters")
	if !ok {
		return 0
	}
	return countNamedChildrenOfType(file, params, "parameter")
}

func javaFormalParameterArity(file *File, idx uint32) int {
	params, ok := file.FlatFindChild(idx, "formal_parameters")
	if !ok {
		return 0
	}
	return countNamedChildrenOfType(file, params, "formal_parameter", "spread_parameter")
}

func countNamedChildrenOfType(file *File, parent uint32, nodeTypes ...string) int {
	want := make(map[string]bool, len(nodeTypes))
	for _, typ := range nodeTypes {
		want[typ] = true
	}
	count := 0
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if want[file.FlatType(child)] {
			count++
		}
	}
	return count
}

func collectJavaFieldDeclarations(file *File, idx uint32, pkg string, symbols *[]Symbol) {
	owner := symbolOwner(file, idx, pkg)
	file.FlatWalkAllNodes(idx, func(child uint32) {
		if file.FlatType(child) != "variable_declarator" {
			return
		}
		name := file.FlatChildTextOrEmpty(child, "identifier")
		if name == "" {
			return
		}
		name = internString(name)
		*symbols = append(*symbols, Symbol{
			Name:       name,
			Kind:       "field",
			Visibility: javaVisibility(file, idx),
			File:       file.Path,
			Line:       file.FlatRow(child) + 1,
			StartByte:  int(file.FlatStartByte(child)),
			EndByte:    int(file.FlatEndByte(child)),
			Language:   LangJava,
			Package:    pkg,
			FQN:        symbolFQN(pkg, owner, name),
			Owner:      owner,
			Signature:  symbolSignature(owner, name, 0),
			Arity:      0,
			IsStatic:   file.FlatHasModifier(idx, "static"),
			IsFinal:    file.FlatHasModifier(idx, "final"),
		})
	})
}

func hasFlatAncestorType(file *File, idx uint32, want string) bool {
	for current, ok := file.FlatParent(idx); ok; current, ok = file.FlatParent(current) {
		if file.FlatType(current) == want {
			return true
		}
	}
	return false
}

func collectJavaReferencesFlat(file *File, refs *[]Reference) {
	if file == nil {
		return
	}
	if file.ReferencesPrecomputed {
		*refs = append(*refs, file.PrecomputedReferences...)
		return
	}
	collectJavaReferencesFlatUncached(file, refs)
}

func collectJavaReferencesFlatUncached(file *File, refs *[]Reference) {
	if file == nil || file.FlatTree == nil {
		return
	}
	identifierID, hasIdentifier := lookupNodeType("identifier")
	typeIdentifierID, hasTypeIdentifier := lookupNodeType("type_identifier")
	scopedIDs := lookupExistingNodeTypes("scoped_identifier", "scoped_type_identifier")
	if !hasIdentifier && !hasTypeIdentifier && len(scopedIDs) == 0 {
		return
	}
	nodes := file.FlatTree.Nodes
	for i := range nodes {
		node := nodes[i]
		isSimple := (hasIdentifier && node.Type == identifierID) || (hasTypeIdentifier && node.Type == typeIdentifierID)
		isScoped := hasNodeType(scopedIDs, node.Type)
		if !isSimple && !isScoped {
			continue
		}
		idx := uint32(i)
		name := FlatNodeText(file.FlatTree, idx, file.Content)
		if name == "" {
			continue
		}
		if isScoped && !strings.Contains(name, ".") {
			continue
		}
		*refs = append(*refs, Reference{
			Name: name,
			File: file.Path,
			Line: int(node.StartRow) + 1,
		})
		if isSimple {
			if prop := javaAccessorPropertyName(name); prop != "" {
				*refs = append(*refs, Reference{
					Name: prop,
					File: file.Path,
					Line: int(node.StartRow) + 1,
				})
			}
		}
	}
}

func javaAccessorPropertyName(name string) string {
	switch {
	case strings.HasPrefix(name, "get") && len(name) > len("get") && isASCIIUpper(name[len("get")]):
		return lowerASCIIInitial(name[len("get"):])
	case strings.HasPrefix(name, "set") && len(name) > len("set") && isASCIIUpper(name[len("set")]):
		return lowerASCIIInitial(name[len("set"):])
	case strings.HasPrefix(name, "is") && len(name) > len("is") && isASCIIUpper(name[len("is")]):
		return "is" + name[len("is"):]
	default:
		return ""
	}
}

func lowerASCIIInitial(value string) string {
	if value == "" {
		return ""
	}
	b := []byte(value)
	if b[0] >= 'A' && b[0] <= 'Z' {
		b[0] = b[0] - 'A' + 'a'
	}
	return string(b)
}

func isASCIIUpper(ch byte) bool {
	return ch >= 'A' && ch <= 'Z'
}

func lookupExistingNodeTypes(types ...string) []uint16 {
	out := make([]uint16, 0, len(types))
	for _, typ := range types {
		if id, ok := lookupNodeType(typ); ok {
			out = append(out, id)
		}
	}
	return out
}

func hasNodeType(types []uint16, typ uint16) bool {
	for _, candidate := range types {
		if candidate == typ {
			return true
		}
	}
	return false
}

// xmlCacheFile is a pre-loaded XML source whose content and hash are
// consumed by both the cross-file cache fingerprint and the reference
// walk, so each file is read from disk once.
type xmlCacheFile struct {
	Path    string
	Content []byte
	Hash    string
}

// collectXmlReferences scans for XML files in the project and extracts class name references.
// Android references Kotlin/Java classes from XML in: layouts, navigation graphs, manifest, etc.
func collectXmlReferences(ktFiles []*File) []Reference {
	return collectXmlReferencesFromLoaded(loadXMLFilesForCache(ktFiles))
}

// loadXMLFilesForCache walks the project for XML reference-candidate
// files, reads them, and hashes each. The result feeds both the cache
// fingerprint and the reference extraction in a single I/O pass.
func loadXMLFilesForCache(ktFiles []*File) []*xmlCacheFile {
	if len(ktFiles) == 0 {
		return nil
	}

	// Find project roots from kotlin file paths
	roots := make(map[string]bool)
	for _, f := range ktFiles {
		// Walk up to find src/ parent
		dir := filepath.Dir(f.Path)
		for dir != "/" && dir != "." {
			if filepath.Base(dir) == "src" {
				roots[filepath.Dir(dir)] = true
				break
			}
			dir = filepath.Dir(dir)
		}
	}

	// Walk each project root in its own goroutine. Roots are
	// independent subtrees (one per Gradle module, typically) so the
	// walks do not contend. Per-root results are appended under a
	// single mutex.
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []*xmlCacheFile
	)
	for r := range roots {
		wg.Add(1)
		go func(root string) {
			defer wg.Done()
			var local []*xmlCacheFile
			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					if info != nil && info.IsDir() {
						base := info.Name()
						if base == ".git" || base == "build" || base == "node_modules" ||
							base == ".idea" || base == ".gradle" || base == "out" ||
							base == ".kotlin" || base == "target" ||
							base == "third-party" || base == "third_party" ||
							base == "vendor" || base == "external" ||
							strings.HasPrefix(base, "values") {
							return filepath.SkipDir
						}
					}
					return nil
				}
				if !isXMLReferenceCandidate(path) {
					return nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				local = append(local, &xmlCacheFile{
					Path:    path,
					Content: content,
					Hash:    contentHashForFile(path, content),
				})
				return nil
			})
			if len(local) > 0 {
				mu.Lock()
				out = append(out, local...)
				mu.Unlock()
			}
		}(r)
	}
	wg.Wait()
	return out
}

func collectXmlReferencesFromLoaded(files []*xmlCacheFile) []Reference {
	if len(files) == 0 {
		return nil
	}
	var refs []Reference
	for _, f := range files {
		appendXMLReferences(&refs, f.Path, f.Content)
	}
	return refs
}

func isXMLReferenceCandidate(path string) bool {
	if !strings.HasSuffix(path, ".xml") {
		return false
	}
	base := filepath.Base(path)
	if base == "AndroidManifest.xml" {
		return true
	}
	dir := filepath.Base(filepath.Dir(path))
	switch {
	case strings.HasPrefix(dir, "layout"):
		return true
	case strings.HasPrefix(dir, "menu"):
		return true
	case strings.HasPrefix(dir, "navigation"):
		return true
	case dir == "xml":
		return true
	case strings.HasPrefix(dir, "values"):
		return false
	default:
		return false
	}
}

func appendXMLReferences(refs *[]Reference, path string, content []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		for _, re := range classRefPatterns {
			matches := re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				className := m[1]
				fullName := strings.TrimPrefix(className, ".")
				if fullName != "" && strings.Contains(fullName, ".") {
					*refs = append(*refs, Reference{
						Name: fullName,
						File: path,
						Line: lineNo,
					})
				}
				if idx := strings.LastIndex(className, "."); idx >= 0 {
					className = className[idx+1:]
				}
				className = strings.TrimPrefix(className, ".")
				if className == "" {
					continue
				}
				*refs = append(*refs, Reference{
					Name: className,
					File: path,
					Line: lineNo,
				})
			}
		}
	}
}
