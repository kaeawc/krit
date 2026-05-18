package scanner

import (
	"fmt"
	"sync"
	"testing"
)

// TestCodeIndexRace exercises concurrent reads against
// addReferenceLookup writes to catch unguarded mutations of the
// reference lookup maps and the underlying bloom filter. Runs as part
// of `go test -race ./internal/scanner/`; without the RWMutex on
// CodeIndex the race detector trips on map and bloom-filter access.
func TestCodeIndexRace(t *testing.T) {
	t.Parallel()

	const (
		seedNames   = 64
		writerCount = 4
		readerCount = 8
		opsPerLoop  = 256
	)

	symbols := make([]Symbol, 0, seedNames)
	refs := make([]Reference, 0, seedNames)
	for i := 0; i < seedNames; i++ {
		name := fmt.Sprintf("seedSym%03d", i)
		symbols = append(symbols, Symbol{
			Name:       name,
			Kind:       "function",
			Visibility: "public",
			File:       fmt.Sprintf("seed_%03d.kt", i),
			Line:       i + 1,
		})
		refs = append(refs, Reference{
			Name: name,
			File: fmt.Sprintf("seed_%03d.kt", i),
			Line: i + 1,
		})
	}
	idx := BuildIndexFromData(symbols, refs)

	var wg sync.WaitGroup
	wg.Add(writerCount + readerCount)

	for w := 0; w < writerCount; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerLoop; i++ {
				ref := Reference{
					Name: fmt.Sprintf("writer%d_sym%d", workerID, i%seedNames),
					File: fmt.Sprintf("writer%d_file%d.kt", workerID, i%16),
					Line: i + 1,
				}
				idx.addReferenceLookup(ref)
				if i%3 == 0 {
					idx.removeReferenceLookup(ref)
				}
			}
		}(w)
	}

	for r := 0; r < readerCount; r++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerLoop; i++ {
				name := fmt.Sprintf("seedSym%03d", i%seedNames)
				_ = idx.MayHaveReference(name)
				_ = idx.ReferenceCount(name)
				for file := range idx.ReferenceFiles(name) {
					_ = file
				}
				_ = idx.IsReferencedOutsideFile(name, "seed_000.kt")
				_ = idx.IsReferencedOutsideFileExcludingComments(name, "seed_000.kt")
				_ = idx.CountNonCommentRefsInFile(name, "seed_000.kt")
				writerName := fmt.Sprintf("writer%d_sym%d", workerID%writerCount, i%seedNames)
				_ = idx.MayHaveReference(writerName)
			}
		}(r)
	}

	wg.Wait()
}
