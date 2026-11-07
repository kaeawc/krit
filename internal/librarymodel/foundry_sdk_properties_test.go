package librarymodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileFromGradlePathsReadsFoundrySdkProperties(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) string {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("gradle.properties", "org.gradle.jvmargs=-Xmx1g\nfoundry.android.minSdkVersion=29\nfoundry.android.compileSdkVersion=android-35\nfoundry.android.targetSdkVersion=34\n")
	module := write("feature/build.gradle.kts", "plugins { id(\"com.android.library\") }\n")
	profile := ProfileFromGradlePaths([]string{module})
	if profile.MinSdkVersion != 29 || profile.TargetSdkVersion != 34 || profile.CompileSdkVersion != 35 {
		t.Fatalf("want min 29, target 34, compile 35, got %d/%d/%d", profile.MinSdkVersion, profile.TargetSdkVersion, profile.CompileSdkVersion)
	}
	declared := write("other/build.gradle.kts", "android {\n  defaultConfig {\n    minSdk = 24\n  }\n}\n")
	profile = ProfileFromGradlePaths([]string{module, declared})
	if profile.MinSdkVersion != 24 {
		t.Errorf("a declared minSdk must win over the property, got %d", profile.MinSdkVersion)
	}
}
