package serve

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
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

// walkedUnder returns the walked directories at or under dir.
func (fw *fileWatcher) walkedUnder(dir string) []string {
	fw.dirsMu.Lock()
	defer fw.dirsMu.Unlock()
	var under []string
	for path := range fw.dirs {
		if path == dir || strings.HasPrefix(path, dir+string(filepath.Separator)) {
			under = append(under, path)
		}
	}
	return under
}

// A directory moved out of the tree delivers one event, for itself, and none for its subdirectories, so forgetting it
// must forget them too, or a long-lived daemon keeps every one it ever walked (#1965). A sibling whose name extends the
// moved one's stays.
func TestFileWatcher_ForgetsSubdirectoriesOfMovedDirectory(t *testing.T) {
	eachBackend(t, func(t *testing.T, kind watcherBackendKind) {
		root := t.TempDir()
		moved, sibling := filepath.Join(root, "a"), filepath.Join(root, "ab", "c")
		for _, dir := range []string{filepath.Join(moved, "b", "c"), filepath.Join(moved, "d"), sibling} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		fw, state := startDirWatcher(t, root, kind)
		if got := len(fw.walkedUnder(moved)); got != 4 {
			t.Fatalf("walked %d directories under a, want 4", got)
		}
		if err := os.Rename(moved, filepath.Join(t.TempDir(), "elsewhere")); err != nil {
			t.Fatal(err)
		}
		fw.handle(backendEvent{Path: moved, Op: opRename})
		if !state.has(&state.gone, moved) {
			t.Fatal("the moved directory was not reported gone")
		}
		if left := fw.walkedUnder(moved); len(left) != 0 {
			t.Errorf("still remembers %q after a was moved out", left)
		}
		if len(fw.walkedUnder(sibling)) != 1 {
			t.Errorf("forgot %s, which was not moved", sibling)
		}
	})
}

// walkedTree remembers a tree of fanout^depth leaf directories under root, as the initial walk does, and returns
// every directory deepest first, the order rm -rf deletes and reports them in.
func walkedTree(fw *fileWatcher, root string, fanout, depth int) []string {
	level := []string{root}
	all := []string{root}
	fw.rememberDir(root)
	for d := 0; d < depth; d++ {
		var next []string
		for _, parent := range level {
			for i := 0; i < fanout; i++ {
				child := filepath.Join(parent, "d"+strconv.Itoa(i))
				fw.rememberDir(child)
				next = append(next, child)
			}
		}
		all = append(all, next...)
		level = next
	}
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all
}

func newDirIndexWatcher() *fileWatcher {
	return &fileWatcher{dirs: map[string]struct{}{}, children: map[string]map[string]struct{}{}}
}

// rm -rf reports each directory itself, deepest first: every report is a walked directory, and afterwards neither the
// walked set nor its parent index holds anything.
func TestForgetDirDeepestFirstLeavesNothing(t *testing.T) {
	fw := newDirIndexWatcher()
	root := filepath.Join(string(filepath.Separator), "project", "src")
	for _, dir := range walkedTree(fw, root, 3, 3) {
		if !fw.forgetDir(dir) {
			t.Fatalf("%s was walked but forgetDir did not know it", dir)
		}
	}
	if len(fw.dirs) != 0 || len(fw.children) != 0 {
		t.Fatalf("after removing every directory: %d walked, %d parent entries left", len(fw.dirs), len(fw.children))
	}
	if fw.forgetDir(root) {
		t.Fatal("forgetDir knew a directory already forgotten")
	}
}

// BenchmarkForgetDirRmRf reports what rm -rf of a 4,681-directory tree costs the walked-directory index.
func BenchmarkForgetDirRmRf(b *testing.B) {
	root := filepath.Join(string(filepath.Separator), "project", "src")
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		fw := newDirIndexWatcher()
		dirs := walkedTree(fw, root, 8, 4)
		b.StartTimer()
		for _, dir := range dirs {
			fw.forgetDir(dir)
		}
	}
}
