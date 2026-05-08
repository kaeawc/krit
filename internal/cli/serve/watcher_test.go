package serve

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

// waitForCondition polls fn every 5ms up to 2s. Returns true when fn
// turns true; false on timeout. Used to bridge the async fsnotify
// event delivery without sleeping a fixed pessimistic interval.
func waitForCondition(fn func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

func TestFileWatcher_InvalidatesOnWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if got := ws.Stats().ParsedEntries; got != 1 {
		t.Fatalf("setup: got %d entries, want 1", got)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 42 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected workspace entry to be invalidated after write, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_InvalidatesOnRemove(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected workspace entry to be invalidated after remove, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_IgnoresNonKotlinFiles(t *testing.T) {
	root := t.TempDir()
	kt := filepath.Join(root, "Foo.kt")
	java := filepath.Join(root, "Bar.java")
	if err := os.WriteFile(kt, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write kt: %v", err)
	}
	if err := os.WriteFile(java, []byte("class Bar {}\n"), 0o644); err != nil {
		t.Fatalf("write java: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), kt, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	// Modifying the .java file must not invalidate the .kt entry.
	if err := os.WriteFile(java, []byte("class Bar2 {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite java: %v", err)
	}
	// Give the watcher a moment to process the event before asserting
	// it didn't act.
	time.Sleep(50 * time.Millisecond)
	if got := ws.Stats().ParsedEntries; got != 1 {
		t.Errorf("expected the .kt entry to survive a .java change, got %d entries", got)
	}
}

func TestFileWatcher_AddsNewSubdir(t *testing.T) {
	root := t.TempDir()
	ws := pipeline.NewWorkspaceState(root)

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	sub := filepath.Join(root, "newsub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Wait briefly so the watcher's Create handler picks up the new
	// directory and starts watching it.
	time.Sleep(50 * time.Millisecond)

	path := filepath.Join(sub, "New.kt")
	if err := os.WriteFile(path, []byte("fun n() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun n() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if err := os.WriteFile(path, []byte("fun n() { 1 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected new-subdir watch to invalidate on write, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_InvalidatesCodeIndexOnKotlinChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.CodeIndex("ci-fp", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	if !ws.CrossFileStats().HasCodeIndex {
		t.Fatal("setup: codeIndex slot should be populated")
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 1 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return !ws.CrossFileStats().HasCodeIndex }) {
		t.Errorf("expected codeIndex slot to clear after kotlin write, stats=%+v", ws.CrossFileStats())
	}
}

func TestFileWatcher_InvalidatesLibraryFactsOnGradleChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("// gradle\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("// gradle changed\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Both slots should clear: library facts because the gradle file
	// drove them, codeIndex because dependency changes can shift
	// every cross-file lookup.
	if !waitForCondition(func() bool {
		s := ws.CrossFileStats()
		return !s.HasLibraryFacts && !s.HasCodeIndex
	}) {
		t.Errorf("expected both cross-file slots to clear after gradle write, stats=%+v", ws.CrossFileStats())
	}
}

func TestFileWatcher_InvalidatesLibraryFactsOnVersionsTomlChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "libs.versions.toml")
	if err := os.WriteFile(path, []byte("[versions]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("[versions]\nfoo = \"1.0\"\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return !ws.CrossFileStats().HasLibraryFacts }) {
		t.Errorf("expected libraryFacts to clear after versions-catalog write, stats=%+v", ws.CrossFileStats())
	}
}

func TestIsLibraryConfigPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"build.gradle", true},
		{"build.gradle.kts", true},
		{"settings.gradle", true},
		{"settings.gradle.kts", true},
		{"libs.versions.toml", true},
		{"foo/libs.versions.toml", true},
		{"app/build.gradle.kts", true},
		{"Foo.kt", false},
		{"build.gradle.txt", false},
		{"versions.toml", false}, // doesn't end in .versions.toml
	}
	for _, tt := range tests {
		if got := isLibraryConfigPath(tt.path); got != tt.want {
			t.Errorf("isLibraryConfigPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsKotlinPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"Foo.kt", true},
		{"Foo.kts", true},
		{"Foo.java", false},
		{"Foo.txt", false},
		{"a/b/c.kt", true},
	}
	for _, tt := range tests {
		if got := isKotlinPath(tt.path); got != tt.want {
			t.Errorf("isKotlinPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
