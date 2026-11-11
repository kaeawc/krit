package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicOutputKeepsPreviousReportUntilFinish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := openAtomicOutput(path)
	if out == nil {
		t.Fatal("open returned nil")
	}
	_, _ = out.file.WriteString("new")
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("report replaced before Finish: %q", got)
	}
	if err := out.Finish(1); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("report = %q, want new", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	assertOnlyFile(t, path)
}

func TestAtomicOutputDiscardedOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	out := openAtomicOutput(path)
	if out == nil {
		t.Fatal("open returned nil")
	}
	_, _ = out.file.WriteString("partial")
	if err := out.Finish(2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("partial report published: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 0 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

func TestAtomicOutputSkipsNonRegularTargets(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "target"), link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if out := openAtomicOutput(link); out != nil {
		t.Fatal("symlink target must be opened directly")
	}
}

func assertOnlyFile(t *testing.T, want ...string) {
	t.Helper()
	if len(want) == 0 {
		return
	}
	entries, _ := os.ReadDir(filepath.Dir(want[0]))
	if len(entries) != 1 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

func TestDaemonOutputWriterKeepsPreviousReportOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &scanFlags{Output: &path}
	w, closer, err := openDaemonOutputWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("partial"))
	if err := closer(2); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("failed run replaced report: %q", got)
	}
	assertOnlyFile(t, path)

	w, closer, err = openDaemonOutputWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("new"))
	if err := closer(0); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("report = %q, want new", got)
	}
	assertOnlyFile(t, path)
}
