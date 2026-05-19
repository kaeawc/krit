package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestCacheRace hammers Cache.Files from many goroutines through the public
// read/write/prune surface (CheckFiles, UpdateEntryColumns, Prune) to catch
// unsynchronised map access under the race detector.
//
// Run with: go test -race ./internal/cache/ -count=1 -run TestCacheRace
func TestCacheRace(t *testing.T) {
	dir := t.TempDir()

	const numFiles = 32
	files := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%d.kt", i))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("fun f%d() {}\n", i)), 0o644); err != nil {
			t.Fatalf("seed file %d: %v", i, err)
		}
		files[i] = p
	}

	c := &Cache{
		RuleHash: "samehash",
		Files:    make(map[string]FileEntry),
	}

	// Pre-populate so readers and the pruner immediately have entries to
	// chew on. Without this the race window between insertion and prune
	// closes after a single iteration.
	for _, p := range files {
		cols := scanner.CollectFindings([]scanner.Finding{
			{File: p, Line: 1, Col: 1, Severity: "warning", RuleSet: "race", Rule: "Seed", Message: "seed"},
		})
		c.UpdateEntryColumns(p, &cols)
	}

	const (
		readers    = 8
		writers    = 4
		pruners    = 2
		iterations = 200
	)

	var wg sync.WaitGroup

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = c.CheckFiles(files, "samehash")
				_ = c.CheckFilesIncremental(files, files[:numFiles/2], "samehash")
				_ = c.MutatedSinceFlush()
			}
		}()
	}

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				p := files[(seed+i)%numFiles]
				cols := scanner.CollectFindings([]scanner.Finding{
					{File: p, Line: 1, Col: 1, Severity: "warning", RuleSet: "race", Rule: "W", Message: "w"},
				})
				c.UpdateEntryColumns(p, &cols)
				if i%16 == 0 {
					c.MarkFlushed()
				}
			}
		}(w)
	}

	for p := 0; p < pruners; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				c.Prune()
			}
		}()
	}

	// Concurrent Save: exercises the read path inside encodeBinary, which
	// iterates Files under the same RWMutex.
	wg.Add(1)
	go func() {
		defer wg.Done()
		savePath := filepath.Join(dir, "race.cache")
		for i := 0; i < iterations/4; i++ {
			_ = c.Save(savePath)
		}
	}()

	// Concurrent header writes: dispatch.writeCacheBack assigns Version,
	// RuleHash, and ScanPaths after each run. Without filesMu coverage of
	// the header fields, this races against Save/CheckFiles in the
	// daemon.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			c.SetHeader(
				fmt.Sprintf("v%d", i),
				"samehash",
				[]string{fmt.Sprintf("/scan/%d", i)},
			)
		}
	}()

	wg.Wait()
}
