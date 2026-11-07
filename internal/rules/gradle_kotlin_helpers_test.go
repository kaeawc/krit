package rules_test

import (
	"context"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// kotlinGradleRun describes one rule run against a synthetic script.
type kotlinGradleRun struct {
	rule    string
	path    string
	content string
	// kotlin and agp are the project versions; empty means unknown.
	kotlin, agp string
}

func (c kotlinGradleRun) findings(t *testing.T) []scanner.Finding {
	t.Helper()
	return c.run(t, true)
}

// findingsWithoutFacts runs the rule with no injected library facts, so versions come from the script and its catalog.
func (c kotlinGradleRun) findingsWithoutFacts(t *testing.T) []scanner.Finding {
	t.Helper()
	return c.run(t, false)
}

func (c kotlinGradleRun) run(t *testing.T, injectFacts bool) []scanner.Finding {
	t.Helper()
	r := findGradleRule(t, c.rule)
	if c.path == "" {
		c.path = "build.gradle.kts"
	}
	cfg, _ := android.ParseBuildGradleContent(c.content)
	file := scanner.ParseGradleScript(context.Background(), c.path, []byte(c.content), cfg)
	var profile librarymodel.ProjectProfile
	if c.kotlin != "" {
		profile.Kotlin.Compiler = librarymodel.KnownVersion(c.kotlin)
	}
	if c.agp != "" {
		profile.Android.AGP = librarymodel.KnownVersion(c.agp)
	}
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{
		File: file, GradlePath: c.path, GradleContent: c.content, GradleConfig: cfg,
		Rule: r, Collector: collector, DefaultConfidence: api.ConfidenceMedium,
	}
	if injectFacts {
		ctx.LibraryFacts = &librarymodel.Facts{Profile: profile}
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

// applyByteFix applies a byte-mode fix to content.
func applyByteFix(t *testing.T, content string, fix *scanner.Fix) string {
	t.Helper()
	if fix == nil || !fix.ByteMode {
		t.Fatalf("fix = %+v, want a byte-mode fix", fix)
	}
	return content[:fix.StartByte] + fix.Replacement + content[fix.EndByte:]
}

// applyAllByteFixes applies every fix back to front so earlier offsets stay valid.
func applyAllByteFixes(t *testing.T, content string, fs []scanner.Finding) string {
	t.Helper()
	var fixes []*scanner.Fix
	for i := range fs {
		if fs[i].Fix != nil {
			fixes = append(fixes, fs[i].Fix)
		}
	}
	sort.Slice(fixes, func(i, j int) bool { return fixes[i].StartByte > fixes[j].StartByte })
	for _, f := range fixes {
		content = applyByteFix(t, content, f)
	}
	return content
}

func sortedLines(fs []scanner.Finding) []int {
	var out []int
	for _, f := range fs {
		out = append(out, f.Line)
	}
	sort.Ints(out)
	return out
}
