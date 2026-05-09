package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureStandaloneKotlinFile(t *testing.T) {
	repo := t.TempDir()
	srcDir := filepath.Join(repo, "src", "main", "kotlin", "demo")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := []byte("package demo\n\nfun greet(name: String): String {\n  return \"hi $name\"\n}\n")
	if err := os.WriteFile(filepath.Join(srcDir, "Greet.kt"), src, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	res, err := Capture(CaptureOptions{
		RepoRoot:    repo,
		CommitSHA:   "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		KritVersion: "test",
		Now:         func() time.Time { return time.Unix(1700000000, 0) },
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	blob := res.Blob
	if blob.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion: %d", blob.SchemaVersion)
	}
	if blob.CapturedAt != 1700000000*1000 {
		t.Fatalf("CapturedAt: %d", blob.CapturedAt)
	}
	if len(blob.Files) != 1 || blob.Files[0].Path != "src/main/kotlin/demo/Greet.kt" {
		t.Fatalf("Files: %+v", blob.Files)
	}
	if blob.Files[0].Language != "kotlin" {
		t.Fatalf("expected kotlin, got %s", blob.Files[0].Language)
	}
	foundGreet := false
	for _, s := range blob.Symbols {
		if s.Name == "greet" && s.Kind == "function" {
			foundGreet = true
			if s.File != "src/main/kotlin/demo/Greet.kt" {
				t.Fatalf("greet symbol file: %s", s.File)
			}
		}
	}
	if !foundGreet {
		t.Fatalf("expected greet symbol in blob: %+v", blob.Symbols)
	}

	if res.Metrics == nil || res.Metrics.CommitSHA != blob.CommitSHA {
		t.Fatalf("metrics not populated: %+v", res.Metrics)
	}
	if len(res.Metrics.Files) != 1 || res.Metrics.Files[0].Path != "src/main/kotlin/demo/Greet.kt" {
		t.Fatalf("metrics files: %+v", res.Metrics.Files)
	}
	if res.Metrics.Files[0].Symbols == 0 {
		t.Fatalf("expected at least one symbol on greet file: %+v", res.Metrics.Files[0])
	}
	if res.Metrics.Files[0].Cyclomatic < 1 {
		t.Fatalf("expected cyclomatic >= 1, got %d", res.Metrics.Files[0].Cyclomatic)
	}
}

func TestCaptureRequiresInputs(t *testing.T) {
	if _, err := Capture(CaptureOptions{CommitSHA: "abc"}); err == nil {
		t.Fatal("expected error without RepoRoot")
	}
	if _, err := Capture(CaptureOptions{RepoRoot: t.TempDir()}); err == nil {
		t.Fatal("expected error without CommitSHA")
	}
}
