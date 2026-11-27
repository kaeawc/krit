package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// slackNamedParamsScaleProject lays out n files in one package, each with a subclass overriding an annotated member,
// a subclass inheriting it, an unrelated class with a same-named method, and calls on all of them (#1899's repros A
// and B).
func slackNamedParamsScaleProject(tb testing.TB, n int) []*scanner.File {
	tb.Helper()
	dir := tb.TempDir()
	var files []*scanner.File
	write := func(name, body string) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
		f, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			tb.Fatal(err)
		}
		files = append(files, f)
	}
	write("Base.kt", "package p\n\nimport slack.lint.annotations.MustUseNamedParams\n\n"+
		"open class Base {\n  @MustUseNamedParams\n  open fun create(a: Int, b: Int) = a + b\n}\n\nopen class Mid : Base()\n")
	for i := 0; i < n; i++ {
		write(fmt.Sprintf("F%d.kt", i), fmt.Sprintf("package p\n\n"+
			"class S%[1]d : Base() {\n  override fun create(a: Int, b: Int) = a\n}\n\n"+
			"class U%[1]d : Mid()\n\n"+
			"class C%[1]d {\n  fun create(a: Int, b: Int) = a\n  fun build(a: Int) = a\n}\n\n"+
			"fun use%[1]d(s: S%[1]d, u: U%[1]d, b: Base, c: C%[1]d) {\n"+
			"  s.create(1, 2)\n  u.create(1, 2)\n  b.create(1, 2)\n  c.create(1, 2)\n  c.build(1)\n}\n", i))
	}
	return files
}

func slackRuleByID(tb testing.TB, id string) *api.Rule {
	tb.Helper()
	for _, r := range api.Registry {
		if r.ID == id {
			return r
		}
	}
	tb.Fatalf("rule %s not registered", id)
	return nil
}

// slackNamedParamsBytes returns the bytes allocated by one MustUseNamedParams run over an n-file project, and the
// number of findings.
func slackNamedParamsBytes(t *testing.T, n int) (uint64, int) {
	t.Helper()
	files := slackNamedParamsScaleProject(t, n)
	rule := slackRuleByID(t, "MustUseNamedParams")
	runCrossFileRuleOnFiles(rule, files...) // warm the files' lazily built caches
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	findings := runCrossFileRuleOnFiles(rule, files...)
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, len(findings)
}

// TestMustUseNamedParamsScalesLinearly guards #1899: every call that reached the owner's hierarchy rescanned every
// same-named declaration in the project, so the work grew with calls × declarations (126 s at 4,000 files).
// Allocation is a deterministic proxy for that work: with 8x the files a linear pass allocates about 8x the bytes,
// and the quadratic one about 50x.
func TestMustUseNamedParamsScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a few hundred files")
	}
	const small, large = 40, 320
	smallBytes, smallFindings := slackNamedParamsBytes(t, small)
	largeBytes, largeFindings := slackNamedParamsBytes(t, large)
	// u.create and b.create in each file call the annotated member positionally; s.create calls the override.
	if smallFindings != 2*small || largeFindings != 2*large {
		t.Fatalf("findings: %d files -> %d, %d files -> %d; want 2 per file", small, smallFindings, large, largeFindings)
	}
	ratio := float64(largeBytes) / float64(smallBytes)
	t.Logf("allocated %d bytes for %d files, %d bytes for %d files (ratio %.1f)", smallBytes, small, largeBytes, large, ratio)
	if ratio > 14 {
		t.Fatalf("allocation grew %.1fx for %dx the files; the pass is doing per-call work over the whole project",
			ratio, large/small)
	}
}

// BenchmarkMustUseNamedParams times the pass over a 2,000-file project.
func BenchmarkMustUseNamedParams(b *testing.B) {
	files := slackNamedParamsScaleProject(b, 2000)
	rule := slackRuleByID(b, "MustUseNamedParams")
	b.ReportAllocs()
	for b.Loop() {
		runCrossFileRuleOnFiles(rule, files...)
	}
}
