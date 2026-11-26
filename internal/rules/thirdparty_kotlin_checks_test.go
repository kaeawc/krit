package rules_test

import "testing"

// ExceptionMessageDetector does not run on test sources.
func TestExceptionMessageSkipsTestSources(t *testing.T) {
	const src = "package app\n\nfun f(x: Int?) {\n  requireNotNull(x)\n}\n"
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", src); len(got) != 1 {
		t.Fatalf("main source: want one finding, got %v", got)
	}
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/test/java/app/FTest.kt", src); len(got) != 0 {
		t.Errorf("test source: want no finding, got %v", got)
	}
}

// A precondition imported from elsewhere is not kotlin's.
func TestExceptionMessageRequiresTheStdlibFunction(t *testing.T) {
	const src = "package app\n\nimport org.junit.Assume.check\n\nfun f(x: Boolean) {\n  check(x)\n}\n"
	if got := runRuleByNameOnPath(t, "ExceptionMessage", "src/main/java/app/F.kt", src); len(got) != 0 {
		t.Errorf("want no finding for an imported check(), got %v", got)
	}
}
