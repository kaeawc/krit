package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kaptScript = `plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.kapt")
    id("kotlin-kapt")
    kotlin("kapt")
    id("com.google.devtools.ksp")
}

kapt {
    correctErrorTypes = true
}

dependencies {
    kapt("androidx.room:room-compiler:2.8.0")
    kapt 'com.squareup.moshi:moshi-kotlin-codegen:1.15.2'
    kaptTest("com.github.bumptech.glide:compiler:5.0.0")
    kapt("com.google.dagger:hilt-android-compiler:2.60")
    kaptAndroidTest("com.google.dagger:dagger-compiler:2.60")
    kapt("com.example:custom-processor:1.0")
    ksp("androidx.room:room-compiler:2.8.0")
    implementation("com.example:kapt-runtime:1.0")
    // kapt("androidx.room:room-compiler:2.8.0")
}
`

func TestKaptUsageReportsPluginsAndConfigurations(t *testing.T) {
	fs := kotlinGradleRun{rule: "KaptUsage", content: kaptScript, kotlin: "2.4.20", agp: "8.13.0"}.findings(t)
	got := sortedLines(fs)
	want := []int{3, 4, 5, 14, 15, 16, 17, 18, 19}
	if len(got) != len(want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lines = %v, want %v", got, want)
		}
	}
	for _, f := range fs {
		if f.Fix != nil {
			t.Errorf("KaptUsage has no autofix: %+v", f)
		}
	}
}

func TestKaptUsageNamesOnlyVerifiedKspProcessors(t *testing.T) {
	fs := kotlinGradleRun{rule: "KaptUsage", content: kaptScript, kotlin: "2.4.20"}.findings(t)
	byLine := map[int]string{}
	for _, f := range fs {
		byLine[f.Line] = f.Message
		if kaptScript[f.StartByte:f.EndByte] == "" {
			t.Errorf("empty span on line %d", f.Line)
		}
	}
	for line, want := range map[int]string{
		14: "Room supports KSP: use `ksp`",
		15: "Moshi codegen supports KSP",
		16: "Glide supports KSP: use `kspTest`",
		17: "Hilt's KSP support is stable as of Dagger 2.60",
		18: "Dagger's KSP support is stable as of Dagger 2.60 and KSP 2.3.9: use `kspAndroidTest`",
		19: "Check the supported libraries",
	} {
		if !strings.Contains(byLine[line], want) {
			t.Errorf("line %d message = %q, want %q", line, byLine[line], want)
		}
	}
	if strings.Contains(byLine[19], "supports KSP") {
		t.Errorf("an unverified processor must not be claimed to support KSP: %q", byLine[19])
	}
	if first := fs[3]; first.Col != 5 || kaptScript[first.StartByte:first.EndByte] != "kapt" {
		t.Errorf("configuration anchor = %d span %q", first.Col, kaptScript[first.StartByte:first.EndByte])
	}
}

func TestKaptUsagePluginMessageGuidance(t *testing.T) {
	plugin := func(agp string) string {
		fs := kotlinGradleRun{rule: "KaptUsage", content: "plugins {\n    id(\"kotlin-kapt\")\n}\n", kotlin: "2.4.20", agp: agp}.findings(t)
		if len(fs) != 1 {
			t.Fatalf("agp %q: findings = %v", agp, fs)
		}
		return fs[0].Message
	}
	if msg := plugin("9.1.0"); !strings.Contains(msg, "com.android.legacy-kapt") || !strings.Contains(msg, "kapt.include.compile.classpath=false") {
		t.Errorf("AGP 9 message = %q", msg)
	}
	if msg := plugin("8.13.0"); strings.Contains(msg, "legacy-kapt") {
		t.Errorf("AGP 8 must not mention legacy-kapt: %q", msg)
	}
}

func TestKaptUsageSkipsIncludeCompileClasspathHintWhenSet(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gradle.properties"), []byte("kapt.include.compile.classpath=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs := kotlinGradleRun{rule: "KaptUsage", path: filepath.Join(root, "build.gradle.kts"), content: "plugins {\n    kotlin(\"kapt\")\n}\n", kotlin: "2.4.20"}.findings(t)
	if len(fs) != 1 || strings.Contains(fs[0].Message, "kapt.include.compile.classpath") {
		t.Fatalf("findings = %+v", fs)
	}
}

func TestKaptUsageResolvesCatalogReferences(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := "[libraries]\nandroidx-room-compiler = { module = \"androidx.room:room-compiler\", version = \"2.8.0\" }\nother = { module = \"com.example:other\", version = \"1\" }\n[plugins]\nkotlin-kapt = { id = \"org.jetbrains.kotlin.kapt\", version = \"2.4.20\" }\n"
	if err := os.WriteFile(filepath.Join(root, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	content := "plugins {\n    alias(libs.plugins.kotlin.kapt)\n}\ndependencies {\n    kapt(libs.androidx.room.compiler)\n    kapt(libs.other)\n}\n"
	fs := kotlinGradleRun{rule: "KaptUsage", path: filepath.Join(root, "build.gradle.kts"), content: content}.findingsWithoutFacts(t)
	if got := sortedLines(fs); len(got) != 3 || got[0] != 2 || got[1] != 5 || got[2] != 6 {
		t.Fatalf("lines = %v", got)
	}
	for _, f := range fs {
		if f.Line == 5 && !strings.Contains(f.Message, "Room supports KSP") {
			t.Errorf("catalog room-compiler message = %q", f.Message)
		}
		if f.Line == 6 && strings.Contains(f.Message, "supports KSP") {
			t.Errorf("unknown catalog library message = %q", f.Message)
		}
	}
}

func TestKaptUsageIgnoresApplyFalseAndSettings(t *testing.T) {
	content := "plugins {\n    id(\"org.jetbrains.kotlin.kapt\") version \"2.4.20\" apply false\n    alias(libs.plugins.kapt) apply false\n}\n"
	if fs := (kotlinGradleRun{rule: "KaptUsage", content: content, kotlin: "2.4.20"}).findings(t); len(fs) != 0 {
		t.Fatalf("findings = %v", fs)
	}
	if fs := (kotlinGradleRun{rule: "KaptUsage", path: "settings.gradle.kts", content: kaptScript, kotlin: "2.4.20"}).findings(t); len(fs) != 0 {
		t.Fatalf("settings findings = %v", fs)
	}
}
