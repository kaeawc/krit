package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAliasBundleOutputCache_CopiesPriorEntry pins the path that
// makes structural-replay benefit from the BundleOutput cache: when
// the prior bundle was last formatted (so its key has a cached
// CachedBundleOutput), aliasing carries the same bytes forward under
// the new runFP. Without it the first warm+ABI cycle pays the full
// ~100 ms findings serialization cost even though the bytes already
// exist under a sibling key.
func TestAliasBundleOutputCache_CopiesPriorEntry(t *testing.T) {
	priorFP := scanner.RunFingerprint{Version: "v1", Rules: "r", Config: "r", SourceSet: "old"}
	newFP := scanner.RunFingerprint{Version: "v1", Rules: "r", Config: "r", SourceSet: "new"}
	priorKey := scanner.FindingsBundleKey(priorFP)
	newKey := scanner.FindingsBundleKey(newFP)
	if priorKey == newKey {
		t.Fatalf("test fixture broken: prior and new keys collide")
	}

	store := map[string]*CachedBundleOutput{
		priorKey: {FindingsBytes: []byte("cached bytes"), Total: 7},
	}
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(k string) *CachedBundleOutput { return store[k] },
			StoreBundleOutput: func(k string, v *CachedBundleOutput) { store[k] = v },
		},
	}

	aliasBundleOutputCache(host, priorFP, newFP)

	got := store[newKey]
	if got == nil {
		t.Fatalf("alias did not copy: store=%v", store)
	}
	if got.Total != 7 || string(got.FindingsBytes) != "cached bytes" {
		t.Errorf("alias content drifted from prior: %+v", got)
	}
}

// TestAliasBundleOutputCache_NoStoreIsNoOp pins the CLI-path
// contract: when BundleOutput / StoreBundleOutput aren't wired the
// helper exits cleanly without nil-deref.
func TestAliasBundleOutputCache_NoStoreIsNoOp(t *testing.T) {
	host := ProjectHostState{}
	aliasBundleOutputCache(host,
		scanner.RunFingerprint{SourceSet: "a"},
		scanner.RunFingerprint{SourceSet: "b"},
	)
	// success = no panic
}

// TestAliasBundleOutputCache_NoPriorEntryIsNoOp confirms the helper
// doesn't accidentally store nil under newKey when priorKey has no
// entry — would shadow a future legitimate cache write.
func TestAliasBundleOutputCache_NoPriorEntryIsNoOp(t *testing.T) {
	priorFP := scanner.RunFingerprint{SourceSet: "a"}
	newFP := scanner.RunFingerprint{SourceSet: "b"}
	store := map[string]*CachedBundleOutput{}
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(k string) *CachedBundleOutput { return store[k] },
			StoreBundleOutput: func(k string, v *CachedBundleOutput) { store[k] = v },
		},
	}
	aliasBundleOutputCache(host, priorFP, newFP)
	if len(store) != 0 {
		t.Errorf("alias must not write when prior has no entry; store=%v", store)
	}
}

// TestAliasBundleOutputCache_SameKeyNoSelfWrite covers the edge where
// the planner happens to leave runFP == prior.Fingerprint (delta
// planner accepted a no-op edit). Avoid the spurious StoreBundleOutput
// call to keep the cache-write counter clean for tests that assert on
// it.
func TestAliasBundleOutputCache_SameKeyNoSelfWrite(t *testing.T) {
	fp := scanner.RunFingerprint{SourceSet: "x"}
	stored := 0
	host := ProjectHostState{
		DaemonCaches: DaemonCaches{
			BundleOutput:      func(k string) *CachedBundleOutput { return &CachedBundleOutput{Total: 1} },
			StoreBundleOutput: func(k string, v *CachedBundleOutput) { stored++ },
		},
	}
	aliasBundleOutputCache(host, fp, fp)
	if stored != 0 {
		t.Errorf("same-key alias must not write; stored=%d", stored)
	}
}
