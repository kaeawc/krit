package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/fsutil"
)

// ManifestSchemaVersion versions the JSON manifest layout.
//
//	v1 — flat Files / Symbols / Modules count fields at the top level.
//	v2 — counts moved into a nested Counts struct so future additions
//	     (LinesOfCode, FailedFiles, FindingsTotal) don't widen the
//	     top-level shape. v1 readers still see the flat fields: writers
//	     populate both, and the v1->v2 migrator copies them in on read
//	     so older payloads compare correctly.
const ManifestSchemaVersion = 2

const manifestFileName = "manifest.json"

// ManifestCounts is the per-snapshot count rollup nested under
// Manifest at v2. New count fields land here without touching the
// top-level shape.
type ManifestCounts struct {
	Files   int `json:"files"`
	Symbols int `json:"symbols"`
	Modules int `json:"modules"`
}

// Manifest is the JSON sidecar describing a captured snapshot. Greppable
// without krit; lets callers answer "what shas have we captured, at
// what krit version, with what parents" without decoding the binary
// blobs.
//
// RuleSetHash is reserved for the findings-rollup phase: when non-empty
// it identifies the rule registry + config used to compute findings,
// so cross-version diffs can refuse to compare incomparable counts.
//
// The flat Files / Symbols / Modules fields and the nested Counts
// struct carry the same numbers — writers populate both so v1 readers
// (and grep/jq pipelines) keep working through the transition. New
// code should prefer Counts.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	CommitSHA     string   `json:"commit_sha"`
	ParentSHAs    []string `json:"parent_shas,omitempty"`
	CapturedAt    int64    `json:"captured_at"`
	KritVersion   string   `json:"krit_version"`
	BlobSchema    int      `json:"blob_schema"`
	MetricsSchema int      `json:"metrics_schema"`
	// Files / Symbols / Modules are kept at the top level for v1
	// reader compatibility. Prefer Counts in new code; the two move
	// in lockstep.
	Files       int            `json:"files"`
	Symbols     int            `json:"symbols"`
	Modules     int            `json:"modules"`
	Counts      ManifestCounts `json:"counts"`
	RuleSetHash string         `json:"rule_set_hash,omitempty"`
}

func manifestPath(root, sha string) (string, error) {
	dir, err := shaDir(root, sha)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, manifestFileName), nil
}

func SaveManifest(root string, m *Manifest) (string, error) {
	if m == nil {
		return "", errors.New("snapshot: nil manifest")
	}
	if m.CommitSHA == "" {
		return "", errors.New("snapshot: manifest has no CommitSHA")
	}
	path, err := manifestPath(root, m.CommitSHA)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", filepath.Dir(path), err)
	}
	payload, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("snapshot: marshal manifest: %w", err)
	}
	payload = append(payload, '\n')
	if err := fsutil.WriteFileAtomic(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	// Best-effort rollup update so `snapshot status` is O(1). A failure
	// here doesn't fail capture — LoadManifests falls back to scanning.
	if err := upsertIndex(root, m); err != nil {
		// Surface as a non-fatal warning via stderr would belong here,
		// but the snapshot package is reporter-free; swallow and let
		// the next status fall back to the scan path.
		_ = err
	}
	return path, nil
}

func LoadManifest(root, sha string) (*Manifest, error) {
	path, err := manifestPath(root, sha)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("snapshot: parse %s: %w", path, err)
	}
	migrated, err := MigrateManifest(&m)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %s: %w", path, err)
	}
	return migrated, nil
}

// LoadManifests returns every captured sha's manifest under root,
// sorted by sha. Missing or malformed manifests are skipped silently.
//
// Prefers the index.json rollup (O(1) read) when present; falls back
// to the legacy per-sha scan when the rollup is missing (older
// captures), unreadable (corruption), or carries a newer schema.
func LoadManifests(root string) ([]Manifest, error) {
	if idx, err := LoadIndex(root); err == nil && idx != nil {
		out := make([]Manifest, len(idx.Entries))
		copy(out, idx.Entries)
		sort.Slice(out, func(i, j int) bool { return out[i].CommitSHA < out[j].CommitSHA })
		return out, nil
	}
	entries, err := List(root)
	if err != nil {
		return nil, err
	}
	out := make([]Manifest, 0, len(entries))
	for _, e := range entries {
		m, err := LoadManifest(root, e.CommitSHA)
		if err != nil {
			continue
		}
		out = append(out, *m)
	}
	return out, nil
}

// CaptureManifest builds a Manifest from a captured Result and writes
// it next to the blob and metrics files. repoRoot is used for the git
// parent lookup; pass "" to skip the lookup.
func CaptureManifest(root string, res *Result, repoRoot, kritVersion string) (string, error) {
	m := buildManifest(res, repoRoot, kritVersion)
	if m == nil {
		return "", errors.New("snapshot: cannot build manifest from nil result")
	}
	return SaveManifest(root, m)
}

// SaveResult persists the graph blob, metrics rollup, and a
// freshly-built manifest for a captured Result. Returns the blob path.
func SaveResult(root string, res *Result, repoRoot, kritVersion string) (string, error) {
	if res == nil || res.Blob == nil {
		return "", errors.New("snapshot: nil result")
	}
	blobPath, err := Save(root, res.Blob)
	if err != nil {
		return "", err
	}
	if res.Metrics != nil {
		if _, err := SaveMetrics(root, res.Metrics); err != nil {
			return "", err
		}
	}
	if res.Findings != nil {
		if _, err := SaveFindings(root, res.Findings); err != nil {
			return "", err
		}
	}
	if _, err := CaptureManifest(root, res, repoRoot, kritVersion); err != nil {
		return "", err
	}
	return blobPath, nil
}

// buildManifest swallows git-lookup failures so capture can still
// succeed when git is unavailable or the sha is unreachable; the
// resulting manifest just has no ParentSHAs.
func buildManifest(res *Result, repoRoot, kritVersion string) *Manifest {
	if res == nil || res.Blob == nil {
		return nil
	}
	parents, _ := ResolveParentSHAs(repoRoot, res.Blob.CommitSHA)
	counts := ManifestCounts{
		Files:   len(res.Blob.Files),
		Symbols: len(res.Blob.Symbols),
		Modules: len(res.Blob.Modules),
	}
	m := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     res.Blob.CommitSHA,
		ParentSHAs:    parents,
		CapturedAt:    res.Blob.CapturedAt,
		KritVersion:   kritVersion,
		BlobSchema:    res.Blob.SchemaVersion,
		// Flat fields kept for v1 reader compatibility; mirror Counts.
		Files:   counts.Files,
		Symbols: counts.Symbols,
		Modules: counts.Modules,
		Counts:  counts,
	}
	if res.Metrics != nil {
		m.MetricsSchema = res.Metrics.SchemaVersion
	}
	if res.Findings != nil {
		m.RuleSetHash = res.Findings.RuleSetHash
	}
	return m
}
