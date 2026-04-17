package rules_test

import "testing"

// --- TrailingWhitespace ---

func TestTrailingWhitespace_Positive(t *testing.T) {
	findings := runRuleByName(t, "TrailingWhitespace", "package test\nfun main() {   \n    println(\"hi\")\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for trailing whitespace")
	}
}

func TestTrailingWhitespace_Negative(t *testing.T) {
	findings := runRuleByName(t, "TrailingWhitespace", "package test\nfun main() {\n    println(\"hi\")\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- NoTabs ---

func TestNoTabs_Positive(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nfun main() {\n\tprintln(\"hi\")\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for tab character")
	}
}

func TestNoTabs_Negative(t *testing.T) {
	findings := runRuleByName(t, "NoTabs", "package test\nfun main() {\n    println(\"hi\")\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- MaxLineLength ---

func TestMaxLineLength_Positive(t *testing.T) {
	longLine := "val result = aaaaaaaa + bbbbbbbb + cccccccc + dddddddd + eeeeeeee + ffffffff + gggggggg + hhhhhhhh + iiiiiiii + jjjjjjjj"
	code := "package test\nfun main() {\n    " + longLine + "\n}\n"
	findings := runRuleByName(t, "MaxLineLength", code)
	if len(findings) == 0 {
		t.Fatal("expected finding for line exceeding max length")
	}
}

func TestMaxLineLength_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxLineLength", "package test\nfun main() {\n    val x = 1\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- NewLineAtEndOfFile ---

func TestNewLineAtEndOfFile_Positive(t *testing.T) {
	findings := runRuleByName(t, "NewLineAtEndOfFile", "package test\nfun main() {\n}")
	if len(findings) == 0 {
		t.Fatal("expected finding for missing newline at end of file")
	}
}

func TestNewLineAtEndOfFile_Negative(t *testing.T) {
	findings := runRuleByName(t, "NewLineAtEndOfFile", "package test\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- SpacingAfterPackageAndImports ---

func TestSpacingAfterPackageAndImports_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\nfun main() {\n}\n")
	if len(findings) == 0 {
		t.Fatal("expected finding for missing blank line after package")
	}
}

func TestSpacingAfterPackageAndImports_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpacingAfterPackageAndImports", "package test\n\nfun main() {\n}\n")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- MaxChainedCallsOnSameLine ---

func TestMaxChainedCallsOnSameLine_Positive(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val x = a.b.c.d.e.f.g()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for too many chained calls")
	}
}

func TestMaxChainedCallsOnSameLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "MaxChainedCallsOnSameLine", `
package test

fun main() {
    val x = a.b.c()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- UnderscoresInNumericLiterals ---

func TestUnderscoresInNumericLiterals_Positive(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test

fun main() {
    val x = 1000000
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for large number without underscores")
	}
}

func TestUnderscoresInNumericLiterals_Negative(t *testing.T) {
	findings := runRuleByName(t, "UnderscoresInNumericLiterals", `
package test

fun main() {
    val x = 1_000_000
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- EqualsOnSignatureLine ---

func TestEqualsOnSignatureLine_Positive(t *testing.T) {
	findings := runRuleByName(t, "EqualsOnSignatureLine", `
package test

fun add(a: Int, b: Int): Int
    = a + b
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for equals on next line")
	}
}

func TestEqualsOnSignatureLine_Negative(t *testing.T) {
	findings := runRuleByName(t, "EqualsOnSignatureLine", `
package test

fun add(a: Int, b: Int): Int = a + b
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- CascadingCallWrapping ---

func TestCascadingCallWrapping_Positive(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val x = list.filter { it > 0 }
    .map { it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for cascading call with bad indentation")
	}
}

func TestCascadingCallWrapping_Negative(t *testing.T) {
	findings := runRuleByName(t, "CascadingCallWrapping", `
package test

fun main() {
    val x = list
        .filter { it > 0 }
        .map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
