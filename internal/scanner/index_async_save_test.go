package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndexCachedAsyncSaveIsLoadableAfterWait(t *testing.T) {
	SetAsyncCrossFileCacheSave(true)
	t.Cleanup(func() { SetAsyncCrossFileCacheSave(false) })

	dir := t.TempDir()
	var files []*File
	for name, src := range map[string]string{
		"A.kt": "class A { fun b(): B = B() }",
		"B.kt": "class B { fun a() = A() }",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := ParseFile(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	cacheDir := CrossFileCacheDir(dir)
	built, hit := BuildIndexCached(cacheDir, files, 2, nil)
	if hit {
		t.Fatal("first build must miss")
	}
	WaitCrossFileCacheSaves()
	loaded, hit := BuildIndexCached(cacheDir, files, 2, nil)
	if !hit {
		t.Fatal("second build must load the cache the background save wrote")
	}
	if len(loaded.Symbols) != len(built.Symbols) || len(loaded.References) != len(built.References) {
		t.Fatalf("loaded %d/%d symbols/refs, built %d/%d", len(loaded.Symbols), len(loaded.References), len(built.Symbols), len(built.References))
	}
}
