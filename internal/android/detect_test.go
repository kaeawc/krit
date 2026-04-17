package android

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsEmpty(t *testing.T) {
	p := &AndroidProject{}
	if !p.IsEmpty() {
		t.Fatal("empty project should return true")
	}

	p.ManifestPaths = []string{"/some/path"}
	if p.IsEmpty() {
		t.Fatal("project with ManifestPaths should not be empty")
	}

	p2 := &AndroidProject{ResDirs: []string{"/res"}}
	if p2.IsEmpty() {
		t.Fatal("project with ResDirs should not be empty")
	}

	p3 := &AndroidProject{GradlePaths: []string{"/build.gradle"}}
	if p3.IsEmpty() {
		t.Fatal("project with GradlePaths should not be empty")
	}
}

func TestDetectAndroidProject_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	proj := DetectAndroidProject([]string{dir})
	if !proj.IsEmpty() {
		t.Fatal("expected empty project for empty directory")
	}
}

func TestDetectAndroidProject_ManifestFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectAndroidProject([]string{dir})
	if len(proj.ManifestPaths) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(proj.ManifestPaths))
	}
}

func TestDetectAndroidProject_GradleFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	proj := DetectAndroidProject([]string{dir})
	if len(proj.GradlePaths) != 2 {
		t.Fatalf("expected 2 gradle paths, got %d", len(proj.GradlePaths))
	}
}

func TestDetectAndroidProject_ResDir(t *testing.T) {
	dir := t.TempDir()
	resDir := filepath.Join(dir, "app", "src", "main", "res")
	if err := os.MkdirAll(resDir, 0755); err != nil {
		t.Fatal(err)
	}

	proj := DetectAndroidProject([]string{dir})
	if len(proj.ResDirs) != 1 {
		t.Fatalf("expected 1 res dir, got %d", len(proj.ResDirs))
	}
}

func TestDetectAndroidProject_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hiddenDir := filepath.Join(dir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "AndroidManifest.xml"), []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectAndroidProject([]string{dir})
	if len(proj.ManifestPaths) != 0 {
		t.Fatalf("expected 0 manifests (hidden dir), got %d", len(proj.ManifestPaths))
	}
}

func TestDetectAndroidProject_SingleFile(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest/>`), 0644); err != nil {
		t.Fatal(err)
	}

	proj := DetectAndroidProject([]string{manifestPath})
	if len(proj.ManifestPaths) != 1 {
		t.Fatalf("expected 1 manifest from single file, got %d", len(proj.ManifestPaths))
	}
	if proj.ManifestPaths[0] != manifestPath {
		t.Fatalf("expected path %s, got %s", manifestPath, proj.ManifestPaths[0])
	}
}

func TestDetectAndroidProject_NonExistentPath(t *testing.T) {
	proj := DetectAndroidProject([]string{"/nonexistent/path/that/does/not/exist"})
	if !proj.IsEmpty() {
		t.Fatal("expected empty project for non-existent path")
	}
}
