package parity_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/scanner"
)

// The FIR half of the Kotlin 2.4 matrix (#785; the oracle half is in
// oracle_kotlin24_parity_test.go). The rule fixtures under
// tests/fixtures/{positive,negative}/kotlin24/ hold Kotlin 2.4 constructs
// (explicit backing fields, named context parameters) in the layout the
// whole-corpus TestFirFixtureParity already compiles under the embedded
// compiler's defaults. Here the same fixtures run under explicit language
// modes: each krit-fir finding must equal the Go rule's, line for line.

// kotlin24RuleFixtures loads the kotlin24 rule fixtures into dir, each
// rewritten into its own package like TestFirFixtureParity, and returns them
// with the FIR rules they cover.
func kotlin24RuleFixtures(t *testing.T, root, dir string) (fixtures []firFixture, rules []string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	jar, _ := firJar(t)
	known := listFirRules(t, jar)
	seen := map[string]bool{}
	for _, kind := range []string{"positive", "negative"} {
		matches, _ := filepath.Glob(filepath.Join(root, "tests", "fixtures", kind, "kotlin24", "*.kt"))
		sort.Strings(matches)
		for _, m := range matches {
			id := strings.TrimSuffix(filepath.Base(m), ".kt")
			if !containsString(known, id) {
				t.Fatalf("%s: %s is not a rule the krit-fir jar implements", m, id)
			}
			fixtures = append(fixtures, writeParityFixture(t, root, dir, id, kind, m))
			if !seen[id] {
				seen[id] = true
				rules = append(rules, id)
			}
		}
	}
	if len(fixtures) == 0 {
		t.Fatal("no kotlin24 rule fixtures found")
	}
	sort.Strings(rules)
	return fixtures, rules
}

// TestFirKotlin24Parity holds krit-fir to the Go rules on the Kotlin 2.4
// fixtures under explicit language modes, and proves the mode reaches the
// compiler that checks them:
//
//   - native language 2.4, through the persistent daemon: cold, then served
//     from the result cache, then by the warm daemon session. Findings are
//     equal to the Go rules' and identical each time;
//   - language 2.3 with the opt-in flags: the same findings;
//   - language 2.3 without them, over the 2.4 run's own cache: the three 2.4
//     features are compile errors, so neither the cache nor the daemon session
//     of the 2.4 run answered;
//   - language 2.3 with only -Xannotation-target-all: it enables `@all:` and
//     nothing else. The flag must be one krit applies, or a project pinned
//     below 2.4 that uses `@all:` would lose every checker verdict for the
//     file.
func TestFirKotlin24Parity(t *testing.T) {
	root := repoRoot(t)
	jar, stdlib := firJar(t)
	stubs, stubClasspath := firStubLibrary(t)
	classpath := append([]string{stdlib}, stubClasspath...)
	tmp := t.TempDir()

	fixtures, rules := kotlin24RuleFixtures(t, root, filepath.Join(tmp, "rules"))
	oracleDir := filepath.Join(tmp, "oracle")
	writeKotlin24Sources(t, oracleDir)
	files := append([]string{}, stubs...)
	for _, f := range fixtures {
		files = append(files, f.tmp)
	}
	var oracleFiles []string
	for name := range kotlin24FixtureSources() {
		oracleFiles = append(oracleFiles, filepath.Join(oracleDir, name))
	}
	sort.Strings(oracleFiles)
	files = append(files, oracleFiles...)
	sourceDirs := []string{filepath.Dir(stubs[0]), tmp}

	target := func(name string) firchecks.CompileTarget {
		return firchecks.CompileTarget{Kotlin: kotlin24Settings(t, kotlin24ModeNamed(name).args)}
	}
	gated := firchecks.CompileTarget{Kotlin: kotlin24Settings(t, []string{"-language-version=2.3", "-api-version=2.3"})}
	onlyAll := firchecks.CompileTarget{Kotlin: kotlin24Settings(t, []string{"-language-version=2.3", "-api-version=2.3", "-Xannotation-target-all"})}

	invoke := func(t *testing.T, repo string, daemon bool, target firchecks.CompileTarget) *firchecks.Result {
		t.Helper()
		res, err := firchecks.InvokeCached(jar, files, sourceDirs, classpath, rules, nil, firchecks.FileFacts{}, repo, daemon, false, target)
		if err != nil {
			t.Fatalf("krit-fir invoke: %v", err)
		}
		if len(res.Crashed) > 0 {
			t.Fatalf("compiler crashed: %v", res.Crashed)
		}
		return res
	}
	// checkAgainstGo requires a clean compile and the Go rules' findings, per fixture.
	checkAgainstGo := func(t *testing.T, res *firchecks.Result) map[string]string {
		t.Helper()
		if len(res.ErrorFiles) > 0 {
			t.Fatalf("compile errors: %v", res.ErrorFiles)
		}
		byPath := map[string][]scanner.Finding{}
		for _, f := range res.Findings {
			byPath[f.File] = append(byPath[f.File], f)
		}
		got := map[string]string{}
		for _, f := range fixtures {
			t.Run(f.kind+"/"+f.rule, func(t *testing.T) {
				checkParityFixture(t, root, f, map[string]string{}, byPath[f.tmp], map[string]bool{})
			})
			got[f.rel] = summarizeFindings(byPath[f.tmp])
		}
		return got
	}
	assertSame := func(t *testing.T, want, got map[string]string) {
		t.Helper()
		for rel, w := range want {
			if got[rel] != w {
				t.Errorf("findings for %s changed\nfirst: %s\nnow:   %s", rel, w, got[rel])
			}
		}
	}
	// assertGates requires the compile error of the 2.4 language-version gate
	// on exactly the listed oracle fixtures (the others use no 2.4 feature).
	assertGates := func(t *testing.T, res *firchecks.Result, gatedFiles map[string]string) {
		t.Helper()
		got := map[string]string{}
		for path, msg := range res.ErrorFiles {
			if filepath.Dir(path) == oracleDir {
				got[filepath.Base(path)] = msg
			}
		}
		for _, name := range []string{"ContextParameters.kt", "SingleContextParameter.kt", "BackingFields.kt", "AnnotationTargets.kt", "AnnotationUse.kt", "Lookalikes.kt"} {
			feature, wantErr := gatedFiles[name]
			msg, hasErr := got[name]
			switch {
			case wantErr && !hasErr:
				t.Errorf("%s compiled; the language mode did not reach the compiler", name)
			case !wantErr && hasErr:
				t.Errorf("%s has a compile error: %s", name, msg)
			case wantErr && (!strings.Contains(msg, feature) || !strings.Contains(msg, "only available since language version 2.4")):
				t.Errorf("%s: error %q is not the %q language-version gate", name, msg, feature)
			}
		}
	}

	repo := t.TempDir()
	var want map[string]string
	t.Run("language-2.4/cold-daemon", func(t *testing.T) {
		want = checkAgainstGo(t, invoke(t, repo, true, target("language-2.4")))
	})
	t.Run("language-2.4/result-cache", func(t *testing.T) {
		assertSame(t, want, checkAgainstGo(t, invoke(t, repo, true, target("language-2.4"))))
	})
	t.Run("language-2.4/warm-daemon", func(t *testing.T) {
		assertSame(t, want, checkAgainstGo(t, invoke(t, t.TempDir(), true, target("language-2.4"))))
	})
	t.Run("language-2.3-with-flags", func(t *testing.T) {
		assertSame(t, want, checkAgainstGo(t, invoke(t, "", false, target("language-2.3-with-flags"))))
	})
	t.Run("language-2.3-without-flags", func(t *testing.T) {
		res := invoke(t, repo, false, gated)
		assertGates(t, res, map[string]string{
			"ContextParameters.kt":      "context parameters",
			"SingleContextParameter.kt": "context parameters",
			"BackingFields.kt":          "explicit backing fields",
			"AnnotationTargets.kt":      "annotation all use site target",
		})
	})
	t.Run("language-2.3-annotation-target-all-only", func(t *testing.T) {
		res := invoke(t, "", false, onlyAll)
		assertGates(t, res, map[string]string{
			"ContextParameters.kt":      "context parameters",
			"SingleContextParameter.kt": "context parameters",
			"BackingFields.kt":          "explicit backing fields",
		})
	})
}
