package scanner

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/perf"
)

// AndroidCacheWriter defers SaveAndroidFindings calls onto a bounded
// background pool so the AndroidPhase doesn't block on disk writes
// between input units (manifests, gradle files, resource dirs). Mirrors
// internal/oracle.CacheWriter.
type AndroidCacheWriter struct {
	writer *cacheutil.AsyncWriter

	queued    atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
	bytes     atomic.Int64
	syncSaves atomic.Int64
}

// AndroidCacheWriterStats is a point-in-time snapshot of deferred Android
// findings cache writes.
type AndroidCacheWriterStats struct {
	Queued    int64
	Completed int64
	Failed    int64
	Bytes     int64
	SyncSaves int64
}

// NewAndroidCacheWriter starts a bounded writer for Android findings cache
// entries. Worker counts below one are clamped to one and counts above
// four are capped to keep cold persistence from competing too aggressively
// with rule execution.
func NewAndroidCacheWriter(workers int) *AndroidCacheWriter {
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	return &AndroidCacheWriter{
		writer: cacheutil.NewAsyncWriter(workers, workers*64),
	}
}

// Save queues a write of cols under key. When the writer is nil or the
// queue is saturated the save runs synchronously so cache persistence
// remains best-effort complete.
func (w *AndroidCacheWriter) Save(cacheDir, key string, cols FindingColumns) {
	if cacheDir == "" || key == "" {
		return
	}
	if w == nil {
		// Nil writers are silent no-ops. Useful for callers that haven't
		// opted into caching this run.
		return
	}
	if w.writer == nil {
		w.runSync(cacheDir, key, cols)
		return
	}
	w.queued.Add(1)
	job := func() (int64, error) {
		return w.runJob(cacheDir, key, cols), nil
	}
	if !w.writer.Submit(job) {
		w.runSync(cacheDir, key, cols)
	}
}

// runJob always returns 0 today; the int64 result is required by
// AsyncWriter.Submit's job signature (func() (int64, error)) where it
// represents bytes-written. SaveAndroidFindings doesn't expose a byte
// count, so the slot stays as a placeholder until that plumbing lands.
//
//nolint:unparam // (int64) result is part of the AsyncWriter job contract
func (w *AndroidCacheWriter) runJob(cacheDir, key string, cols FindingColumns) int64 {
	if err := SaveAndroidFindings(cacheDir, key, cols); err != nil {
		w.failed.Add(1)
		return 0
	}
	w.completed.Add(1)
	return 0
}

func (w *AndroidCacheWriter) runSync(cacheDir, key string, cols FindingColumns) {
	w.syncSaves.Add(1)
	if err := SaveAndroidFindings(cacheDir, key, cols); err != nil {
		w.failed.Add(1)
		return
	}
	w.completed.Add(1)
}

// Flush waits for queued writes to finish. A canceled context returns
// early, but the underlying writer continues draining in its goroutine.
func (w *AndroidCacheWriter) Flush(ctx context.Context) error {
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

// Close flushes synchronously.
func (w *AndroidCacheWriter) Close() error {
	return w.Flush(context.Background())
}

func (w *AndroidCacheWriter) Stats() AndroidCacheWriterStats {
	if w == nil {
		return AndroidCacheWriterStats{}
	}
	return AndroidCacheWriterStats{
		Queued:    w.queued.Load(),
		Completed: w.completed.Load(),
		Failed:    w.failed.Load(),
		Bytes:     w.bytes.Load(),
		SyncSaves: w.syncSaves.Load(),
	}
}

// AddPerfEntries records summary timings/counts on t when enabled.
func (w *AndroidCacheWriter) AddPerfEntries(t perf.Tracker) {
	if t == nil || !t.IsEnabled() {
		return
	}
	stats := w.Stats()
	perf.AddEntryDetails(t, "androidCacheWriterSummary", time.Duration(0), map[string]int64{
		"queued":    stats.Queued,
		"completed": stats.Completed,
		"failed":    stats.Failed,
		"syncSaves": stats.SyncSaves,
	}, nil)
}
