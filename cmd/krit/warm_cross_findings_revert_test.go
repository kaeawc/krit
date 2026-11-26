package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A file returned to an earlier version is a hit in every per-file cache, so
// the warm cross-findings reuse used to see the same paths and serve the
// previous run's cross findings (#1820). B.kt removes, re-adds and removes
// again its call to A.kt's helper(), with an unrelated edit in between; each
// warm run must report what a fresh run does.
func TestWarmCrossFindingsFollowRevertedFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scan := func(extra ...string) string {
		t.Helper()
		args := append([]string{"--no-fir", "--no-daemon", "--no-type-oracle", "--enable-rules", "DeadCode", "-f", "json", "-q"}, extra...)
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
		var keys []string
		for _, f := range report.Findings {
			if f.Rule == "DeadCode" {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " ")
	}
	const calls = "package p\n\nfun caller(): Int = helper()\n"
	const noCall = "package p\n\nfun caller(): Int = 0\n"
	write("A.kt", "package p\n\nfun helper(): Int = 1\n")
	write("B.kt", calls)
	write("C.kt", "package p\n\nfun other(): Int = 2\n")

	steps := []struct{ name, file, content string }{
		{"cold", "", ""},
		{"remove call", "B.kt", noCall},
		{"re-add call", "B.kt", calls},
		{"unrelated edit", "C.kt", "package p\n\nfun other(): Int = 3\n"},
		{"remove call again", "B.kt", noCall},
		{"re-add call again", "B.kt", calls},
	}
	for _, s := range steps {
		if s.file != "" {
			write(s.file, s.content)
		}
		warm := scan()
		fresh := scan("--no-cache", "--no-cross-file-cache")
		if warm != fresh {
			t.Fatalf("%s: warm DeadCode findings [%s], fresh [%s]", s.name, warm, fresh)
		}
	}
}
