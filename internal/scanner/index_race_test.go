package scanner

import (
	"fmt"
	"sync"
	"testing"
)

// TestCodeIndex_ConcurrentReadWrite exercises CodeIndex's public lookup API
// under concurrent mutation. It must run cleanly under `go test -race` —
// without internal synchronization, concurrent writers (BuildIndexIncremental)
// racing with readers (MayHaveReference, ReferenceCount, ReferenceFiles,
// IsReferencedOutsideFile, SymbolsNamed, SymbolByFQN) tear bloom/map state.
//
// The pipeline currently serializes mutations, so this test defends the
// contract for future callers (background flush, parallel indexer) rather
// than reproducing a live bug.
func TestCodeIndex_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	const initialSymbols = 64
	const initialRefs = 256

	symbols := make([]Symbol, 0, initialSymbols)
	refs := make([]Reference, 0, initialRefs)
	for i := 0; i < initialSymbols; i++ {
		name := fmt.Sprintf("Sym%d", i)
		symbols = append(symbols, Symbol{
			Name:       name,
			Kind:       "function",
			Visibility: "public",
			File:       fmt.Sprintf("file_%d.kt", i),
			FQN:        "com.example." + name,
		})
	}
	for i := 0; i < initialRefs; i++ {
		refs = append(refs, Reference{
			Name: fmt.Sprintf("Sym%d", i%initialSymbols),
			File: fmt.Sprintf("ref_%d.kt", i%32),
		})
	}

	idx := BuildIndexFromData(symbols, refs)

	const readers = 8
	const writers = 2
	const opsPerGoroutine = 200

	var wg sync.WaitGroup

	// Readers: hit every public read path concurrently.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				name := fmt.Sprintf("Sym%d", (seed+i)%initialSymbols)
				_ = idx.MayHaveReference(name)
				_ = idx.ReferenceCount(name)
				for range idx.ReferenceFiles(name) {
				}
				_ = idx.IsReferencedOutsideFile(name, "ref_0.kt")
				_ = idx.IsReferencedOutsideFileExcludingComments(name, "ref_0.kt")
				_ = idx.CountNonCommentRefsInFile(name, "ref_0.kt")
				_ = idx.SymbolsNamed(name)
				_, _ = idx.SymbolByFQN("com.example." + name)
			}
		}(r)
	}

	// Writers: feed BuildIndexIncremental adds (the documented mutation path)
	// concurrently with readers. The pipeline today serializes this externally,
	// but the test asserts that the index's internal state is itself safe.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				name := fmt.Sprintf("NewSym%d_%d", seed, i)
				addSyms := []Symbol{{
					Name:       name,
					Kind:       "function",
					Visibility: "public",
					File:       fmt.Sprintf("added_%d_%d.kt", seed, i),
					FQN:        "com.example." + name,
				}}
				addRefs := []Reference{{
					Name: name,
					File: fmt.Sprintf("added_%d_%d.kt", seed, i),
				}}
				BuildIndexIncremental(idx, nil, addSyms, addRefs)
			}
		}(w)
	}

	wg.Wait()
}
