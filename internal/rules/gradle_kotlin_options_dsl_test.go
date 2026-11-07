package rules_test

import (
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

const kotlinOptionsTaskScript = `import org.jetbrains.kotlin.gradle.tasks.KotlinCompile

tasks.withType<KotlinCompile>().configureEach {
    kotlinOptions {
        jvmTarget = "17"
        allWarningsAsErrors = true
        freeCompilerArgs += listOf("-opt-in=kotlin.RequiresOptIn", "-Xsam-conversions=class")
        freeCompilerArgs += "-Xjvm-default=all"
    }
}
`

func TestKotlinOptionsDslSeverityFollowsKotlinVersion(t *testing.T) {
	cases := []struct {
		kotlin   string
		severity string
		conf     float64
		contains string
	}{
		{"2.4.20", "error", api.ConfidenceHigh, "every property too since 2.3.0"},
		{"2.2.0", "error", api.ConfidenceHigh, "error since Kotlin 2.2.0."},
		{"2.1.20", "warning", api.ConfidenceHigh, "deprecated since Kotlin 2.0.0"},
		{"", "warning", api.ConfidenceMedium, "not known"},
	}
	for _, c := range cases {
		fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: "android {\n    kotlinOptions {\n        jvmTarget = \"11\"\n    }\n}\n", kotlin: c.kotlin}.findings(t)
		if len(fs) != 1 {
			t.Fatalf("kotlin %q: %d findings, want 1", c.kotlin, len(fs))
		}
		f := fs[0]
		if f.Severity != c.severity || f.Confidence != c.conf || !strings.Contains(f.Message, c.contains) {
			t.Errorf("kotlin %q: severity %q conf %v message %q", c.kotlin, f.Severity, f.Confidence, f.Message)
		}
		if !strings.Contains(f.Message, "kotlin { compilerOptions") {
			t.Errorf("android block message must point at kotlin { compilerOptions }: %q", f.Message)
		}
		if f.Fix != nil {
			t.Errorf("android { kotlinOptions } is report-only")
		}
	}
}

func TestKotlinOptionsDslSkipsOldKotlin(t *testing.T) {
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: "android { kotlinOptions { jvmTarget = \"11\" } }\n", kotlin: "1.9.24"}.findings(t)
	if len(fs) != 0 {
		t.Fatalf("Kotlin 1.9 must not report: %v", fs)
	}
}

func TestKotlinOptionsDslAnchorsOnTheIdentifier(t *testing.T) {
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: kotlinOptionsTaskScript, kotlin: "2.4.20"}.findings(t)
	if len(fs) != 1 {
		t.Fatalf("findings = %v", fs)
	}
	f := fs[0]
	if f.Line != 4 || f.Col != 5 || kotlinOptionsTaskScript[f.StartByte:f.EndByte] != "kotlinOptions" {
		t.Fatalf("anchor = %d:%d span %q", f.Line, f.Col, kotlinOptionsTaskScript[f.StartByte:f.EndByte])
	}
}

func TestKotlinOptionsDslFixesTaskBlock(t *testing.T) {
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: kotlinOptionsTaskScript, kotlin: "2.4.20"}.findings(t)
	got := applyByteFix(t, kotlinOptionsTaskScript, fs[0].Fix)
	for _, want := range []string{
		"compilerOptions {",
		`jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.fromTarget("17"))`,
		"allWarningsAsErrors.set(true)",
		`freeCompilerArgs.addAll(listOf("-opt-in=kotlin.RequiresOptIn", "-Xsam-conversions=class"))`,
		`freeCompilerArgs.add("-Xjvm-default=all")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fixed script lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "kotlinOptions") || strings.Contains(got, "+=") {
		t.Errorf("fixed script still uses kotlinOptions:\n%s", got)
	}
}

func TestKotlinOptionsDslFixesSingleStatements(t *testing.T) {
	content := `tasks.withType<KotlinCompile> {
    kotlinOptions.jvmTarget = "11"
    kotlinOptions.freeCompilerArgs += "-Xexplicit-api=strict" // keep
    kotlinOptions.noJdk = true
}
`
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: content, kotlin: "2.3.0"}.findings(t)
	if got := sortedLines(fs); len(got) != 3 || got[0] != 2 || got[2] != 4 {
		t.Fatalf("lines = %v", got)
	}
	fixed := applyAllByteFixes(t, content, fs)
	if !strings.Contains(fixed, `compilerOptions.jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.fromTarget("11"))`) ||
		!strings.Contains(fixed, `compilerOptions.freeCompilerArgs.add("-Xexplicit-api=strict") // keep`) ||
		!strings.Contains(fixed, "kotlinOptions.noJdk = true") {
		t.Fatalf("fixed:\n%s", fixed)
	}
}

func TestKotlinOptionsDslLeavesUnmappableBlocksAlone(t *testing.T) {
	cases := map[string]string{
		"unknown property": "tasks.withType<KotlinCompile> {\n    kotlinOptions {\n        jvmTarget = \"17\"\n        moduleName = \"x\"\n    }\n}\n",
		"comment in body":  "tasks.withType<KotlinCompile> {\n    kotlinOptions {\n        // why\n        jvmTarget = \"17\"\n    }\n}\n",
		"computed target":  "tasks.withType<KotlinCompile> {\n    kotlinOptions {\n        jvmTarget = javaVersion\n    }\n}\n",
		"groovy dsl":       "tasks.withType(KotlinCompile) {\n    kotlinOptions {\n        jvmTarget = \"17\"\n    }\n}\n",
	}
	for name, content := range cases {
		path := "build.gradle.kts"
		if name == "groovy dsl" {
			path = "build.gradle"
		}
		fs := kotlinGradleRun{rule: "KotlinOptionsDsl", path: path, content: content, kotlin: "2.4.20"}.findings(t)
		if len(fs) != 1 {
			t.Fatalf("%s: %d findings", name, len(fs))
		}
		if fs[0].Fix != nil {
			t.Errorf("%s: must be report-only, got fix %+v", name, fs[0].Fix)
		}
	}
}

func TestKotlinOptionsDslIgnoresCommentsStringsAndDeclarations(t *testing.T) {
	content := `// kotlinOptions { }
/* kotlinOptions { } */
val doc = "kotlinOptions { jvmTarget = 17 }"
val kotlinOptions = 1
compilerOptions {
    jvmTarget.set(JvmTarget.JVM_17)
}
`
	if fs := (kotlinGradleRun{rule: "KotlinOptionsDsl", content: content, kotlin: "2.4.20"}).findings(t); len(fs) != 0 {
		t.Fatalf("findings = %v", fs)
	}
}

func TestKotlinOptionsDslReportsKmpCompilationUse(t *testing.T) {
	content := "kotlin {\n    jvm {\n        compilations.all {\n            kotlinOptions.jvmTarget = \"11\"\n        }\n    }\n}\n"
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", content: content, kotlin: "2.4.20"}.findings(t)
	if len(fs) != 1 || fs[0].Line != 4 || fs[0].Fix != nil {
		t.Fatalf("findings = %+v", fs)
	}
}

func TestKotlinOptionsDslSkipsSettingsScripts(t *testing.T) {
	fs := kotlinGradleRun{rule: "KotlinOptionsDsl", path: "settings.gradle.kts", content: "kotlinOptions { }\n", kotlin: "2.4.20"}.findings(t)
	if len(fs) != 0 {
		t.Fatalf("findings = %v", fs)
	}
}
