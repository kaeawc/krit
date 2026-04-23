package oracle

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

const daemonPoolMinMisses = 50

// DaemonPool is an opt-in set of persistent krit-types daemons for parallel
// warm miss analysis. Slot 0 intentionally uses the legacy single-daemon PID
// files; additional slots use {sourcesHash}.{slot}.{pid,port}.
type DaemonPool struct {
	Members   []*Daemon
	Requested int
	Connected int
	Started   int
}

func configuredDaemonPoolSize(misses int) int {
	if misses <= 1 {
		return 1
	}
	raw := strings.TrimSpace(os.Getenv("KRIT_DAEMON_POOL"))
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 1 {
		return 1
	}
	if n > misses {
		return misses
	}
	return n
}

func shouldUseDaemonPool(misses int, poolSize int) bool {
	return poolSize > 1 && misses >= daemonPoolMinMisses
}

// ConnectOrStartDaemonPool connects or starts each requested pool slot. It is
// deliberately separate from ConnectOrStartDaemon so the default path keeps
// exactly one daemon and the old PID-file layout.
func ConnectOrStartDaemonPool(jarPath string, sourceDirs []string, classpath []string, size int, verbose bool) (*DaemonPool, error) {
	if size < 1 {
		size = 1
	}
	pool := &DaemonPool{Requested: size, Members: make([]*Daemon, 0, size)}
	for slot := 0; slot < size; slot++ {
		d, err := connectExistingDaemonSlot(sourceDirs, verbose, slot)
		if err == nil {
			pool.Members = append(pool.Members, d)
			pool.Connected++
			continue
		}

		cleanStaleDaemonSlot(sourceDirs, verbose, slot)
		d, err = StartDaemonWithPortSlot(jarPath, sourceDirs, classpath, verbose, slot)
		if err != nil {
			_ = pool.Release()
			return nil, fmt.Errorf("start daemon pool slot %d: %w", slot, err)
		}
		pool.Members = append(pool.Members, d)
		pool.Started++
	}
	return pool, nil
}

func (p *DaemonPool) Release() error {
	if p == nil {
		return nil
	}
	var firstErr error
	for _, d := range p.Members {
		if d == nil {
			continue
		}
		if err := d.Release(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *DaemonPool) MatchesRepo(sourceDirs []string) bool {
	if p == nil || len(p.Members) == 0 {
		return false
	}
	for _, d := range p.Members {
		if d == nil || !d.MatchesRepo(sourceDirs) {
			return false
		}
	}
	return true
}

func (p *DaemonPool) AnalyzeWithDepsSharded(files []string, collectTimings bool, callFilter *CallTargetFilterSummary, declarationProfile *DeclarationProfileSummary, tracker perf.Tracker) (*OracleData, *CacheDepsFile, error) {
	if p == nil || len(p.Members) == 0 {
		return nil, nil, fmt.Errorf("daemon pool has no members")
	}
	groups := splitMissesForKAA(files, len(p.Members))
	if len(groups) == 0 {
		return mergeOracleData(), nil, nil
	}
	addOracleInstant(tracker, "daemonPoolAnalyzeSummary", map[string]int64{
		"poolSize": int64(len(p.Members)),
		"files":    int64(len(files)),
		"groups":   int64(len(groups)),
	}, nil)

	results := make([]shardResult, len(groups))
	var wg sync.WaitGroup
	for i, group := range groups {
		i, group := i, group
		member := p.Members[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			child := tracker
			if tracker != nil {
				child = tracker.Serial(fmt.Sprintf("daemonPoolMember/%d", i))
				defer child.End()
			}
			results[i].Files = len(group)
			start := time.Now()
			fresh, deps, kotlinTimings, err := member.AnalyzeWithDepsWithTimings(group, collectTimings, callFilter, declarationProfile)
			perf.AddEntryDetails(child, "daemonAnalyzeWithDeps", time.Since(start), map[string]int64{"files": int64(len(group))}, nil)
			if err != nil {
				results[i].Err = err
				return
			}
			if child != nil && len(kotlinTimings) > 0 {
				kt := child.Serial("kotlinTimings")
				perf.AddEntries(kt, kotlinTimings)
				kt.End()
			}
			results[i] = shardResult{
				Fresh: fresh,
				Deps:  deps,
				Files: len(group),
			}
		}()
	}
	wg.Wait()

	for i, result := range results {
		if result.Err != nil {
			return nil, nil, fmt.Errorf("daemon pool member %d failed: %w", i, result.Err)
		}
	}

	var fresh *OracleData
	if err := trackOracle(tracker, "mergeDaemonPoolOracleJSON", func() error {
		parts := make([]*OracleData, 0, len(results))
		for _, result := range results {
			parts = append(parts, result.Fresh)
		}
		fresh = mergeOracleData(parts...)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	var deps *CacheDepsFile
	if err := trackOracle(tracker, "mergeDaemonPoolCacheDeps", func() error {
		parts := make([]*CacheDepsFile, 0, len(results))
		for _, result := range results {
			parts = append(parts, result.Deps)
		}
		deps = mergeCacheDeps(parts...)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return fresh, deps, nil
}
