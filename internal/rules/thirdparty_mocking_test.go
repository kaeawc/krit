package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestSlackLintUpstreamParity checks slack-lints ports whose detectors only run on test sources, which the Lint CLI
// cannot mark in a directory scan, against slack-lints' own detector tests.
//
// Each directory under testdata/slack_lint_upstream is one @Test method of slack-lints 0.11.1
// (slack-lint-checks/src/test/java/slack/lint/...): its files, with `test/` paths moved under src/test/java, and
// upstream-0.11.1.txt, the findings the test expects for the IDs in checks.txt, as `path:line:column: ID: message`
// with the column read from Lint's underline. divergences.txt has the format TestSlackLintParity uses.
func TestSlackLintUpstreamParity(t *testing.T) {
	root := filepath.Join("testdata", "slack_lint_upstream")
	cases, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		ran++
		dir := filepath.Join(root, c.Name())
		t.Run(c.Name(), func(t *testing.T) {
			checks := strings.Split(strings.TrimSpace(readParityFile(t, filepath.Join(dir, "checks.txt"))), ",")
			want := upstreamParityExpected(t, dir)
			got := runCrossFileParityChecks(t, dir, checks)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("krit findings differ from slack-lints' tests (with divergences.txt applied)\n--- want\n%s\n--- got\n%s",
					strings.Join(want, "\n"), strings.Join(got, "\n"))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no upstream parity cases found")
	}
}

// upstreamParityExpected applies divergences.txt to the upstream test's findings.
func upstreamParityExpected(t *testing.T, dir string) []string {
	t.Helper()
	remaining := map[string]int{}
	expected := parityLines(readParityFile(t, filepath.Join(dir, "upstream-0.11.1.txt")))
	for _, line := range expected {
		remaining[line]++
	}
	var added []string
	for _, line := range parityLines(readOptionalParityFile(t, filepath.Join(dir, "divergences.txt"))) {
		finding, reason, _ := strings.Cut(line, " # ")
		if strings.TrimSpace(reason) == "" {
			t.Errorf("divergences.txt line %q has no ` # reason`", line)
		}
		switch {
		case strings.HasPrefix(finding, "- "):
			finding = strings.TrimSpace(strings.TrimPrefix(finding, "- "))
			if remaining[finding] == 0 {
				t.Errorf("divergences.txt removes %q, which the upstream test does not expect", finding)
				continue
			}
			remaining[finding]--
		case strings.HasPrefix(finding, "+ "):
			added = append(added, strings.TrimSpace(strings.TrimPrefix(finding, "+ ")))
		default:
			t.Errorf("divergences.txt line %q must start with `- ` or `+ `", line)
		}
	}
	var want []string
	for _, line := range expected {
		if remaining[line] > 0 {
			remaining[line]--
			want = append(want, line)
		}
	}
	return parityLines(strings.Join(append(want, added...), "\n"))
}

// runCrossFileParityChecks parses every file of the case directory as one project, builds the cross-file index and
// runs the checks on it, as the cross-file phase does.
func runCrossFileParityChecks(t *testing.T, dir string, checks []string) []string {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*scanner.File
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		var file *scanner.File
		switch filepath.Ext(path) {
		case ".kt":
			file, err = scanner.ParseFile(context.Background(), path)
		case ".java":
			file, err = scanner.ParseJavaFile(context.Background(), path)
		default:
			return nil
		}
		if err == nil {
			files = append(files, file)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, check := range checks {
		var rule *api.Rule
		for _, r := range api.Registry {
			if r.ID == check {
				rule = r
			}
		}
		if rule == nil {
			t.Fatalf("rule %q not registered", check)
		}
		for _, f := range runCrossFileRuleOnFiles(rule, files...) {
			rel, err := filepath.Rel(abs, f.File)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprintf("%s:%d:%d: %s: %s", filepath.ToSlash(rel), f.Line, f.Col, f.Rule, f.Message))
		}
	}
	sort.Strings(out)
	return out
}

// mockFindings runs the given MockDetector rules over in-memory sources laid out under a temp project, keyed by
// project-relative path, and returns "path:line: rule: message" lines.
func mockFindings(t *testing.T, rules []string, sources map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	for rel, src := range sources {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out []string
	for _, line := range runCrossFileParityChecks(t, root, rules) {
		// Drop the column: these tests pin which sites report, not where.
		parts := strings.SplitN(line, ":", 4)
		out = append(out, parts[0]+":"+parts[1]+":"+parts[3])
	}
	return out
}

const mockModels = `package com.example

data class Data(val a: Int)
object Singleton
@JvmRecord data class Rec(val a: Int)
@com.google.errorprone.annotations.DoNotMock("use the real one") object Marked
open class Plain
`

func TestDoNotMockSitesOutsideTestSourcesAreIgnored(t *testing.T) {
	got := mockFindings(t, []string{"DoNotMockDataClass", "DoNotMockAnything"}, map[string]string{
		"src/main/java/com/example/Models.kt": mockModels,
		"src/main/java/com/example/Helpers.kt": `package com.example

import org.mockito.kotlin.mock

fun notATest(): Data = mock()
`,
	})
	if len(got) != 0 {
		t.Errorf("a mock in main sources must not be reported, got %v", got)
	}
}

func TestDoNotMockRequiresAResolvedFactory(t *testing.T) {
	got := mockFindings(t, []string{"DoNotMockDataClass"}, map[string]string{
		"src/main/java/com/example/Models.kt": mockModels,
		// A local mock() shadows the imported factory.
		"src/test/java/com/example/LocalTest.kt": `package com.example

import org.mockito.kotlin.mock

inline fun <reified T> mock(): T = TODO()

class LocalTest { val d = mock<Data>() }
`,
		// No import: this mock() is not Mockito's.
		"src/test/java/com/example/UnimportedTest.kt": `package com.example

import org.mockito.Mock

class UnimportedTest { val d = mock<Data>() }
`,
		// A receiver that is not Mockito.
		"src/test/java/com/example/ReceiverTest.java": `package com.example;

import org.mockito.Mock;

class ReceiverTest {
  Object d = Factory.mock(Data.class);
}
`,
	})
	if len(got) != 0 {
		t.Errorf("mock calls that do not resolve to a Mockito factory must not be reported, got %v", got)
	}
}

func TestDoNotMockCheckerOrder(t *testing.T) {
	sources := map[string]string{
		"src/main/java/com/example/Models.kt": mockModels,
		"src/test/java/com/example/OrderTest.kt": `package com.example

import com.lib.*
import org.mockito.kotlin.mock

class OrderTest {
  fun test() {
    mock<Data>()
    mock<Singleton>()
    mock<Rec>()
    mock<Marked>()
    mock<Plain>()
    mock<Pair<String, String>>()
    mock<LibraryType>()
  }
}
`,
	}
	got := mockFindings(t, []string{"DoNotMockAnything", "DoNotMockDataClass", "DoNotMockObjectClass", "DoNotMockPlatformTypes"}, sources)
	want := []string{
		"src/test/java/com/example/OrderTest.kt:10: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:11: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:12: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:13: DoNotMockAnything: Do not add new mocks.",
		// kotlin.Pair is a data class, but the platform checker runs first.
		"src/test/java/com/example/OrderTest.kt:13: DoNotMockPlatformTypes: platform type 'kotlin.Pair' should not be mocked",
		// A class known only by a wildcard import resolves for Lint, so only DoNotMockAnything reports.
		"src/test/java/com/example/OrderTest.kt:14: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:8: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:8: DoNotMockDataClass: 'com.example.Data' is a data class, so mocking it should not be necessary",
		"src/test/java/com/example/OrderTest.kt:9: DoNotMockAnything: Do not add new mocks.",
		"src/test/java/com/example/OrderTest.kt:9: DoNotMockObjectClass: 'com.example.Singleton' is an object, so mocking it should not be necessary",
	}
	// Rec is a @JvmRecord data class (DoNotMockRecordClass, not DoNotMockDataClass) and Marked is @DoNotMock, whose
	// checker runs before the object checker; neither reports under the ported IDs.
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDoNotMockVariableTypes(t *testing.T) {
	got := mockFindings(t, []string{"DoNotMockDataClass"}, map[string]string{
		"src/main/java/com/example/Models.kt": mockModels,
		"src/test/java/com/example/VarTest.kt": `package com.example

import org.mockito.Mockito

class VarTest {
  fun test(plain: Plain) {
    val ref = Data::class.java
    run {
      Mockito.mock(ref)
      val ref = Plain::class.java
      Mockito.mock(ref)
    }
    Mockito.spy(plain)
  }
}
`,
		"src/test/java/com/example/VarTest2.java": `package com.example;

import static org.mockito.Mockito.mock;

class VarTest2 {
  void test() {
    var inferred = Data.class;
    mock(inferred);
  }
}
`,
	})
	want := []string{"src/test/java/com/example/VarTest.kt:9: DoNotMockDataClass: 'com.example.Data' is a data class, so mocking it should not be necessary"}
	// The inner `ref` shadows the outer one; a parameter and a Java `var` do not show their type, so they abstain.
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
