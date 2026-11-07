package serve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
)

// TestFileWatcher_StartupIsFastWithDeepTree asserts startFileWatcher
// returns quickly even when the project tree has many directories.
// The recursive add now runs in a background goroutine, so startup
// should be bounded by NewWatcher() + a single root Add().
func TestFileWatcher_StartupIsFastWithDeepTree(t *testing.T) {
	root := t.TempDir()
	// 200 nested+sibling directories. On a busy CI runner a sync
	// recursive Add of this is still fast, but the test exists to
	// keep the contract honest as the tree grows.
	for i := 0; i < 200; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%03d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	ws := pipeline.NewWorkspaceState(root)

	// Take the fastest of several starts so a load spike during one start
	// cannot fail the test; a sync walk would be slow on every attempt.
	const attempts = 5
	best := time.Duration(1<<63 - 1)
	var last <-chan struct{}
	for i := 0; i < attempts; i++ {
		start := time.Now()
		w, err := startFileWatcher(context.Background(), root, ws, nil)
		if err != nil {
			t.Fatalf("startFileWatcher: %v", err)
		}
		if d := time.Since(start); d < best {
			best = d
		}
		last = w.Ready()
		defer w.Stop()
	}

	// 100ms is generous on any developer/CI machine; if startup ever
	// regresses to a sync walk it'll blow well past this on every attempt.
	if best > 100*time.Millisecond {
		t.Errorf("startFileWatcher best of %d took %v; expected <100ms with async populate", attempts, best)
	}

	// Ready should fire within a reasonable window once the walk
	// completes — bounded loosely so a slow CI runner doesn't flake.
	select {
	case <-last:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher Ready() did not fire within 2s")
	}
}
