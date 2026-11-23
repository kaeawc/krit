package bazel

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseFileBuildShapes(t *testing.T) {
	src := `load("@rules_kotlin//kotlin:jvm.bzl", "kt_jvm_library")
# A comment with kt_jvm_library(name = "commented")
COMMON = ["//lib/common", "@maven//:com_google_guava_guava"]

def helper(name):
    native.genrule(name = name)

kt_jvm_library(
    name = "app",
    srcs = glob(["src/**/*.kt"], exclude = ["src/**/Ignored.kt"]),
    deps = COMMON + [
        ":local",
        "//lib/net:net",
    ] + select({
        "//conditions:default": ["//lib/default"],
        ":android": ["//lib/android"],
    }),
    testonly = True,
    visibility = ["//visibility:public"],
)

android_library(
    name = "ui",
    resource_files = glob(["res/**"]),
    manifest = "AndroidManifest.xml",
    custom_package = """com.example.ui""",
)
`
	calls := ParseFile([]byte(src))
	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want 2", calls)
	}
	app := calls[0]
	if app.Kind != "kt_jvm_library" || app.Attr("name").Str != "app" || app.Line != 8 {
		t.Errorf("app = %s %q line %d", app.Kind, app.Attr("name").Str, app.Line)
	}
	if got, want := app.Attr("srcs").Globs(), []string{"src/**/*.kt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("srcs globs = %v, want %v", got, want)
	}
	wantDeps := []string{"//lib/common", "@maven//:com_google_guava_guava", ":local", "//lib/net:net", "//lib/default", "//lib/android"}
	if got := app.Attr("deps").Strings(); !reflect.DeepEqual(got, wantDeps) {
		t.Errorf("deps = %v, want %v", got, wantDeps)
	}
	if !app.Attr("testonly").Bool {
		t.Error("testonly not read")
	}
	ui := calls[1]
	if got := ui.Attr("resource_files").Globs(); !reflect.DeepEqual(got, []string{"res/**"}) {
		t.Errorf("resource_files = %v", got)
	}
	if ui.Attr("custom_package").Str != "com.example.ui" {
		t.Errorf("custom_package = %q", ui.Attr("custom_package").Str)
	}
}

func TestParseFileToleratesUnmodeledSyntax(t *testing.T) {
	src := `x = [s + "_test" for s in SRCS]
y = "%s/lib" % NAME
kt_jvm_library(
    name = "a" if FLAG else "b",
    srcs = [f for f in glob(["*.kt"])],
    deps = ["//ok"] + EXTRA.get("deps", []),
    **kwargs
)
java_library(name = "after", srcs = ["A.java"])
`
	calls := ParseFile([]byte(src))
	if len(calls) != 2 || calls[1].Kind != "java_library" || calls[1].Attr("name").Str != "after" {
		t.Fatalf("calls = %+v", calls)
	}
}

func write(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDiscoverWorkspace(t *testing.T) {
	root := t.TempDir()
	write(t, root, "MODULE.bazel", `
bazel_dep(name = "rules_kotlin", version = "2.1.0")
maven = use_extension("@rules_jvm_external//:extensions.bzl", "maven")
maven.install(
    artifacts = [
        "androidx.room:room-runtime:2.6.1",
        "org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0",
        "not a coordinate",
    ],
)
maven.artifact(group = "com.squareup.okhttp3", artifact = "okhttp", version = "4.12.0")
`)
	write(t, root, "maven_install.json", `{"artifacts": {"androidx.compose.runtime:runtime": {"version": "1.7.0"}}}`)
	write(t, root, "app/BUILD.bazel", `
load("@rules_android//android:rules.bzl", "android_binary")
android_binary(
    name = "app",
    manifest = "AndroidManifest.xml",
    resource_files = glob(["res/**"]),
    deps = ["//lib:lib", "//lib/missing", "@maven//:androidx_room_room_runtime"],
)
kt_jvm_test(name = "app_test", srcs = ["AppTest.kt"], deps = [":app", "//testing"])
`)
	write(t, root, "app/res/values/strings.xml", "<resources/>")
	write(t, root, "lib/BUILD", `kt_android_library(name = "lib", srcs = glob(["**/*.kt"]))`)
	write(t, root, "docs/BUILD", `filegroup(name = "docs", srcs = glob(["*.md"]))`)
	write(t, root, "bazel-out/k8/bin/BUILD", `kt_jvm_library(name = "generated", srcs = ["X.kt"])`)
	// A nested workspace resolves // labels against its own root.
	write(t, root, "examples/sample/WORKSPACE", ``)
	write(t, root, "examples/sample/app/BUILD", `kt_jvm_library(name = "s", srcs = ["S.kt"], deps = ["//core"])`)

	ws := Discover(root)
	if ws.IsEmpty() {
		t.Fatal("empty workspace")
	}
	byName := map[string]Target{}
	for _, tg := range ws.Targets {
		byName[tg.Name] = tg
	}
	if _, ok := byName["docs"]; ok {
		t.Error("filegroup without JVM sources counted as a target")
	}
	if _, ok := byName["generated"]; ok {
		t.Error("bazel-out was scanned")
	}
	app := byName["app"]
	if app.Package != "app" || app.Test {
		t.Errorf("app = %+v", app)
	}
	if want := []string{"lib", "lib/missing"}; !reflect.DeepEqual(app.Deps, want) {
		t.Errorf("app deps = %v, want %v", app.Deps, want)
	}
	if want := []string{"app/res"}; !reflect.DeepEqual(app.ResourceDirs, want) {
		t.Errorf("resource dirs = %v, want %v", app.ResourceDirs, want)
	}
	if test := byName["app_test"]; !test.Test || !reflect.DeepEqual(test.Deps, []string{"testing"}) {
		t.Errorf("app_test = %+v", test)
	}
	if s := byName["s"]; !reflect.DeepEqual(s.Deps, []string{"examples/sample/core"}) {
		t.Errorf("nested workspace deps = %v", s.Deps)
	}

	var coords []string
	for _, a := range ws.Artifacts {
		coords = append(coords, a.Group+":"+a.Name+":"+a.Version)
	}
	sort.Strings(coords)
	want := []string{
		"androidx.compose.runtime:runtime:",
		"androidx.room:room-runtime:2.6.1",
		"com.squareup.okhttp3:okhttp:4.12.0",
		"org.jetbrains.kotlinx:kotlinx-coroutines-core:1.9.0",
	}
	if !reflect.DeepEqual(coords, want) {
		t.Errorf("artifacts = %v, want %v", coords, want)
	}
}

func TestArtifactsFromWorkspaceMavenInstall(t *testing.T) {
	root := t.TempDir()
	f := write(t, root, "WORKSPACE", `
load("@rules_jvm_external//:defs.bzl", "maven_install")
ANDROIDX = ["androidx.core:core-ktx:1.13.1"]
maven_install(
    artifacts = ANDROIDX + [
        "junit:junit:4.13.2",
        "com.google.guava:guava:jar:33.0.0-android",
    ],
    repositories = ["https://maven.google.com"],
)
`)
	var coords []string
	for _, a := range ArtifactsFromWorkspaceFile(f) {
		coords = append(coords, a.Group+":"+a.Name+":"+a.Version)
	}
	want := []string{"androidx.core:core-ktx:1.13.1", "junit:junit:4.13.2", "com.google.guava:guava:33.0.0-android"}
	if !reflect.DeepEqual(coords, want) {
		t.Errorf("artifacts = %v, want %v", coords, want)
	}
}

func TestDiscoverNonBazelTreeIsEmpty(t *testing.T) {
	root := t.TempDir()
	write(t, root, "settings.gradle.kts", `include(":app")`)
	if ws := Discover(root); !ws.IsEmpty() || len(ws.Targets) != 0 {
		t.Errorf("workspace = %+v, want empty", ws)
	}
}
