package main

import (
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// --no-cache is the fresh-analysis baseline every cache test compares a warm
// run against, so it must not read the cross-file findings cache (#1818). A
// stale slot under the current key used to replay into a --no-cache run.
func TestNoCacheIgnoresStaleCrossFindingsCache(t *testing.T) {
	dir := copyTree(t, "../../playground/kotlin-webservice")
	// Earlier scans of the playground leave a .krit there; start cold.
	if err := os.RemoveAll(filepath.Join(dir, ".krit")); err != nil {
		t.Fatal(err)
	}
	count := func(extra ...string) int {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []json.RawMessage `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		return len(report.Findings)
	}

	fresh := count("--no-cache")
	// A cold --no-cache run writes none of the caches it skips.
	for _, name := range []string{"cross-findings-cache", "cross-file-cache", "filewalk-cache", "resource-cache", "findings-bundle-cache", "tracked-files-cache"} {
		if _, err := os.Stat(filepath.Join(dir, ".krit", name)); !os.IsNotExist(err) {
			t.Errorf("cold --no-cache run created .krit/%s (stat err %v)", name, err)
		}
	}
	count() // fills .krit/cross-findings-cache
	slot := crossFindingsSlot(t, dir)
	// A finding no rule reports: --no-cache must never print it.
	stale := scanner.CollectFindings([]scanner.Finding{{
		File: filepath.Join(dir, "StaleCrossFinding.kt"), Line: 1, Col: 1,
		RuleSet: "stale", Rule: "StaleCrossFinding", Severity: "warning", Message: "replayed from the cross-findings cache",
	}})
	if err := scanner.SaveCrossFindings(filepath.Dir(slot), crossFindingsSlotKey(t, slot), stale); err != nil {
		t.Fatal(err)
	}
	if got := count("--no-cache"); got != fresh {
		t.Fatalf("--no-cache over a stale cross-findings cache = %d findings, fresh = %d", got, fresh)
	}
}

// --no-cache must not read the cross-file index cache either (#1831). The
// seed is a real index from an earlier project state, saved under the
// current state's key: B.kt referenced helper() then, so a run that trusts
// the seed stops reporting it as dead code.
func TestNoCacheIgnoresStaleCrossFileIndex(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	deadCode := func(extra ...string) int {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "--enable-rules", "DeadCode", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []struct {
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "DeadCode" {
				n++
			}
		}
		return n
	}
	cacheDir := scanner.CrossFileCacheDir(dir)

	write("A.kt", "package p\n\nfun helper() = 1\n")
	write("B.kt", "package p\n\nfun main() {\n    println(helper())\n}\n")
	if got := deadCode(); got != 0 {
		t.Fatalf("referenced helper(): %d DeadCode findings, want 0", got)
	}
	staleIdx, staleMeta, ok := scanner.LoadCurrentCrossFileCacheIndex(cacheDir)
	if !ok {
		t.Fatal("the first run left no cross-file index cache")
	}

	write("B.kt", "package p\n\nfun main() {\n    println(1)\n}\n")
	fresh := deadCode("--no-cache")
	if fresh != 1 {
		t.Fatalf("fresh --no-cache run: %d DeadCode findings, want 1", fresh)
	}
	deadCode() // writes the current state's cross-file index
	current, ok := scanner.LoadCurrentCrossFileCacheMeta(cacheDir)
	if !ok {
		t.Fatal("no cross-file index cache for the current state")
	}
	// Keep only the cross-file index, so no findings cache answers first.
	entries, err := os.ReadDir(filepath.Join(dir, ".krit"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(cacheDir) {
			if err := os.RemoveAll(filepath.Join(dir, ".krit", e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	current.PayloadEntries = staleMeta.PayloadEntries
	if err := scanner.SaveCrossFileCacheIndex(cacheDir, current.Fingerprint, current, staleIdx); err != nil {
		t.Fatal(err)
	}
	if got := deadCode(); got != 0 {
		t.Fatalf("a run that reads the seeded index reported %d DeadCode findings, want 0; the seed no longer reproduces a stale index", got)
	}
	if got := deadCode("--no-cache"); got != fresh {
		t.Fatalf("--no-cache over a stale cross-file index = %d DeadCode findings, fresh = %d", got, fresh)
	}
}

// crossFindingsSlot returns the cross-findings cache file a scan of dir wrote.
func crossFindingsSlot(t *testing.T, dir string) string {
	t.Helper()
	var slot string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(filepath.Dir(path)) == "cross-findings-cache" {
			slot = path
		}
		return nil
	})
	if slot == "" {
		t.Fatalf("no cross-findings cache under %s", dir)
	}
	return slot
}

// crossFindingsSlotKey reads the key from the slot's header: magic, version,
// then a big-endian uint16 key length and the key.
func crossFindingsSlotKey(t *testing.T, slot string) string {
	t.Helper()
	data, err := os.ReadFile(slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 10 {
		t.Fatalf("cross-findings slot is %d bytes", len(data))
	}
	n := int(binary.BigEndian.Uint16(data[8:10]))
	if len(data) < 10+n {
		t.Fatalf("cross-findings slot key length %d exceeds the file", n)
	}
	return string(data[10 : 10+n])
}
