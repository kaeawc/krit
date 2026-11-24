package fsutil

import (
	"encoding/gob"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// TestRoundTrip writes a small payload and reads it back, asserting equality.
func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	want := []byte("hello, atomic world")

	if err := WriteFileAtomic(path, want, 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestOverwrite writes a file twice and asserts the second content wins.
func TestOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	if err := WriteFileAtomic(path, []byte("first"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}

	want := []byte("second content wins")
	if err := WriteFileAtomic(path, want, 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestParentDirMissing asserts that WriteFileAtomic returns an error when the
// parent directory does not exist. The helper must NOT auto-create directories.
func TestParentDirMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "data.bin")

	err := WriteFileAtomic(path, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("expected error for missing parent dir, got nil")
	}
}

// TestTempfileCleanup simulates a write failure and asserts no .tmp-* files
// remain in the directory afterward.
func TestTempfileCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	writeErr := errors.New("simulated write failure")
	err := WriteFileAtomicStream(path, 0o644, func(w io.Writer) error {
		return writeErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Assert no .tmp-* files remain.
	matches, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	// Also check for the pattern the helper actually uses.
	matches2, err := filepath.Glob(filepath.Join(dir, ".data.bin.tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	all := append(matches, matches2...)
	if len(all) > 0 {
		t.Errorf("tempfiles not cleaned up: %v", all)
	}
}

// TestStreamVariant verifies that data written via the callback matches what
// WriteFileAtomicStream persists to disk.
func TestStreamVariant(t *testing.T) {
	type payload struct {
		Name  string
		Value int
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "data.gob")
	want := payload{Name: "krit", Value: 42}

	err := WriteFileAtomicStream(path, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(want)
	})
	if err != nil {
		t.Fatalf("WriteFileAtomicStream: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	var got payload
	if err := gob.NewDecoder(f).Decode(&got); err != nil {
		t.Fatalf("gob Decode: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestRace has two goroutines race writes to the same path. Both must succeed
// and exactly one final content must be readable. Run with -race to detect
// data races in the helper.
func TestRace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	const n = 50
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := []byte{'A' + byte(idx%26)}
			errs[idx] = WriteFileAtomic(path, data, 0o644)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}

	// File must be readable and contain exactly one byte.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after race: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 byte, got %d: %q", len(got), got)
	}
}

// TestParentDirSyncDoesNotBreakHappyPath asserts that the parent-directory
// fsync added to guard against rename-without-dirent-flush data loss does not
// regress the normal write path. We exercise nested and shallow directories
// and verify the contents land correctly.
func TestParentDirSyncDoesNotBreakHappyPath(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cases := []struct {
		name string
		dir  string
	}{
		{"root", root},
		{"nested", nested},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(tc.dir, "out.bin")
			want := []byte("parent-dir fsync happy path")
			if err := WriteFileAtomic(path, want, 0o644); err != nil {
				t.Fatalf("WriteFileAtomic: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// countSyncs replaces the sync hooks for the duration of the test and returns
// counters for tempfile and parent-directory fsyncs. The tempfile counter also
// records whether the fsync happened before the rename made the file visible.
func countSyncs(t *testing.T, path string) (fileSyncs, dirSyncs, syncedBeforeRename *int) {
	t.Helper()
	var files, dirs, before int
	origFile, origDir := syncFile, syncParentDir
	syncFile = func(f *os.File) error {
		files++
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			before++
		}
		return origFile(f)
	}
	syncParentDir = func(dir string) {
		dirs++
		origDir(dir)
	}
	t.Cleanup(func() { syncFile, syncParentDir = origFile, origDir })
	return &files, &dirs, &before
}

// TestSyncCalled is the regression guard for the fsync guarantee of the
// durable helpers: each must fsync the tempfile before the rename and the
// parent directory after it.
func TestSyncCalled(t *testing.T) {
	writers := map[string]func(path string, data []byte) error{
		"WriteFileAtomic": func(path string, data []byte) error {
			return WriteFileAtomic(path, data, 0o644)
		},
		"WriteFileAtomicKeepMode": func(path string, data []byte) error {
			return WriteFileAtomicKeepMode(path, data, 0o644)
		},
		"WriteFileAtomicStream": func(path string, data []byte) error {
			return WriteFileAtomicStream(path, 0o644, func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			})
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "sync-check.bin")
			data := []byte("sync durability check")
			fileSyncs, dirSyncs, before := countSyncs(t, path)

			if err := write(path, data); err != nil {
				t.Fatalf("write: %v", err)
			}
			if *fileSyncs != 1 || *before != 1 {
				t.Errorf("tempfile fsyncs = %d (%d before rename), want 1 before rename", *fileSyncs, *before)
			}
			if *dirSyncs != 1 {
				t.Errorf("parent dir fsyncs = %d, want 1", *dirSyncs)
			}
			assertWritten(t, dir, path, data)
		})
	}
}

// TestNoSyncSkipsFsync asserts the cache-only helpers skip both fsyncs while
// keeping the tempfile + rename, so the content still lands whole and no
// tempfile is left behind.
func TestNoSyncSkipsFsync(t *testing.T) {
	writers := map[string]func(path string, data []byte) error{
		"WriteFileAtomicNoSync": func(path string, data []byte) error {
			return WriteFileAtomicNoSync(path, data, 0o644)
		},
		"WriteFileAtomicStreamNoSync": func(path string, data []byte) error {
			return WriteFileAtomicStreamNoSync(path, 0o644, func(w io.Writer) error {
				_, err := w.Write(data)
				return err
			})
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "nosync.bin")
			if err := os.WriteFile(path, []byte("previous, longer contents"), 0o644); err != nil {
				t.Fatal(err)
			}
			fileSyncs, dirSyncs, _ := countSyncs(t, path)

			data := []byte("cache entry")
			if err := write(path, data); err != nil {
				t.Fatalf("write: %v", err)
			}
			if *fileSyncs != 0 || *dirSyncs != 0 {
				t.Errorf("fsyncs = file %d, dir %d; want none", *fileSyncs, *dirSyncs)
			}
			assertWritten(t, dir, path, data)
		})
	}
}

// TestNoSyncWriteFailureLeavesOriginal asserts a failed no-sync write neither
// touches the existing file nor leaves a tempfile.
func TestNoSyncWriteFailureLeavesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keep.bin")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := WriteFileAtomicStreamNoSync(path, 0o644, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	assertWritten(t, dir, path, []byte("original"))
}

func assertWritten(t *testing.T, dir, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "."+filepath.Base(path)+".tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) > 0 {
		t.Errorf("tempfile still present: %v", matches)
	}
}

func TestWriteFileAtomicKeepMode(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomicKeepMode(existing, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(existing); info.Mode().Perm() != 0o600 {
		t.Fatalf("existing mode = %v, want 0600", info.Mode().Perm())
	}
	fresh := filepath.Join(dir, "b.txt")
	if err := WriteFileAtomicKeepMode(fresh, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(fresh); info.Mode().Perm() != 0o640 {
		t.Fatalf("new mode = %v, want 0640", info.Mode().Perm())
	}
}

func TestWriteFileAtomicWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared", "baseline.xml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "baseline.xml")
	if err := os.Symlink(filepath.Join("shared", "baseline.xml"), link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if err := WriteFileAtomicKeepMode(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %v", info, err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Fatalf("target = %q, want new", got)
	}
	if info, _ := os.Stat(target); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("target mode = %v, want 0600", info.Mode().Perm())
	}

	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink("created.txt", dangling); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(dangling, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "created.txt")); string(got) != "x" {
		t.Fatalf("dangling target = %q, want x", got)
	}
}

func TestWriteFileAtomicHonorsUmaskForNewFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no umask on windows")
	}
	restore := setUmask(0o077)
	defer restore()
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.json")
	if err := WriteFileAtomic(fresh, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(fresh); info.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode = %v, want 0600 under umask 077", info.Mode().Perm())
	}
	existing := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomicKeepMode(existing, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(existing); info.Mode().Perm() != 0o644 {
		t.Fatalf("existing mode = %v, want 0644 kept", info.Mode().Perm())
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

// Relative targets are literal strings: filepath.Join would clean away the
// "alias/.." these tests depend on.
func readString(t *testing.T, path string) string {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// Link.kt -> alias/../Real.kt with alias -> shared/subdir names shared/Real.kt,
// not a Real.kt beside the link.
func TestWriteFileAtomicResolvesDotDotThroughSymlinkedDir(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := os.MkdirAll(filepath.Join(shared, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(shared, "Real.kt")
	if err := os.WriteFile(real, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, filepath.Join(shared, "subdir"), filepath.Join(work, "alias"))
	link := filepath.Join(work, "Link.kt")
	symlinkOrSkip(t, "alias/../Real.kt", link)

	if err := WriteFileAtomicKeepMode(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, real); got != "new" {
		t.Fatalf("target = %q, want new", got)
	}
	if _, err := os.Lstat(filepath.Join(work, "Real.kt")); !os.IsNotExist(err) {
		t.Fatalf("unrelated file created beside the link: %v", err)
	}
	if info, _ := os.Stat(real); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("target mode = %v, want 0600", info.Mode().Perm())
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %v", info, err)
	}
}

func TestWriteFileAtomicFollowsAbsoluteAndChainedLinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real", "Real.kt")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(dir, "abs.kt")
	symlinkOrSkip(t, real, abs)
	mid := filepath.Join(dir, "mid")
	if err := os.Mkdir(mid, 0o755); err != nil {
		t.Fatal(err)
	}
	// chain -> mid/hop.kt -> ../abs.kt -> real/Real.kt
	symlinkOrSkip(t, filepath.Join("..", "abs.kt"), filepath.Join(mid, "hop.kt"))
	chain := filepath.Join(dir, "chain.kt")
	symlinkOrSkip(t, filepath.Join("mid", "hop.kt"), chain)

	if err := WriteFileAtomic(abs, []byte("abs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, real); got != "abs" {
		t.Fatalf("after absolute link write: %q", got)
	}
	if err := WriteFileAtomic(chain, []byte("chain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, real); got != "chain" {
		t.Fatalf("after chained link write: %q", got)
	}
	for _, l := range []string{abs, chain, filepath.Join(mid, "hop.kt")} {
		if info, err := os.Lstat(l); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s replaced: %v %v", l, info, err)
		}
	}
}

func TestWriteFileAtomicDanglingLinks(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := os.MkdirAll(filepath.Join(shared, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, filepath.Join(shared, "subdir"), filepath.Join(dir, "alias"))

	// A dangling link whose target directory exists creates the target there.
	dangling := filepath.Join(dir, "New.kt")
	symlinkOrSkip(t, "alias/../New.kt", dangling)
	if err := WriteFileAtomic(dangling, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, filepath.Join(shared, "New.kt")); got != "x" {
		t.Fatalf("dangling target = %q, want x", got)
	}

	// A target in a missing directory fails and creates nothing.
	missing := filepath.Join(dir, "Missing.kt")
	symlinkOrSkip(t, "nowhere/../Missing.kt", missing)
	if err := WriteFileAtomic(missing, []byte("x"), 0o644); err == nil {
		t.Fatal("want error for a target in a missing directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		switch e.Name() {
		case "shared", "alias", "New.kt", "Missing.kt":
		default:
			t.Fatalf("unexpected entry %q", e.Name())
		}
	}
	if info, err := os.Lstat(missing); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dangling link replaced: %v %v", info, err)
	}
}

func TestWriteFileAtomicRejectsSymlinkLoop(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	symlinkOrSkip(t, "b", a)
	symlinkOrSkip(t, "a", b)
	if err := WriteFileAtomic(a, []byte("x"), 0o644); err == nil {
		t.Fatal("want error for a symlink loop")
	}
	for _, l := range []string{a, b} {
		if info, err := os.Lstat(l); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s replaced: %v %v", l, info, err)
		}
	}
}

func TestWriteFileAtomicRejectsDirectoryNamedTarget(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "up")
	symlinkOrSkip(t, "..", link)
	if err := WriteFileAtomic(link, []byte("x"), 0o644); err == nil {
		t.Fatal("want error for a link naming a directory")
	}
}
