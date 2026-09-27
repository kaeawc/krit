package mcp

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
)

func TestProjectAnalysisDoesNotWaitForFIRNotice(t *testing.T) {
	blocked := make(chan struct{})
	defer close(blocked)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Example.kt"), []byte("class Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(bufio.NewReader(nil), io.Discard)
	s.firNoticeCheck = func(context.Context, []string, *config.Config) error {
		<-blocked
		return nil
	}
	s.buildDispatcher()
	done := make(chan struct{})
	go func() {
		_ = s.analyzeProject(analyzeArgs{Paths: []string{dir}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("project analysis waited for FIR notice preflight")
	}
}

func TestFIRNoticeCacheUsesProjectAndCallerConfig(t *testing.T) {
	seen := make(chan bool, 4)
	dir := t.TempDir()
	withClasspath := filepath.Join(dir, "with.yml")
	withoutClasspath := filepath.Join(dir, "without.yml")
	if err := os.WriteFile(withClasspath, []byte("oracle:\n  classpath: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(withoutClasspath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(bufio.NewReader(nil), io.Discard)
	s.firNoticeCheck = func(_ context.Context, _ []string, cfg *config.Config) error {
		seen <- cfg.HasTopLevelOption("oracle", "classpath")
		return nil
	}
	s.firNoticeFor([]string{filepath.Join(dir, "one")}, withClasspath)
	s.firNoticeFor([]string{filepath.Join(dir, "one")}, withClasspath)
	s.firNoticeFor([]string{filepath.Join(dir, "one")}, withoutClasspath)
	s.firNoticeFor([]string{filepath.Join(dir, "two")}, withClasspath)
	var got []bool
	for len(got) < 3 {
		select {
		case value := <-seen:
			got = append(got, value)
		case <-time.After(2 * time.Second):
			t.Fatalf("expected three project/config checks, got %v", got)
		}
	}
	declared := 0
	for _, value := range got {
		if value {
			declared++
		}
	}
	if declared != 2 {
		t.Fatalf("caller config not passed through: %v", got)
	}
	select {
	case <-seen:
		t.Fatal("same project/config checked twice")
	case <-time.After(50 * time.Millisecond):
	}
}
