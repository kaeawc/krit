package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const builtInKotlinScript = `plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    kotlin("android")
    id 'kotlin-android'
    id("org.jetbrains.kotlin.android") version "2.4.0" apply false
    id("com.example.convention")
}
apply(plugin = "kotlin-android")
// id("org.jetbrains.kotlin.android")
val note = "kotlin(\"android\") is gone"
`

func TestKotlinAndroidPluginWithBuiltInKotlinNeedsAgp9(t *testing.T) {
	for _, agp := range []string{"", "8.13.0", "8.99.9"} {
		fs := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", content: builtInKotlinScript, kotlin: "2.4.20", agp: agp}.findings(t)
		if len(fs) != 0 {
			t.Errorf("agp %q: want no findings, got %d", agp, len(fs))
		}
	}
	for _, agp := range []string{"9.0.0", "9.0.0-alpha05", "9.4.1", "10.0.0"} {
		fs := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", content: builtInKotlinScript, kotlin: "2.4.20", agp: agp}.findings(t)
		if got := sortedLines(fs); len(got) != 4 || got[0] != 3 || got[1] != 4 || got[2] != 5 || got[3] != 9 {
			t.Errorf("agp %q: lines %v, want [3 4 5 9] (apply false, comments and strings stay quiet)", agp, got)
		}
	}
}

func TestKotlinAndroidPluginWithBuiltInKotlinAnchorsAndMessage(t *testing.T) {
	fs := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", content: builtInKotlinScript, kotlin: "2.4.20", agp: "9.1.0"}.findings(t)
	spans := map[int]string{}
	for _, f := range fs {
		spans[f.Line] = builtInKotlinScript[f.StartByte:f.EndByte]
		if f.Severity != "" && f.Severity != "error" {
			t.Errorf("severity = %q", f.Severity)
		}
	}
	for line, want := range map[int]string{3: `"org.jetbrains.kotlin.android"`, 4: `kotlin("android")`, 5: `'kotlin-android'`, 9: `"kotlin-android"`} {
		if spans[line] != want {
			t.Errorf("line %d span = %q, want %q", line, spans[line], want)
		}
	}
	if fs[0].Col != 8 {
		t.Errorf("column = %d, want 8", fs[0].Col)
	}
	for _, want := range []string{"AGP 9.1", "no longer required", "android.builtInKotlin=false"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message %q lacks %q", fs[0].Message, want)
		}
	}
}

func TestKotlinAndroidPluginWithBuiltInKotlinFixDeletesPluginLines(t *testing.T) {
	content := "plugins {\n    id(\"com.android.library\")\n    id(\"org.jetbrains.kotlin.android\")\n}\n"
	fs := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", content: content, kotlin: "2.4.20", agp: "9.0.0"}.findings(t)
	if len(fs) != 1 {
		t.Fatalf("findings = %v", fs)
	}
	if got := applyByteFix(t, content, fs[0].Fix); got != "plugins {\n    id(\"com.android.library\")\n}\n" {
		t.Fatalf("fixed script = %q", got)
	}
	for name, body := range map[string]string{
		"kapt plugin":    "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n    id(\"kotlin-kapt\")\n}\n",
		"kotlinOptions":  "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n}\nandroid { kotlinOptions { jvmTarget = \"17\" } }\n",
		"kotlin sources": "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n}\nkotlin.sourceSets.named(\"main\") { }\n",
		"version clause": "plugins {\n    id(\"org.jetbrains.kotlin.android\") version \"2.4.0\"\n}\n",
	} {
		got := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", content: body, kotlin: "2.4.20", agp: "9.0.0"}.findings(t)
		if len(got) != 1 || got[0].Fix != nil {
			t.Errorf("%s: want one report-only finding, got %+v", name, got)
		}
	}
}

func TestKotlinAndroidPluginWithBuiltInKotlinHonorsOptOut(t *testing.T) {
	root := t.TempDir()
	module := filepath.Join(root, "app")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte("include(\":app\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gradle.properties"), []byte("android.builtInKotlin=false\nandroid.newDsl=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(module, "build.gradle.kts")
	plain := "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n}\n"
	if fs := (kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", path: script, content: plain, kotlin: "2.4.20", agp: "9.0.0"}).findings(t); len(fs) != 0 {
		t.Fatalf("a project that opted out keeps the plugin: %v", fs)
	}
	optedIn := "plugins {\n    id(\"com.android.built-in-kotlin\")\n    id(\"org.jetbrains.kotlin.android\")\n}\n"
	if fs := (kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", path: script, content: optedIn, kotlin: "2.4.20", agp: "9.0.0"}).findings(t); len(fs) != 1 {
		t.Fatalf("a module that opts back in to built-in Kotlin must drop the plugin: %v", fs)
	}
}

func TestKotlinAndroidPluginWithBuiltInKotlinResolvesCatalogAlias(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := "[versions]\nkotlin = \"2.4.20\"\nagp = \"9.2.0\"\n[plugins]\nkotlin-android = { id = \"org.jetbrains.kotlin.android\", version.ref = \"kotlin\" }\nkotlin-jvm = { id = \"org.jetbrains.kotlin.jvm\", version.ref = \"kotlin\" }\nandroid-app = { id = \"com.android.application\", version.ref = \"agp\" }\n"
	if err := os.WriteFile(filepath.Join(root, "gradle", "libs.versions.toml"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "build.gradle.kts")
	content := "plugins {\n    alias(libs.plugins.android.app)\n    alias(libs.plugins.kotlin.android)\n    alias(libs.plugins.kotlin.jvm)\n}\n"
	// No injected facts: the rule reads the versions and the alias from the catalog next to the script.
	fs := kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", path: script, content: content}.findingsWithoutFacts(t)
	if len(fs) != 1 || fs[0].Line != 3 || fs[0].Fix == nil {
		t.Fatalf("findings = %+v", fs)
	}
	root2 := `plugins {
    alias(libs.plugins.kotlin.android) apply false
}
`
	if fs := (kotlinGradleRun{rule: "KotlinAndroidPluginWithBuiltInKotlin", path: script, content: root2}).findingsWithoutFacts(t); len(fs) != 0 {
		t.Fatalf("apply false must stay quiet: %+v", fs)
	}
}
