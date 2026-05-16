package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestWorkspaceState_XMLFiles_HitCacheUntilBump pins the version-
// counter contract: a cache hit short-circuits the build closure
// entirely; a watcher .xml bump rotates the counter and the next
// call rebuilds. Mirrors the JavaSourceIndex contract from #264 —
// the XML walk is the heavier of the two on Android-leaning corpora.
func TestWorkspaceState_XMLFiles_HitCacheUntilBump(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	build := func() []*scanner.XMLCacheFile {
		builds++
		return []*scanner.XMLCacheFile{{Path: "a.xml"}}
	}

	first := w.XMLFiles(build)
	if builds != 1 {
		t.Fatalf("first call: builds=%d, want 1", builds)
	}
	again := w.XMLFiles(build)
	if builds != 1 {
		t.Errorf("second call after no bump must hit cache; builds=%d, want 1", builds)
	}
	if &again[0] != &first[0] {
		t.Errorf("cached slice must return identical backing array; got fresh pointer")
	}

	w.BumpXMLFilesVersion()

	_ = w.XMLFiles(build)
	if builds != 2 {
		t.Errorf("post-bump call must rebuild; builds=%d, want 2", builds)
	}
}

// TestWorkspaceState_XMLFiles_ConcurrentBumpInvalidates mirrors the
// race semantics that motivate snapshotting the version BEFORE the
// build runs: a watcher event during the build must NOT result in a
// "clean" cached entry under a now-stale version.
func TestWorkspaceState_XMLFiles_ConcurrentBumpInvalidates(t *testing.T) {
	w := NewWorkspaceState("")

	files := w.XMLFiles(func() []*scanner.XMLCacheFile {
		w.BumpXMLFilesVersion()
		return []*scanner.XMLCacheFile{{Path: "a.xml"}}
	})
	if len(files) != 1 {
		t.Errorf("returned slice must come from the build call; got %v", files)
	}

	var builds int
	_ = w.XMLFiles(func() []*scanner.XMLCacheFile {
		builds++
		return files
	})
	if builds == 0 {
		t.Errorf("post-race call must rebuild (the stale version mustn't survive)")
	}
}

// TestWorkspaceState_XMLFiles_NilSafety mirrors the safety contract
// the rest of WorkspaceState's caches honor.
func TestWorkspaceState_XMLFiles_NilSafety(t *testing.T) {
	var w *WorkspaceState
	called := 0
	got := w.XMLFiles(func() []*scanner.XMLCacheFile {
		called++
		return []*scanner.XMLCacheFile{{Path: "a.xml"}}
	})
	if len(got) != 1 {
		t.Errorf("nil receiver must still return the build result; got %v", got)
	}
	if called != 1 {
		t.Errorf("nil receiver must call build; called=%d, want 1", called)
	}
	w.BumpXMLFilesVersion() // must not panic
}

// TestWorkspaceState_XMLFiles_InvalidateAllClears confirms InvalidateAll
// drops the XML slot alongside the other resident caches — symmetric
// with the codeIndexSnapshot / javaSourceIndex resets.
func TestWorkspaceState_XMLFiles_InvalidateAllClears(t *testing.T) {
	w := NewWorkspaceState("")
	var builds int
	build := func() []*scanner.XMLCacheFile {
		builds++
		return []*scanner.XMLCacheFile{{Path: "a.xml"}}
	}

	_ = w.XMLFiles(build)
	_ = w.XMLFiles(build)
	if builds != 1 {
		t.Fatalf("warm cache state: builds=%d, want 1", builds)
	}

	w.InvalidateAll()

	_ = w.XMLFiles(build)
	if builds != 2 {
		t.Errorf("InvalidateAll must drop the XML slot; builds=%d, want 2", builds)
	}
}
