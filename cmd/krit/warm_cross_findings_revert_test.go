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
	const calls = "package p\n\nfun caller(): Int = helper()\n"
	const noCall = "package p\n\nfun caller(): Int = 0\n"
	files := map[string]string{
		"A.kt": "package p\n\nfun helper(): Int = 1\n",
		"B.kt": calls,
		"C.kt": "package p\n\nfun other(): Int = 2\n",
	}
	steps := []warmCrossStep{
		{"cold", "", ""},
		{"remove call", "B.kt", noCall},
		{"re-add call", "B.kt", calls},
		{"unrelated edit", "C.kt", "package p\n\nfun other(): Int = 3\n"},
		{"remove call again", "B.kt", noCall},
		{"re-add call again", "B.kt", calls},
	}
	t.Run("default rules", func(t *testing.T) { runWarmCrossSteps(t, "DeadCode", false, files, steps) })
	t.Run("DeadCode only", func(t *testing.T) { runWarmCrossSteps(t, "DeadCode", true, files, steps) })
}

// With only DeadCode active no rule needs Java before dispatch, so Java files
// sit outside the per-file cache and a Java edit leaves every Kotlin entry a
// hit. The warm cross-findings reuse must hash those Java files from disk
// (#1820). J.java's call is the only reference to A.kt's helper(); dropping
// it, adding it back and dropping it again must each match a fresh run.
func TestWarmCrossFindingsFollowJavaEdit(t *testing.T) {
	const calls = "package p;\n\npublic class J {\n    public int run() {\n        return AKt.helper();\n    }\n}\n"
	const noCall = "package p;\n\npublic class J {\n    public int run() {\n        return 0;\n    }\n}\n"
	runWarmCrossSteps(t, "DeadCode", true, map[string]string{
		"A.kt":   "package p\n\nfun helper(): Int = 1\n",
		"J.java": calls,
	}, []warmCrossStep{
		{"cold", "", ""},
		{"remove Java call", "J.java", noCall},
		{"re-add Java call", "J.java", calls},
		{"remove Java call again", "J.java", noCall},
	})
}

// With a per-file rule alone, a warm run parses only its cache misses, and
// hashing that result keyed the findings bundle on the misses only: once A.kt
// returned to an earlier content, the run loaded the bundle of that earlier
// state although B.kt had changed since (#1854 review).
func TestWarmFindingsFollowMissOnlyRevert(t *testing.T) {
	const plain = "package p\n\nfun %s() {\n    val x = 0\n}\n"
	const magic = "package p\n\nfun %s() {\n    val x = %d\n}\n"
	runWarmCrossSteps(t, "MagicNumber", true, map[string]string{
		"A.kt": fmt.Sprintf(plain, "a"),
		"B.kt": fmt.Sprintf(plain, "b"),
	}, []warmCrossStep{
		{"cold", "", ""},
		{"A to a2", "A.kt", fmt.Sprintf(magic, "a", 42)},
		{"B to b2", "B.kt", fmt.Sprintf(magic, "b", 42)},
		{"A to a3", "A.kt", fmt.Sprintf(plain, "a")},
		{"A back to a2", "A.kt", fmt.Sprintf(magic, "a", 42)},
	})
}

type warmCrossStep struct{ name, file, content string }

// runWarmCrossSteps writes files, then applies each step's edit and checks
// that two warm runs of rule report what a run with no caches does. With only,
// rule is the only active rule. The second warm run sees every file a
// per-file cache hit, which with only lets it skip parsing; it once loaded the
// findings bundle an earlier state saved, because without content the bundle
// key covered paths only.
func runWarmCrossSteps(t *testing.T, rule string, only bool, files map[string]string, steps []warmCrossStep) {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := []string{"--no-fir", "--no-daemon", "--no-type-oracle", "--enable-rules", rule, "-f", "json", "-q"}
	if only {
		base = append(base, "--config", allCategoriesOffConfig(t))
	}
	scan := func(extra ...string) string {
		t.Helper()
		args := append(append([]string(nil), base...), extra...)
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
			if f.Rule == rule {
				keys = append(keys, fmt.Sprintf("%s:%d", filepath.Base(f.File), f.Line))
			}
		}
		sort.Strings(keys)
		return strings.Join(keys, " ")
	}
	for name, content := range files {
		write(name, content)
	}
	for _, s := range steps {
		if s.file != "" {
			write(s.file, s.content)
		}
		warm := scan()
		warmAgain := scan()
		fresh := scan("--no-cache", "--no-cross-file-cache")
		if warm != fresh || warmAgain != fresh {
			t.Fatalf("%s: warm %s findings [%s], second warm [%s], fresh [%s]", s.name, rule, warm, warmAgain, fresh)
		}
	}
}

// allCategoriesOffConfig writes a config, outside the scanned tree, that turns
// off every rule category, so --enable-rules leaves only the rules it names
// active. The categories come from --list-rules so a new one stays off.
func allCategoriesOffConfig(t *testing.T) string {
	t.Helper()
	stdout, stderr, code := runKrit(t, "--list-rules", "-f", "json")
	if code != 0 {
		t.Fatalf("krit --list-rules exited %d: %s", code, stderr)
	}
	var listed struct {
		Rules []struct {
			Category string `json:"category"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatalf("invalid --list-rules JSON: %v", err)
	}
	seen := map[string]bool{}
	var b strings.Builder
	for _, r := range listed.Rules {
		if r.Category != "" && !seen[r.Category] {
			seen[r.Category] = true
			fmt.Fprintf(&b, "%s:\n  active: false\n", r.Category)
		}
	}
	path := filepath.Join(t.TempDir(), "krit.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A miss is reused without re-running the cross-file rules only when it is
// lexically irrelevant to them. Removing @VisibleForTesting leaves TestHooks.kt
// with nothing the rule looks for, but the content that produced the reused
// findings had it, so the reuse must still be refused (#1844).
func TestWarmCrossFindingsFollowLexicallyRelevantRemoval(t *testing.T) {
	const hooks = "app/src/main/java/com/example/TestHooks.kt"
	const annotated = "package com.example\n\nobject TestHooks {\n    @VisibleForTesting\n    fun resetForTests() = Unit\n}\n"
	const plain = "package com.example\n\nobject TestHooks {\n    fun resetForTests() = Unit\n}\n"
	runWarmCrossSteps(t, "VisibleForTestingCallerInNonTest", true, map[string]string{
		hooks: annotated,
		"app/src/main/java/com/example/Production.kt": "package com.example\n\nfun production() {\n    TestHooks.resetForTests()\n}\n",
	}, []warmCrossStep{
		{"cold", "", ""},
		{"remove @VisibleForTesting", hooks, plain},
		{"re-add @VisibleForTesting", hooks, annotated},
		{"remove @VisibleForTesting again", hooks, plain},
	})
}

// VisibleForTestingCallerInNonTest reports on the caller, which need not
// mention @VisibleForTesting, so an edit to the caller alone must re-run it:
// removing the call drops the finding and adding it back restores it (#1893).
func TestWarmCrossFindingsFollowCallerEdits(t *testing.T) {
	const production = "app/src/main/java/com/example/Production.kt"
	const calls = "package com.example\n\nfun production() {\n    TestHooks.reset()\n}\n"
	const noCall = "package com.example\n\nfun production() {\n    println(\"x\")\n}\n"
	runWarmCrossSteps(t, "VisibleForTestingCallerInNonTest", true, map[string]string{
		"app/src/main/java/com/example/TestHooks.kt": "package com.example\n\nobject TestHooks {\n    @VisibleForTesting\n    fun reset() = Unit\n}\n",
		production: calls,
	}, []warmCrossStep{
		{"cold", "", ""},
		{"caller drops the call", production, noCall},
		{"caller adds the call back", production, calls},
	})
}
