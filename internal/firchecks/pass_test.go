package firchecks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const verdictRule = "InjectDispatcher"

// verdictProject is a scan fixture: parsed Kotlin files spelled relative to
// the working directory (as a `krit .` scan spells them), plus helpers to
// build Go and FIR findings at a snippet's position.
type verdictProject struct {
	t      *testing.T
	parsed map[string]*scanner.File
}

func newVerdictProject(t *testing.T, files map[string]string, excludes map[string][]string) *verdictProject {
	t.Helper()
	t.Chdir(t.TempDir())
	p := &verdictProject{t: t, parsed: map[string]*scanner.File{}}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(files[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		// Same filter the parse phase installs.
		f.Suppression = scanner.BuildSuppressionFilter(f, nil, excludes, "").WithRuleAliases(rules.AllSuppressionAliases())
		p.parsed[name] = f
	}
	return p
}

func (p *verdictProject) abs(name string) string {
	abs, err := filepath.Abs(name)
	if err != nil {
		p.t.Fatal(err)
	}
	return abs
}

// at locates snippet in name and returns a finding over it.
func (p *verdictProject) at(name, snippet, rule, message string) scanner.Finding {
	p.t.Helper()
	content := string(p.parsed[name].Content)
	start := strings.Index(content, snippet)
	if start < 0 {
		p.t.Fatalf("%q not in %s", snippet, name)
	}
	return scanner.Finding{
		File: name, Line: strings.Count(content[:start], "\n") + 1, Col: start - strings.LastIndex(content[:start], "\n"),
		StartByte: start, EndByte: start + len(snippet), Rule: rule, RuleSet: "coroutines", Message: message,
	}
}

// fir is at() spelled the way the checker reports it: the absolute path.
func (p *verdictProject) fir(name, snippet, rule string) scanner.Finding {
	f := p.at(name, snippet, rule, "fir: "+snippet)
	f.File = p.abs(name)
	f.Confidence = 1
	return f
}

func (p *verdictProject) files() []*scanner.File {
	out := make([]*scanner.File, 0, len(p.parsed))
	for _, f := range p.parsed {
		out = append(out, f)
	}
	return out
}

func describe(findings []scanner.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		fix := ""
		if f.Fix != nil {
			fix = " +fix"
		}
		out = append(out, fmt.Sprintf("%s:%d %s %q%s", f.File, f.Line, f.Rule, f.Message, fix))
	}
	sort.Strings(out)
	return out
}

func TestRunPassAppliesFIRAuthoritativeVerdict(t *testing.T) {
	files := map[string]string{
		"A.kt": `package a

class Repo {
    fun confirmed() = run(Dispatchers.IO)
    fun goOnly() = run(Dispatchers.Default)
    fun firOnly() = run(Custom.IO)
    @Suppress("InjectDispatcher")
    fun suppressed() = run(Custom.Default)
    fun otherRule() = compute(42)
}
`,
		"B.kt": `@file:Suppress("InjectDispatcher")
package b

class FileSuppressed { fun f() = run(Custom.IO) }
`,
		"gen/C.kt": `package c

class Excluded { fun f() = run(Custom.IO) }
`,
		"Err.kt": `package e

class Broken { fun f() = run(Dispatchers.IO) + missing() }
`,
		"Crash.kt": `package cr

class Crashes { fun f() = run(Dispatchers.IO) }
`,
		"src/jsMain/kotlin/J.kt": `package j

class Js { fun f() = run(Dispatchers.IO) }
`,
		"build.gradle.kts": `val io = run(Dispatchers.IO)
`,
	}
	p := newVerdictProject(t, files, map[string][]string{verdictRule: {"**/C.kt"}})

	confirmedGo := p.at("A.kt", "Dispatchers.IO", verdictRule, "go: hardcoded Dispatchers.IO")
	confirmedGo.Fix = &scanner.Fix{ByteMode: true, StartByte: confirmedGo.StartByte, EndByte: confirmedGo.EndByte, Replacement: "dispatcher"}
	goFindings := []scanner.Finding{
		confirmedGo,
		p.at("A.kt", "Dispatchers.Default", verdictRule, "go: Go-only false positive"),
		p.at("A.kt", "42", "MagicNumber", "go: rule without a checker"),
		p.at("Err.kt", "Dispatchers.IO", verdictRule, "go: kept, file has compiler errors"),
		p.at("Crash.kt", "Dispatchers.IO", verdictRule, "go: kept, checker crashed"),
		p.at("src/jsMain/kotlin/J.kt", "Dispatchers.IO", verdictRule, "go: kept, not in the JVM compilation"),
		p.at("build.gradle.kts", "Dispatchers.IO", verdictRule, "go: kept, scripts are not compiled"),
		{File: "Unrequested.kt", Line: 3, Rule: verdictRule, Message: "go: kept, file not checked"},
	}

	checker := NewFakeFirChecker()
	checker.Rules = []string{verdictRule}
	// FIR anchors on the property access, a sub-range of Go's finding.
	firConfirm := p.fir("A.kt", "Dispatchers", verdictRule)
	checker.Findings = []scanner.Finding{
		firConfirm,
		p.fir("A.kt", "Custom.IO", verdictRule),
		p.fir("A.kt", "Custom.Default", verdictRule),
		p.fir("A.kt", "42", "MagicNumber"), // not advertised: discarded
		p.fir("B.kt", "Custom.IO", verdictRule),
		p.fir("gen/C.kt", "Custom.IO", verdictRule),
		p.fir("Err.kt", "missing", verdictRule),
		p.fir("Crash.kt", "Dispatchers.IO", verdictRule),
	}
	checker.ErrorFiles = map[string]string{p.abs("Err.kt"): "Unresolved reference 'missing'."}
	checker.Crashed = map[string]string{p.abs("Crash.kt"): "internal error"}

	var verbose strings.Builder
	opts := PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule, Category: "coroutines"}, {ID: "MagicNumber", Category: "style"}},
		ParsedFiles: p.files(),
		KotlinPaths: slices.Sorted(func(yield func(string) bool) {
			for name := range files {
				if !yield(name) {
					return
				}
			}
		}),
		SourceDirs: []string{"src/main/kotlin"},
		Classpath:  []string{"lib.jar"},
		Tracker:    perf.New(false),
		Verbose:    true,
		VerboseOut: &verbose,
	}
	got := RunPass(opts, slices.Clone(goFindings))
	if !strings.Contains(verbose.String(), "3 authoritative files, 2 gated (compiler error or crash), 2 excluded") {
		t.Fatalf("verbose summary must count the script and the jsMain file as excluded:\n%s", verbose.String())
	}
	opts.Verbose = false

	want := describe([]scanner.Finding{
		confirmedGo, // FIR-confirmed: Go's form, message, and fix
		p.at("A.kt", "Custom.IO", verdictRule, "fir: Custom.IO"), // FIR-only: added
		goFindings[2], goFindings[3], goFindings[4], goFindings[5], goFindings[6], goFindings[7],
	})
	if d := describe(got); !reflect.DeepEqual(d, want) {
		t.Fatalf("verdict mismatch\ngot:  %s\nwant: %s", strings.Join(d, "\n      "), strings.Join(want, "\n      "))
	}
	for _, f := range got {
		if f.Message == "fir: Custom.IO" && (f.File != "A.kt" || f.Confidence != 1) {
			t.Fatalf("FIR-only finding must use the scan's path spelling and keep FIR's confidence: %+v", f)
		}
	}

	if len(checker.Called) != 1 {
		t.Fatalf("expected one Check call, got %d", len(checker.Called))
	}
	requested := checker.Called[0]
	if slices.Contains(requested, p.abs("src/jsMain/kotlin/J.kt")) {
		t.Fatalf("non-JVM source-set file must not be sent to the checker: %v", requested)
	}
	if slices.Contains(requested, p.abs("build.gradle.kts")) {
		t.Fatalf("Kotlin scripts must not be sent to the checker: %v", requested)
	}
	if !slices.Contains(requested, p.abs("A.kt")) || !slices.IsSorted(requested) {
		t.Fatalf("requested files must be the sorted absolute scan files: %v", requested)
	}
	if !reflect.DeepEqual(checker.CalledSourceDirs[0], opts.SourceDirs) || !reflect.DeepEqual(checker.CalledClasspath[0], opts.Classpath) {
		t.Fatalf("compile context not forwarded: sourceDirs=%v classpath=%v", checker.CalledSourceDirs[0], checker.CalledClasspath[0])
	}

	again := RunPass(opts, slices.Clone(goFindings))
	if !reflect.DeepEqual(describeOrdered(got), describeOrdered(again)) {
		t.Fatalf("verdict order is not deterministic:\n%v\n%v", describeOrdered(got), describeOrdered(again))
	}
}

func describeOrdered(findings []scanner.Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = fmt.Sprintf("%s:%d:%d %s", f.File, f.Line, f.Col, f.Rule)
	}
	return out
}

// Warm runs parse only cache misses; files the checker is sent must still
// include every collected Kotlin path, and a FIR-only finding in an unparsed
// file still honors its @Suppress.
func TestRunPassChecksUnparsedPathsAndSuppressesFromDisk(t *testing.T) {
	p := newVerdictProject(t, map[string]string{
		"Parsed.kt":          "package p\n\nclass P { fun f() = run(Custom.IO) }\n",
		"Unparsed.kt":        "package u\n\nclass U {\n    @Suppress(\"InjectDispatcher\")\n    fun f() = run(Custom.IO)\n    fun g() = run(Custom.Default)\n}\n",
		"sub/generated/G.kt": "package g\n",
	}, nil)
	checker := NewFakeFirChecker()
	checker.Rules = []string{verdictRule}
	checker.Findings = []scanner.Finding{
		p.fir("Unparsed.kt", "Custom.IO", verdictRule),
		p.fir("Unparsed.kt", "Custom.Default", verdictRule),
	}
	got := RunPass(PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule}},
		ParsedFiles: []*scanner.File{p.parsed["Parsed.kt"]},
		KotlinPaths: []string{"Parsed.kt", "Unparsed.kt", "sub/generated/G.kt"},
	}, nil)
	if want := []string{p.abs("Parsed.kt"), p.abs("Unparsed.kt")}; !reflect.DeepEqual(checker.Called[0], want) {
		t.Fatalf("requested = %v, want %v (generated paths skipped)", checker.Called[0], want)
	}
	if d := describe(got); !reflect.DeepEqual(d, []string{`Unparsed.kt:6 InjectDispatcher "fir: Custom.Default"`}) {
		t.Fatalf("got %v", d)
	}
}

// A checker failure keeps every Go finding.
func TestRunPassCheckerErrorKeepsGo(t *testing.T) {
	checker := NewFakeFirChecker()
	checker.Err = fmt.Errorf("daemon gone")
	base := []scanner.Finding{{File: "A.kt", Line: 1, Rule: verdictRule}}
	got := RunPass(PassOptions{Enabled: true, Checker: checker, ActiveRules: []*api.Rule{{ID: verdictRule}}, KotlinPaths: []string{"A.kt"}}, base)
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("got %v, want %v", got, base)
	}
}

func TestApplyVerdictMatchesOneToOneAndFallsBackToLine(t *testing.T) {
	const file = "/src/A.kt"
	goA := scanner.Finding{File: file, Line: 3, StartByte: 10, EndByte: 30, Rule: verdictRule, Message: "go A"}
	goB := scanner.Finding{File: file, Line: 3, StartByte: 40, EndByte: 50, Rule: verdictRule, Message: "go B"}
	goNoBytes := scanner.Finding{File: file, Line: 7, Rule: verdictRule, Message: "go no bytes"}
	fir := []scanner.Finding{
		{File: file, Line: 3, StartByte: 12, EndByte: 20, Rule: verdictRule, Message: "fir overlaps A"},
		// Same line as B but no overlap with anything: claims B via the line fallback.
		{File: file, Line: 3, StartByte: 60, EndByte: 70, Rule: verdictRule, Message: "fir same line"},
		{File: file, Line: 7, StartByte: 90, EndByte: 95, Rule: verdictRule, Message: "fir line 7"},
	}
	got, stats := ApplyVerdict(VerdictInput{
		Go:        []scanner.Finding{goA, goB, goNoBytes},
		FIR:       &Result{Findings: fir, Rules: []string{verdictRule}},
		Requested: []string{file},
	})
	if d, want := describe(got), describe([]scanner.Finding{goA, goB, goNoBytes}); !reflect.DeepEqual(d, want) {
		t.Fatalf("got %v, want %v", d, want)
	}
	if r := stats.Rules[verdictRule]; r.Confirmed != 3 || r.GoDropped != 0 || r.FirAdded != 0 {
		t.Fatalf("stats = %+v", *r)
	}
}
