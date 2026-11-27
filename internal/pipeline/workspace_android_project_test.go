package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// An invalidation that lands while a detection runs may describe a file the walk already passed, so that detection
// is returned to its caller but not cached (#1911).
func TestAndroidProjectDropsADetectionAnInvalidationOverlapped(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "app", "src", "main", "AndroidManifest.xml")
	for _, invalidate := range []struct {
		name string
		fn   func(w *WorkspaceState)
	}{
		{"file", func(w *WorkspaceState) { w.InvalidateAndroidProjectFor(manifest) }},
		{"directory", func(w *WorkspaceState) { w.InvalidateAndroidProjectUnder(filepath.Dir(manifest)) }},
		{"build script", func(w *WorkspaceState) { w.InvalidateLibraryFacts() }},
	} {
		t.Run(invalidate.name, func(t *testing.T) {
			w := NewWorkspaceState(root)
			stale := &android.Project{}
			got := w.AndroidProject("fp", func() *android.Project {
				invalidate.fn(w)
				return stale
			})
			if got != stale {
				t.Fatal("the overlapped detection was not returned to its caller")
			}
			if w.HasAndroidProject() {
				t.Fatal("a detection an invalidation overlapped was cached")
			}
			fresh := &android.Project{ManifestPaths: []string{manifest}}
			if got := w.AndroidProject("fp", func() *android.Project { return fresh }); got != fresh {
				t.Fatal("the next call did not detect again")
			}
			if got := w.AndroidProject("fp", func() *android.Project { return stale }); got != fresh {
				t.Fatal("a detection no invalidation overlapped was not cached")
			}
		})
	}
}

// A file event that cannot change detection (a Kotlin source, or a manifest in a directory detection skips) leaves
// a running detection free to store its result.
func TestAndroidProjectKeepsADetectionUnrelatedEventsOverlapped(t *testing.T) {
	root := t.TempDir()
	w := NewWorkspaceState(root)
	built := &android.Project{}
	w.AndroidProject("fp", func() *android.Project {
		w.InvalidateAndroidProjectFor(filepath.Join(root, "app", "src", "main", "java", "A.kt"))
		w.InvalidateAndroidProjectFor(filepath.Join(root, "app", ".cxx", "AndroidManifest.xml"))
		w.InvalidateAndroidProjectFor(filepath.Join(root, "app", "build", "intermediates", "AndroidManifest.xml"))
		return built
	})
	if !w.HasAndroidProject() {
		t.Fatal("an unrelated file event discarded the running detection")
	}
}

// A case-only rename of a manifest on a case-insensitive filesystem leaves a file that still stats under the old
// name; detection, which matches names exactly, no longer lists it.
func TestExactPathExists(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "AndroidManifest.xml")
	if exactPathExists(manifest) {
		t.Fatal("a missing manifest exists")
	}
	if err := os.WriteFile(manifest, []byte("<manifest/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !exactPathExists(manifest) {
		t.Fatal("an existing manifest does not exist")
	}
	if err := os.Rename(manifest, filepath.Join(dir, "androidmanifest.xml")); err != nil {
		t.Fatal(err)
	}
	if exactPathExists(manifest) {
		t.Fatal("a manifest renamed to another case still exists under its old name")
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Log("case-sensitive filesystem: only the plain missing-file path ran")
	}
}
