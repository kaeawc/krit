package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
)

const blobFileName = "graph.gob.zst"

// SnapshotsDir returns the snapshots root inside repoRoot.
func SnapshotsDir(repoRoot string) string {
	if repoRoot == "" {
		repoRoot = "."
	}
	return filepath.Join(repoRoot, ".krit", "snapshots")
}

// shaDir returns the per-sha directory under root. The two-character
// prefix mirrors git's loose-object layout so directory fan-out stays
// bounded on repos with many captured shas.
func shaDir(root, sha string) (string, error) {
	if len(sha) < 2 {
		return "", fmt.Errorf("snapshot: sha %q too short", sha)
	}
	return filepath.Join(root, "graphs", sha[:2], sha), nil
}

func BlobPath(root, sha string) (string, error) {
	dir, err := shaDir(root, sha)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, blobFileName), nil
}

// Save writes b atomically to its content-addressed location under root
// and returns the path written.
func Save(root string, b *Blob) (string, error) {
	if b == nil {
		return "", errors.New("snapshot: nil blob")
	}
	if b.CommitSHA == "" {
		return "", errors.New("snapshot: blob has no CommitSHA")
	}
	path, err := BlobPath(root, b.CommitSHA)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("snapshot: mkdir %s: %w", filepath.Dir(path), err)
	}
	payload, err := cacheutil.EncodeZstdGob(b)
	if err != nil {
		return "", fmt.Errorf("snapshot: encode blob: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("snapshot: write %s: %w", path, err)
	}
	return path, nil
}

func Load(root, sha string) (*Blob, error) {
	path, err := BlobPath(root, sha)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot: open %s: %w", path, err)
	}
	defer f.Close()
	var b Blob
	if err := cacheutil.DecodeZstdGob(f, &b); err != nil {
		return nil, fmt.Errorf("snapshot: decode %s: %w", path, err)
	}
	migrated, err := MigrateBlob(&b)
	if err != nil {
		return nil, fmt.Errorf("snapshot: %s: %w", path, err)
	}
	return migrated, nil
}

// Entry is one captured snapshot, returned by List.
type Entry struct {
	CommitSHA string
	Path      string
	Bytes     int64
}

// List returns every captured snapshot under root, sorted by sha.
// Entries that fail to stat are skipped silently.
func List(root string) ([]Entry, error) {
	graphsDir := filepath.Join(root, "graphs")
	prefixes, err := os.ReadDir(graphsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, prefix := range prefixes {
		if !prefix.IsDir() || len(prefix.Name()) != 2 {
			continue
		}
		shaEntries, err := os.ReadDir(filepath.Join(graphsDir, prefix.Name()))
		if err != nil {
			continue
		}
		for _, shaEntry := range shaEntries {
			if !shaEntry.IsDir() {
				continue
			}
			sha := shaEntry.Name()
			if !strings.HasPrefix(sha, prefix.Name()) {
				continue
			}
			blob := filepath.Join(graphsDir, prefix.Name(), sha, blobFileName)
			info, err := os.Stat(blob)
			if err != nil {
				continue
			}
			out = append(out, Entry{CommitSHA: sha, Path: blob, Bytes: info.Size()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CommitSHA < out[j].CommitSHA })
	return out, nil
}
