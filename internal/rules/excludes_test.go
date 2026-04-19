package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestMatchExcludePattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{"test dir pattern matches", "src/test/kotlin/Foo.kt", "**/test/**", true},
		{"test dir pattern no match", "src/main/kotlin/Foo.kt", "**/test/**", false},
		{"suffix Test.kt matches", "src/main/kotlin/FooTest.kt", "**/*Test.kt", true},
		{"suffix Test.kt no match", "src/main/kotlin/Foo.kt", "**/*Test.kt", false},
		{"suffix Spec.kt matches", "src/main/kotlin/FooSpec.kt", "**/*Spec.kt", true},
		{"suffix Spec.kt no match", "src/main/kotlin/Foo.kt", "**/*Spec.kt", false},
		{"exact filename match", "src/main/kotlin/Foo.kt", "**/Foo.kt", true},
		{"exact filename no match", "src/main/kotlin/Bar.kt", "**/Foo.kt", false},
		{"plain glob basename", "src/main/kotlin/Foo.kt", "*.kt", true},
		{"plain glob no match", "src/main/kotlin/Foo.java", "*.kt", false},
		{"nested test dir", "project/module/src/test/java/Foo.kt", "**/test/**", true},
		{"androidTest dir", "src/androidTest/kotlin/Foo.kt", "**/androidTest/**", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.IsFileExcluded(tt.path, []string{tt.pattern})
			if got != tt.want {
				t.Errorf("IsFileExcluded(%q, [%q]) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestExcludes_SkipsRuleForMatchingFile(t *testing.T) {
	code := `package test
fun check(x: Boolean) {
    if (x) {
        foo()
    } else { }
}
`

	// Write to a path that looks like a test file
	dir := t.TempDir()
	testPath := filepath.Join(dir, "src", "test", "kotlin", "FooTest.kt")
	if err := os.MkdirAll(filepath.Dir(testPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testPath, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(testPath)
	if err != nil {
		t.Fatal(err)
	}

	// Find EmptyElseBlock rule
	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "EmptyElseBlock" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("EmptyElseBlock rule not found in registry")
	}

	// Clear any prior excludes
	rules.SetRuleExcludes("EmptyElseBlock", nil)

	// Without excludes: should produce findings
	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	findings := d.Run(file)
	if len(findings) == 0 {
		t.Fatal("expected EmptyElseBlock to fire without excludes")
	}

	// Set excludes for EmptyElseBlock
	rules.SetRuleExcludes("EmptyElseBlock", []string{"**/test/**", "**/*Test.kt"})
	defer rules.SetRuleExcludes("EmptyElseBlock", nil) // cleanup

	// With excludes: should skip the rule for this test file
	d2 := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	findings2 := d2.Run(file)
	if len(findings2) != 0 {
		t.Errorf("expected EmptyElseBlock to be excluded for test file, got %d findings", len(findings2))
	}
}

func TestExcludes_DoesNotSkipNonMatchingFile(t *testing.T) {
	code := `package test
fun check(x: Boolean) {
    if (x) {
        foo()
    } else { }
}
`

	// Write to a path that does NOT match excludes
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "src", "main", "kotlin", "Foo.kt")
	if err := os.MkdirAll(filepath.Dir(mainPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}

	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "EmptyElseBlock" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("EmptyElseBlock rule not found in registry")
	}

	// Set excludes that should NOT match this file
	rules.SetRuleExcludes("EmptyElseBlock", []string{"**/test/**", "**/*Test.kt"})
	defer rules.SetRuleExcludes("EmptyElseBlock", nil)

	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	findings := d.Run(file)
	if len(findings) == 0 {
		t.Error("expected EmptyElseBlock to fire on non-excluded main file")
	}
}

func TestExcludes_MultiplePatterns(t *testing.T) {
	code := `package test
fun check(x: Boolean) {
    if (x) {
        foo()
    } else { }
}
`

	dir := t.TempDir()

	// Test with *Spec.kt pattern
	specPath := filepath.Join(dir, "src", "main", "kotlin", "FooSpec.kt")
	if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	file, err := scanner.ParseFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	var rule *v2rules.Rule
	for _, r := range v2rules.Registry {
		if r.ID == "EmptyElseBlock" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("EmptyElseBlock rule not found in registry")
	}

	// Set excludes with multiple patterns; second should match
	rules.SetRuleExcludes("EmptyElseBlock", []string{"**/test/**", "**/*Spec.kt"})
	defer rules.SetRuleExcludes("EmptyElseBlock", nil)

	d := rules.NewDispatcherV2([]*v2rules.Rule{rule})
	findings := d.Run(file)
	if len(findings) != 0 {
		t.Errorf("expected EmptyElseBlock to be excluded for Spec file, got %d findings", len(findings))
	}
}

func TestExcludes_ConfigIntegration(t *testing.T) {
	// Verify that SetRuleExcludes/GetRuleExcludes roundtrip works
	rules.SetRuleExcludes("TestRule", []string{"**/test/**"})
	defer rules.SetRuleExcludes("TestRule", nil)

	got := rules.GetRuleExcludes("TestRule")
	if len(got) != 1 || got[0] != "**/test/**" {
		t.Errorf("expected [**/test/**], got %v", got)
	}

	// Clearing
	rules.SetRuleExcludes("TestRule", nil)
	got = rules.GetRuleExcludes("TestRule")
	if got != nil {
		t.Errorf("expected nil after clearing, got %v", got)
	}
}
