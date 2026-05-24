package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAugmentDirtyWithStatDrift_DetectsAcrossSessionEdit is the
// regression guard for the across-session manifest/bundle staleness
// bug. Scenario the test reconstructs:
//
//  1. Session A analyzes a file with content X. Manifest persists
//     ContentHashes[F]=Hx and FileStats[F]=(sizeX, mtimeX). Bundle saved
//     keyed by RunFingerprint built from Hx.
//  2. Session A dies. File F is edited externally to content Y while
//     the daemon is down — new stat (sizeY, mtimeY).
//  3. Session B starts. The watcher's dirty set is empty (it hasn't
//     observed any events yet — the edit happened before startup).
//     host.PriorContentHashes still contains Hx from the persisted
//     manifest; host.PriorFileStats contains (sizeX, mtimeX).
//
// Without augmentDirtyWithStatDrift, computeRunFingerprint's
// priorOrCompute returns Hx (because dirty is nil/empty), producing a
// runFP that matches the stale bundle key. Stale findings get served.
//
// With the augmentation, F is added to the dirty set because its
// current stat doesn't match host.PriorFileStats[F], priorOrCompute
// recomputes the hash from the parsed content (Hy), runFP differs from
// the prior key, and the stale bundle is correctly skipped.
func TestAugmentDirtyWithStatDrift_DetectsAcrossSessionEdit(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "Drifted.kt")
	if err := os.WriteFile(path, []byte("package demo\nclass DriftedY\n"), 0o644); err != nil {
		t.Fatalf("write Drifted.kt: %v", err)
	}
	current, ok := statForPath(path)
	if !ok {
		t.Fatalf("stat current %s", path)
	}
	// Synthesize a prior stat that differs from the file's actual
	// stat — the across-session bug shape.
	priorStat := scanner.FileStat{
		Size:            current.Size + 17,
		ModTimeUnixNano: current.ModTimeUnixNano - int64(time.Second),
	}
	host := ProjectHostState{
		SourceSetDirty: nil, // watcher has observed no events yet
		PriorFileStats: map[string]scanner.FileStat{
			path: priorStat,
		},
	}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{
			Path:     path,
			Language: scanner.LangKotlin,
			Content:  []byte("package demo\nclass DriftedY\n"),
		}},
	}

	out := augmentDirtyWithStatDrift(host, parseResult)
	if len(out) != 1 || out[0] != path {
		t.Fatalf("expected drifted path %q in dirty set; got %v", path, out)
	}
}

// TestAugmentDirtyWithStatDrift_NoOpWhenStatMatches confirms the warm
// fast path: when PriorFileStats[path] matches the current on-disk
// stat, the path is NOT added to dirty. Without this, every analyze
// would force-recompute every hash on the daemon path, defeating the
// priorOrCompute optimization.
func TestAugmentDirtyWithStatDrift_NoOpWhenStatMatches(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "Stable.kt")
	if err := os.WriteFile(path, []byte("package demo\nclass Stable\n"), 0o644); err != nil {
		t.Fatalf("write Stable.kt: %v", err)
	}
	stat, ok := statForPath(path)
	if !ok {
		t.Fatalf("stat %s", path)
	}
	host := ProjectHostState{
		SourceSetDirty: nil,
		PriorFileStats: map[string]scanner.FileStat{
			path: stat,
		},
	}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{
			Path:     path,
			Language: scanner.LangKotlin,
			Content:  []byte("package demo\nclass Stable\n"),
		}},
	}

	out := augmentDirtyWithStatDrift(host, parseResult)
	if out != nil {
		t.Errorf("nil dirty + no drift should round-trip nil; got %v", out)
	}
}

// TestAugmentDirtyWithStatDrift_PreservesWatcherDirty pins the union
// behavior: a watcher-observed dirty path must still appear in the
// returned slice even when the stat-drift sweep finds no additional
// paths. Without this, switching to the augmented set would lose the
// watcher's events on warm same-session runs.
func TestAugmentDirtyWithStatDrift_PreservesWatcherDirty(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "A.kt")
	b := filepath.Join(tmp, "B.kt")
	if err := os.WriteFile(a, []byte("package demo\nclass A\n"), 0o644); err != nil {
		t.Fatalf("write A.kt: %v", err)
	}
	if err := os.WriteFile(b, []byte("package demo\nclass B\n"), 0o644); err != nil {
		t.Fatalf("write B.kt: %v", err)
	}
	statB, ok := statForPath(b)
	if !ok {
		t.Fatalf("stat B.kt")
	}
	host := ProjectHostState{
		SourceSetDirty: []string{a},
		PriorFileStats: map[string]scanner.FileStat{
			b: statB, // B's stat matches; only A is dirty per watcher
		},
	}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{
			{Path: a, Language: scanner.LangKotlin},
			{Path: b, Language: scanner.LangKotlin},
		},
	}

	out := augmentDirtyWithStatDrift(host, parseResult)
	if len(out) != 1 || out[0] != a {
		t.Errorf("expected watcher dirty preserved; got %v", out)
	}
}

// TestAugmentDirtyWithStatDrift_NilPriorIsCLINoOp pins the CLI-path
// contract: PriorFileStats=nil means "no prior session to compare
// against, trust the watcher's dirty set verbatim." CLI callers never
// populate PriorFileStats, so this case must short-circuit before
// reading any stats.
func TestAugmentDirtyWithStatDrift_NilPriorIsCLINoOp(t *testing.T) {
	host := ProjectHostState{
		SourceSetDirty: []string{"/some/path.kt"},
		PriorFileStats: nil,
	}
	parseResult := ParseResult{}

	out := augmentDirtyWithStatDrift(host, parseResult)
	if len(out) != 1 || out[0] != "/some/path.kt" {
		t.Errorf("nil PriorFileStats must pass SourceSetDirty through; got %v", out)
	}
}

// TestSourceSetFingerprint_FlipsAfterAugmentedDirty closes the loop on
// the across-session staleness fix: with stat-drifted paths added to
// the dirty set by augmentDirtyWithStatDrift, sourceSetFingerprint
// must produce a hash that reflects the file's CURRENT content rather
// than the stale prior. Without this, runFP would still match the
// prior bundle key and the daemon would serve stale findings.
//
// Drives sourceSetFingerprint directly: same call shape
// computeRunFingerprint uses, so this is the smallest scope where
// the priorOrCompute interaction is visible without spinning a full
// pipeline run.
func TestSourceSetFingerprint_FlipsAfterAugmentedDirty(t *testing.T) {
	currentContent := []byte("package demo\nclass X { fun a() = 999 }\n")
	path := "/virtual/X.kt"

	priorHashes := map[string]string{path: "STALE-HASH"}

	// Stale path: dirty empty → priorOrCompute returns prior[path].
	staleFP := sourceSetFingerprint(
		[]*scanner.File{{Path: path, Language: scanner.LangKotlin, Content: currentContent}},
		nil,
		priorHashes,
		nil, // dirty=nil → uses prior
	)

	// Fixed path: dirty contains path → priorOrCompute recomputes from
	// f.Content, returning a fresh hash that captures currentContent.
	freshDirty := map[string]bool{path: true}
	freshFP := sourceSetFingerprint(
		[]*scanner.File{{Path: path, Language: scanner.LangKotlin, Content: currentContent}},
		nil,
		priorHashes,
		freshDirty,
	)
	if staleFP == freshFP {
		t.Fatalf("expected stale and fresh fingerprints to differ; both = %s", staleFP)
	}
	// And freshFP must equal the no-prior recompute (i.e., what the
	// pipeline would produce on a cold run with the same currentContent).
	coldFP := sourceSetFingerprint(
		[]*scanner.File{{Path: path, Language: scanner.LangKotlin, Content: currentContent}},
		nil,
		nil,
		nil,
	)
	if freshFP != coldFP {
		t.Errorf("dirty=path should compute identical hash to no-prior cold run\nfresh=%s\n cold=%s", freshFP, coldFP)
	}
}

// TestAugmentDirtyWithStatDrift_MissingFileMarkedDirty covers the
// deletion case: a path that appears in PriorFileStats but whose
// os.Stat now fails (file gone) is flagged dirty so the prior content
// hash can't haunt the next manifest.
func TestAugmentDirtyWithStatDrift_MissingFileMarkedDirty(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "Gone.kt") // never written

	host := ProjectHostState{
		SourceSetDirty: nil,
		PriorFileStats: map[string]scanner.FileStat{
			missing: {Size: 42, ModTimeUnixNano: 1},
		},
	}
	parseResult := ParseResult{
		KotlinFiles: []*scanner.File{{
			Path:     missing,
			Language: scanner.LangKotlin,
		}},
	}

	out := augmentDirtyWithStatDrift(host, parseResult)
	if len(out) != 1 || out[0] != missing {
		t.Errorf("missing file should be flagged dirty; got %v", out)
	}
}
