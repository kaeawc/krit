package scanner

import (
	"context"
	"strings"
	"testing"
)

func TestScriptKindOf(t *testing.T) {
	tests := map[string]ScriptKind{
		"app/build.gradle.kts":            ScriptGradle,
		"settings.gradle.kts":             ScriptGradle,
		"buildSrc/src/conv.gradle.kts":    ScriptGradle,
		"init.gradle.kts":                 ScriptGradle,
		"tools/report.main.kts":           ScriptStandalone,
		"tools/other.kts":                 ScriptStandalone,
		"TOOLS/Build.GRADLE.KTS":          ScriptNone,
		"src/Main.kt":                     ScriptNone,
		"build.gradle":                    ScriptNone,
		"notes.kts.txt":                   ScriptNone,
		"src/kts/Foo.java":                ScriptNone,
		"":                                ScriptNone,
		"a/b/script.main.kts":             ScriptStandalone,
		"a/b/.kts":                        ScriptStandalone,
		"a/b/gradle.kts":                  ScriptStandalone,
		"a/b/my.gradle.kts.bak":           ScriptNone,
		"C:\\work\\app\\build.gradle.kts": ScriptGradle,
	}
	for path, want := range tests {
		if got := ScriptKindOf(path); got != want {
			t.Errorf("ScriptKindOf(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestFileScriptKindRequiresKotlinLanguage(t *testing.T) {
	kotlin := &File{Path: "tool.main.kts", Language: LangKotlin}
	if kotlin.ScriptKind() != ScriptStandalone || !kotlin.IsScript() {
		t.Fatalf("Kotlin .main.kts = %v, want standalone script", kotlin.ScriptKind())
	}
	gradle := &File{Path: "build.gradle.kts", Language: LangGradle}
	if gradle.ScriptKind() != ScriptNone || gradle.IsScript() {
		t.Fatalf("Gradle-language file = %v, want none so Gradle rules ignore script policy", gradle.ScriptKind())
	}
	var nilFile *File
	if nilFile.IsScript() {
		t.Fatal("nil file must not be a script")
	}
}

func parseScriptForTest(t *testing.T, path, src string) *File {
	t.Helper()
	parser := GetKotlinParser()
	defer PutKotlinParser(parser)
	f, err := ParseKotlinFile(context.Background(), parser, path, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func scriptTopLevelTypes(f *File) []string {
	var out []string
	f.FlatForEachChild(0, func(c uint32) { out = append(out, f.FlatType(c)) })
	return out
}

func assertNoErrorNodes(t *testing.T, f *File) {
	t.Helper()
	for i, flags := range f.FlatTree.Flags {
		if flags&flatNodeFlagIsError != 0 {
			t.Errorf("error node %q at row %d", f.FlatNodeText(uint32(i)), f.FlatTree.StartRows[i]+1)
		}
	}
}

func TestParseGradleKotlinDslScript(t *testing.T) {
	src := `@file:Suppress("UnstableApiUsage")

import org.gradle.api.tasks.Delete

buildscript {
  repositories { google(); mavenCentral() }
  dependencies { classpath("com.example:plugin:1.2.3") }
}

plugins {
  id("com.android.application")
  kotlin("android") version "1.9.0" apply false
  alias(libs.plugins.sample)
}

val myProp by extra("v")
val sampleVersion: String by project
val Project.isRelease get() = hasProperty("release")

android {
  namespace = "com.example"
  defaultConfig { minSdk = 21 }
  buildTypes { release { isMinifyEnabled = true } }
}

dependencies {
  implementation(libs.androidx.core)
  testImplementation(project(":core"))
}

tasks.register<Delete>("cleanAll") {
  delete(rootProject.layout.buildDirectory)
}

tasks.withType<Test>().configureEach { useJUnitPlatform() }

fun Project.helper(name: String): String = "x$name"
`
	f := parseScriptForTest(t, "build.gradle.kts", src)
	assertNoErrorNodes(t, f)
	want := []string{
		"file_annotation", "import_list", "call_expression", "call_expression", "property_declaration",
		"property_declaration", "property_declaration", "call_expression", "call_expression",
		"call_expression", "call_expression", "function_declaration",
	}
	if got := scriptTopLevelTypes(f); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("top-level nodes = %v, want %v", got, want)
	}
}

func TestParseSettingsScript(t *testing.T) {
	src := `pluginManagement {
  repositories { gradlePluginPortal(); google() }
  includeBuild("build-logic")
}
dependencyResolutionManagement {
  repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
  versionCatalogs { create("libs") { from(files("gradle/libs.versions.toml")) } }
}
rootProject.name = "demo"
include(":app", ":core")
`
	f := parseScriptForTest(t, "settings.gradle.kts", src)
	assertNoErrorNodes(t, f)
	want := "call_expression,call_expression,assignment,call_expression"
	if got := strings.Join(scriptTopLevelTypes(f), ","); got != want {
		t.Fatalf("top-level nodes = %s, want %s", got, want)
	}
}

func TestParseMainKtsScript(t *testing.T) {
	src := `#!/usr/bin/env kotlin
@file:DependsOn("com.example:lib:1.0")
@file:Repository("https://repo.example.com")
@file:Suppress("MagicNumber")

import java.io.File
import kotlin.system.exitProcess

val root = File(args.firstOrNull() ?: ".")
println("scanning $root")
@Suppress("unused")
@JvmField
private fun annotated() {}

fun walk(f: File): Sequence<File> = f.walkTopDown()
if (!root.exists()) {
  System.err.println("missing")
  exitProcess(1)
}
walk(root).filter { it.extension == "kt" }.forEach { println(it) }
class Cfg(val name: String)
object Single {
  val x = 1
}
`
	f := parseScriptForTest(t, "tool.main.kts", src)
	assertNoErrorNodes(t, f)
	want := []string{
		"shebang_line", "file_annotation", "file_annotation", "file_annotation", "import_list",
		"property_declaration", "call_expression", "function_declaration", "function_declaration",
		"if_expression", "call_expression", "class_declaration", "object_declaration",
	}
	if got := scriptTopLevelTypes(f); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("top-level nodes = %v, want %v", got, want)
	}
}

// A top-level annotated expression statement is legal in a script. The
// annotation repair for misparsed declarations must not move it onto the
// declaration that follows.
func TestParseScriptKeepsAnnotatedExpressionStatement(t *testing.T) {
	src := `@Suppress("DEPRECATION") doSomething()
fun next() {}
@Suppress("DEPRECATION")
legacy.call(1)
class Foo
`
	f := parseScriptForTest(t, "a.main.kts", src)
	assertNoErrorNodes(t, f)
	var fn uint32
	f.FlatWalkNodes(0, "function_declaration", func(idx uint32) { fn = idx })
	if fn == 0 {
		t.Fatal("function_declaration not found")
	}
	if text := f.FlatNodeText(fn); strings.Contains(text, "@Suppress") {
		t.Fatalf("annotation of the previous statement moved onto fun next: %q", text)
	}
}
