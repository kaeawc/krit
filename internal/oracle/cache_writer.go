package oracle

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/store"
)

const freshOracleEntryBatchSize = 32

// OracleCacheWriter defers per-file oracle cache-entry writes out of the
// caller's critical path. It keeps entry-level counters on top of the generic
// async writer, whose stats are job-oriented.
type OracleCacheWriter struct {
	writer *cacheutil.AsyncWriter

	queued    atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	bytes     atomic.Int64

	poisonWrites         atomic.Int64
	depPaths             atomic.Int64
	contentHashNs        atomic.Int64
	closureFingerprintNs atomic.Int64
	marshalNs            atomic.Int64
	storePutNs           atomic.Int64
	atomicWriteNs        atomic.Int64
	packWrites           atomic.Int64

	uniqueMu       sync.Mutex
	uniqueDepPaths map[string]struct{}
}

// OracleCacheWriterStats is a point-in-time snapshot of deferred oracle cache
// writes. Durations are aggregate worker time, not wall-clock flush duration.
type OracleCacheWriterStats struct {
	Queued                 int64
	Completed              int64
	Failed                 int64
	Bytes                  int64
	PoisonWrites           int64
	DepPaths               int64
	UniqueDepPaths         int64
	ContentHashDuration    time.Duration
	ClosureFingerprintTime time.Duration
	MarshalDuration        time.Duration
	StorePutDuration       time.Duration
	AtomicWriteDuration    time.Duration
	PackWrites             int64
}

// NewOracleCacheWriter starts a bounded writer for oracle cache entries.
// Worker counts below one are clamped and counts above four are capped to keep
// cold-cache persistence from competing too aggressively with analysis work.
func NewOracleCacheWriter(workers int) *OracleCacheWriter {
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	return &OracleCacheWriter{
		writer:         cacheutil.NewAsyncWriter(workers, workers*256),
		uniqueDepPaths: map[string]struct{}{},
	}
}

// QueueFreshEntries queues fresh oracle cache entries. If the writer is nil,
// the existing synchronous path is used.
func (w *OracleCacheWriter) QueueFreshEntries(cacheDir string, fresh *OracleData, deps *CacheDepsFile) (int, error) {
	return w.QueueFreshEntriesToStoreScoped(nil, cacheDir, fresh, deps, "")
}

// QueueFreshEntriesToStore queues fresh oracle cache entries, writing to the
// unified store when s is non-nil. Queue saturation falls back to running that
// batch synchronously so cache persistence remains best-effort complete.
func (w *OracleCacheWriter) QueueFreshEntriesToStore(s *store.FileStore, cacheDir string, fresh *OracleData, deps *CacheDepsFile) (int, error) {
	return w.QueueFreshEntriesToStoreScoped(s, cacheDir, fresh, deps, "")
}

// QueueFreshEntriesToStoreScoped is QueueFreshEntriesToStore with cache
// compatibility metadata for filtered oracle call-target output.
func (w *OracleCacheWriter) QueueFreshEntriesToStoreScoped(s *store.FileStore, cacheDir string, fresh *OracleData, deps *CacheDepsFile, callFilterFingerprint string) (int, error) {
	if w == nil || w.writer == nil {
		return WriteFreshEntriesToStoreWithTrackerScoped(s, cacheDir, fresh, deps, nil, callFilterFingerprint)
	}
	jobs := freshOracleEntryJobs(fresh, deps)
	if len(jobs) == 0 {
		return 0, nil
	}

	w.queued.Add(int64(len(jobs)))
	memo := newOracleCacheHashMemo(len(jobs))
	if s == nil {
		batch := append([]freshOracleEntryJob(nil), jobs...)
		if !w.writer.Submit(func() (int64, error) {
			return w.writePackedBatch(cacheDir, memo, batch, callFilterFingerprint), nil
		}) {
			w.writePackedBatch(cacheDir, memo, batch, callFilterFingerprint)
		}
		return len(jobs), nil
	}
	for start := 0; start < len(jobs); start += freshOracleEntryBatchSize {
		end := start + freshOracleEntryBatchSize
		if end > len(jobs) {
			end = len(jobs)
		}
		batch := append([]freshOracleEntryJob(nil), jobs[start:end]...)
		if !w.writer.Submit(func() (int64, error) {
			return w.writeBatch(s, cacheDir, memo, batch, callFilterFingerprint), nil
		}) {
			w.writeBatch(s, cacheDir, memo, batch, callFilterFingerprint)
		}
	}
	return len(jobs), nil
}

// Flush waits for queued writes to finish. A canceled context returns early,
// but the underlying writer continues draining in its goroutine.
func (w *OracleCacheWriter) Flush(ctx context.Context) error {
	if w == nil || w.writer == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		done <- w.writer.Close()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *OracleCacheWriter) Close() error {
	return w.Flush(context.Background())
}

func (w *OracleCacheWriter) Stats() OracleCacheWriterStats {
	if w == nil {
		return OracleCacheWriterStats{}
	}
	w.uniqueMu.Lock()
	uniqueDepPaths := int64(len(w.uniqueDepPaths))
	w.uniqueMu.Unlock()
	return OracleCacheWriterStats{
		Queued:                 w.queued.Load(),
		Completed:              w.completed.Load(),
		Failed:                 w.failed.Load(),
		Bytes:                  w.bytes.Load(),
		PoisonWrites:           w.poisonWrites.Load(),
		DepPaths:               w.depPaths.Load(),
		UniqueDepPaths:         uniqueDepPaths,
		ContentHashDuration:    time.Duration(w.contentHashNs.Load()),
		ClosureFingerprintTime: time.Duration(w.closureFingerprintNs.Load()),
		MarshalDuration:        time.Duration(w.marshalNs.Load()),
		StorePutDuration:       time.Duration(w.storePutNs.Load()),
		AtomicWriteDuration:    time.Duration(w.atomicWriteNs.Load()),
		PackWrites:             w.packWrites.Load(),
	}
}

func (w *OracleCacheWriter) AddPerfEntries(t perf.Tracker, storeBacked bool) {
	if t == nil || !t.IsEnabled() {
		return
	}
	stats := w.Stats()
	perf.AddEntryDetails(t, "freshEntryContentHash", stats.ContentHashDuration, map[string]int64{"files": stats.Queued}, nil)
	perf.AddEntryDetails(t, "freshEntryClosureFingerprint", stats.ClosureFingerprintTime, map[string]int64{
		"entries":        stats.Queued - stats.PoisonWrites,
		"depPaths":       stats.DepPaths,
		"uniqueDepPaths": stats.UniqueDepPaths,
	}, nil)
	perf.AddEntryDetails(t, "freshEntryMarshal", stats.MarshalDuration, map[string]int64{"entries": stats.Completed, "bytes": stats.Bytes}, nil)
	if storeBacked {
		perf.AddEntryDetails(t, "freshEntryStorePut", stats.StorePutDuration, map[string]int64{"entries": stats.Completed}, nil)
	} else {
		perf.AddEntryDetails(t, "freshEntryAtomicWrite", stats.AtomicWriteDuration, map[string]int64{"entries": stats.Completed}, nil)
		perf.AddEntryDetails(t, "oraclePackWrite", stats.AtomicWriteDuration, map[string]int64{"packs": stats.PackWrites, "entries": stats.Completed}, nil)
	}
	perf.AddEntryDetails(t, "freshEntryPoisonWrites", 0, map[string]int64{"entries": stats.PoisonWrites}, nil)
	perf.AddEntryDetails(t, "freshEntrySummary", 0, map[string]int64{
		"queued":         stats.Queued,
		"written":        stats.Completed,
		"skipped":        stats.Failed,
		"bytes":          stats.Bytes,
		"depPaths":       stats.DepPaths,
		"uniqueDepPaths": stats.UniqueDepPaths,
		"poisonWrites":   stats.PoisonWrites,
	}, nil)
	perf.AddEntryDetails(t, "oracleCacheWriterSummary", 0, map[string]int64{
		"queued":     stats.Queued,
		"completed":  stats.Completed,
		"failed":     stats.Failed,
		"bytes":      stats.Bytes,
		"packWrites": stats.PackWrites,
	}, nil)
}

func (w *OracleCacheWriter) writeBatch(s *store.FileStore, cacheDir string, memo *oracleCacheHashMemo, batch []freshOracleEntryJob, callFilterFingerprint string) int64 {
	var bytes int64
	for _, job := range batch {
		if n, ok := w.writeOne(s, cacheDir, memo, job, callFilterFingerprint); ok {
			bytes += n
		}
	}
	return bytes
}

func (w *OracleCacheWriter) writePackedBatch(cacheDir string, memo *oracleCacheHashMemo, batch []freshOracleEntryJob, callFilterFingerprint string) int64 {
	writes := make([]oracleEncodedEntryWrite, 0, len(batch))
	var bytes int64
	var poisonWrites int64
	for _, job := range batch {
		entry, data, poison, ok := w.buildEntryData(memo, job, callFilterFingerprint)
		if !ok {
			continue
		}
		writes = append(writes, oracleEncodedEntryWrite{hash: entry.ContentHash, data: data})
		bytes += int64(len(data))
		if poison {
			poisonWrites++
		}
	}
	if len(writes) == 0 {
		return 0
	}
	writeStart := time.Now()
	err := writeEntriesData(cacheDir, writes)
	w.atomicWriteNs.Add(time.Since(writeStart).Nanoseconds())
	if err != nil {
		w.failed.Add(int64(len(writes)))
		return 0
	}
	w.completed.Add(int64(len(writes)))
	w.bytes.Add(bytes)
	w.poisonWrites.Add(poisonWrites)
	w.packWrites.Add(countOraclePackGroups(writes))
	return bytes
}

func (w *OracleCacheWriter) writeOne(s *store.FileStore, cacheDir string, memo *oracleCacheHashMemo, job freshOracleEntryJob, callFilterFingerprint string) (int64, bool) {
	entry, data, poison, ok := w.buildEntryData(memo, job, callFilterFingerprint)
	if !ok {
		return 0, false
	}

	writeStart := time.Now()
	var err error
	if s != nil {
		err = writeEntryDataToStore(s, entry, data)
		w.storePutNs.Add(time.Since(writeStart).Nanoseconds())
	} else {
		err = writeEntryData(cacheDir, entry, data)
		w.atomicWriteNs.Add(time.Since(writeStart).Nanoseconds())
		if err == nil {
			w.packWrites.Add(1)
		}
	}
	if err != nil {
		w.failed.Add(1)
		return 0, false
	}

	n := int64(len(data))
	w.completed.Add(1)
	w.bytes.Add(n)
	if poison {
		w.poisonWrites.Add(1)
	}
	return n, true
}

func (w *OracleCacheWriter) buildEntryData(memo *oracleCacheHashMemo, job freshOracleEntryJob, callFilterFingerprint string) (*CacheEntry, []byte, bool, bool) {
	hashStart := time.Now()
	hash, err := memo.contentHash(job.path)
	w.contentHashNs.Add(time.Since(hashStart).Nanoseconds())
	if err != nil {
		w.failed.Add(1)
		return nil, nil, false, false
	}

	entry := &CacheEntry{
		V:                     CacheVersion,
		ContentHash:           hash,
		FilePath:              job.path,
		Approximation:         job.approximation,
		CallFilterFingerprint: callFilterFingerprint,
	}
	if job.crashed {
		entry.Crashed = true
		entry.CrashError = job.crashError
	} else {
		w.recordDepPaths(job.depPaths)
		fpStart := time.Now()
		fp, err := closureFingerprintWithMemo(job.depPaths, memo)
		w.closureFingerprintNs.Add(time.Since(fpStart).Nanoseconds())
		if err != nil {
			w.failed.Add(1)
			return nil, nil, false, false
		}
		entry.FileResult = job.fileResult
		entry.PerFileDeps = job.perFileDeps
		entry.Closure = CacheClosure{DepPaths: job.depPaths, Fingerprint: fp}
	}

	marshalStart := time.Now()
	data, err := marshalCacheEntry(entry)
	w.marshalNs.Add(time.Since(marshalStart).Nanoseconds())
	if err != nil {
		w.failed.Add(1)
		return nil, nil, false, false
	}

	return entry, data, job.crashed, true
}

func (w *OracleCacheWriter) recordDepPaths(paths []string) {
	w.depPaths.Add(int64(len(paths)))
	if len(paths) == 0 {
		return
	}
	w.uniqueMu.Lock()
	for _, p := range paths {
		w.uniqueDepPaths[p] = struct{}{}
	}
	w.uniqueMu.Unlock()
}
