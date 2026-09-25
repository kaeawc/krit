package parity_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// firOnlyRules are FIR checkers with no Go rule: never enabled in production,
// so there is nothing to be at parity with. Mirrors FIR_ONLY_RULES in the
// krit-fir FixtureParityTest. Do not add to it; a new checker ports a Go rule.
var firOnlyRules = map[string]string{
	"UnsafeCastWhenNullable": "no Go rule or Go fixtures (pre-harness FIR-only checker)",
}

// projectScopeNeeds are capabilities a single-file Go dispatch cannot
// provide, so the Go side of per-file parity would be vacuous.
const projectScopeNeeds = api.NeedsModuleIndex | api.NeedsCrossFile | api.NeedsParsedFiles |
	api.NeedsManifest | api.NeedsResources | api.NeedsGradle

var firParitySkipRe = regexp.MustCompile(`(?m)^\s*//\s*fir-parity:\s*skip\b(.*)$`)

// firFixture is one Go rule fixture prepared for the batched krit-fir run.
type firFixture struct {
	rule, kind string
	rel        string // repo-relative path of the original fixture
	tmp        string // rewritten copy (unique package) that krit-fir compiles
	lineOffset int    // lines the rewrite inserted before the original line 1
	hasSkip    bool
	skip       string
}

// TestFirFixtureParity is the exact tier of FIR/Go parity (the fast tier is
// the krit-fir FixtureParityTest). For every rule the krit-fir jar implements
// (`--list-rules`), each Go fixture tests/fixtures/{positive,negative}/*/<RuleId>.kt
// is compiled against the compiler-tests stub library, and the FIR findings
// for that rule must equal the Go rule's findings line for line (the count
// per line; the Go rule runs in-process on the original fixture, with source
// inference and no oracle). api.Registry decides whether a Go rule exists.
//
// All fixtures go through one krit-fir run, each rewritten into its own
// package so their declarations cannot collide. A fixture with a compile
// error fails unless it carries `// fir-parity: skip <reason>`, and a marker
// on a fixture that compiles cleanly is stale and fails.
func TestFirFixtureParity(t *testing.T) {
	root := repoRoot(t)
	jar, _ := firJar(t)
	ids := listFirRules(t, jar)

	tmp := t.TempDir()
	var fixtures []firFixture
	var checked []string
	for _, id := range ids {
		if _, ok := firOnlyRules[id]; ok {
			continue
		}
		checked = append(checked, id)
		for _, kind := range []string{"positive", "negative"} {
			matches, _ := filepath.Glob(filepath.Join(root, "tests", "fixtures", kind, "*", id+".kt"))
			sort.Strings(matches)
			for _, m := range matches {
				fixtures = append(fixtures, writeParityFixture(t, root, tmp, id, kind, m))
			}
		}
	}
	for id := range firOnlyRules {
		if !containsString(ids, id) {
			t.Errorf("firOnlyRules lists %q, which the krit-fir jar does not implement; remove it", id)
		}
	}

	firByPath, crashed := runParityBatch(t, checked, fixtures)

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			if reason, ok := firOnlyRules[id]; ok {
				t.Skipf("%s is exempt from fixture parity: %s", id, reason)
			}
			rule := findGoRule(id)
			if rule == nil {
				t.Fatalf("FIR rule %q has no Go rule in api.Registry; krit only sends krit-fir the IDs of active Go rules, so it never runs", id)
			}
			if rule.Needs.HasAny(projectScopeNeeds) {
				t.Fatalf("Go rule %q needs project context (Needs=%v); per-file fixture parity cannot run it", id, rule.Needs)
			}
			kinds := map[string]bool{}
			for _, f := range fixtures {
				if f.rule != id {
					continue
				}
				kinds[f.kind] = true
				t.Run(f.kind+"/"+filepath.Base(filepath.Dir(f.rel)), func(t *testing.T) {
					checkParityFixture(t, root, f, crashed, firByPath[f.tmp])
				})
			}
			for _, kind := range []string{"positive", "negative"} {
				if !kinds[kind] {
					t.Errorf("no %s fixture tests/fixtures/%s/<category>/%s.kt; every ported rule needs a Kotlin (.kt) positive and negative fixture (a .java-only fixture does not count)", kind, kind, id)
				}
			}
		})
	}
}

// runParityBatch compiles every fixture in one krit-fir run and returns the
// findings by file plus the fixtures with compile errors. K2 reports no
// warnings once any file in the compilation has an error, and KRIT_RULE is a
// warning, so one broken fixture would silence every checker. The fixtures
// that failed are therefore dropped and the rest recompiled.
func runParityBatch(t *testing.T, rules []string, fixtures []firFixture) (map[string][]scanner.Finding, map[string]string) {
	t.Helper()
	firByPath := map[string][]scanner.Finding{}
	crashed := map[string]string{}
	var files []string
	for _, f := range fixtures {
		files = append(files, f.tmp)
	}
	if len(files) == 0 {
		return firByPath, crashed
	}
	res := firInvoke(t, rules, files)
	if broken := brokenFixtures(res); len(broken) > 0 {
		crashed = broken
		var compiling []string
		for _, f := range files {
			if _, bad := crashed[f]; !bad {
				compiling = append(compiling, f)
			}
		}
		if len(compiling) == 0 {
			return firByPath, crashed
		}
		res = firInvoke(t, rules, compiling)
		if broken := brokenFixtures(res); len(broken) > 0 {
			t.Fatalf("krit-fir still reports compile errors after dropping the broken fixtures: %v", broken)
		}
	}
	// An error without a location (bad classpath, missing source) is not
	// attributed to any file; a run in which no checker fired at all means
	// nothing was checked.
	if len(res.Findings) == 0 {
		t.Fatalf("krit-fir reported no findings for any fixture (compile errors: %v); the checkers did not run", crashed)
	}
	for _, f := range res.Findings {
		firByPath[f.File] = append(firByPath[f.File], f)
	}
	return firByPath, crashed
}

// brokenFixtures collects the fixtures krit-fir could not check: files with
// compiler errors (ErrorFiles) and files lost to a compiler crash (Crashed).
func brokenFixtures(res *firchecks.Result) map[string]string {
	broken := map[string]string{}
	for path, msg := range res.Crashed {
		broken[path] = msg
	}
	for path, msg := range res.ErrorFiles {
		broken[path] = msg
	}
	return broken
}

func checkParityFixture(t *testing.T, root string, f firFixture, crashed map[string]string, firFindings []scanner.Finding) {
	t.Helper()
	compileErr, broken := crashed[f.tmp]
	if f.hasSkip {
		if f.skip == "" {
			t.Fatalf("%s: `// fir-parity: skip` needs a reason", f.rel)
		}
		if !broken {
			t.Fatalf("%s carries `// fir-parity: skip %s` but compiles cleanly against the stubs: stale skip marker; remove it", f.rel, f.skip)
		}
		t.Skipf("%s opted out of FIR fixture parity: %s", f.rel, f.skip)
	}
	if broken {
		t.Fatalf("%s does not compile against the stub library (%s), so its FIR verdict would be vacuous. "+
			"Extend the stubs (%s/README.md) or add `// fir-parity: skip <reason>`", f.rel, compileErr, firStubsRel)
	}

	goFindings := runGoRule(t, root, f.rule, f.rel)
	goCounts := findingLineCounts(goFindings, f.rule)
	firCounts := map[int]int{}
	for line, n := range findingLineCounts(firFindings, f.rule) {
		firCounts[line-f.lineOffset] += n
	}
	if !sameCounts(goCounts, firCounts) {
		t.Fatalf("FIR/Go parity mismatch for %s on %s (line -> findings)\nGo:  %v\nFIR: %v\nGo findings:  %s\nFIR findings: %s",
			f.rule, f.rel, goCounts, firCounts, summarizeFindings(goFindings), summarizeFindings(firFindings))
	}
	if f.kind == "positive" && len(goCounts) == 0 {
		t.Errorf("positive fixture %s produces no %s finding on either side", f.rel, f.rule)
	}
}

// writeParityFixture copies a fixture into dir under a package unique to the
// fixture, recording any skip marker and the line shift of the rewrite.
func writeParityFixture(t *testing.T, root, dir, id, kind, path string) firFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(root, path)
	category := filepath.Base(filepath.Dir(path))
	ident := func(s string) string { return strings.NewReplacer("-", "_", ".", "_").Replace(s) }
	pkg := fmt.Sprintf("firparity.%s.%s.%s", kind, ident(category), strings.ToLower(id))
	source, offset := rewritePackage(string(data), pkg)
	f := firFixture{rule: id, kind: kind, rel: filepath.ToSlash(rel), lineOffset: offset}
	if m := firParitySkipRe.FindStringSubmatch(source); m != nil {
		f.hasSkip, f.skip = true, strings.TrimSpace(m[1])
	}
	f.tmp = filepath.Join(dir, fmt.Sprintf("%s_%s_%s.kt", kind, ident(category), id))
	if err := os.WriteFile(f.tmp, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// rewritePackage replaces the package header in place, or inserts one line
// when there is none; it returns the number of inserted lines.
func rewritePackage(source, packageName string) (string, int) {
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "package ") {
			lines[i] = "package " + packageName
			return strings.Join(lines, "\n"), 0
		}
	}
	return "package " + packageName + "\n" + source, 1
}

// listFirRules asks the krit-fir jar for the rule IDs it implements.
func listFirRules(t *testing.T, jar string) []string {
	t.Helper()
	java := "java"
	if home := os.Getenv("JAVA_HOME"); home != "" {
		if p := filepath.Join(home, "bin", "java"); fileExists(p) {
			java = p
		}
	}
	out, err := exec.Command(java, "-jar", jar, "--list-rules").Output()
	if err != nil {
		t.Fatalf("krit-fir --list-rules failed (%v); rebuild the jar: `cd tools/krit-fir && ./gradlew shadowJar`", err)
	}
	var ids []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			ids = append(ids, line)
		}
	}
	if len(ids) == 0 {
		t.Fatal("krit-fir --list-rules returned no rules")
	}
	return ids
}

func findGoRule(id string) *api.Rule {
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repository root")
		}
		wd = parent
	}
}

func isExecutableJar(path string) bool {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "Main-Class: dev.jasonpearson.krit.fir.MainKt")
	}
	return false
}

func findKotlinStdlib() string {
	if path := os.Getenv("KOTLIN_STDLIB_JAR"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", "org.jetbrains.kotlin", "kotlin-stdlib", "*", "*", "kotlin-stdlib-*.jar"))
	sort.Strings(matches)
	for i := len(matches) - 1; i >= 0; i-- {
		name := filepath.Base(matches[i])
		if strings.Contains(name, "sources") || strings.Contains(name, "javadoc") {
			continue
		}
		return matches[i]
	}
	return ""
}

// runGoRule runs one Go rule in-process on a repo fixture: single-file
// dispatch, source inference when the rule needs a resolver, no oracle.
func runGoRule(t *testing.T, root, ruleName, fixture string) []scanner.Finding {
	t.Helper()
	file, err := scanner.ParseFile(context.Background(), filepath.Join(root, fixture))
	if err != nil {
		t.Fatal(err)
	}
	rule := findGoRule(ruleName)
	if rule == nil {
		t.Fatalf("rule %q not found", ruleName)
	}
	var resolver typeinfer.TypeResolver
	if rule.Needs.Has(api.NeedsResolver) {
		r := typeinfer.NewResolver()
		r.IndexFilesParallel([]*scanner.File{file}, 1)
		resolver = r
	}
	cols, stats := rules.NewDispatcher([]*api.Rule{rule}, resolver).RunWithStats(file)
	if len(stats.Errors) > 0 {
		// An erroring rule emits nothing, which would read as "Go says no
		// finding" and pass a negative vacuously.
		t.Fatalf("Go rule %q errored on %s: %v", ruleName, fixture, stats.Errors)
	}
	return cols.Findings()
}

func findingLineCounts(findings []scanner.Finding, ruleName string) map[int]int {
	out := map[int]int{}
	for _, f := range findings {
		if f.Rule != ruleName {
			continue
		}
		out[f.Line]++
	}
	return out
}

func sameCounts(a, b map[int]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		if b[k] != av {
			return false
		}
	}
	return true
}

func summarizeFindings(findings []scanner.Finding) string {
	if len(findings) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, fmt.Sprintf("%s:%d:%d:%s", filepath.Base(f.File), f.Line, f.Col, f.Rule))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ", ") + "]"
}
