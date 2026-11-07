package rules_test

import (
	"strings"
	"testing"
)

const jvmDefaultScript = `kotlin {
    compilerOptions {
        freeCompilerArgs.addAll("-Xjvm-default=all", "-opt-in=kotlin.RequiresOptIn")
        freeCompilerArgs.add('-Xjvm-default=all-compatibility')
    }
}
tasks.withType<KotlinCompile> {
    compilerOptions.freeCompilerArgs.add("-Xjvm-default=disable")
    compilerOptions.freeCompilerArgs.add("-Xjvm-default=enable")
    compilerOptions.freeCompilerArgs.add("-Xjvm-default")
}
`

func TestJvmDefaultFlagReportsLegacyValues(t *testing.T) {
	fs := kotlinGradleRun{rule: "JvmDefaultFlag", content: jvmDefaultScript, kotlin: "2.2.20"}.findings(t)
	if got := sortedLines(fs); len(got) != 5 || got[0] != 3 || got[1] != 4 || got[2] != 8 {
		t.Fatalf("lines = %v", got)
	}
	first := fs[0]
	if first.Col != 33 || jvmDefaultScript[first.StartByte:first.EndByte] != `"-Xjvm-default=all"` {
		t.Fatalf("anchor = %d span %q", first.Col, jvmDefaultScript[first.StartByte:first.EndByte])
	}
	if !strings.Contains(first.Message, "JvmDefaultMode.NO_COMPATIBILITY") || !strings.Contains(first.Message, "-jvm-default=no-compatibility") {
		t.Errorf("message = %q", first.Message)
	}
	if !strings.Contains(fs[1].Message, "JvmDefaultMode.ENABLE") {
		t.Errorf("all-compatibility message = %q", fs[1].Message)
	}
}

func TestJvmDefaultFlagFixesDocumentedMappingsOnly(t *testing.T) {
	fs := kotlinGradleRun{rule: "JvmDefaultFlag", content: jvmDefaultScript, kotlin: "2.3.0"}.findings(t)
	fixed := applyAllByteFixes(t, jvmDefaultScript, fs)
	for _, want := range []string{
		`"-jvm-default=no-compatibility", "-opt-in=kotlin.RequiresOptIn"`,
		`'-jvm-default=enable'`,
		`"-jvm-default=disable"`,
		`"-Xjvm-default=enable"`, // no documented mapping, left alone
		`"-Xjvm-default")`,       // bare flag, left alone
	} {
		if !strings.Contains(fixed, want) {
			t.Errorf("fixed script lacks %q:\n%s", want, fixed)
		}
	}
	for _, f := range fs {
		unmapped := strings.Contains(f.Message, "no mapping")
		if unmapped && f.Fix != nil {
			t.Errorf("unmapped value must be report-only: %+v", f)
		}
	}
}

func TestJvmDefaultFlagNeedsKotlin22(t *testing.T) {
	for _, kotlin := range []string{"", "2.1.21", "1.9.24"} {
		if fs := (kotlinGradleRun{rule: "JvmDefaultFlag", content: jvmDefaultScript, kotlin: kotlin}).findings(t); len(fs) != 0 {
			t.Errorf("kotlin %q: want no findings, got %d", kotlin, len(fs))
		}
	}
}

func TestJvmDefaultFlagIgnoresCommentsAndOtherFlags(t *testing.T) {
	content := `// freeCompilerArgs.add("-Xjvm-default=all")
/* "-Xjvm-default=all" */
freeCompilerArgs.add("-Xjsr305=strict")
compilerOptions { jvmDefault.set(JvmDefaultMode.NO_COMPATIBILITY) }
`
	if fs := (kotlinGradleRun{rule: "JvmDefaultFlag", content: content, kotlin: "2.4.20"}).findings(t); len(fs) != 0 {
		t.Fatalf("findings = %v", fs)
	}
}
