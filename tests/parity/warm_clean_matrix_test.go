package parity_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Warm-versus-clean mutation matrix (#786).
//
// Every case seeds a real warm cache or session on a small project, applies
// one mutation, runs warm, and then runs a genuinely fresh session with an
// isolated cache over a copy of the same final tree. The two results must be
// identical. A seed run that did not populate the cache, or a no-op control
// run that did not hit it, fails the case: otherwise a pipeline that simply
// recomputes everything would pass vacuously.
//
// This file holds the project, the mutation table and the comparison helpers
// shared by the structural (no oracle), krit-fir and krit-types (KAA) runners.

// wcEffect says how the mutation is expected to move the result, so a
// mutation that silently stopped mattering (and would pass any comparison)
// is caught.
type wcEffect int

const (
	// wcEffectSame: the clean result after the mutation equals the clean
	// result before it.
	wcEffectSame wcEffect = iota
	// wcEffectChanges: the clean result after the mutation differs from the
	// clean result before it, so a stale cache entry has something to be
	// stale about.
	wcEffectChanges
	// wcEffectAny: no expectation (a rename changes keys, nothing more).
	wcEffectAny
)

func (e wcEffect) String() string {
	switch e {
	case wcEffectSame:
		return "unchanged"
	case wcEffectChanges:
		return "changes"
	}
	return "any"
}

// wcMutation is one row of the matrix.
type wcMutation struct {
	name string
	// extra are base files (keyed by path under the project root) added to
	// the base project for this case only.
	extra map[string]string
	// apply edits the project tree between the seed and the warm run. Nil
	// for mutations that change an input other than a source file.
	apply func(t *testing.T, root string)
	// effect is the expected effect on the clean result.
	effect wcEffect
	// oracleOnly marks mutations that only mean something with a compiler
	// backend (the structural runner skips them).
	oracleOnly bool
	// structuralOnly marks mutations whose interesting effect is on the
	// pipeline's findings caches, not on compiler facts, so the (slow)
	// compiler-backed runners skip them.
	structuralOnly bool
	// raceAnalyze makes the daemon rows analyze once straight after the edit,
	// before the watcher's event can have landed, and again once it has.
	raceAnalyze bool
	// classpath, when set, changes the oracle classpath between seed and
	// warm run by adding the compiled helper jar (see wcClasspathJar).
	classpath bool
	// jarIdentity, when set, swaps the backend helper jar for a byte-different
	// copy between seed and warm run.
	jarIdentity bool
	// skip, when non-empty, records the case as skipped with this reason.
	skip string
}

const (
	wcLib = `package p

class R {
    val name: String? = null
}

fun helper(): R? = R()

fun pick(x: Any): Int? = null

fun compute(x: Int): Int = x

fun fresh(): Int = 1

class Box private constructor() {
    companion object {
        operator fun invoke(): Box = Box()
    }
}
`
	wcAlt = `package p.alt

import p.R

fun helper(): R = R()
`
	wcUse = `package q

import p.Box
import p.compute
import p.fresh
import p.helper
import p.pick

fun useHelper() = helper()!!

fun usePick() = pick(1)!!

fun useName(r: p.R) = r.name!!

fun useCompute(): Int = compute(3)

fun useFresh(): Int = fresh()

fun useBox(): Box = Box()
`
	// wcUsesJar reaches into the helper jar built for the classpath case.
	// Only the classpath case carries it; elsewhere the reference would be a
	// constant unresolved-reference error.
	wcUsesJar = `package q

fun jarGreeting(): String = lib.Greeter.greet()
`
	wcOver = `package p

fun pick(x: Int): Int = 1
`
)

// wcBaseFiles is the project every case starts from, keyed by path relative
// to the project root. Sources live under src/ so a relative scan root can be
// the short, `krit .`-style "src".
func wcBaseFiles(m wcMutation) map[string]string {
	files := map[string]string{
		"src/Lib.kt": wcLib,
		"src/Alt.kt": wcAlt,
		"src/Use.kt": wcUse,
	}
	for rel, body := range m.extra {
		files[rel] = body
	}
	return files
}

func wcReplace(t *testing.T, root, rel, old, repl string) {
	t.Helper()
	path := filepath.Join(root, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), old) != 1 {
		t.Fatalf("mutation anchor %q does not occur exactly once in %s", old, rel)
	}
	writeFile(t, path, strings.Replace(string(raw), old, repl, 1))
}

// wcMutations is the matrix. The "effective compiler-setting change" row is a
// clearly marked skip until #784 wires that propagation.
func wcMutations() []wcMutation {
	return []wcMutation{
		{
			name: "comment-only",
			apply: func(t *testing.T, root string) {
				path := filepath.Join(root, "src", "Lib.kt")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, string(raw)+"\n// trailing note\n")
			},
			effect: wcEffectSame,
		},
		{
			// A body edit that adds a finding and touches no declaration or
			// reference: the case the structural bundle replay (#154) is built
			// to serve from the previous run.
			name: "body-edit-adds-finding",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Use.kt", "fun useFresh(): Int = fresh()", "fun useFresh(): Int = fresh()!!")
			},
			effect:         wcEffectChanges,
			structuralOnly: true,
		},
		{
			// The same edit, but the daemon is asked to analyze in the instant
			// after the save, before its watcher has delivered the event.
			name: "annotation-deprecated-raced",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "fun fresh(): Int = 1", "@Deprecated(\"old\")\nfun fresh(): Int = 1")
			},
			effect:         wcEffectChanges,
			structuralOnly: true,
			raceAnalyze:    true,
		},
		{
			name: "return-type",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "fun helper(): R? = R()", "fun helper(): R = R()")
			},
			effect: wcEffectChanges,
		},
		{
			name: "nullability",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "val name: String? = null", "val name: String = \"\"")
			},
			effect: wcEffectChanges,
		},
		{
			name: "signature",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "fun compute(x: Int): Int = x", "fun compute(x: String): Int = x.length")
			},
			effect: wcEffectChanges,
		},
		{
			name: "annotation-deprecated",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "fun fresh(): Int = 1", "@Deprecated(\"old\")\nfun fresh(): Int = 1")
			},
			effect: wcEffectChanges,
		},
		{
			name: "file-add",
			apply: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "src", "New.kt"), wcOver)
			},
			effect: wcEffectChanges,
		},
		{
			name:  "file-delete",
			extra: map[string]string{"src/Over.kt": wcOver},
			apply: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "src", "Over.kt")); err != nil {
					t.Fatal(err)
				}
			},
			effect: wcEffectChanges,
		},
		{
			name:  "file-rename",
			extra: map[string]string{"src/Over.kt": wcOver},
			apply: func(t *testing.T, root string) {
				if err := os.Rename(filepath.Join(root, "src", "Over.kt"), filepath.Join(root, "src", "Over2.kt")); err != nil {
					t.Fatal(err)
				}
			},
			effect: wcEffectAny,
		},
		{
			name: "import-change",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Use.kt", "import p.helper", "import p.alt.helper")
			},
			effect: wcEffectChanges,
		},
		{
			name: "overload-change",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "fun pick(x: Any): Int? = null", "fun pick(x: Any): Int? = null\n\nfun pick(x: Int): Int = 1")
			},
			effect: wcEffectChanges,
		},
		{
			name: "companion-invoke",
			apply: func(t *testing.T, root string) {
				wcReplace(t, root, "src/Lib.kt", "operator fun invoke(): Box = Box()", "fun invoke(): Box = Box()")
			},
			effect: wcEffectChanges,
		},
		{
			name:       "classpath-change",
			extra:      map[string]string{"src/UsesJar.kt": wcUsesJar},
			oracleOnly: true,
			classpath:  true,
			effect:     wcEffectChanges,
		},
		{
			name:        "helper-identity-change",
			oracleOnly:  true,
			jarIdentity: true,
			effect:      wcEffectSame,
		},
		{
			name:       "compiler-setting-change",
			oracleOnly: true,
			skip:       "effective compiler-setting propagation (language/api version, opt-ins) is wired by #784; enable once it lands",
		},
	}
}

// wcWriteProject writes the base project into root.
func wcWriteProject(t *testing.T, root string, m wcMutation) {
	t.Helper()
	for rel, body := range wcBaseFiles(m) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, body)
	}
}

// wcCopyTree copies the source files of src into dst, skipping every cache
// and state directory, so the fresh session sees exactly the final tree and
// nothing the warm session left behind.
func wcCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	wcCopy(t, src, dst, false)
}

// wcCopyTreeKeepCache copies src into dst including the .krit cache directory,
// for backing up and restoring a seeded warm state.
func wcCopyTreeKeepCache(t *testing.T, src, dst string) {
	t.Helper()
	wcCopy(t, src, dst, true)
}

func wcCopy(t *testing.T, src, dst string, keepCache bool) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".krit" && !keepCache {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), raw, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// wcTreeIdentity lists every source file with a short content hash. It is the
// "input identity" half of a failure report: when warm and clean disagree,
// this shows whether they really saw the same inputs.
func wcTreeIdentity(root string) string {
	var lines []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if d.IsDir() {
			if d.Name() == ".krit" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(raw)
		rel, _ := filepath.Rel(root, path)
		lines = append(lines, fmt.Sprintf("%s=%s", filepath.ToSlash(rel), hex.EncodeToString(sum[:4])))
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, " ")
}

// wcSnapshot is the normalized, comparable result of one run: every entry is
// a stable key mapped to a canonical string, so two snapshots compare by key
// and a mismatch prints as a compact per-key diff.
type wcSnapshot map[string]string

// wcDiff returns a compact description of how got differs from want, or ""
// when they are identical. want is the clean baseline.
func wcDiff(got, want wcSnapshot) string {
	keys := map[string]bool{}
	for k := range got {
		keys[k] = true
	}
	for k := range want {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	var b strings.Builder
	shown := 0
	total := 0
	for _, k := range sorted {
		g, gok := got[k]
		w, wok := want[k]
		if gok && wok && g == w {
			continue
		}
		total++
		if shown >= 12 {
			continue
		}
		shown++
		switch {
		case !gok:
			fmt.Fprintf(&b, "  - %s: missing in warm; clean has %s\n", k, wcClip(w, 160))
		case !wok:
			fmt.Fprintf(&b, "  + %s: only in warm: %s\n", k, wcClip(g, 160))
		default:
			gc, wc := wcFirstDivergence(g, w)
			fmt.Fprintf(&b, "  ~ %s:\n      warm : %s\n      clean: %s\n", k, gc, wc)
		}
	}
	if total > shown {
		fmt.Fprintf(&b, "  ... and %d more differing keys\n", total-shown)
	}
	return b.String()
}

func wcClip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// wcFirstDivergence returns windows of a and b around their first differing
// byte, so a large canonical JSON value prints as the part that matters.
func wcFirstDivergence(a, b string) (string, string) {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := i - 40
	if lo < 0 {
		lo = 0
	}
	window := func(s string) string {
		hi := i + 90
		if hi > len(s) {
			hi = len(s)
		}
		if lo > len(s) {
			return "<ends at " + fmt.Sprint(len(s)) + ">"
		}
		prefix := ""
		if lo > 0 {
			prefix = "..."
		}
		return fmt.Sprintf("%s%s (diverges at byte %d)", prefix, s[lo:hi], i)
	}
	return window(a), window(b)
}

// wcSnapshotsDiffer reports whether two snapshots differ at all.
func wcSnapshotsDiffer(a, b wcSnapshot) bool {
	return wcDiff(a, b) != ""
}

// wcKnownStale lists the rows whose result is known to be wrong because of a
// stale-cache bug in the product (not in this test). Keys are
// "<backend>/<path>/<spelling>/<mutation>" with <backend> one of structural,
// fir or kaa, <path> cli, daemon, one-shot, and <spelling> abs or rel. A listed
// row is skipped, with the report logged, whatever goes wrong in it (a warm
// result that differs from a clean scan, or a cache-served control that
// differs from its seed run); once the bug is fixed the row passes and fails
// the suite until its entry is deleted, so the list cannot go stale itself.
var wcKnownStale = wcBuildKnownStale()

func wcBuildKnownStale() map[string]string {
	known := map[string]string{}
	// add records why a row is known-bad; a row with several causes cites all
	// of them rather than letting the last write win.
	add := func(key, why string) {
		if prior, ok := known[key]; ok {
			why = prior + "; and " + why
		}
		known[key] = why
	}
	both := func(prefix, mutation, why string) {
		for _, spelling := range []string{"abs", "rel"} {
			add(prefix+"/"+spelling+"/"+mutation, why)
		}
	}
	// The krit-types cache key does not include the classpath, so a
	// classpath change leaves cached facts for existing files in place.
	for _, mode := range []string{"one-shot", "daemon"} {
		both("kaa/"+mode, "classpath-change", "#827 KAA-CLASSPATH-KEY: the krit-types cache key does not include the classpath (krit-fir's does)")
	}
	return known
}

// wcResolveRow reports a row's outcome. problem is the first thing that went
// wrong ("" when nothing did) and detail the full report. A row listed in
// wcKnownStale is skipped on a problem and fails when it has none.
func wcResolveRow(t *testing.T, key, problem, detail string) {
	t.Helper()
	bug, known := wcKnownStale[key]
	switch {
	case problem != "" && known:
		t.Logf("%s: %s\n%s", key, problem, detail)
		t.Skipf("known stale-cache bug %s", bug)
	case problem != "":
		t.Errorf("%s", detail)
	case known:
		t.Errorf("row %s is correct now; delete its wcKnownStale entry (%s)", key, bug)
	}
}

// wcMatrixEnv enables the warm-versus-clean matrix. The matrix builds krit,
// spawns daemons and JVMs and takes minutes, so only the CI job that provides
// the helper jars (oracle-backend-parity's sibling, warm-clean-matrix) sets it;
// a plain `make test` skips it. Run it locally with KRIT_WARM_CLEAN_MATRIX=1.
const wcMatrixEnv = "KRIT_WARM_CLEAN_MATRIX"

func wcRequireMatrix(t *testing.T) {
	t.Helper()
	if os.Getenv(wcMatrixEnv) != "1" {
		t.Skipf("warm-versus-clean matrix is opt-in; set %s=1 (the warm-clean-matrix CI job does)", wcMatrixEnv)
	}
}
