package mcp

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// Code mode records the buffer's marked root too, so on a fresh server a file
// of a project checked out under a test/ folder is not taken for a test
// source, and the result does not depend on whether a project scan ran first
// (#1834).
func TestAnalyzeCodeSetsDispatcherProjectRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "test", "proj")
	if err := os.MkdirAll(filepath.Join(root, ".hg"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "app", "src", "main", "kotlin", "p", "Timeouts.kt")
	const code = "package p\n\nfun timeout(): Int {\n    val millis = 3000\n    return millis\n}\n"
	analyze := func(s *Server, path string) string {
		t.Helper()
		res := s.analyzeCode(analyzeArgs{Code: code, Path: path, Rules: []string{"MagicNumber"}})
		if res.IsError {
			t.Fatalf("code analysis failed: %+v", res)
		}
		var b strings.Builder
		for _, c := range res.Content {
			b.WriteString(c.Text)
		}
		return b.String()
	}
	fresh := func() *Server {
		s := NewServer(bufio.NewReader(nil), io.Discard)
		s.firNoticeCheck = func(context.Context, []string, *config.Config) error { return nil }
		s.buildDispatcher()
		return s
	}

	s := fresh()
	if got := analyze(s, path); !strings.Contains(got, "MagicNumber") {
		t.Fatalf("fresh server, code mode under a test/ ancestor: no MagicNumber finding:\n%s", got)
	}
	if got := s.analyzer.Dispatcher.ProjectRoot(); got != root {
		t.Fatalf("dispatcher root after code mode = %q, want %q", got, root)
	}
	analyze(s, "")
	if got := s.analyzer.Dispatcher.ProjectRoot(); got != "" {
		t.Fatalf("dispatcher root after code mode without a path = %q, want none", got)
	}
}
