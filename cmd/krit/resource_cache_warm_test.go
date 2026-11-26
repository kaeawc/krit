package main

import (
	"encoding/json"
	"testing"
)

// A warm run that misses the Android findings cache but loads the resource
// index cache must see the same index as a fresh parse (#1830). The cached
// values index used to lose its <item type="id"> declarations, so
// NotSiblingResource dropped its finding on @id/predeclared.
func TestWarmResourceIndexKeepsDeclaredIDs(t *testing.T) {
	dir := copyTree(t, "../../tests/fixtures/android-lint/UnknownId/negative")
	count := func(extra ...string) int {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "-f", "json", "-q"}, extra...)
		stdout, stderr, code := runKrit(t, append(args, dir)...)
		if code > 1 {
			t.Fatalf("krit %v exited %d: %s", extra, code, stderr)
		}
		var report struct {
			Findings []scopeFinding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("invalid JSON for %v: %v", extra, err)
		}
		n := 0
		for _, f := range report.Findings {
			if f.Rule == "NotSiblingResource" {
				n++
			}
		}
		return n
	}
	count() // fills the resource index and Android findings caches
	// Another rule set misses the Android findings cache but not the
	// resource index cache.
	warm := count("--disable-rules", "MagicNumber")
	fresh := count("--no-cache", "--no-resource-cache", "--disable-rules", "MagicNumber")
	if fresh == 0 {
		t.Fatal("no NotSiblingResource finding on a fresh scan; the fixture exercises nothing")
	}
	if warm != fresh {
		t.Fatalf("warm NotSiblingResource findings = %d, fresh = %d", warm, fresh)
	}
}
