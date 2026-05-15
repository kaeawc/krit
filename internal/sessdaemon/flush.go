package sessdaemon

import (
	"context"
	"time"
)

// defaultFlushInterval is the cadence at which the resident analysis
// cache is persisted to disk. 30s mirrors the scoping doc target: short
// enough that a SIGKILL between flushes loses only ~30s of work, long
// enough that the typical analyze->analyze->analyze workflow doesn't
// pay Save's cost on the hot path.
const defaultFlushInterval = 30 * time.Second

// flushLoop persists the resident *cache.Cache to disk every
// flushInterval, skipping the Save when nothing has changed since the
// last flush. Exits on ctx cancel; the final-save on shutdown lives in
// Stop so a SIGTERM right after a mutation still produces a durable
// cache.
func (s *Server) flushLoop(ctx context.Context) {
	defer s.flushWG.Done()
	t := time.NewTicker(s.flushInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.flushAnalysisCache()
		}
	}
}
