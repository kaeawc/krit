package mcp

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

// A project scan records the scanned project's marked root on the shared
// dispatcher, so rules read the paths relative to it as a CLI scan does, and
// a later scan of an unmarked tree does not inherit that root (#1834).
func TestAnalyzeProjectSetsDispatcherProjectRoot(t *testing.T) {
	base := t.TempDir()
	marked := filepath.Join(base, "marked")
	unmarked := filepath.Join(base, "unmarked")
	for _, dir := range []string{filepath.Join(marked, ".hg"), unmarked} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{marked, unmarked} {
		if err := os.WriteFile(filepath.Join(dir, "Example.kt"), []byte("class Example\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := NewServer(bufio.NewReader(nil), io.Discard)
	s.firNoticeCheck = func(context.Context, []string, *config.Config) error { return nil }
	s.buildDispatcher()

	if res := s.analyzeProject(analyzeArgs{Paths: []string{marked}}); res.IsError {
		t.Fatalf("marked project scan failed: %+v", res)
	}
	if got := s.analyzer.Dispatcher.ProjectRoot(); got != marked {
		t.Fatalf("dispatcher root after a marked project scan = %q, want %q", got, marked)
	}
	if res := s.analyzeProject(analyzeArgs{Paths: []string{unmarked}}); res.IsError {
		t.Fatalf("unmarked project scan failed: %+v", res)
	}
	if got := s.analyzer.Dispatcher.ProjectRoot(); got != "" {
		t.Fatalf("dispatcher root after an unmarked project scan = %q, want none", got)
	}
}
