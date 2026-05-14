package scan

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestNewSession_InitializesWorkspaceAndDefaults(t *testing.T) {
	dir := t.TempDir()
	sess, err := NewSession(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Close()

	if sess.Workspace == nil {
		t.Fatal("expected Workspace to be initialized")
	}
	if got := sess.Workspace.RepoRoot(); got != dir {
		t.Fatalf("Workspace.RepoRoot = %q, want %q", got, dir)
	}
	if sess.LibraryFacts == nil {
		t.Fatal("expected LibraryFacts to be initialized with defaults")
	}
	if sess.RepoDir() != dir {
		t.Fatalf("RepoDir = %q, want %q", sess.RepoDir(), dir)
	}
}

func TestSession_CloseIsIdempotent(t *testing.T) {
	sess, err := NewSession(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// Calling Close on a nil receiver must also be safe.
	var nilSess *Session
	if err := nilSess.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

// TestSession_CloseDrainsParseCacheWriter verifies that Close waits for
// background parse-cache jobs to finish, which is the contract the daemon
// relies on so a teardown does not race with in-flight cache persistence.
func TestSession_CloseDrainsParseCacheWriter(t *testing.T) {
	dir := t.TempDir()
	pc, err := scanner.NewParseCacheWithCap(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewParseCacheWithCap: %v", err)
	}
	writer := cacheutil.NewAsyncWriter(1, 4)
	pc.SetAsyncWriter(writer)

	var ran atomic.Bool
	if !writer.Submit(func() (int64, error) {
		time.Sleep(20 * time.Millisecond)
		ran.Store(true)
		return 0, nil
	}) {
		t.Fatal("AsyncWriter.Submit rejected the job")
	}

	sess, err := NewSession(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	sess.ParseCache = pc

	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !ran.Load() {
		t.Fatal("expected background job to run before Close returned")
	}
	stats := writer.Stats()
	if stats.Queued != stats.Completed+stats.Failed {
		t.Fatalf("AsyncWriter not drained: stats=%+v", stats)
	}
}
