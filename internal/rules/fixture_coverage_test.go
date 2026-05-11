package rules

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestFixtureCoverage asserts that every rule registered in
// api.Registry has both a positive and a negative fixture under
// tests/fixtures/. Pre-existing gaps are listed in
// tests/fixtures/.coverage-allowlist.txt as a ratchet: the test fails
// if a rule NOT on the allowlist is missing a fixture (so a new rule
// cannot land without one), and it also fails if a rule ON the
// allowlist now has the fixture (so the list shrinks instead of
// becoming silent dead weight).
//
// Fixture filename convention: tests/fixtures/<positive|negative>/<category>/<RuleID>.kt
// or .java. The test does not enforce a category; it only checks that
// some file with the rule's ID as basename exists in the right kind
// directory.
func TestFixtureCoverage(t *testing.T) {
	root := repoRoot(t)
	fixtureRoot := filepath.Join(root, "tests", "fixtures")

	posIndex := indexFixtures(t, filepath.Join(fixtureRoot, "positive"))
	negIndex := indexFixtures(t, filepath.Join(fixtureRoot, "negative"))

	allow, err := readAllowlist(filepath.Join(fixtureRoot, ".coverage-allowlist.txt"))
	if err != nil {
		t.Fatalf("read allowlist: %v", err)
	}

	var missing, stale []string
	seenIDs := make(map[string]bool, len(api.Registry))
	for _, r := range api.Registry {
		if r == nil || r.ID == "" {
			continue
		}
		if seenIDs[r.ID] {
			continue
		}
		seenIDs[r.ID] = true
		if r.Category == api.CategoryPrecompile {
			continue
		}

		for _, kind := range []string{"positive", "negative"} {
			has := posIndex[r.ID]
			if kind == "negative" {
				has = negIndex[r.ID]
			}
			key := kind + " " + r.ID
			onAllow := allow[key]
			switch {
			case has && onAllow:
				stale = append(stale, key)
			case !has && !onAllow:
				missing = append(missing, key)
			}
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("rules missing fixtures (add files under tests/fixtures/<kind>/<category>/<RuleID>.kt):\n  %s",
			strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("allowlist entries that now have fixtures — remove from tests/fixtures/.coverage-allowlist.txt:\n  %s",
			strings.Join(stale, "\n  "))
	}
}

// indexFixtures walks dir and records every basename (without
// extension) as a key in the returned set. The fixture directory tree
// uses category subfolders, so a flat walk is the simplest match
// against rule IDs.
func indexFixtures(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := make(map[string]bool)
	if err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		ext := filepath.Ext(p)
		if ext != ".kt" && ext != ".java" && ext != ".kts" {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(p), ext)
		out[base] = true
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// readAllowlist parses the coverage allowlist into a set keyed by
// "<kind> <RuleID>". Comment lines (#) and blank lines are skipped.
func readAllowlist(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make(map[string]bool)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, &allowlistError{line: line}
		}
		kind := fields[0]
		if kind != "positive" && kind != "negative" {
			return nil, &allowlistError{line: line}
		}
		out[kind+" "+fields[1]] = true
	}
	return out, s.Err()
}

type allowlistError struct{ line string }

func (e *allowlistError) Error() string {
	return "allowlist: malformed line (want \"positive|negative <RuleID>\"): " + e.line
}

// TestReadAllowlist_ParsesAndRejects covers the allowlist parser
// directly so a malformed line surfaces with a clear error rather than
// silently being treated as covered.
func TestReadAllowlist_ParsesAndRejects(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "allow.txt")
	contents := "# header\n\npositive RuleA\nnegative RuleB\n"
	if err := os.WriteFile(tmp, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readAllowlist(tmp)
	if err != nil {
		t.Fatalf("readAllowlist: %v", err)
	}
	if !got["positive RuleA"] || !got["negative RuleB"] {
		t.Errorf("missing expected entries: %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}

	bad := filepath.Join(t.TempDir(), "bad.txt")
	if err := os.WriteFile(bad, []byte("garbled line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readAllowlist(bad); err == nil {
		t.Error("expected error on malformed line, got nil")
	}

	wrongKind := filepath.Join(t.TempDir(), "wk.txt")
	if err := os.WriteFile(wrongKind, []byte("maybe RuleX\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readAllowlist(wrongKind); err == nil {
		t.Error("expected error on unknown kind, got nil")
	}
}

// repoRoot returns the module root by walking up from the test source
// file until go.mod is found. The test runs from internal/rules/, so
// relative paths to tests/fixtures/ require this anchor.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate test source")
	}
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("walked to filesystem root without finding go.mod")
		}
		dir = parent
	}
}
