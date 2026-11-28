package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reportShortcutUsageCount runs krit with ReportShortcutUsage alone on dir and counts its findings.
func reportShortcutUsageCount(t *testing.T, dir string, args ...string) int {
	t.Helper()
	stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--enable-rules", "ReportShortcutUsage", "-f", "json"}, args...), dir)...)
	var report struct {
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
	}
	n := 0
	for _, f := range report.Findings {
		if f.Rule == "ReportShortcutUsage" {
			n++
		}
	}
	return n
}

// writeProjectFile writes body at rel under project, creating its directories.
func writeProjectFile(t *testing.T, project, rel, body string) {
	t.Helper()
	path := filepath.Join(project, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ReportShortcutUsage stays silent when every module that makes shortcut calls is a library. A module that only
// mentions the word, here an app whose comment says "Shortcut", makes no calls and must not count.
func TestReportShortcutUsageCountsOnlyModulesWithShortcutCalls(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "fixtures", "android-lint", "ReportShortcutUsage", "positive")
	source, err := os.ReadFile(filepath.Join(fixture, "src", "main", "java", "com", "example", "Shortcuts.kt"))
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "project")
	manifest := "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n"
	writeProjectFile(t, project, "settings.gradle.kts", "include(\":app\", \":lib\")\n")
	writeProjectFile(t, project, "app/build.gradle.kts", "plugins {\n    id(\"com.android.application\")\n}\n")
	writeProjectFile(t, project, "app/src/main/AndroidManifest.xml", manifest)
	writeProjectFile(t, project, "app/src/main/java/com/example/Main.kt", "package com.example\n\n// Shortcut screen\nclass Main\n")
	writeProjectFile(t, project, "lib/build.gradle.kts", "plugins {\n    id(\"com.android.library\")\n}\n")
	writeProjectFile(t, project, "lib/src/main/AndroidManifest.xml", manifest)
	writeProjectFile(t, project, "lib/src/main/java/com/example/Shortcuts.kt", string(source))
	if n := reportShortcutUsageCount(t, project, "--no-cache"); n != 0 {
		t.Fatalf("library-only shortcut calls next to an app that only mentions shortcuts: %d findings, want 0", n)
	}
	// Moving the calls into the app makes them count.
	writeProjectFile(t, project, "app/src/main/java/com/example/Main.kt", string(source))
	if n := reportShortcutUsageCount(t, project, "--no-cache"); n == 0 {
		t.Fatal("shortcut calls in the app module are not reported")
	}
}

// Lint checks each module on its own (checkDependencies is off by default) and skips library modules: with an app and
// a library that both publish shortcuts, only the app's calls are reported, and a usage report in the library does not
// count for the app.
func TestReportShortcutUsageReportsApplicationModulesOnly(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "fixtures", "android-lint", "ReportShortcutUsage", "positive")
	source, err := os.ReadFile(filepath.Join(fixture, "src", "main", "java", "com", "example", "Shortcuts.kt"))
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "project")
	manifest := "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n"
	writeProjectFile(t, project, "settings.gradle.kts", "include(\":app\", \":lib\")\n")
	writeProjectFile(t, project, "app/build.gradle.kts", "plugins {\n    id(\"com.android.application\")\n}\n")
	writeProjectFile(t, project, "app/src/main/AndroidManifest.xml", manifest)
	writeProjectFile(t, project, "app/src/main/java/com/example/Shortcuts.kt", string(source))
	writeProjectFile(t, project, "lib/build.gradle.kts", "plugins {\n    id(\"com.android.library\")\n}\n")
	writeProjectFile(t, project, "lib/src/main/AndroidManifest.xml", manifest)
	writeProjectFile(t, project, "lib/src/main/java/com/example/lib/Shortcuts.kt", strings.Replace(string(source), "package com.example", "package com.example.lib", 1))
	stdout, stderr, _ := runKrit(t, "--no-daemon", "--no-cache", "--enable-rules", "ReportShortcutUsage", "-f", "json", project)
	var report struct {
		Findings []struct {
			Rule string `json:"rule"`
			File string `json:"file"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
	}
	app := 0
	for _, f := range report.Findings {
		if f.Rule != "ReportShortcutUsage" {
			continue
		}
		if strings.Contains(filepath.ToSlash(f.File), "lib/src/") {
			t.Errorf("reported a library call site: %s", f.File)
		}
		app++
	}
	if app == 0 {
		t.Fatal("the app module's shortcut calls are not reported")
	}
	// A usage report in the library leaves the app's calls reported.
	writeProjectFile(t, project, "lib/src/main/java/com/example/lib/Report.kt",
		"package com.example.lib\n\nimport android.content.Context\nimport androidx.core.content.pm.ShortcutManagerCompat\n\n"+
			"fun used(context: Context) {\n    ShortcutManagerCompat.reportShortcutUsed(context, \"id\")\n}\n")
	if n := reportShortcutUsageCount(t, project, "--no-cache"); n != app {
		t.Fatalf("a usage report in the library changed the app's findings: %d, want %d", n, app)
	}
}

// ReportShortcutUsage stays silent when every module that sets shortcuts is a library, so it reads each module's
// plugins too. A warm run must follow a build-script switch between application and library, a commented-out plugin
// included, exactly as a cache-free run does, at every scan root. Each step edits forward only.
func TestReportShortcutUsageWarmRunFollowsModulePlugin(t *testing.T) {
	fixture := filepath.Join("..", "..", "tests", "fixtures", "android-lint", "ReportShortcutUsage", "positive")
	source, err := os.ReadFile(filepath.Join(fixture, "src", "main", "java", "com", "example", "Shortcuts.kt"))
	if err != nil {
		t.Fatal(err)
	}
	scriptOf := func(plugins string) string {
		return "plugins {\n" + plugins + "}\n\nandroid {\n    namespace = \"com.example\"\n    defaultConfig {\n        minSdk = 25\n    }\n}\n"
	}
	for _, scan := range []string{"", "lib/src/main", "lib/src/main/java"} {
		t.Run("scan /"+scan, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "project")
			write := func(rel, body string) {
				t.Helper()
				path := filepath.Join(project, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("settings.gradle.kts", "include(\":lib\")\n")
			write("lib/build.gradle.kts", scriptOf("    id(\"com.android.application\")\n"))
			write("lib/src/main/AndroidManifest.xml", "<manifest xmlns:android=\"http://schemas.android.com/apk/res/android\" />\n")
			write("lib/src/main/java/com/example/Shortcuts.kt", string(source))
			count := func(args ...string) int {
				t.Helper()
				stdout, stderr, _ := runKrit(t, append(append([]string{"--no-daemon", "--enable-rules", "ReportShortcutUsage", "-f", "json"}, args...), filepath.Join(project, scan))...)
				var report struct {
					Findings []struct {
						Rule string `json:"rule"`
					} `json:"findings"`
				}
				if err := json.Unmarshal([]byte(stdout), &report); err != nil {
					t.Fatalf("parse report: %v\nstderr: %s", err, stderr)
				}
				n := 0
				for _, f := range report.Findings {
					if f.Rule == "ReportShortcutUsage" {
						n++
					}
				}
				return n
			}
			truth := func() int { return count("--no-cache", "--no-cross-file-cache") }
			cold := truth()
			if cold == 0 {
				t.Fatal("the application module reports nothing cache-free; the fixture no longer exercises the rule")
			}
			if warm := count(); warm != cold {
				t.Fatalf("cold run: %d findings, cache-free %d", warm, cold)
			}
			for _, step := range []struct {
				name, plugins string
				want          int
			}{
				{"library plugin", "    id(\"com.android.library\")\n", 0},
				{"library commented out, application", "    // id(\"com.android.library\")\n    id(\"com.android.application\")\n", cold},
			} {
				write("lib/build.gradle.kts", scriptOf(step.plugins))
				if warm, free := count(), truth(); warm != free || free != step.want {
					t.Fatalf("after %s: warm run %d findings, cache-free %d, want %d for both", step.name, warm, free, step.want)
				}
			}
		})
	}
}
