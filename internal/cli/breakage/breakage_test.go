package breakage

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	brk "github.com/kaeawc/krit/internal/breakage"
	snap "github.com/kaeawc/krit/internal/snapshot"
)

// captureStdout swaps os.Stdout for the duration of fn and returns
// what fn writes. Kept local so this package has no test-only
// dependency on other CLI packages.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	<-done
	os.Stdout = orig
	return buf.String()
}

const junitFixture = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="OrderTests">
    <testcase classname="com.acme.OrderTest" name="testPlace" file="OrderTest.kt">
      <failure message="expected 1 but got 2" type="AssertionError">stack trace here</failure>
    </testcase>
    <testcase classname="com.acme.OrderTest" name="testCancel"/>
  </testsuite>
</testsuites>`

func TestRunRecordJUnitAndList(t *testing.T) {
	repoRoot := t.TempDir()
	junitPath := filepath.Join(repoRoot, "junit.xml")
	if err := os.WriteFile(junitPath, []byte(junitFixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	code := Run([]string{
		"record",
		"--repo", repoRoot,
		"--kind", "junit",
		"--from", junitPath,
		"--commit", "1111111111111111111111111111111111111111",
	})
	if code != 0 {
		t.Fatalf("record exit = %d", code)
	}

	root := snap.SnapshotsDir(repoRoot)
	events, err := brk.LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1 (only the failed case)", len(events))
	}
	if events[0].FailureKind != brk.KindTestFailure {
		t.Errorf("kind = %q, want %q", events[0].FailureKind, brk.KindTestFailure)
	}
	if events[0].Symbol != "com.acme.OrderTest.testPlace" {
		t.Errorf("symbol = %q", events[0].Symbol)
	}

	out := captureStdout(t, func() {
		code := Run([]string{"list", "--repo", repoRoot})
		if code != 0 {
			t.Fatalf("list exit = %d", code)
		}
	})
	var listed []brk.Event
	if err := json.Unmarshal([]byte(out), &listed); err != nil {
		t.Fatalf("decode list: %v: %s", err, out)
	}
	if len(listed) != 1 {
		t.Fatalf("list len = %d, want 1: %s", len(listed), out)
	}
}

func TestRunDedupesRecordedEvents(t *testing.T) {
	repoRoot := t.TempDir()
	path := filepath.Join(repoRoot, "j.xml")
	if err := os.WriteFile(path, []byte(junitFixture), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for i := 0; i < 2; i++ {
		code := Run([]string{
			"record",
			"--repo", repoRoot,
			"--kind", "junit",
			"--from", path,
			"--commit", "1111111111111111111111111111111111111111",
		})
		if code != 0 {
			t.Fatalf("record %d exit = %d", i, code)
		}
	}
	events, err := brk.LoadAll(snap.SnapshotsDir(repoRoot))
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("dedup failed: len = %d", len(events))
	}
}

func TestRunRejectsUnknownKind(t *testing.T) {
	repoRoot := t.TempDir()
	code := Run([]string{
		"record",
		"--repo", repoRoot,
		"--kind", "no-such-format",
		"--commit", "1111111111111111111111111111111111111111",
	})
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown kind")
	}
}

func TestRunUnknownSubcommand(t *testing.T) {
	if code := Run([]string{"bogus"}); code == 0 {
		t.Fatalf("expected non-zero exit for unknown subcommand")
	}
	if code := Run(nil); code == 0 {
		t.Fatalf("expected non-zero exit for no args")
	}
}
