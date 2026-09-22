package pipeline

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestTryLoadStructurallyStableBundle_RejectsOracleFactChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dependency.kt")
	caller := filepath.Join(dir, "Caller.kt")
	if err := os.WriteFile(path, []byte("package demo\nfun dependency() = \"value\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	callerSource := []byte("package demo\nfun caller() = dependency().length\n")
	if err := os.WriteFile(caller, callerSource, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse dependency before edit: %v", err)
	}
	structuralFP := scanner.FileStructuralFingerprint(before)
	if err := os.WriteFile(path, []byte("package demo\nfun dependency() = null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("parse dependency after edit: %v", err)
	}
	if afterFP := scanner.FileStructuralFingerprint(after); afterFP != structuralFP {
		t.Fatalf("body-only inferred-type edit changed structural fingerprint: before=%s after=%s", structuralFP, afterFP)
	}
	callerAfter, err := os.ReadFile(caller)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(callerSource, callerAfter) {
		t.Fatal("caller bytes changed while dependency facts changed")
	}
	priorFP := scanner.RunFingerprint{
		Version:   "test",
		Rules:     "rules",
		Config:    "config",
		SourceSet: "before",
		CrossFile: "structural",
	}
	currentFP := priorFP
	currentFP.SourceSet = "after"
	cols := scanner.CollectFindings([]scanner.Finding{{
		File: caller, Rule: "NullabilityRule", Message: "stale",
	}})
	store := &recordingPreviewStore{loaded: &cols}
	prior := scanner.FindingsBundleManifest{
		Fingerprint:      priorFP,
		ContentHashes:    map[string]string{path: "before"},
		StructuralFPs:    map[string]string{path: structuralFP},
		OracleBlobHashes: map[string]string{path: "old-facts"},
	}
	host := ProjectHostState{
		Reporter:                &diag.Reporter{},
		FindingsBundleStore:     store,
		FindingsBundleCacheRoot: dir,
		FindingsBundleManifestLoader: func(string) (scanner.FindingsBundleManifest, bool) {
			return prior, true
		},
	}
	manifest := deltaManifestData{
		enabled:          true,
		manifestKey:      "manifest",
		contentHashes:    map[string]string{path: "after"},
		structuralFPs:    map[string]string{path: structuralFP},
		oracleBlobHashes: map[string]string{path: "new-facts"},
	}

	if got, ok := tryLoadStructurallyStableBundle(host, currentFP, manifest); ok || got != nil {
		t.Fatalf("oracle fact change replayed stale caller findings: ok=%v got=%v", ok, got)
	}
	if store.loadCalls != 0 {
		t.Fatalf("oracle fact gate must reject before loading prior bundle; loadCalls=%d", store.loadCalls)
	}
}
