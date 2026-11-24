package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The package-manager scripts in scripts/release render the Homebrew cask and
// Scoop manifest that release.yml pushes to kaeawc/homebrew-tap and
// kaeawc/scoop-krit. testdata/v0.2.0 holds the files those repositories carry
// for v0.2.0 (the cask passes the tap's `brew style` audit) and the v0.2.0
// checksums.txt, so rendering v0.2.0 must reproduce them byte for byte.

func renderPackageManifest(t *testing.T, script, tag, distDir string) (string, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("release scripts run on the ubuntu release runner")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "release", script))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, scriptPath)
	// The scripts read dist/checksums.txt relative to the working directory.
	cmd.Dir = filepath.Dir(distDir)
	cmd.Env = append(os.Environ(), "TAG="+tag, "REPO=kaeawc/krit", "RENDER_ONLY=1", "GH_TOKEN=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

func TestPackageManifestsMatchPublishedV020(t *testing.T) {
	for _, tc := range []struct {
		script, golden string
	}{
		{"update-brew-tap.sh", "krit.rb"},
		{"update-scoop-bucket.sh", "krit.json"},
	} {
		t.Run(tc.script, func(t *testing.T) {
			got, err := renderPackageManifest(t, tc.script, "v0.2.0", filepath.Join("testdata", "v0.2.0", "dist"))
			if err != nil {
				t.Fatalf("render failed: %v\n%s", err, got)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "v0.2.0", tc.golden))
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("rendered %s differs from the published v0.2.0 file\n--- got\n%s\n--- want\n%s", tc.golden, got, want)
			}
		})
	}
}

func TestPackageManifestsFailWithoutChecksum(t *testing.T) {
	dist := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only the musl archive is listed: every published platform is missing.
	line := "9f5a9abf83455438ebc00b841dc4a661d009efc2e4976adea79caf779a86d084  krit_0.4.0_linux_musl_amd64.tar.gz\n"
	if err := os.WriteFile(filepath.Join(dist, "checksums.txt"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"update-brew-tap.sh", "update-scoop-bucket.sh"} {
		t.Run(script, func(t *testing.T) {
			out, err := renderPackageManifest(t, script, "v0.4.0", dist)
			if err == nil {
				t.Fatalf("render succeeded without checksums:\n%s", out)
			}
			if !strings.Contains(out, "no checksum for krit_0.4.0_") {
				t.Errorf("stderr = %q, want a missing-checksum error", out)
			}
		})
	}
}
