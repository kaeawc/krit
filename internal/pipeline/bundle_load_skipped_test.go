package pipeline

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestBundleHit_SkipsLoadOnFormatCacheHit pins the fast-fast-path:
// when BundleOutput already has formatted bytes for the fingerprint,
// the bundle store's Load must NOT fire. Verifies the cache hit
// avoids the 30 MB zstd+gob decode that otherwise dominates the
// remaining daemon-side cost on a warm baseline.
func TestBundleHit_SkipsLoadOnFormatCacheHit(t *testing.T) {
	w := NewWorkspaceState("")

	// Seed the format cache so the first-and-only call should hit it.
	fp := scanner.RunFingerprint{Version: "test", Rules: "rh", Config: "cfg"}
	key := scanner.FindingsBundleKey(fp)
	w.StoreBundleOutput(key, &CachedBundleOutput{
		FindingsBytes: []byte(`[]`),
		Total:         0,
		ByRuleSet:     map[string]int{},
		ByRule:        map[string]int{},
	})

	store := &loadCountingStore{}

	args := ProjectArgs{
		Format:      "json",
		JSONCompact: true,
		ActiveRules: nil,
		Paths:       []string{"."},
	}
	host := ProjectHostState{
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: t.TempDir(),
		BundleOutput:            w.BundleOutput,
		StoreBundleOutput:       w.StoreBundleOutput,
	}

	// Invoke serveBundleHitFromOutputCache directly with cached=nil
	// to simulate the fast-fast-path's bypass of the bundle Load.
	var buf strings.Builder
	res, ok, err := serveBundleHitFromOutputCache(
		nil, fp, nil, nil, args, host, &buf,
		time.Time{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("serveBundleHitFromOutputCache: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true on format-cache hit with nil cached columns")
	}
	if got := store.loadCalls.Load(); got != 0 {
		t.Errorf("FindingsBundleStore.Load fired %d times on format-cache hit; want 0", got)
	}
	if res.FindingsCount != 0 {
		t.Errorf("FindingsCount: got %d, want 0 (matches cached output Total)", res.FindingsCount)
	}
	if !strings.Contains(buf.String(), `"findings":[]`) {
		t.Errorf("output missing cached findings bytes: %q", buf.String())
	}
}

type loadCountingStore struct {
	loadCalls atomic.Int64
}

func (s *loadCountingStore) Load(string, scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	s.loadCalls.Add(1)
	return nil, false
}

func (s *loadCountingStore) Save(string, scanner.RunFingerprint, *scanner.FindingColumns) error {
	return nil
}

// (time import needed by serveBundleHitFromOutputCache signature)
var _ = context.Background
