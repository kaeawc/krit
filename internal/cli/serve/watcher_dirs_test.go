package serve

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"
)

// dirRecordingState is a watcherState fake that records Invalidate paths and the directories reported gone.
type dirRecordingState struct {
	mu          sync.Mutex
	invalidated []string
	gone        []string
}

func (s *dirRecordingState) Invalidate(path string) {
	s.mu.Lock()
	s.invalidated = append(s.invalidated, path)
	s.mu.Unlock()
}
func (s *dirRecordingState) InvalidateAndroidProjectUnder(dir string) {
	s.mu.Lock()
	s.gone = append(s.gone, dir)
	s.mu.Unlock()
}
func (s *dirRecordingState) InvalidateCodeIndex()               {}
func (s *dirRecordingState) InvalidateDependents()              {}
func (s *dirRecordingState) InvalidateLibraryFacts()            {}
func (s *dirRecordingState) InvalidateResolver()                {}
func (s *dirRecordingState) InvalidateAndroidProjectFor(string) {}
func (s *dirRecordingState) Touch(string)                       {}
func (s *dirRecordingState) BumpSourceMTimeVersion()            {}
func (s *dirRecordingState) BumpJavaSourceVersion()             {}
func (s *dirRecordingState) BumpXMLFilesVersion()               {}

func (s *dirRecordingState) has(list *[]string, want string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range *list {
		if p == want {
			return true
		}
	}
	return false
}

func (s *dirRecordingState) anyUnder(dir string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.invalidated {
		if strings.HasPrefix(p, dir+string(filepath.Separator)) {
			return p
		}
	}
	return ""
}

func startDirWatcher(t *testing.T, root string, kind watcherBackendKind) (*fileWatcher, *dirRecordingState) {
	t.Helper()
	state := &dirRecordingState{}
	fw, err := startFileWatcherWithState(context.Background(), root, state, nil, withBackendKind(kind))
	if err != nil {
		t.Skipf("%v backend unavailable: %v", kind, err)
	}
	t.Cleanup(fw.Stop)
	if fw.backendKind != kind {
		t.Skipf("%v backend unavailable: fell back to %v", kind, fw.backendKind)
	}
	select {
	case <-fw.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher never finished its initial walk")
	}
	return fw, state
}

func eachBackend(t *testing.T, fn func(t *testing.T, kind watcherBackendKind)) {
	for _, kind := range []watcherBackendKind{kindFsnotify, kindFanotify} {
		t.Run(kind.String(), func(t *testing.T) { fn(t, kind) })
	}
}

// Source collection and Android detection walk with filepath.WalkDir, which does not follow a symlinked directory,
// so the watcher must not walk into one either. Walking a symlink root reported the link itself, which handle
// stat'ed as a directory and walked again, until the stack overflowed (#1911). A symlink to a source file is
// collected, so it is still reported.
func TestFileWatcher_DoesNotWalkSymlinkedDirectories(t *testing.T) {
	eachBackend(t, func(t *testing.T, kind watcherBackendKind) {
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "Outside.kt"), []byte("fun o() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		app := filepath.Join(root, "app")
		if err := os.MkdirAll(app, 0o755); err != nil {
			t.Fatal(err)
		}
		fw, state := startDirWatcher(t, root, kind)

		// Fail fast instead of growing the stack to its 1 GB default.
		defer debug.SetMaxStack(debug.SetMaxStack(32 << 20))

		linked := filepath.Join(app, "linked")
		loop := filepath.Join(app, "loop")
		linkedFile := filepath.Join(app, "Linked.kt")
		for link, target := range map[string]string{linked: outside, loop: ".", linkedFile: filepath.Join(outside, "Outside.kt")} {
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			fw.handle(backendEvent{Path: link, Op: opCreate})
		}
		// A directory moved in holding a symlink to a directory goes through touchExistingSources.
		staged := filepath.Join(t.TempDir(), "moved")
		if err := os.MkdirAll(staged, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(".", filepath.Join(staged, "self")); err != nil {
			t.Fatal(err)
		}
		moved := filepath.Join(app, "moved")
		if err := os.Rename(staged, moved); err != nil {
			t.Fatal(err)
		}
		fw.handle(backendEvent{Path: moved, Op: opCreate})

		if !waitForCondition(func() bool { return state.has(&state.invalidated, linkedFile) }) {
			t.Error("a symlink to a Kotlin file was not reported")
		}
		// Let the backend deliver its own events for the links too.
		time.Sleep(200 * time.Millisecond)
		for _, dir := range []string{linked, loop, filepath.Join(moved, "self")} {
			if p := state.anyUnder(dir); p != "" {
				t.Errorf("reported %s, under the symlinked directory %s", p, dir)
			}
		}
	})
}

// A directory moved out of the tree is reported gone whatever its name: package directories (com.example) and
// resource qualifiers (values-b+sr+Latn) contain dots. An extensionless file that leaves (gradlew, an editor's
// probe file) is not a directory and does not drop the cross-file state.
func TestFileWatcher_ReportsDottedDirectoryGone(t *testing.T) {
	eachBackend(t, func(t *testing.T, kind watcherBackendKind) {
		root := t.TempDir()
		pkg := filepath.Join(root, "app", "src", "main", "java", "com.example")
		if err := os.MkdirAll(pkg, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkg, "A.kt"), []byte("fun a() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		probe := filepath.Join(root, "app", "4913")
		if err := os.WriteFile(probe, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		_, state := startDirWatcher(t, root, kind)

		if err := os.Rename(pkg, filepath.Join(t.TempDir(), "gone")); err != nil {
			t.Fatal(err)
		}
		if !waitForCondition(func() bool { return state.has(&state.gone, pkg) }) {
			t.Error("a directory with a dot in its name left the tree unreported")
		}

		if err := os.Remove(probe); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
		if state.has(&state.gone, probe) {
			t.Error("removing an extensionless file was reported as a directory leaving")
		}
	})
}
