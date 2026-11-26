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
