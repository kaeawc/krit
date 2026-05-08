package rules_test

import (
	"strings"
	"testing"
)

func TestDiCycleDetection(t *testing.T) {
	file := parseInline(t, `
package test
class A @Inject constructor(val b: B)
class B @Inject constructor(val c: C)
class C @Inject constructor(val a: A)
`)
	findings := runParsedFilesRule(t, "DiCycleDetection", file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "test.A") {
		t.Fatalf("unexpected message: %q", findings[0].Message)
	}
}

func TestDiCycleDetectionIgnoresLazyCycle(t *testing.T) {
	file := parseInline(t, `
package test
class A @Inject constructor(val b: B)
class B @Inject constructor(val a: Lazy<A>)
`)
	findings := runParsedFilesRule(t, "DiCycleDetection", file)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}
