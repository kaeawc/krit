package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func writeModuleFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestManifestGradleInfoResolvesVersionCatalogAliases(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "gradle/libs.versions.toml", `
[versions]
agp = "9.0.0"

[plugins]
android-library = { id = "com.android.library", version.ref = "agp" }
android-application = { id = "com.android.application", version.ref = "agp" }
kotlin-android = { id = "org.jetbrains.kotlin.android", version = "2.0.0" }

[libraries]
junit = { module = "junit:junit", version = "4.13" }
`)
	writeModuleFile(t, root, "lib/build.gradle.kts", "plugins {\n  alias(libs.plugins.android.library)\n  alias(libs.plugins.kotlin.android)\n}\n")
	writeModuleFile(t, root, "lib/src/main/AndroidManifest.xml", "<manifest/>")
	writeModuleFile(t, root, "app/build.gradle.kts", "plugins {\n  alias(libs.plugins.android.application)\n}\n")
	writeModuleFile(t, root, "app/src/main/AndroidManifest.xml", "<manifest/>")

	if !isLibraryOrTestModuleManifest(filepath.Join(root, "lib/src/main/AndroidManifest.xml")) {
		t.Error("a module applying alias(libs.plugins.android.library) should be a library module")
	}
	if isLibraryOrTestModuleManifest(filepath.Join(root, "app/src/main/AndroidManifest.xml")) {
		t.Error("a module applying alias(libs.plugins.android.application) should not be a library module")
	}
}
