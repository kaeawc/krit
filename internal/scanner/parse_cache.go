package scanner

// On-disk cache of tree-sitter FlatTree results keyed by
// SHA-256(file_content). Invalidation is implicit: the content hash
// changes with any byte of the file, and the grammar version stored on
// each entry makes a tree-sitter-kotlin bump nuke every entry it ever
// wrote.

import (
	"bufio"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
)

const (
	parseCacheVersion    uint32 = 1
	parseCacheVersionStr        = "1"

	// Files below this threshold parse in under a millisecond; the gob
	// serialization + filesystem round-trip dominates the savings.
	parseCacheMinFileSize = 1024

	parseCacheDirName = "parse-cache"
	parseCacheEntries = "entries"
	parseCacheExt     = ".gob"
)

// parseCacheEntry is the on-disk gob payload. NodeTypeTable maps the
// entry's local FlatNode.Type indices back to node-type strings so a
// reader can re-intern them into its own global NodeTypeTable — crucial
// because the type table grows lazily and a fresh process's global
// indices won't match the writer's.
type parseCacheEntry struct {
	Version       uint32
	GrammarVer    string
	ContentHash   string
	NodeTypeTable []string
	Nodes         []FlatNode
}

// ParseCache persists FlatTree parse results keyed by content hash.
// A nil *ParseCache is a valid disabled cache — every method is a
// safe no-op.
type ParseCache struct {
	dir string
}

var (
	grammarVerOnce sync.Once
	grammarVer     string
)

// GrammarVersion returns a stable identifier for the tree-sitter grammar
// binding in use. Included on every cache entry so a grammar upgrade
// silently invalidates prior entries.
func GrammarVersion() string {
	grammarVerOnce.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/smacker/go-tree-sitter" {
					grammarVer = "smacker/go-tree-sitter@" + dep.Version
					return
				}
			}
		}
		grammarVer = "smacker/go-tree-sitter@unknown"
	})
	return grammarVer
}

// NewParseCache returns a ParseCache rooted at repoDir/.krit/parse-cache.
// A schema-version or grammar-version mismatch in the existing metadata
// clears the entries subtree.
func NewParseCache(repoDir string) (*ParseCache, error) {
	if repoDir == "" {
		return nil, errors.New("scanner: NewParseCache requires a non-empty repoDir")
	}
	dir := filepath.Join(repoDir, ".krit", parseCacheDirName)
	entriesDir := filepath.Join(dir, parseCacheEntries)
	if err := os.MkdirAll(entriesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create parse cache dir: %w", err)
	}
	gv := GrammarVersion()

	vPath := filepath.Join(dir, "version")
	gPath := filepath.Join(dir, "grammar-version")
	nuke := false
	if b, err := os.ReadFile(vPath); err == nil && string(b) != parseCacheVersionStr {
		nuke = true
	}
	if !nuke {
		if b, err := os.ReadFile(gPath); err == nil && string(b) != gv {
			nuke = true
		}
	}
	if nuke {
		_ = os.RemoveAll(entriesDir)
		_ = os.MkdirAll(entriesDir, 0o755)
	}
	_ = os.WriteFile(vPath, []byte(parseCacheVersionStr), 0o644)
	_ = os.WriteFile(gPath, []byte(gv), 0o644)

	return &ParseCache{dir: dir}, nil
}

// Dir returns the root directory of the on-disk cache.
func (pc *ParseCache) Dir() string {
	if pc == nil {
		return ""
	}
	return pc.dir
}

func hashContent(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// entryPath returns the sharded on-disk path for a content hash.
// Layout: entries/{hash[:2]}/{hash[2:]}.gob — two-level sharding so no
// single directory grows past 256 shards even on huge repos.
func (pc *ParseCache) entryPath(hash string) string {
	if len(hash) < 3 {
		return filepath.Join(pc.dir, parseCacheEntries, "_", hash+parseCacheExt)
	}
	return filepath.Join(pc.dir, parseCacheEntries, hash[:2], hash[2:]+parseCacheExt)
}

// Load tries to load a cached FlatTree for the given content. Returns
// (tree, true) on hit, (nil, false) on miss, small file, or any
// read/decode error. A nil ParseCache is always a miss.
func (pc *ParseCache) Load(content []byte) (*FlatTree, bool) {
	if pc == nil {
		return nil, false
	}
	if len(content) < parseCacheMinFileSize {
		return nil, false
	}
	hash := hashContent(content)
	return pc.loadByHash(hash)
}

func (pc *ParseCache) loadByHash(hash string) (*FlatTree, bool) {
	path := pc.entryPath(hash)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var entry parseCacheEntry
	if err := gob.NewDecoder(f).Decode(&entry); err != nil {
		// Corrupt entry: drop it so we don't keep re-reading a doomed
		// payload on every run.
		_ = os.Remove(path)
		return nil, false
	}
	if entry.Version != parseCacheVersion ||
		entry.GrammarVer != GrammarVersion() ||
		entry.ContentHash != hash {
		return nil, false
	}

	remapEntryNodes(entry.Nodes, entry.NodeTypeTable)
	return &FlatTree{Nodes: entry.Nodes}, true
}

// remapEntryNodes rewrites each node's Type from the entry's local
// index (into localTable) to the current process's global NodeTypeTable
// index. Done in place on the node slice returned in the entry.
func remapEntryNodes(nodes []FlatNode, localTable []string) {
	if len(localTable) == 0 {
		return
	}
	remap := make([]uint16, len(localTable))
	for i, name := range localTable {
		remap[i] = internNodeType(name)
	}
	for i := range nodes {
		if int(nodes[i].Type) < len(remap) {
			nodes[i].Type = remap[nodes[i].Type]
		}
	}
}

// Save persists the parse result for content under its SHA-256. Small
// files are skipped. A returned error means the write failed and the
// next run will miss; callers typically discard it.
func (pc *ParseCache) Save(content []byte, tree *FlatTree) error {
	if pc == nil || tree == nil {
		return nil
	}
	if len(content) < parseCacheMinFileSize {
		return nil
	}
	return pc.saveEntry(hashContent(content), tree)
}

func (pc *ParseCache) saveEntry(hash string, tree *FlatTree) error {
	local, cloned := buildLocalTableAndNodes(tree.Nodes)

	target := pc.entryPath(hash)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".entry-*.tmp")
	if err != nil {
		return fmt.Errorf("parse cache tempfile: %w", err)
	}
	tmpName := tmp.Name()

	bw := bufio.NewWriter(tmp)
	err = gob.NewEncoder(bw).Encode(&parseCacheEntry{
		Version:       parseCacheVersion,
		GrammarVer:    GrammarVersion(),
		ContentHash:   hash,
		NodeTypeTable: local,
		Nodes:         cloned,
	})
	if err == nil {
		err = bw.Flush()
	}
	if err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("encode parse cache entry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close parse cache tempfile: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename parse cache entry: %w", err)
	}
	return nil
}

// buildLocalTableAndNodes walks nodes, collects the set of global Type
// IDs actually used, and produces a parallel array of their string names
// plus a clone of the node slice with Type rewritten to indices into
// that local table.
func buildLocalTableAndNodes(nodes []FlatNode) ([]string, []FlatNode) {
	var maxType uint16
	for _, n := range nodes {
		if n.Type > maxType {
			maxType = n.Type
		}
	}
	// Dense lookup: globalToLocal[g] == 0 is the "unseen" sentinel, so
	// stored slots are offset by 1 and subtracted on read.
	globalToLocal := make([]uint16, int(maxType)+1)
	local := make([]string, 0, 32)
	cloned := make([]FlatNode, len(nodes))
	copy(cloned, nodes)
	for i := range cloned {
		g := cloned[i].Type
		slot := globalToLocal[g]
		if slot == 0 {
			local = append(local, nodeTypeName(g))
			slot = uint16(len(local))
			globalToLocal[g] = slot
		}
		cloned[i].Type = slot - 1
	}
	return local, cloned
}

// Clear removes every cache entry. The version / grammar-version
// metadata files are left in place so a subsequent NewParseCache call
// does not see a schema mismatch.
func (pc *ParseCache) Clear() error {
	if pc == nil {
		return nil
	}
	entries := filepath.Join(pc.dir, parseCacheEntries)
	if err := os.RemoveAll(entries); err != nil {
		return fmt.Errorf("clear parse cache: %w", err)
	}
	return os.MkdirAll(entries, 0o755)
}

// ClearParseCache removes the parse-cache directory under repoDir.
// Used by --clear-cache at the CLI boundary; a no-op when the cache
// directory does not exist.
func ClearParseCache(repoDir string) error {
	if repoDir == "" {
		return nil
	}
	dir := filepath.Join(repoDir, ".krit", parseCacheDirName)
	err := os.RemoveAll(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear parse cache: %w", err)
	}
	return nil
}
