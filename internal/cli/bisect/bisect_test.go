package bisect

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/breakage"
	"github.com/kaeawc/krit/internal/snapshot"
)

// captureStdout swaps os.Stdout for the duration of fn and returns
// what fn writes. Kept local so the package test has no test-only
// dependency leaks.
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

// seedTimeline lays down two captured snapshots plus a breakage event
// whose top stack frame maps into the bad-side blob. The fixture is
// the minimum the CLI needs to print a non-empty candidate list.
func seedTimeline(t *testing.T, repoRoot string) string {
	t.Helper()
	root := snapshot.SnapshotsDir(repoRoot)

	const goodSHA = "1111111111111111111111111111111111111111"
	const badSHA = "2222222222222222222222222222222222222222"

	for _, sha := range []string{goodSHA, badSHA} {
		blob := &snapshot.Blob{
			SchemaVersion: snapshot.SchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    1,
			Modules:       []snapshot.Module{{Path: ":core"}},
			Files:         []snapshot.File{{Path: "core/Order.kt", Module: ":core"}},
			Symbols:       []snapshot.Symbol{{FQN: "com.acme.Order", File: "core/Order.kt"}},
		}
		if _, err := snapshot.Save(root, blob); err != nil {
			t.Fatalf("save blob %s: %v", sha, err)
		}
		man := &snapshot.Manifest{
			SchemaVersion: snapshot.ManifestSchemaVersion,
			CommitSHA:     sha,
			CapturedAt:    1,
			BlobSchema:    snapshot.SchemaVersion,
			Files:         1, Modules: 1, Symbols: 1,
		}
		if _, err := snapshot.SaveManifest(root, man); err != nil {
			t.Fatalf("save manifest %s: %v", sha, err)
		}
	}

	ev := breakage.Event{
		OccurredAt:  1,
		CommitSHA:   badSHA,
		FailureKind: breakage.KindRuntimeFailure,
		Signature:   "npe in order.place",
		Source:      breakage.SourceCI,
		Frames:      []string{"com.acme.Order.place(Order.kt:42)"},
		Message:     "NPE in Order.place",
	}
	ev.ID = breakage.HashID(ev.FailureKind, ev.Signature, ev.CommitSHA, ev.Source)
	if _, err := breakage.Record(root, ev); err != nil {
		t.Fatalf("record event: %v", err)
	}
	return ev.ID
}

func TestRunCLIPrintsCandidates(t *testing.T) {
	repoRoot := t.TempDir()
	eventID := seedTimeline(t, repoRoot)

	out := captureStdout(t, func() {
		code := Run([]string{
			"--repo", repoRoot,
			"--from", "1111111111111111111111111111111111111111",
			"--to", "2222222222222222222222222222222222222222",
			"--event", eventID,
			"--format", "json",
		})
		if code != 0 {
			t.Fatalf("Run exit = %d", code)
		}
	})

	var result struct {
		Candidates []struct {
			Module     string  `json:"module"`
			Confidence float64 `json:"confidence"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v: %s", err, out)
	}
	if len(result.Candidates) == 0 {
		t.Fatalf("no candidates printed: %s", out)
	}
	if result.Candidates[0].Module != ":core" {
		t.Fatalf("top module = %q, want :core", result.Candidates[0].Module)
	}
	if result.Candidates[0].Confidence < 0.5 {
		t.Fatalf("top confidence = %g, want >= 0.5", result.Candidates[0].Confidence)
	}
}

func TestRunCLIRejectsMissingEvent(t *testing.T) {
	code := Run([]string{"--from", "a", "--to", "b"})
	if code == 0 {
		t.Fatalf("Run with no --event should fail, got 0")
	}
}

func TestRunCLIUnknownEventID(t *testing.T) {
	repoRoot := t.TempDir()
	// snapshots dir doesn't exist yet — exercise the "no event" path
	if err := os.MkdirAll(filepath.Join(repoRoot, ".krit", "snapshots"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	code := Run([]string{
		"--repo", repoRoot,
		"--from", "a", "--to", "b",
		"--event", "does-not-exist",
	})
	if code == 0 {
		t.Fatalf("Run with unknown event should fail, got 0")
	}
}
