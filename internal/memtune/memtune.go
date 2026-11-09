// Package memtune caps GC headroom for large one-shot scans.
//
// With the default GOGC=100 the heap may grow to twice the live set before a
// collection. On a large project the live set is several GB, so the doubling
// dominates peak RSS. Collections are cheap relative to analysis work (a few
// percent of CPU), so this package lowers the GC percent as the live heap
// grows, keeping headroom near a fixed number of bytes instead.
package memtune

import (
	"os"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"time"
)

const (
	// headroomBytes is the target heap growth allowed between collections
	// once the live heap is large.
	headroomBytes = 4 << 30
	// minPercent bounds how aggressive collection gets for very large heaps.
	minPercent = 20
	// maxPercent is the Go default; small heaps keep it.
	maxPercent = 100
	// percentStep quantizes the result so small live-heap changes do not
	// reconfigure the collector every tick.
	percentStep = 5
	pollEvery   = 250 * time.Millisecond
)

// PercentForLive returns the GC percent to use for a live heap of liveBytes.
func PercentForLive(liveBytes uint64) int {
	if liveBytes == 0 || liveBytes <= headroomBytes {
		return maxPercent
	}
	pct := int(uint64(headroomBytes) * 100 / liveBytes)
	pct -= pct % percentStep
	if pct < minPercent {
		return minPercent
	}
	if pct > maxPercent {
		return maxPercent
	}
	return pct
}

// Start adjusts the GC percent in the background as the live heap grows. It is
// a no-op when GOGC or GOMEMLIMIT is set, so explicit user tuning wins. The
// returned function stops the adjuster and is safe to call more than once.
func Start() (stop func()) {
	if os.Getenv("GOGC") != "" || os.Getenv("GOMEMLIMIT") != "" {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	go run(done)
	return func() { once.Do(func() { close(done) }) }
}

func run(done <-chan struct{}) {
	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	current := maxPercent
	t := time.NewTicker(pollEvery)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		metrics.Read(sample)
		if sample[0].Value.Kind() != metrics.KindUint64 {
			return
		}
		next := PercentForLive(sample[0].Value.Uint64())
		if next != current {
			debug.SetGCPercent(next)
			current = next
		}
	}
}
