package scanner

import "sync/atomic"

// JavaIndexPerf aggregates Java parse/reference timings across parallel
// workers. Durations are stored in nanoseconds and emitted once by callers.
type JavaIndexPerf struct {
	Files       atomic.Int64
	Bytes       atomic.Int64
	CacheHits   atomic.Int64
	CacheMisses atomic.Int64

	FileReadNs            atomic.Int64
	ParseCacheLoadNs      atomic.Int64
	TreeSitterParseNs     atomic.Int64
	FlattenTreeNs         atomic.Int64
	QueueParseCacheSaveNs atomic.Int64
	ReferenceExtractionNs atomic.Int64
}

// JavaIndexPerfSnapshot is an immutable copy of JavaIndexPerf counters.
type JavaIndexPerfSnapshot struct {
	Files       int64
	Bytes       int64
	CacheHits   int64
	CacheMisses int64

	FileReadNs            int64
	ParseCacheLoadNs      int64
	TreeSitterParseNs     int64
	FlattenTreeNs         int64
	QueueParseCacheSaveNs int64
	ReferenceExtractionNs int64
}

// Snapshot returns a point-in-time copy of the aggregate counters.
func (p *JavaIndexPerf) Snapshot() JavaIndexPerfSnapshot {
	if p == nil {
		return JavaIndexPerfSnapshot{}
	}
	return JavaIndexPerfSnapshot{
		Files:                 p.Files.Load(),
		Bytes:                 p.Bytes.Load(),
		CacheHits:             p.CacheHits.Load(),
		CacheMisses:           p.CacheMisses.Load(),
		FileReadNs:            p.FileReadNs.Load(),
		ParseCacheLoadNs:      p.ParseCacheLoadNs.Load(),
		TreeSitterParseNs:     p.TreeSitterParseNs.Load(),
		FlattenTreeNs:         p.FlattenTreeNs.Load(),
		QueueParseCacheSaveNs: p.QueueParseCacheSaveNs.Load(),
		ReferenceExtractionNs: p.ReferenceExtractionNs.Load(),
	}
}
