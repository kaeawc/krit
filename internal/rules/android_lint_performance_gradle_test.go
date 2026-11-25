package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// The expected findings are what Android Lint 32.4.1 reports for the same scripts, through lint-tests' TestLintTask
// (GradleModelMocker), as "line:col severity message". The Lint command line downgrades NotShrinkingResources to a warning,
// and ProguardAndroidTxtUsage (an error) to a warning before AGP 9.

func perfGradleFindings(t *testing.T, run kotlinGradleRun) []string {
	t.Helper()
	var out []string
	for _, f := range run.findings(t) {
		out = append(out, fmt.Sprintf("%d:%d %s %s", f.Line, f.Col, f.Severity, f.Message))
	}
	sort.Strings(out)
	return out
}

const perfGroovyScript = `apply plugin: 'com.android.application'

android {
    compileOptions {
        sourceCompatibility JavaVersion.VERSION_1_8
        targetCompatibility JavaVersion.VERSION_1_8
    }
    buildTypes {
        release {
            minifyEnabled true
            shrinkResources false
            proguardFiles getDefaultProguardFile("proguard-android.txt")
        }
        debug {
            shrinkResources false
        }
        staging {
            minifyEnabled false
        }
    }
}
dependencies {
    kapt 'com.github.bumptech.glide:compiler:4.16.0'
    kaptTest 'androidx.room:room-compiler:2.6.1'
    kapt group: 'com.squareup.moshi', name: 'moshi-kotlin-codegen', version: '1.15.0'
    annotationProcessor 'androidx.lifecycle:lifecycle-compiler:2.6.0'
    kapt "android.arch.lifecycle:compiler:1.1.1"
    implementation "androidx.lifecycle:lifecycle-compiler:2.6.0"
    // kapt 'androidx.room:room-compiler:2.6.1'
}
`

func TestPerformanceGradle_GroovyMatchesLint(t *testing.T) {
	const ksp = "This library supports using KSP instead of kapt, which greatly improves performance. Learn more: https://developer.android.com/studio/build/migrate-to-ksp"
	const lifecycle = "Use the Lifecycle Java 8 API provided by the `lifecycle-common` library instead of Lifecycle annotations for faster incremental build."
	cases := map[string][]string{
		"ProguardAndroidTxtUsage": {"12:27 warning Avoid `getDefaultProguardFile('proguard-android.txt')`"},
		"NotShrinkingResources":   {"11:29 warning Avoid setting shrinkResources = false"},
		"KaptUsageInsteadOfKsp":   {"23:5 warning " + ksp, "25:5 warning " + ksp},
		"LifecycleAnnotationProcessorWithJava8": {
			"26:25 warning " + lifecycle, "27:10 warning " + lifecycle, "28:20 warning " + lifecycle,
		},
	}
	for rule, want := range cases {
		got := perfGradleFindings(t, kotlinGradleRun{rule: rule, path: "build.gradle", content: perfGroovyScript, agp: "8.5.0"})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", rule, got, want)
		}
	}
}

const perfKtsScript = `plugins {
    id("com.android.application")
}

android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
    }
    buildTypes {
        getByName("release") {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android.txt"), "proguard-rules.pro")
        }
        getByName("debug") {
            isShrinkResources = false
        }
    }
}
dependencies {
    kapt("com.squareup.moshi:moshi-kotlin-codegen:1.15.0")
    implementation("androidx.lifecycle:lifecycle-compiler:2.6.0")
    val s = "kapt(\"androidx.room:room-compiler:2.6.1\")"
}
`

func TestPerformanceGradle_KtsMatchesLint(t *testing.T) {
	cases := map[string][]string{
		// Lint pairs blocks by name: getByName("debug") shares getByName("release")'s minifyEnabled.
		"NotShrinkingResources":   {"15:33 warning Avoid setting isShrinkResources = false"},
		"ProguardAndroidTxtUsage": {"12:27 error Avoid `getDefaultProguardFile('proguard-android.txt')`"},
		"KaptUsageInsteadOfKsp": {"20:5 warning This library supports using KSP instead of kapt, which greatly improves performance. " +
			"Learn more: https://developer.android.com/studio/build/migrate-to-ksp"},
		"LifecycleAnnotationProcessorWithJava8": {"21:20 warning Use the Lifecycle Java 8 API provided by the `lifecycle-common` " +
			"library instead of Lifecycle annotations for faster incremental build."},
	}
	for rule, want := range cases {
		got := perfGradleFindings(t, kotlinGradleRun{rule: rule, content: perfKtsScript, agp: "9.0.0"})
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", rule, got, want)
		}
	}
}

func TestNotShrinkingResources_MissingShrinkReportsLastMinify(t *testing.T) {
	script := `apply plugin: 'com.android.application'

android {
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_7
    }
    buildTypes {
        debug {
            minifyEnabled = true
        }
        release {
            minifyEnabled true
            // getDefaultProguardFile('proguard-android.txt')
            proguardFiles "proguard-android.txt"
        }
        qa {
            minifyEnabled false
        }
    }
}
dependencies {
    annotationProcessor 'androidx.lifecycle:lifecycle-compiler:2.6.0'
}
`
	run := kotlinGradleRun{rule: "NotShrinkingResources", path: "build.gradle", content: script, agp: "8.5.0"}
	got := perfGradleFindings(t, run)
	if want := []string{"12:13 warning If enabling minification, also set shrinkResources = true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// A comment and a plain string are not getDefaultProguardFile('proguard-android.txt'); Java 7 is below 8.
	for _, rule := range []string{"ProguardAndroidTxtUsage", "LifecycleAnnotationProcessorWithJava8"} {
		if got := perfGradleFindings(t, kotlinGradleRun{rule: rule, path: "build.gradle", content: script, agp: "8.5.0"}); len(got) != 0 {
			t.Errorf("%s: got %q", rule, got)
		}
	}
	// Lint stops reporting a missing shrinkResources from AGP 10.
	run.agp = "10.0.0"
	if got := perfGradleFindings(t, run); len(got) != 0 {
		t.Errorf("AGP 10: got %q", got)
	}
}

func TestNotShrinkingResources_ShrinkSetAnywhereSilencesMissing(t *testing.T) {
	script := `plugins {
    id("com.android.application")
}

android {
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
        }
        create("staging") {
            isMinifyEnabled = true
        }
    }
}
`
	if got := perfGradleFindings(t, kotlinGradleRun{rule: "NotShrinkingResources", content: script, agp: "8.5.0"}); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestPerformanceGradle_LibraryModulesSkipR8Checks(t *testing.T) {
	script := `apply plugin: 'com.android.library'

android {
    buildTypes {
        release {
            minifyEnabled true
            proguardFiles getDefaultProguardFile('proguard-android.txt')
        }
    }
}
dependencies {
    kapt 'androidx.room:room-compiler:2.6.1'
}
`
	for _, rule := range []string{"ProguardAndroidTxtUsage", "NotShrinkingResources"} {
		if got := perfGradleFindings(t, kotlinGradleRun{rule: rule, path: "build.gradle", content: script, agp: "8.5.0"}); len(got) != 0 {
			t.Errorf("%s: a library module is skipped, got %q", rule, got)
		}
	}
	got := perfGradleFindings(t, kotlinGradleRun{rule: "KaptUsageInsteadOfKsp", path: "build.gradle", content: script})
	if len(got) != 1 {
		t.Errorf("KaptUsageInsteadOfKsp still checks libraries, got %q", got)
	}
}

func TestProguardAndroidTxtUsage_SeverityFollowsAGP(t *testing.T) {
	script := `buildscript {
    dependencies {
        classpath 'com.android.tools.build:gradle:8.5.0'
    }
}
apply plugin: 'com.android.application'

android {
    buildTypes {
        release {
            proguardFiles getDefaultProguardFile('proguard-android.txt')
        }
    }
}
`
	// No injected facts: the version comes from the buildscript classpath, as Lint's model would have it.
	fs := kotlinGradleRun{rule: "ProguardAndroidTxtUsage", path: "build.gradle", content: script}.findingsWithoutFacts(t)
	if len(fs) != 1 || fs[0].Severity != "warning" {
		t.Fatalf("AGP 8.5 is a warning, got %+v", fs)
	}
	fs = kotlinGradleRun{rule: "ProguardAndroidTxtUsage", path: "build.gradle", content: script, agp: "9.0.0"}.findings(t)
	if len(fs) != 1 || fs[0].Severity != "error" {
		t.Fatalf("AGP 9 keeps the error, got %+v", fs)
	}
}

func TestLifecycleAnnotationProcessorWithJava8_JavaLevelSources(t *testing.T) {
	deps := "\ndependencies {\n    kapt(\"androidx.lifecycle:lifecycle-compiler:2.6.0\")\n}\n"
	cases := []struct {
		name, script, agp string
		want              int
	}{
		{"toolchain 17", "kotlin {\n    jvmToolchain(17)\n}" + deps, "", 1},
		{"explicit 1.7", "android {\n    compileOptions {\n        sourceCompatibility = JavaVersion.VERSION_1_7\n    }\n}" + deps, "8.5.0", 0},
		{"AGP default", deps, "8.5.0", 1},
		{"old AGP default", deps, "4.1.0", 0},
		{"unknown", deps, "", 0},
	}
	for _, c := range cases {
		got := perfGradleFindings(t, kotlinGradleRun{rule: "LifecycleAnnotationProcessorWithJava8", content: c.script, agp: c.agp})
		if len(got) != c.want {
			t.Errorf("%s: got %q, want %d finding(s)", c.name, got, c.want)
		}
	}
}

func TestKaptUsageInsteadOfKsp_VersionCatalog(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := "[libraries]\nroom-compiler = { module = \"androidx.room:room-compiler\", version = \"2.6.1\" }\n" +
		"dagger-compiler = { group = \"com.google.dagger\", name = \"dagger-compiler\", version = \"2.50\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "dependencies {\n    kapt(libs.room.compiler)\n    kapt(libs.dagger.compiler)\n}\n"
	got := perfGradleFindings(t, kotlinGradleRun{rule: "KaptUsageInsteadOfKsp", path: filepath.Join(dir, "build.gradle.kts"), content: script})
	if len(got) != 1 || got[0][:4] != "2:5 " {
		t.Fatalf("only the Room catalog entry has a KSP replacement in Lint's table, got %q", got)
	}
}

func TestPerformanceGradle_Fixes(t *testing.T) {
	apply := func(content string, fs []scanner.Finding) string {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Fix.StartByte > fs[j].Fix.StartByte })
		for _, f := range fs {
			content = content[:f.Fix.StartByte] + f.Fix.Replacement + content[f.Fix.EndByte:]
		}
		return content
	}
	script := "android {\n    buildTypes {\n        release {\n            isMinifyEnabled = true\n" +
		"            proguardFiles(getDefaultProguardFile(\"proguard-android.txt\"))\n        }\n    }\n}\n"
	fs := kotlinGradleRun{rule: "ProguardAndroidTxtUsage", content: script, agp: "8.5.0"}.findings(t)
	fs = append(fs, kotlinGradleRun{rule: "NotShrinkingResources", content: script, agp: "8.5.0"}.findings(t)...)
	want := "android {\n    buildTypes {\n        release {\n            isMinifyEnabled = true\n            isShrinkResources = true\n" +
		"            proguardFiles(getDefaultProguardFile(\"proguard-android-optimize.txt\"))\n        }\n    }\n}\n"
	if got := apply(script, fs); got != want {
		t.Fatalf("fixed:\n%s\nwant:\n%s", got, want)
	}
	falseScript := "android {\n    buildTypes {\n        release {\n            minifyEnabled true\n            shrinkResources false\n        }\n    }\n}\n"
	fs = kotlinGradleRun{rule: "NotShrinkingResources", path: "build.gradle", content: falseScript, agp: "8.5.0"}.findings(t)
	if got := apply(falseScript, fs); got != "android {\n    buildTypes {\n        release {\n            minifyEnabled true\n            shrinkResources true\n        }\n    }\n}\n" {
		t.Fatalf("fixed:\n%s", got)
	}
}
