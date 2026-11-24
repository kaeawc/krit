package cache

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
)

// Incremental cache writes skip fsync, so after an OS crash or power loss the
// snapshot or a store entry can be empty, cut short, zero-filled, or hold
// unrelated bytes. Each must load as a miss (forcing re-analysis), never as
// cached findings.

func tornCacheVariants(valid []byte) map[string][]byte {
	variants := map[string][]byte{
		"empty":       {},
		"zero-filled": make([]byte, len(valid)),
		"garbage":     bytes.Repeat([]byte("garbage!"), len(valid)/8+1),
	}
	for _, n := range []int{1, 4, len(valid) / 4, len(valid) / 2, len(valid) - 1} {
		if n > 0 && n < len(valid) {
			variants["truncated-"+strconv.Itoa(n)] = append([]byte(nil), valid[:n]...)
		}
	}
	return variants
}

func TestCacheSnapshot_TornFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.cache")
	if err := makePortabilityCache().Save(path); err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(Load(path).Files); got == 0 {
		t.Fatal("valid snapshot loaded no files")
	}
	for name, data := range tornCacheVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if got := len(Load(path).Files); got != 0 {
				t.Fatalf("damaged snapshot loaded %d files, want 0", got)
			}
		})
	}
}

func TestStoreBackedCache_TornEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "torn.kt")
	if err := os.WriteFile(filePath, []byte("fun torn() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()
	c := &Cache{Files: make(map[string]FileEntry)}
	c.AttachStore(store.New(storeDir), [16]byte{})
	cols := scanner.CollectFindings([]scanner.Finding{
		{File: filePath, Line: 1, Col: 1, Severity: "warning", RuleSet: "style", Rule: "R", Message: "m"},
	})
	c.UpdateEntryColumns(filePath, &cols)

	var entry string
	_ = filepath.WalkDir(storeDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".bin" {
			entry = p
		}
		return nil
	})
	if entry == "" {
		t.Fatal("no store entry written")
	}
	valid, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.CheckFiles([]string{filePath}, "h").TotalCached; got != 1 {
		t.Fatalf("valid entry: cached = %d, want 1", got)
	}
	for name, data := range tornCacheVariants(valid) {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(entry, data, 0o644); err != nil {
				t.Fatal(err)
			}
			result := c.CheckFiles([]string{filePath}, "h")
			if result.TotalCached != 0 || result.CachedColumns.Len() != 0 {
				t.Fatalf("damaged entry: cached = %d, findings = %d; want a miss", result.TotalCached, result.CachedColumns.Len())
			}
		})
	}
}
