package deadcode

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findFinding(findings []ProjectFinding, name string) *ProjectFinding {
	for i := range findings {
		if findings[i].Name == name {
			return &findings[i]
		}
	}
	return nil
}

func TestAnalyzeProject_ReportsUnreachableAndKeepsRoots(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "main", "kotlin")

	writeFile(t, filepath.Join(src, "App.kt"), `
package com.acme

fun main(args: Array<String>) {
    Greeter().hello()
}

class Greeter {
    fun hello() = "hi"
}

class OldOAuthHandler {
    fun obsoleteAuth() = "no-op"
}

fun parseLegacyConfig(raw: String): String = raw
`)

	findings, err := AnalyzeProject(dir, ProjectOptions{Paths: []string{dir}})
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	if findFinding(findings, "OldOAuthHandler") == nil {
		t.Errorf("expected OldOAuthHandler to be reported as dead, got %+v", findings)
	}
	if findFinding(findings, "parseLegacyConfig") == nil {
		t.Errorf("expected parseLegacyConfig to be reported as dead, got %+v", findings)
	}
	if f := findFinding(findings, "Greeter"); f != nil {
		t.Errorf("Greeter should be reachable from main(), got %+v", f)
	}
	if f := findFinding(findings, "main"); f != nil {
		t.Errorf("main should not be reported, got %+v", f)
	}
}

func TestAnalyzeProject_HiltAndTestRootsReachable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "main", "kotlin")

	writeFile(t, filepath.Join(src, "Hilt.kt"), `
package com.acme

@HiltAndroidApp
class MyApp

@AndroidEntryPoint
class HomeActivity

@Test
fun runsSomething() {}

class TrulyDead { fun gone() = 0 }
`)

	findings, err := AnalyzeProject(dir, ProjectOptions{Paths: []string{dir}})
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}

	for _, name := range []string{"MyApp", "HomeActivity", "runsSomething"} {
		if f := findFinding(findings, name); f != nil {
			t.Errorf("%s should be a root and not reported, got %+v", name, f)
		}
	}
	if findFinding(findings, "TrulyDead") == nil {
		t.Errorf("TrulyDead should be reported as dead, got %+v", findings)
	}
}

func TestAnalyzeProject_UserSuppliedRoot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "main", "kotlin")

	writeFile(t, filepath.Join(src, "Lib.kt"), `
package com.acme

class ReflectivelyUsed { fun work() = "ok" }
`)

	findings, err := AnalyzeProject(dir, ProjectOptions{
		Paths: []string{dir},
		Roots: []string{"com.acme.ReflectivelyUsed"},
	})
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	if f := findFinding(findings, "ReflectivelyUsed"); f != nil {
		t.Errorf("ReflectivelyUsed declared as user root should not be reported, got %+v", f)
	}
}

func TestAnalyzeProject_AndroidManifestRoots(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "main", "kotlin")
	manifestDir := filepath.Join(dir, "src", "main")

	writeFile(t, filepath.Join(src, "Components.kt"), `
package com.acme

class HomeActivity
class UnusedScreen
`)
	writeFile(t, filepath.Join(manifestDir, "AndroidManifest.xml"), `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <application>
    <activity android:name="com.acme.HomeActivity" />
  </application>
</manifest>
`)

	findings, err := AnalyzeProject(dir, ProjectOptions{Paths: []string{dir}})
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	if f := findFinding(findings, "HomeActivity"); f != nil {
		t.Errorf("HomeActivity should be a manifest root, got %+v", f)
	}
	if findFinding(findings, "UnusedScreen") == nil {
		t.Errorf("UnusedScreen should be reported as dead, got %+v", findings)
	}
}

func TestAnalyzeProject_IncludesJavaAndUsesKotlinReferences(t *testing.T) {
	dir := t.TempDir()
	javaSrc := filepath.Join(dir, "src", "main", "java", "com", "acme")
	ktSrc := filepath.Join(dir, "src", "main", "kotlin")

	writeFile(t, filepath.Join(javaSrc, "JavaApi.java"), `package com.acme;

public class JavaApi {
  public void usedFromKotlin() {}
  public void trulyUnused() {}
}
`)
	writeFile(t, filepath.Join(ktSrc, "UseJava.kt"), `package com.acme

fun main(api: JavaApi) {
    api.usedFromKotlin()
}
`)

	findings, err := AnalyzeProject(dir, ProjectOptions{Paths: []string{dir}})
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	if f := findFinding(findings, "usedFromKotlin"); f != nil {
		t.Errorf("usedFromKotlin should be reachable from Kotlin, got %+v", f)
	}
	if findFinding(findings, "trulyUnused") == nil {
		t.Errorf("expected trulyUnused Java method to be reported, got %+v", findings)
	}
}
