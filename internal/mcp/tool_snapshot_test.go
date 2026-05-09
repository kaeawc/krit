package mcp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	snap "github.com/kaeawc/krit/internal/snapshot"
)

func TestToolSnapshotStatusReturnsManifests(t *testing.T) {
	repo := t.TempDir()
	root := snap.SnapshotsDir(repo)

	if _, err := snap.Save(root, &snap.Blob{SchemaVersion: snap.SchemaVersion, CommitSHA: "1111111111111111111111111111111111111111", CapturedAt: 1}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := snap.SaveManifest(root, &snap.Manifest{
		SchemaVersion: snap.ManifestSchemaVersion,
		CommitSHA:     "1111111111111111111111111111111111111111",
		CapturedAt:    1,
		KritVersion:   "test",
		Files:         42,
	}); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	server := &Server{}
	args, _ := json.Marshal(snapshotArgs{Operation: "status", RepoRoot: repo})
	result := server.toolSnapshot(args)
	if result.IsError {
		t.Fatalf("unexpected error: %#v", result)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"krit_version": "test"`) || !strings.Contains(text, `"files": 42`) {
		t.Fatalf("status output missing manifest fields: %s", text)
	}
}

func TestToolSnapshotDiffSurfacesAddedSymbols(t *testing.T) {
	repo := t.TempDir()
	root := snap.SnapshotsDir(repo)

	from := &snap.Blob{
		SchemaVersion: snap.SchemaVersion, CommitSHA: "1111111111111111111111111111111111111111", CapturedAt: 1,
		Symbols: []snap.Symbol{{FQN: "demo.A", Signature: "demo.A"}},
	}
	to := &snap.Blob{
		SchemaVersion: snap.SchemaVersion, CommitSHA: "2222222222222222222222222222222222222222", CapturedAt: 2,
		Symbols: []snap.Symbol{{FQN: "demo.A", Signature: "demo.A"}, {FQN: "demo.B", Signature: "demo.B"}},
	}
	for _, b := range []*snap.Blob{from, to} {
		if _, err := snap.Save(root, b); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	server := &Server{}
	args, _ := json.Marshal(snapshotArgs{
		Operation: "diff", RepoRoot: repo,
		From: "1111111111111111111111111111111111111111",
		To:   "2222222222222222222222222222222222222222",
	})
	result := server.toolSnapshot(args)
	if result.IsError {
		t.Fatalf("unexpected error: %#v", result)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, `"fqn": "demo.B"`) {
		t.Fatalf("diff missing demo.B added symbol: %s", text)
	}
}

func TestToolSnapshotInfoRequiresCommitSHA(t *testing.T) {
	server := &Server{}
	result := server.toolSnapshot(json.RawMessage(`{"operation":"info","repo_root":"` + t.TempDir() + `"}`))
	if !result.IsError {
		t.Fatal("expected error for missing commit_sha")
	}
}

func TestToolSnapshotUnknownOperation(t *testing.T) {
	server := &Server{}
	result := server.toolSnapshot(json.RawMessage(`{"operation":"weird"}`))
	if !result.IsError {
		t.Fatal("expected error for unknown operation")
	}
}

func TestToolSnapshotTimelineDefaultsToRepoLoc(t *testing.T) {
	repo := t.TempDir()
	root := snap.SnapshotsDir(repo)

	if _, err := snap.Save(root, &snap.Blob{SchemaVersion: snap.SchemaVersion, CommitSHA: "1111111111111111111111111111111111111111", CapturedAt: 1}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	m := &snap.Metrics{
		SchemaVersion: snap.MetricsSchemaVersion,
		CommitSHA:     "1111111111111111111111111111111111111111",
		CapturedAt:    1,
		Files:         []snap.FileMetrics{{Path: "a.kt", LOC: 99}},
	}
	if _, err := snap.SaveMetrics(root, m); err != nil {
		t.Fatalf("SaveMetrics: %v", err)
	}

	server := &Server{}
	result := server.toolSnapshot(json.RawMessage(`{"operation":"timeline","repo_root":"` + filepath.ToSlash(repo) + `"}`))
	if result.IsError {
		t.Fatalf("unexpected error: %#v", result)
	}
	if !strings.Contains(result.Content[0].Text, `"Value": 99`) {
		t.Fatalf("timeline output missing value 99: %s", result.Content[0].Text)
	}
}
