package android

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type fakeProjectFileIndex struct {
	files []string
	ok    bool
	calls int
}

func (f *fakeProjectFileIndex) Files(root string) ([]string, bool) {
	f.calls++
	return append([]string(nil), f.files...), f.ok
}

func TestIsEmpty(t *testing.T) {
	p := &Project{}
	if !p.IsEmpty() {
		t.Fatal("empty project should return true")
	}

	p.ManifestPaths = []string{"/some/path"}
	if p.IsEmpty() {
		t.Fatal("project with ManifestPaths should not be empty")
	}

	p2 := &Project{ResDirs: []string{"/res"}}
	if p2.IsEmpty() {
		t.Fatal("project with ResDirs should not be empty")
	}

	p3 := &Project{GradlePaths: []string{"/build.gradle"}}
	if p3.IsEmpty() {
		t.Fatal("project with GradlePaths should not be empty")
	}
}

func TestDetectProject_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	proj := DetectProject([]string{dir})
	if !proj.IsEmpty() {
		t.Fatal("expected empty project for empty directory")
	}
}

func TestDetectProject_ManifestFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectProject([]string{dir})
	if len(proj.ManifestPaths) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(proj.ManifestPaths))
	}
}

func TestDetectProject_GradleFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	proj := DetectProject([]string{dir})
	if len(proj.GradlePaths) != 2 {
		t.Fatalf("expected 2 gradle paths, got %d", len(proj.GradlePaths))
	}
}

func TestDetectProject_ResDir(t *testing.T) {
	dir := t.TempDir()
	resDir := filepath.Join(dir, "app", "src", "main", "res")
	if err := os.MkdirAll(resDir, 0755); err != nil {
		t.Fatal(err)
	}

	proj := DetectProject([]string{dir})
	if len(proj.ResDirs) != 1 {
		t.Fatalf("expected 1 res dir, got %d", len(proj.ResDirs))
	}
}

func TestDetectProject_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "AndroidManifest.xml"), []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectProject([]string{dir})
	if len(proj.ManifestPaths) != 0 {
		t.Fatalf("expected 0 manifests (hidden dir), got %d", len(proj.ManifestPaths))
	}
}

func TestDetectProject_SingleFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectProject([]string{manifestPath})
	if len(proj.ManifestPaths) != 1 {
		t.Fatalf("expected 1 manifest from single file, got %d", len(proj.ManifestPaths))
	}
	if proj.ManifestPaths[0] != manifestPath {
		t.Fatalf("expected path %s, got %s", manifestPath, proj.ManifestPaths[0])
	}
}

func TestDetectProject_NonExistentPath(t *testing.T) {
	proj := DetectProject([]string{"/nonexistent/path/that/does/not/exist"})
	if !proj.IsEmpty() {
		t.Fatal("expected empty project for non-existent path")
	}
}

func TestDetectProjectWithIndex_UsesTrackedFileListing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app", "src", "main", "res", "layout"), 0o755); err != nil {
		t.Fatal(err)
	}
	index := &fakeProjectFileIndex{
		ok: true,
		files: []string{
			"app/src/main/AndroidManifest.xml",
			"app/build.gradle.kts",
			"app/src/main/res/layout/main.xml",
			"app/src/main/java/NotAndroid.kt",
		},
	}

	proj := DetectProjectWithIndex([]string{root}, index)
	if index.calls != 1 {
		t.Fatalf("Files calls = %d, want 1", index.calls)
	}
	wantManifest := []string{filepath.Join(root, "app", "src", "main", "AndroidManifest.xml")}
	if !reflect.DeepEqual(proj.ManifestPaths, wantManifest) {
		t.Fatalf("ManifestPaths = %v, want %v", proj.ManifestPaths, wantManifest)
	}
	wantGradle := []string{filepath.Join(root, "app", "build.gradle.kts")}
	if !reflect.DeepEqual(proj.GradlePaths, wantGradle) {
		t.Fatalf("GradlePaths = %v, want %v", proj.GradlePaths, wantGradle)
	}
	wantRes := []string{filepath.Join(root, "app", "src", "main", "res")}
	if !reflect.DeepEqual(proj.ResDirs, wantRes) {
		t.Fatalf("ResDirs = %v, want %v", proj.ResDirs, wantRes)
	}
}

func TestDetectProjectWithIndex_FallsBackWhenListingUnavailable(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "AndroidManifest.xml")
	if err := os.WriteFile(manifest, []byte(`<manifest/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	index := &fakeProjectFileIndex{ok: false}

	proj := DetectProjectWithIndex([]string{root}, index)
	if index.calls != 1 {
		t.Fatalf("Files calls = %d, want 1", index.calls)
	}
	if len(proj.ManifestPaths) != 1 || proj.ManifestPaths[0] != manifest {
		t.Fatalf("ManifestPaths = %v, want [%s]", proj.ManifestPaths, manifest)
	}
}
