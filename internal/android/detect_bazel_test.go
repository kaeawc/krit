package android

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Bazel Android targets keep resources wherever their BUILD file's
// resource_files glob points (java/com/app/res, module/res), and the
// project's library inputs include MODULE.bazel and lock files but not
// BUILD files.
func TestDetectProjectBazelResourceDirsAndLibraryInputs(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("MODULE.bazel", `bazel_dep(name = "rules_android", version = "0.6.0")`)
	write("maven_install.json", `{"artifacts": {}}`)
	write("java/com/basicapp/BUILD", `android_library(name = "lib", manifest = "AndroidManifest.xml", resource_files = glob(["res/**"]))`)
	write("java/com/basicapp/AndroidManifest.xml", `<manifest package="com.basicapp"/>`)
	write("java/com/basicapp/res/values/strings.xml", `<resources/>`)
	write("libKt/BUILD.bazel", `kt_android_library(name = "kt", resource_files = glob(["res/**/*.xml"]) + ["extra/x.xml"])`)
	write("libKt/res/layout/main.xml", `<FrameLayout/>`)

	proj := DetectProjectWithIndex([]string{root}, nil)
	want := []string{filepath.Join(root, "java/com/basicapp/res"), filepath.Join(root, "libKt/res")}
	for _, dir := range want {
		if !slices.Contains(proj.ResDirs, dir) {
			t.Errorf("ResDirs = %v, missing %s", proj.ResDirs, dir)
		}
	}
	if len(proj.ManifestPaths) != 1 {
		t.Errorf("ManifestPaths = %v", proj.ManifestPaths)
	}
	inputs := proj.LibraryInputPaths()
	for _, base := range []string{"MODULE.bazel", "maven_install.json"} {
		if !slices.Contains(inputs, filepath.Join(root, base)) {
			t.Errorf("LibraryInputPaths = %v, missing %s", inputs, base)
		}
	}
	for _, in := range inputs {
		if filepath.Base(in) == "BUILD" || filepath.Base(in) == "BUILD.bazel" {
			t.Errorf("LibraryInputPaths includes BUILD file %s", in)
		}
	}
}
