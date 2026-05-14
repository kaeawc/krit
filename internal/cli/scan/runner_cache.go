package scan

import (
	"fmt"
	"os"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

// close flushes parse / xml / resource caches and the oracle cache
// writer under the "cacheBackgroundFlush" tracker label, then stops the
// CPU profile and closes the krit-types daemon if one was launched.
//
// Idempotent: subsequent calls are no-ops. The cache flush ordering is
// load-bearing for the perfTiming JSON tree (parseCacheFlush →
// xmlParseCacheFlush → resourceCacheFlush → oracleCacheFlush, all
// nested under cacheBackgroundFlush).
func (r *runner) close() {
	if r == nil {
		return
	}
	r.flushCaches()
	stopCPUProfile(r.cpuProfileFile)
}

func (r *runner) flushCaches() {
	if r.cachesClosed {
		return
	}
	r.cachesClosed = true
	hasParseCacheWrites := r.sess.ParseCache != nil && r.sess.ParseCache.HasWrites()
	hasXMLParseCacheWrites := r.sess.XMLParseCache != nil && r.sess.XMLParseCache.HasWrites()
	hasResourceCacheWrites := r.sess.ResourceCache != nil && r.sess.ResourceCache.HasWrites()
	hasOracleCacheWrites := r.oracleCacheWriter != nil && r.oracleCacheWriter.Stats().Queued > 0
	hasAndroidCacheWrites := r.androidCacheWriter != nil && (r.androidCacheWriter.Stats().Queued+r.androidCacheWriter.Stats().SyncSaves) > 0
	if !hasParseCacheWrites && !hasXMLParseCacheWrites && !hasResourceCacheWrites && !hasOracleCacheWrites && !hasAndroidCacheWrites {
		r.closeIdleCacheWriters()
		return
	}
	cacheFlushTracker := r.tracker.Serial("cacheBackgroundFlush")
	if hasParseCacheWrites {
		r.flushParseCache(cacheFlushTracker)
	} else {
		r.closeIdleParseCache()
	}
	if hasXMLParseCacheWrites {
		r.flushXMLParseCache(cacheFlushTracker)
	} else {
		r.closeIdleXMLParseCache()
	}
	if hasResourceCacheWrites {
		r.flushResourceCache(cacheFlushTracker)
	} else {
		r.closeIdleResourceCache()
	}
	if hasOracleCacheWrites {
		r.flushOracleCache(cacheFlushTracker)
	}
	if hasAndroidCacheWrites {
		r.flushAndroidCache(cacheFlushTracker)
	}
	cacheFlushTracker.End()
}

func (r *runner) closeIdleCacheWriters() {
	r.closeIdleParseCache()
	r.closeIdleXMLParseCache()
	r.closeIdleResourceCache()
	if r.oracleCacheWriter != nil {
		_ = r.oracleCacheWriter.Close()
	}
	if r.androidCacheWriter != nil {
		_ = r.androidCacheWriter.Close()
	}
}

func (r *runner) closeIdleParseCache() {
	if r.sess.ParseCache != nil {
		_ = r.sess.ParseCache.CloseIdle()
	}
}

func (r *runner) closeIdleXMLParseCache() {
	if r.sess.XMLParseCache != nil {
		_ = r.sess.XMLParseCache.CloseIdle()
	}
}

func (r *runner) closeIdleResourceCache() {
	if r.sess.ResourceCache != nil {
		_ = r.sess.ResourceCache.CloseIdle()
	}
}

func (r *runner) flushParseCache(parent perf.Tracker) {
	if r.sess.ParseCache == nil {
		return
	}
	parseFlushTracker := parent.Serial("parseCacheFlush")
	closeErr := r.sess.ParseCache.Close()
	r.sess.ParseCache.AddPerfEntries(parseFlushTracker)
	parseFlushTracker.End()
	if closeErr != nil && *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: parse cache flush failed: %v\n", closeErr)
	}
}

func (r *runner) flushXMLParseCache(parent perf.Tracker) {
	if r.sess.XMLParseCache == nil {
		return
	}
	start := time.Now()
	if err := r.sess.XMLParseCache.Close(); err != nil && *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: xml parse cache flush failed: %v\n", err)
	}
	perf.AddEntry(parent, "xmlParseCacheFlush", time.Since(start))
}

func (r *runner) flushResourceCache(parent perf.Tracker) {
	if r.sess.ResourceCache == nil {
		return
	}
	start := time.Now()
	if err := r.sess.ResourceCache.Close(); err != nil && *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: resource cache flush failed: %v\n", err)
	}
	perf.AddEntry(parent, "resourceCacheFlush", time.Since(start))
}

func (r *runner) flushOracleCache(parent perf.Tracker) {
	oracleFlushTracker := parent.Serial("oracleCacheFlush")
	err := r.oracleCacheWriter.Close()
	r.oracleCacheWriter.AddPerfEntries(oracleFlushTracker, r.oracleStore != nil)
	oracleFlushTracker.End()
	if err != nil && *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: oracle cache flush failed: %v\n", err)
	}
}

func (r *runner) flushAndroidCache(parent perf.Tracker) {
	androidFlushTracker := parent.Serial("androidCacheFlush")
	err := r.androidCacheWriter.Close()
	r.androidCacheWriter.AddPerfEntries(androidFlushTracker)
	androidFlushTracker.End()
	if err != nil && *r.f.Verbose {
		fmt.Fprintf(os.Stderr, "verbose: android findings cache flush failed: %v\n", err)
	}
}
