package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// #1662: module SDK levels are keyed by module directories relative to the
// project, so the same checkout at another absolute path keeps the rule hash.
func TestSDKLevelsFingerprintIsRelocatable(t *testing.T) {
	base := t.TempDir()
	fp := func(root string) string {
		gradle := filepath.Join(root, "app", "build.gradle.kts")
		if err := os.MkdirAll(filepath.Dir(gradle), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(gradle, []byte("android { defaultConfig { minSdk = 21\n targetSdk = 34 } }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lib := filepath.Join(root, "lib", "build.gradle.kts")
		if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lib, []byte("android { defaultConfig { minSdk = 23 } }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return sdkLevelsFingerprint(&android.Project{GradlePaths: []string{gradle, lib}})
	}
	a, b := fp(filepath.Join(base, "agent-1", "repo")), fp(filepath.Join(base, "agent-2", "repo"))
	if a == "" || a != b {
		t.Fatalf("sdk fingerprints at two roots: %q vs %q", a, b)
	}
}
