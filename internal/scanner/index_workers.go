package scanner

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"

	"github.com/kaeawc/krit/internal/perf"
)

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

func buildKotlinShardJobs(files []*File) []shardJob {
	jobs := make([]shardJob, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		file := f
		jobs = append(jobs, shardJob{
			Path:        file.Path,
			ContentHash: contentHashForFile(file.Path, file.Content),
			Fresh:       func() ([]Symbol, []Reference) { return indexFile(file) },
		})
	}
	return jobs
}

func buildJavaShardJobs(javaFiles []*File) []shardJob {
	jobs := make([]shardJob, 0, len(javaFiles))
	for _, f := range javaFiles {
		if f == nil {
			continue
		}
		file := f
		jobs = append(jobs, shardJob{
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
	return jobs
}

func buildXMLShardJobs(xmlFiles []*xmlCacheFile) []shardJob {
	jobs := make([]shardJob, 0, len(xmlFiles))
	for _, f := range xmlFiles {
		if f == nil {
			continue
		}
		file := f
		jobs = append(jobs, shardJob{
			Path:        file.Path,
			ContentHash: file.Hash,
			Fresh: func() ([]Symbol, []Reference) {
				var r []Reference
				appendXMLReferences(&r, file.Path, file.Content)
				return nil, r
			},
		})
	}
	return jobs
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

	// NOTE: This is a long-lived worker-pool fan-out, not the per-item
	// fan-out shape errgroup.SetLimit handles cleanly. workerCount
	// goroutines pull jobs from jobCh and accumulate into their own
	// buffers[workerID] slot, so allocation pressure scales with N
	// workers (typically GOMAXPROCS), not N jobs (potentially thousands
	// of shards). Migrating to errgroup would require allocating a
	// fresh buffer per Go() call and merging M buffers afterward —
	// likely a hot-path regression. Refactor candidate only with a
	// before/after benchmark; for now the raw WaitGroup is the right
	// primitive.
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
			tracker.TrackVoid(label, run)
		} else {
			run()
		}
	}

	runPhase("kotlinIndexCollection", buildKotlinShardJobs(files), localBufferKotlinSymbolPerJob, localBufferRefPerJob)
	runPhase("javaReferenceCollection", buildJavaShardJobs(javaFiles), localBufferKotlinSymbolPerJob, localBufferRefPerJob)
	runPhase("xmlReferenceCollection", buildXMLShardJobs(xmlFiles), 0, localBufferRefPerJob)

	if len(pendingWrites) > 0 {
		writeShards := func() {
			_ = store.SaveEncodedShards(pendingWrites)
		}
		if tracker != nil && tracker.IsEnabled() {
			tracker.TrackVoid("shardWrite", writeShards)
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

	// NOTE: same worker-pool-with-per-worker-buffer shape as
	// collectShardJobsByWorker above; not migrated to errgroup for the
	// reasons documented there. See that comment for the trade-off.
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

	// NOTE: Java sibling of collectKotlinByWorker; same worker-pool
	// rationale for staying on raw WaitGroup. See collectShardJobsByWorker.
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
		tracker.TrackVoid("kotlinIndexCollection", runKotlin)
	} else {
		runKotlin()
	}

	// Index Java files for declarations and references.
	runJava := func() {
		buffers := collectJavaByWorker(javaFiles)
		symbols, refs = appendIndexDataBuffers(symbols, refs, buffers)
	}
	if tracker != nil && tracker.IsEnabled() {
		tracker.TrackVoid("javaReferenceCollection", runJava)
	} else {
		runJava()
	}

	// Index XML files for class/name references (Android layouts, navigation, manifest).
	runXML := func() {
		if preloadedXML != nil {
			refs = append(refs, collectXMLReferencesFromLoaded(preloadedXML)...)
		} else {
			refs = append(refs, collectXMLReferences(files)...)
		}
	}
	if tracker != nil && tracker.IsEnabled() {
		tracker.TrackVoid("xmlReferenceCollection", runXML)
	} else {
		runXML()
	}
	return symbols, refs
}
