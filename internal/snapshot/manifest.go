package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
)

// ManifestSchemaVersion versions the JSON manifest layout. Bumped when a
// field is added that older readers cannot tolerate; field additions
// that older readers can ignore should not bump this.
const ManifestSchemaVersion = 1

const manifestFileName = "manifest.json"

// Manifest is the small, human-readable sidecar describing a captured
// snapshot. Stored as JSON alongside the graph blob and metrics rollup
// so callers can answer "what shas have we captured, at what krit
// version, with what parents" without decoding either binary.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	CommitSHA     string   `json:"commit_sha"`
	ParentSHAs    []string `json:"parent_shas,omitempty"`
	CapturedAt    int64    `json:"captured_at"`
	KritVersion   string   `json:"krit_version"`
	BlobSchema    int      `json:"blob_schema"`
	MetricsSchema int      `json:"metrics_schema"`
	Files         int      `json:"files"`
	Symbols       int      `json:"symbols"`
	Modules       int      `json:"modules"`
	// RuleSetHash, when non-empty, identifies the rule registry + config
	// used to derive any findings rollups for this snapshot. Empty in
	// PRD-1 phase C (findings rollups arrive in a later phase). Captured
	// here so cross-version diffs can refuse to compare incomparable
	// findings counts later without changing the schema again.
	RuleSetHash string `json:"rule_set_hash,omitempty"`
}

// manifestPath returns the on-disk path for a sha's manifest. Sibling of
// BlobPath / metricsPath under the same per-sha directory.
func manifestPath(root, sha string) (string, error) {
	if len(sha) < 2 {
		return "", fmt.Errorf("snapshot: sha %q too short", sha)
	}
	return filepath.Join(root, "graphs", sha[:2], sha, manifestFileName), nil
}

// SaveManifest writes m next to the graph blob and metrics rollup for the
// same sha. JSON (not gob+zstd) so the manifest is greppable and
// debuggable without krit.
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
	return path, nil
}

// LoadManifest reads the manifest for sha from root.
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
	return &m, nil
}

// LoadManifests returns the manifest for every captured sha under root,
// sorted by sha. Snapshots whose manifest is missing or malformed are
// skipped silently — callers concerned with completeness reconcile by
// re-capturing those shas.
func LoadManifests(root string) ([]Manifest, error) {
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
// it next to the blob and metrics files. Returns the on-disk path.
// repoRoot is used for git parent lookup; pass "" to skip the lookup
// (manifest is then written without ParentSHAs populated).
func CaptureManifest(root string, res *Result, repoRoot, kritVersion string) (string, error) {
	m := buildManifest(res, repoRoot, kritVersion)
	if m == nil {
		return "", errors.New("snapshot: cannot build manifest from nil result")
	}
	return SaveManifest(root, m)
}

// buildManifest derives a Manifest from a captured Result. Parent shas
// are looked up via git when repoRoot is non-empty; failures fall back
// to an empty parent list rather than aborting the capture.
func buildManifest(res *Result, repoRoot, kritVersion string) *Manifest {
	if res == nil || res.Blob == nil {
		return nil
	}
	parents, _ := ResolveParentSHAs(repoRoot, res.Blob.CommitSHA)
	m := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     res.Blob.CommitSHA,
		ParentSHAs:    parents,
		CapturedAt:    res.Blob.CapturedAt,
		KritVersion:   kritVersion,
		BlobSchema:    res.Blob.SchemaVersion,
		Files:         len(res.Blob.Files),
		Symbols:       len(res.Blob.Symbols),
		Modules:       len(res.Blob.Modules),
	}
	if res.Metrics != nil {
		m.MetricsSchema = res.Metrics.SchemaVersion
	}
	return m
}
