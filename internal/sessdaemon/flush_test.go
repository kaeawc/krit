package sessdaemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestFlushOnShutdownPersistsResidentCache covers the issue #206 crash
// test: mutate the resident cache, Stop the server, then reload the
// cache file from disk and assert the entry is present. The graceful-
// shutdown flush is what gives a SIGKILL-between-flushes daemon a
// usable starting state on the next cold run.
func TestFlushOnShutdownPersistsResidentCache(t *testing.T) {
	repo := t.TempDir()
	fileA := filepath.Join(repo, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, _ := buildTestServer(t, repo)
	if srv.session.AnalysisCache == nil {
		t.Fatal("session.AnalysisCache should be loaded at startup")
	}
	cols := scanner.CollectFindings([]scanner.Finding{
		{File: fileA, Line: 1, Col: 1, Rule: "X", Message: "m"},
	})
	srv.session.AnalysisCache.UpdateEntryColumns(fileA, &cols)

	cachePath := srv.session.AnalysisCacheFilePath
	if cachePath == "" {
		t.Fatal("session.AnalysisCacheFilePath should be set")
	}

	srv.Stop()
	srv.Wait()

	reloaded := cache.Load(cachePath)
	absA, _ := filepath.Abs(fileA)
	entry, ok := reloaded.Files[absA]
	if !ok {
		t.Fatalf("expected reloaded cache to contain %s; files=%v", absA, keys(reloaded.Files))
	}
	if entry.Columns.Len() != 1 {
		t.Fatalf("expected 1 cached row for %s, got %d", absA, entry.Columns.Len())
	}
}

// TestFlushOnShutdownSkipsWhenNotMutated verifies the "skip flush when
// no UpdateEntryColumns calls happened" guarantee. We snapshot the
// cache file's mtime, do a Stop without any mutation, and assert the
// file is unchanged (either still absent or still at the same mtime).
func TestFlushOnShutdownSkipsWhenNotMutated(t *testing.T) {
	repo := t.TempDir()

	srv, _ := buildTestServer(t, repo)
	cachePath := srv.session.AnalysisCacheFilePath
	if cachePath == "" {
		t.Skip("no cache path resolved for tempdir")
	}

	beforeStat, beforeErr := os.Stat(cachePath)

	srv.Stop()
	srv.Wait()

	afterStat, afterErr := os.Stat(cachePath)
	if beforeErr != nil && afterErr == nil {
		t.Fatal("Stop created a cache file when nothing was mutated")
	}
	if beforeErr == nil && afterErr == nil {
		if beforeStat.ModTime() != afterStat.ModTime() || beforeStat.Size() != afterStat.Size() {
			t.Fatal("Stop rewrote the cache file when nothing was mutated")
		}
	}
}

// TestPeriodicFlushPersistsMutation drives the 30s-flush goroutine on
// a short interval. The test sets srv.flushInterval before Start so
// the loop ticks within the test timeout, then waits for the cache
// file to reflect the mutation.
func TestPeriodicFlushPersistsMutation(t *testing.T) {
	repo := t.TempDir()
	fileA := filepath.Join(repo, "a.kt")
	if err := os.WriteFile(fileA, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sockDir, err := os.MkdirTemp("", "kritd")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	socket := filepath.Join(sockDir, "d.sock")

	srv, err := NewServer(context.Background(), Options{RepoDir: repo, SocketPath: socket})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.flushInterval = 50 * time.Millisecond
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(); srv.Wait() })

	cols := scanner.CollectFindings([]scanner.Finding{
		{File: fileA, Line: 1, Col: 1, Rule: "X", Message: "m"},
	})
	srv.session.AnalysisCache.UpdateEntryColumns(fileA, &cols)

	cachePath := srv.session.AnalysisCacheFilePath
	absA, _ := filepath.Abs(fileA)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		reloaded := cache.Load(cachePath)
		if _, ok := reloaded.Files[absA]; ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("periodic flush never persisted %s; cache=%s", absA, cachePath)
}

func keys(m map[string]cache.FileEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
