package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/scanner"
)

// MetricsSchemaVersion versions the on-disk format of Metrics. Bumped
// independently of Blob's SchemaVersion so adding a new scalar to the
// rollup does not invalidate snapshots' graph blobs.
const MetricsSchemaVersion = 1

const metricsFileName = "metrics.gob.zst"

// Metrics is the scalar rollup that sits next to a Blob on disk. Hot
// path for timeline/diff queries: loading N of these is cheap because
// each one is small (a few KB even on large repos).
type Metrics struct {
	SchemaVersion int
	CommitSHA     string
	CapturedAt    int64
	Files         []FileMetrics
	Modules       []ModuleMetrics
}

// FileMetrics captures per-file scalars. Path is repo-relative.
type FileMetrics struct {
	Path          string
	Module        string
	Language      string
	LOC           int
	Bytes         int
	Symbols       int
	PublicSymbols int
	Cyclomatic    int
}

// ModuleMetrics aggregates per-file metrics into a per-module roll-up
// and adds the graph-derived FanIn/FanOut.
type ModuleMetrics struct {
	Path          string
	Files         int
	LOC           int
	Symbols       int
	PublicSymbols int
	Cyclomatic    int
	FanIn         int
	FanOut        int
}

// computeMetrics builds the rollup from the captured Blob plus the parsed
// source files held during Capture. The blob carries file metadata and
// symbols (deterministic across snapshots); the parsed files are needed
// for source-driven scalars like cyclomatic complexity.
func computeMetrics(blob *Blob, files []*scanner.File) *Metrics {
	if blob == nil {
		return &Metrics{SchemaVersion: MetricsSchemaVersion}
	}

	complexityByPath := make(map[string]int, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		complexityByPath[relPath(f.Path, blob.RepoRoot)] = fileCyclomatic(f.Lines)
	}

	symbolCount := make(map[string]int)
	publicCount := make(map[string]int)
	for _, s := range blob.Symbols {
		symbolCount[s.File]++
		if s.Visibility == "public" || s.Visibility == "" {
			publicCount[s.File]++
		}
	}

	fileMetrics := make([]FileMetrics, 0, len(blob.Files))
	for _, f := range blob.Files {
		fileMetrics = append(fileMetrics, FileMetrics{
			Path:          f.Path,
			Module:        f.Module,
			Language:      f.Language,
			LOC:           f.Lines,
			Bytes:         f.Bytes,
			Symbols:       symbolCount[f.Path],
			PublicSymbols: publicCount[f.Path],
			Cyclomatic:    complexityByPath[f.Path],
		})
	}
	sort.Slice(fileMetrics, func(i, j int) bool { return fileMetrics[i].Path < fileMetrics[j].Path })

	moduleAgg := make(map[string]*ModuleMetrics)
	for _, m := range blob.Modules {
		moduleAgg[m.Path] = &ModuleMetrics{
			Path:   m.Path,
			FanIn:  len(m.Consumers),
			FanOut: len(m.Dependencies),
		}
	}
	for _, fm := range fileMetrics {
		if fm.Module == "" {
			continue
		}
		mm, ok := moduleAgg[fm.Module]
		if !ok {
			mm = &ModuleMetrics{Path: fm.Module}
			moduleAgg[fm.Module] = mm
		}
		mm.Files++
		mm.LOC += fm.LOC
		mm.Symbols += fm.Symbols
		mm.PublicSymbols += fm.PublicSymbols
		mm.Cyclomatic += fm.Cyclomatic
	}
	moduleMetrics := make([]ModuleMetrics, 0, len(moduleAgg))
	for _, mm := range moduleAgg {
		moduleMetrics = append(moduleMetrics, *mm)
	}
	sort.Slice(moduleMetrics, func(i, j int) bool { return moduleMetrics[i].Path < moduleMetrics[j].Path })

	return &Metrics{
		SchemaVersion: MetricsSchemaVersion,
		CommitSHA:     blob.CommitSHA,
		CapturedAt:    blob.CapturedAt,
		Files:         fileMetrics,
		Modules:       moduleMetrics,
	}
}

var cyclomaticDecisionRe = regexp.MustCompile(`\b(if|else\s+if|when|for|while|catch)\b|&&|\|\||\?:`)

// fileCyclomatic is a line-oriented decision-point count modeled on the
// existing riskmap heuristic in internal/cli/riskmap. Rough but stable;
// snapshots only need scalars that compare apples-to-apples across
// commits, not absolute correctness.
func fileCyclomatic(lines []string) int {
	complexity := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		complexity += len(cyclomaticDecisionRe.FindAllString(trimmed, -1))
		if strings.Contains(trimmed, " fun ") || strings.HasPrefix(trimmed, "fun ") {
			complexity++
		}
	}
	if complexity == 0 {
		return 1
	}
	return complexity
}

// metricsPath returns the on-disk location for a snapshot's metrics
// rollup. Sibling of BlobPath under the same per-sha directory so the
// pair is captured-and-deleted together.
func metricsPath(root, sha string) (string, error) {
	if len(sha) < 2 {
		return "", fmt.Errorf("snapshot: sha %q too short", sha)
	}
	return filepath.Join(root, "graphs", sha[:2], sha, metricsFileName), nil
}

// SaveMetrics writes m next to the existing graph blob for the same sha.
func SaveMetrics(root string, m *Metrics) (string, error) {
	if m == nil {
		return "", errors.New("snapshot: nil metrics")
	}
	if m.CommitSHA == "" {
		return "", errors.New("snapshot: metrics has no CommitSHA")
	}
	path, err := metricsPath(root, m.CommitSHA)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", filepath.Dir(path), err)
	}
	payload, err := cacheutil.EncodeZstdGob(m)
	if err != nil {
		return "", fmt.Errorf("snapshot: encode metrics: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	return path, nil
}

// LoadMetrics reads the metrics rollup for sha from root.
func LoadMetrics(root, sha string) (*Metrics, error) {
	path, err := metricsPath(root, sha)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open %s: %w", path, err)
	}
	defer f.Close()
	var m Metrics
	if err := cacheutil.DecodeZstdGob(f, &m); err != nil {
		return nil, fmt.Errorf("snapshot: decode %s: %w", path, err)
	}
	return &m, nil
}
