package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// runPrintlnProject runs PrintlnInProduction over one Kotlin file through the
// whole project pipeline, including the parse-error-region output filter, and
// returns the reported lines plus the number of findings that filter dropped.
func runPrintlnProject(t *testing.T, src string) ([]string, int) {
	t.Helper()
	return runOneFileProject(t, "PrintlnInProduction", src)
}

// runOneFileProject runs one rule over one Kotlin file like runPrintlnProject.
func runOneFileProject(t *testing.T, ruleID, src string) ([]string, int) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "H.kt"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	res, err := RunProject(context.Background(), ProjectInput{Args: ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{root},
		ActiveRules: []*api.Rule{findV2RuleForTest(t, ruleID)},
		Format:      "json",
		Version:     "test",
	}})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	var got []string
	for _, f := range res.FinalFindings.Findings() {
		got = append(got, fmt.Sprintf("%d:%d", f.Line, f.Col))
	}
	sort.Strings(got)
	return got, res.Stats.FindingsInErrorRegions
}

// A one-line nested class body used to turn the whole file into a parse-error
// region, so every finding in the file was dropped before output (#1839).
func TestRunProject_OneLineNestedBodyKeepsFileFindings(t *testing.T) {
	tests := []struct {
		name, line string
	}{
		{"companion object", "class H { companion object { val X = 1 } }"},
		{"object", "class H { object O { const val X = 1 } }"},
		{"nested class", "class H { class I { val x = 1 } }"},
		{"nested interface", "class H { interface I { fun f() } }"},
		{"enum body", "class H { enum class E { A, B; fun g() = 1 } }"},
		// A ';' and a member after the nested body (#1875).
		{"';' and a member", "class H { companion object { val X = 1 }; val y = 2 }"},
		{"' ; ' and a member", "class H { companion object { val X = 1 } ; val y = 2 }"},
		{"';' and two members", "class H { companion object { val X = 1 }; val y = 2; fun g() = 3 }"},
		{"member with no ';'", "class H { companion object { val X = 1 } fun g() {} }"},
		{"';' at three levels", "class A { class B { class C { val x = 1 }; val y = 2 }; val z = 3 }"},
		{"';' in an enum body", "enum class E { A, B; companion object { val X = 1 }; fun g() = 1 }"},
		{"two classes on one line", "class H { companion object { val X = 1 }; val y = 2 }; class J { object O { val Z = 1 }; val w = 3 }"},
		// No byte before the closing braces (#1866).
		{"no space, companion object", "class H { companion object { val X = 1}}"},
		{"no space, object", "class H { object O { const val X = 1}}"},
		{"no space, inner brace only", "class H { companion object { val X = 1} }"},
		{"no space, three levels", "class A { class B { class C { val x = 1}}}"},
		{"no space, nested interface", "class H { interface I { fun f()}}"},
		{"no space, enum body", "class H { enum class E { A, B; fun g() = 1}}"},
		{"no space, object literal", "class H { val o = object { val x = 1}}"},
		{"no space, ';' and a member", "class H { companion object { val X = 1}; val y = 2 }"},
		{"no space, ';' and a member, no space at the end", "class H { companion object { val X = 1}; val y = 2}"},
		{"no space, ';' in an enum body", "enum class E { A, B; companion object { val X = 1}; fun g() = 1}"},
		{"no space, two classes on one line", "class H { companion object { val X = 1}; val y = 2}; class J { object O { val Z = 1}; val w = 3}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.line + "\n\nfun f() { println(1) }\n\nval magic = 42\n"
			got, dropped := runPrintlnProject(t, src)
			if want := []string{"3:11"}; strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("findings = %v, want %v", got, want)
			}
			if dropped != 0 {
				t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
			}
		})
	}

	t.Run("finding inside the nested body", func(t *testing.T) {
		got, dropped := runPrintlnProject(t, "class H { companion object { fun g() = println(2) } }\n")
		if want := []string{"1:40"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("findings = %v, want %v", got, want)
		}
		if dropped != 0 {
			t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
		}
	})

	// The finding sits after the inserted ';' bytes on the same line, so its
	// column is the source's, not the reparsed buffer's (#1866).
	t.Run("no space, finding inside the nested body", func(t *testing.T) {
		got, dropped := runPrintlnProject(t, "class H { companion object { fun g() = println(2)}}\n")
		if want := []string{"1:40"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("findings = %v, want %v", got, want)
		}
		if dropped != 0 {
			t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
		}
	})

	t.Run("no space, finding after the repaired braces on the same line", func(t *testing.T) {
		got, dropped := runPrintlnProject(t, "class H { companion object { val X = 1}; fun g() = println(2)}; fun k() = println(3)\n")
		if want := []string{"1:52", "1:75"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("findings = %v, want %v", got, want)
		}
		if dropped != 0 {
			t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
		}
	})

	t.Run("finding in the member after ';'", func(t *testing.T) {
		got, dropped := runPrintlnProject(t, "class H { companion object { val X = 1 }; fun g() = println(2) }\n")
		if want := []string{"1:53"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("findings = %v, want %v", got, want)
		}
		if dropped != 0 {
			t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
		}
	})
}

// The shapes from #1875: an unused local after a one-line nested body that is
// followed by ';' and a member is reported, as it is when the class spans lines.
func TestRunProject_OneLineNestedBodyBeforeSemicolonKeepsUnusedVariable(t *testing.T) {
	after := "\nfun after() { val unused = 1 }\n"
	for _, line := range []string{
		"class H { companion object { val X = 1 }; val y = 2 }",
		"class H { companion object { val X = 1 } ; val y = 2 }",
		"class H {\n  companion object { val X = 1 }\n  val y = 2\n}",
		// No byte before the inner brace (#1866).
		"class H { companion object { val X = 1}; val y = 2 }",
		"class H { companion object { val X = 1}; val y = 2}",
	} {
		t.Run(line, func(t *testing.T) {
			got, dropped := runOneFileProject(t, "UnusedVariable", line+after)
			want := fmt.Sprintf("%d:19", strings.Count(line, "\n")+2)
			if strings.Join(got, ",") != want {
				t.Errorf("findings = %v, want [%s]", got, want)
			}
			if dropped != 0 {
				t.Errorf("FindingsInErrorRegions = %d, want 0", dropped)
			}
		})
	}
}

// A real syntax error after a one-line nested body still marks its region,
// so findings inside it stay dropped.
func TestRunProject_OneLineNestedBodyRealErrorStaysDropped(t *testing.T) {
	for _, src := range []string{
		"class A { object B { fun g() = println(1) ) }; val y = 2 }\n",
		// The no-space form, where the repair inserts bytes (#1866).
		"class A { object B { fun g() = println(1) )}; val y = 2}\n",
	} {
		_, dropped := runPrintlnProject(t, src)
		if dropped == 0 {
			t.Errorf("%q: FindingsInErrorRegions = 0, want the println inside the broken class dropped", src)
		}
	}
}

// @Suppress on a one-line class still covers only that class: the println
// inside its nested body is suppressed and the top-level one is reported.
func TestRunProject_SuppressOnOneLineNestedBodyStaysScoped(t *testing.T) {
	for _, src := range []string{
		"@Suppress(\"PrintlnInProduction\") class H { companion object { fun g() = println(2) } }\n\nfun f() { println(1) }\n",
		// The no-space form (#1866).
		"@Suppress(\"PrintlnInProduction\") class H { companion object { fun g() = println(2)}}\n\nfun f() { println(1) }\n",
	} {
		got, dropped := runPrintlnProject(t, src)
		if want := []string{"3:11"}; strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: findings = %v, want %v", src, got, want)
		}
		if dropped != 0 {
			t.Errorf("%q: FindingsInErrorRegions = %d, want 0", src, dropped)
		}
	}
}

// An autofix after a repaired no-space line edits the source at the original
// offsets, both on later lines and after the inserted ';' bytes on the same
// line (#1866). UnnecessaryInheritance removes ` : Any()` up to the space
// before it, as it does when the class is split over lines.
func TestRunProject_FixAfterOneLineNoSpaceNestedBody(t *testing.T) {
	tests := []struct{ src, want string }{
		{
			"class H { companion object { val X = 1}}\n\nclass Foo : Any()\n",
			"class H { companion object { val X = 1}}\n\nclass Foo \n",
		},
		{
			"class H { companion object { val X = 1}}; class Foo : Any()\n",
			"class H { companion object { val X = 1}}; class Foo \n",
		},
		{
			"class H { companion object { val X = 1}; class Foo : Any()}\n",
			"class H { companion object { val X = 1}; class Foo }\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "H.kt")
			if err := os.WriteFile(file, []byte(tc.src), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			res, err := RunProject(context.Background(), ProjectInput{Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{root},
				ActiveRules: []*api.Rule{findV2RuleForTest(t, "UnnecessaryInheritance")},
				Format:      "json",
				Version:     "test",
				Fix:         true,
				MaxFixLevel: rules.FixIdiomatic,
			}})
			if err != nil {
				t.Fatalf("RunProject: %v", err)
			}
			if res.Fixup.AppliedFixes != 1 {
				t.Errorf("AppliedFixes = %d, want 1", res.Fixup.AppliedFixes)
			}
			if res.Stats.FindingsInErrorRegions != 0 {
				t.Errorf("FindingsInErrorRegions = %d, want 0", res.Stats.FindingsInErrorRegions)
			}
			got, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("fixed file = %q, want %q", got, tc.want)
			}
		})
	}
}
