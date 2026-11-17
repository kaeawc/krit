// Package memtune caps GC headroom for large one-shot scans.
//
// With the default GOGC=100 the heap may grow to twice the live set before a
// collection. On a large project the live set is several GB, so the doubling
// dominates peak RSS. Collections are cheap relative to analysis work (a few
// percent of CPU), so this package lowers the GC percent as the live heap
// grows, keeping headroom near a fixed number of bytes instead.
//
// Inside a cgroup with a memory limit (a container or a build-system sandbox)
// the package also sets a Go soft memory limit below the cgroup limit, so the
// collector works harder before the kernel kills the process.
package memtune

import (
	"os"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/sysres"
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

// cgroupLimit is a seam over sysres.CgroupMemoryLimitBytes for tests.
var cgroupLimit = sysres.CgroupMemoryLimitBytes

// SoftLimitFor returns the Go soft memory limit for a cgroup memory limit of
// cgroupBytes: three quarters of it, or 0 when there is no cgroup limit. The
// remaining quarter covers tree-sitter's C allocations, goroutine stacks, and
// the JVMs krit starts in the same cgroup.
func SoftLimitFor(cgroupBytes uint64) int64 {
	return int64(cgroupBytes - cgroupBytes/4)
}

// Start adjusts the GC percent in the background as the live heap grows and,
// inside a memory-limited cgroup, sets a soft memory limit below the cgroup
// limit. GOMEMLIMIT, when set, replaces the derived soft limit (the runtime
// applies it; GOMEMLIMIT=off disables it), and GOGC or GOMEMLIMIT disable the
// GC percent adjuster, so explicit user tuning wins. The returned function
// stops the adjuster, restores the previous memory limit, and is safe to call
// more than once.
func Start() (stop func()) {
	restore := applyContainerLimit()
	var once sync.Once
	if os.Getenv("GOGC") != "" || os.Getenv("GOMEMLIMIT") != "" {
		return func() { once.Do(restore) }
	}
	done := make(chan struct{})
	go run(done)
	return func() {
		once.Do(func() {
			close(done)
			restore()
		})
	}
}

// applyContainerLimit sets the derived soft memory limit when GOMEMLIMIT is
// unset and the cgroup has a memory limit. It returns a function that restores
// the previous limit.
func applyContainerLimit() (restore func()) {
	if os.Getenv("GOMEMLIMIT") != "" {
		return func() {}
	}
	limit, ok := cgroupLimit()
	if !ok {
		return func() {}
	}
	soft := SoftLimitFor(limit)
	if soft <= 0 {
		return func() {}
	}
	prev := debug.SetMemoryLimit(soft)
	return func() { debug.SetMemoryLimit(prev) }
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
