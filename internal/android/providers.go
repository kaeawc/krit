package android

import (
	"runtime"
	"sync"
	"time"
)

type resourceScanFunc func(string, int) (*ResourceIndex, ResourceScanStats, error)

// ResourceScanFuture memoizes a background resource scan for a single res/ dir.
type ResourceScanFuture struct {
	resDir       string
	maxWorkers   int
	startLimiter chan struct{}
	scan         resourceScanFunc

	once sync.Once
	done chan struct{}

	idx   *ResourceIndex
	stats ResourceScanStats
	err   error
	dur   time.Duration
}

func NewResourceScanFuture(resDir string, startLimiter chan struct{}, maxWorkers ...int) *ResourceScanFuture {
	workers := runtime.NumCPU()
	if len(maxWorkers) > 0 && maxWorkers[0] > 0 {
		workers = maxWorkers[0]
	}
	return &ResourceScanFuture{
		resDir:       resDir,
		maxWorkers:   workers,
		startLimiter: startLimiter,
		scan:         ScanResourceDirWithStatsWorkers,
		done:         make(chan struct{}),
	}
}

func NewLayoutScanFuture(resDir string, startLimiter chan struct{}, maxWorkers ...int) *ResourceScanFuture {
	f := NewResourceScanFuture(resDir, startLimiter, maxWorkers...)
	f.scan = ScanLayoutResourcesWithStatsWorkers
	return f
}

func NewValuesScanFuture(resDir string, startLimiter chan struct{}, kinds ValuesScanKind, maxWorkers ...int) *ResourceScanFuture {
	f := NewResourceScanFuture(resDir, startLimiter, maxWorkers...)
	f.scan = func(dir string, workers int) (*ResourceIndex, ResourceScanStats, error) {
		return ScanValuesResourcesWithStatsKindsWorkers(dir, workers, kinds)
	}
	return f
}

func (f *ResourceScanFuture) Start() {
	if f == nil {
		return
	}
	f.once.Do(func() {
		go func() {
			if f.startLimiter != nil {
				f.startLimiter <- struct{}{}
				defer func() { <-f.startLimiter }()
			}
			start := time.Now()
			scan := f.scan
			if scan == nil {
				scan = ScanResourceDirWithStatsWorkers
			}
			f.idx, f.stats, f.err = scan(f.resDir, f.maxWorkers)
			f.dur = time.Since(start)
			close(f.done)
		}()
	})
}

func (f *ResourceScanFuture) Await() (*ResourceIndex, ResourceScanStats, error) {
	if f == nil {
		return nil, ResourceScanStats{}, nil
	}
	f.Start()
	<-f.done
	return f.idx, f.stats, f.err
}

func (f *ResourceScanFuture) Duration() time.Duration {
	if f == nil {
		return 0
	}
	f.Start()
	<-f.done
	return f.dur
}

// IconScanFuture memoizes a background icon scan for a single res/ dir.
type IconScanFuture struct {
	resDir       string
	maxWorkers   int
	startLimiter chan struct{}

	once sync.Once
	done chan struct{}

	idx *IconIndex
	err error
	dur time.Duration
}

func NewIconScanFuture(resDir string, startLimiter chan struct{}, maxWorkers ...int) *IconScanFuture {
	workers := runtime.NumCPU()
	if len(maxWorkers) > 0 && maxWorkers[0] > 0 {
		workers = maxWorkers[0]
	}
	return &IconScanFuture{
		resDir:       resDir,
		maxWorkers:   workers,
		startLimiter: startLimiter,
		done:         make(chan struct{}),
	}
}

func (f *IconScanFuture) Start() {
	if f == nil {
		return
	}
	f.once.Do(func() {
		go func() {
			if f.startLimiter != nil {
				f.startLimiter <- struct{}{}
				defer func() { <-f.startLimiter }()
			}
			start := time.Now()
			f.idx, f.err = ScanIconDirsWithWorkers(f.resDir, f.maxWorkers)
			f.dur = time.Since(start)
			close(f.done)
		}()
	})
}

func (f *IconScanFuture) Await() (*IconIndex, error) {
	if f == nil {
		return nil, nil
	}
	f.Start()
	<-f.done
	return f.idx, f.err
}

func (f *IconScanFuture) Duration() time.Duration {
	if f == nil {
		return 0
	}
	f.Start()
	<-f.done
	return f.dur
}
