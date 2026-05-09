package scanner

import (
	"github.com/bits-and-blooms/bloom/v3"

	"github.com/kaeawc/krit/internal/perf"
)

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
	// StartByte/EndByte locate the identifier's text in File.Content. They
	// are populated for Kotlin and Java references; XML and other text-based
	// references leave them as 0 and are not safe to rewrite by offset.
	StartByte int
	EndByte   int
	Language  Language
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
	// carry dead shard data forever.
	(&packStore{cacheDir: cacheDir}).sweepLegacyShardsDir()

	// Pre-load XML files so fingerprint and reference extraction share
	// one disk walk. Also gives the cache a complete file-set snapshot.
	xmlFiles := loadXMLFilesForCache(files)
	entries := crossFileFingerprintEntries(files, javaFiles, xmlFiles)
	fingerprint := fingerprintCrossFileEntries(entries)

	// Warm path: full payload hit via unpackFull (includes lookup maps).
	if cachedIdx, ok := LoadCrossFileCacheIndex(cacheDir, fingerprint); ok {
		cachedIdx.Files = append(cachedIdx.Files, files...)
		cachedIdx.Fingerprint = fingerprint
		return cachedIdx, true
	}

	if idx, ok := buildIndexFromPriorOverlay(cacheDir, entries, files, javaFiles, xmlFiles, workers, tracker); ok {
		idx.Files = append(idx.Files, files...)
		idx.Fingerprint = fingerprint
		return idx, false
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
		Entries:     entries,
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

// BuildIndexIncremental returns base with the listed file contributions
// removed and the supplied fresh contributions added. It is used by the
// cross-file overlay cache so a small edit can update the lookup maps without
// rescanning unchanged files or rewriting the compacted full payload.
func BuildIndexIncremental(base *CodeIndex, removePaths map[string]bool, addSymbols []Symbol, addRefs []Reference) *CodeIndex {
	if base == nil {
		return BuildIndexFromData(addSymbols, addRefs)
	}
	if len(removePaths) > 0 {
		base.removeFileContributions(removePaths)
	}
	if len(addSymbols) > 0 {
		base.Symbols = append(base.Symbols, addSymbols...)
		base.rebuildSymbolLookups()
	}
	if len(addRefs) > 0 {
		base.References = append(base.References, addRefs...)
		for _, ref := range addRefs {
			base.addReferenceLookup(ref)
		}
	}
	return base
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
		tracker.TrackVoid("lookupMapBuild", func() {
			idx = build()
		})
		return idx
	}
	return build()
}

func collectIndexDataWithTracker(files []*File, workers int, tracker perf.Tracker, javaFiles ...*File) ([]Symbol, []Reference) {
	return collectIndexDataInternal(files, workers, tracker, nil, javaFiles...)
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

func (idx *CodeIndex) rebuildSymbolLookups() {
	idx.symbolsByName = make(map[string][]Symbol)
	idx.symbolsByFQN = make(map[string]Symbol)
	for _, sym := range idx.Symbols {
		idx.symbolsByName[sym.Name] = append(idx.symbolsByName[sym.Name], sym)
		if sym.FQN != "" {
			idx.symbolsByFQN[sym.FQN] = sym
			if sym.FQN != sym.Name {
				idx.symbolsByName[sym.FQN] = append(idx.symbolsByName[sym.FQN], sym)
			}
		}
	}
}

func (idx *CodeIndex) removeFileContributions(paths map[string]bool) {
	if len(paths) == 0 {
		return
	}
	if len(idx.Symbols) > 0 {
		dst := idx.Symbols[:0]
		for _, sym := range idx.Symbols {
			if !paths[sym.File] {
				dst = append(dst, sym)
			}
		}
		idx.Symbols = dst
		idx.rebuildSymbolLookups()
	}
	if len(idx.References) > 0 {
		dst := idx.References[:0]
		for _, ref := range idx.References {
			if paths[ref.File] {
				idx.removeReferenceLookup(ref)
				continue
			}
			dst = append(dst, ref)
		}
		idx.References = dst
	}
}
