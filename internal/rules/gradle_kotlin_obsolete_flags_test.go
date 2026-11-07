package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const obsoleteFlagsScript = `kotlin {
    compilerOptions {
        freeCompilerArgs.add("-Xwhen-guards")
        freeCompilerArgs.addAll("-Xnon-local-break-continue", "-Xmulti-dollar-interpolation")
        freeCompilerArgs.addAll(
            "-opt-in=kotlin.RequiresOptIn",
            "-Xcontext-parameters",
            "-Xexplicit-backing-fields",
        )
        freeCompilerArgs += listOf("-Xannotation-default-target=param-property", "-Xsam-conversions=class")
        freeCompilerArgs.add("-Xcontext-receivers")
        freeCompilerArgs.add("-Xannotation-default-target=first-only")
    }
}
`

func TestObsoleteKotlinVersionFlagsByVersion(t *testing.T) {
	cases := []struct {
		kotlin string
		want   []int
	}{
		{"2.4.21", []int{3, 4, 4, 7, 8, 10, 11}},
		{"2.3.20", []int{3, 4, 4, 11}},
		{"2.3.10", []int{3, 4, 4}},
		{"2.2.0", []int{3, 4, 4}},
		{"2.1.21", nil},
		{"", nil},
	}
	for _, c := range cases {
		fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: obsoleteFlagsScript, kotlin: c.kotlin}.findings(t)
		got := sortedLines(fs)
		if len(got) != len(c.want) {
			t.Errorf("kotlin %q: lines %v, want %v", c.kotlin, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("kotlin %q: lines %v, want %v", c.kotlin, got, c.want)
				break
			}
		}
	}
}

func TestObsoleteKotlinVersionFlagsMessagesStateTheVersion(t *testing.T) {
	fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: obsoleteFlagsScript, kotlin: "2.4.21"}.findings(t)
	byArg := map[string]string{}
	for _, f := range fs {
		byArg[obsoleteFlagsScript[f.StartByte:f.EndByte]] = f.Message
	}
	for arg, want := range map[string]string{
		`"-Xwhen-guards"`:                              "stable in Kotlin 2.2.0",
		`"-Xcontext-parameters"`:                       "stable in Kotlin 2.4.0",
		`"-Xexplicit-backing-fields"`:                  "stable in Kotlin 2.4.0",
		`"-Xcontext-receivers"`:                        "no longer supported since Kotlin 2.3.20",
		`"-Xmulti-dollar-interpolation"`:               "stable in Kotlin 2.2.0",
		`"-Xnon-local-break-continue"`:                 "stable in Kotlin 2.2.0",
		`"-Xannotation-default-target=param-property"`: "stable in Kotlin 2.4.0",
	} {
		if !strings.Contains(byArg[arg], want) {
			t.Errorf("%s message = %q, want %q", arg, byArg[arg], want)
		}
	}
	if _, ok := byArg[`"-Xannotation-default-target=first-only"`]; ok {
		t.Error("first-only keeps the old defaults on purpose and must not be reported")
	}
}

func TestObsoleteKotlinVersionFlagsFixDeletesListItems(t *testing.T) {
	fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: obsoleteFlagsScript, kotlin: "2.4.21"}.findings(t)
	got := applyAllByteFixes(t, obsoleteFlagsScript, fs)
	want := `kotlin {
    compilerOptions {
        freeCompilerArgs.addAll(
            "-opt-in=kotlin.RequiresOptIn",
        )
        freeCompilerArgs += listOf("-Xsam-conversions=class")
        freeCompilerArgs.add("-Xcontext-receivers")
        freeCompilerArgs.add("-Xannotation-default-target=first-only")
    }
}
`
	if got != want {
		t.Fatalf("fixed script:\n%s\nwant:\n%s", got, want)
	}
	for _, f := range fs {
		if strings.Contains(f.Message, "-Xcontext-receivers") && f.Fix != nil {
			t.Error("-Xcontext-receivers needs a code migration and must stay report-only")
		}
	}
}

func TestObsoleteKotlinVersionFlagsLanguageVersions(t *testing.T) {
	content := `kotlin {
    compilerOptions {
        languageVersion.set(org.jetbrains.kotlin.gradle.dsl.KotlinVersion.KOTLIN_1_9)
        freeCompilerArgs.add("-language-version=1.8")
        freeCompilerArgs.addAll("-language-version", "1.7")
        freeCompilerArgs.add("-language-version=2.0")
    }
}
android { kotlinOptions { languageVersion = "1.9" } }
java { toolchain { languageVersion.set(JavaLanguageVersion.of(17)) } }
`
	cases := []struct {
		kotlin string
		want   []int
	}{
		{"2.4.0", []int{3, 4, 5, 8}},
		{"2.3.0", []int{4, 5}},
		{"2.2.0", []int{5}},
		{"2.1.0", nil},
		{"", nil},
	}
	for _, c := range cases {
		fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: content, kotlin: c.kotlin}.findings(t)
		got := sortedLines(fs)
		if len(got) != len(c.want) {
			t.Errorf("kotlin %q: lines %v, want %v", c.kotlin, got, c.want)
			continue
		}
		for i, f := range fs {
			if f.Severity != "error" || f.Fix != nil {
				t.Errorf("kotlin %q: finding %d must be an error without a fix: %+v", c.kotlin, i, f)
			}
		}
	}
}

func TestObsoleteKotlinVersionFlagsHonorsPinnedLanguageVersion(t *testing.T) {
	content := `kotlin {
    compilerOptions {
        languageVersion.set(org.jetbrains.kotlin.gradle.dsl.KotlinVersion.KOTLIN_2_1)
        freeCompilerArgs.add("-Xwhen-guards")
    }
}
`
	if fs := (kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: content, kotlin: "2.4.21"}).findings(t); len(fs) != 0 {
		t.Fatalf("a module pinned to language 2.1 still needs -Xwhen-guards: %v", fs)
	}
}

func TestObsoleteKotlinVersionFlagsIgnoresCommentsAndStrings(t *testing.T) {
	content := `// freeCompilerArgs.add("-Xwhen-guards")
/* "-Xcontext-parameters" */
val note = "pass -Xwhen-guards to opt in"
`
	if fs := (kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", content: content, kotlin: "2.4.21"}).findings(t); len(fs) != 0 {
		t.Fatalf("findings = %v", fs)
	}
}

func TestObsoleteKotlinVersionFlagsGradleProperties(t *testing.T) {
	dir := t.TempDir()
	props := `org.gradle.jvmargs=-Xmx2g
kotlin.experimental.tryK2=true
# krit:ignore ObsoleteKotlinVersionFlags
kotlin.mpp.resourcesResolutionStrategy=x
kotlin.mpp.enablePlatformIntegerCommonization=true
kotlin.mpp.enableKotlinToolingMetadataArtifact=true
kotlin.mpp.import.enableKgpDependencyResolution=false
kotlin.mpp.import.enableKgpDependencyResolution=true
`
	if err := os.WriteFile(filepath.Join(dir, "gradle.properties"), []byte(props), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(dir, "settings.gradle.kts")
	run := func(kotlin string) map[string]bool {
		fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", path: settings, content: "rootProject.name = \"x\"\n", kotlin: kotlin}.findings(t)
		keys := map[string]bool{}
		for _, f := range fs {
			if filepath.Base(f.File) != "gradle.properties" {
				t.Fatalf("finding file = %q", f.File)
			}
			keys[props[f.StartByte:f.EndByte]] = f.Fix != nil
			if f.Col != 1 {
				t.Errorf("column = %d, want the key start", f.Col)
			}
		}
		return keys
	}
	got := run("2.4.21")
	want := map[string]bool{
		"kotlin.experimental.tryK2":                       true,
		"kotlin.mpp.enablePlatformIntegerCommonization":   true,
		"kotlin.mpp.enableKotlinToolingMetadataArtifact":  true,
		"kotlin.mpp.import.enableKgpDependencyResolution": false,
	}
	if len(got) != len(want) {
		t.Fatalf("reported keys = %v, want %v", got, want)
	}
	for k, fix := range want {
		if v, ok := got[k]; !ok || v != fix {
			t.Errorf("key %s: reported=%v fixable=%v, want fixable=%v", k, ok, v, fix)
		}
	}
	if old := run("2.2.0"); len(old) != 1 || !old["kotlin.experimental.tryK2"] {
		t.Errorf("Kotlin 2.2.0 reports %v, want only tryK2 (the mpp properties are removed in 2.2.20 or later)", old)
	}
	if none := run(""); len(none) != 0 {
		t.Errorf("unknown Kotlin version reported %v", none)
	}
}

func TestObsoleteKotlinVersionFlagsScriptsDoNotReadProperties(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gradle.properties"), []byte("kotlin.experimental.tryK2=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := kotlinGradleRun{rule: "ObsoleteKotlinVersionFlags", path: filepath.Join(dir, "build.gradle.kts"), content: "plugins {}\n", kotlin: "2.4.21"}.findings(t)
	if len(fs) != 0 {
		t.Fatalf("build scripts must not report properties: %v", fs)
	}
}
