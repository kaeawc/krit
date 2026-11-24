package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The root action.yml (uses: kaeawc/krit@vX.Y.Z) installs releases with
// scripts/action/install-krit.sh and runs krit with scripts/action/run-krit.sh.
// These tests put a fake curl (serving files from a directory) and a fake
// krit on PATH, so they need no network.

const fakeCurl = `#!/usr/bin/env bash
out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -H|--retry) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
src="$FAKE_RELEASE_DIR/$(basename "$url")"
[ -f "$src" ] || exit 22
cp "$src" "$out"
`

func actionScript(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the action scripts are exercised by the action job on Windows runners")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "scripts", "action", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestActionInstallVerifiesChecksumsFailClosed(t *testing.T) {
	script := actionScript(t, "install-krit.sh")
	const asset = "krit_1.2.3_linux_amd64.tar.gz"
	archive := tarGz(t, map[string]string{"krit": "#!/bin/sh\necho krit 1.2.3\n"})
	digest := sha256.Sum256(archive)
	good := hex.EncodeToString(digest[:])
	bad := strings.Repeat("0", 64)

	for _, tc := range []struct {
		name, checksums, wantErr string
	}{
		{"verified", good + "  " + asset + "\n", ""},
		{"digest mismatch", bad + "  " + asset + "\n", "SHA-256 mismatch"},
		{"archive not listed", good + "  krit_1.2.3_darwin_arm64.tar.gz\n", "does not list " + asset},
		{"no checksums.txt", "", "Could not download checksums.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			release, bin, dest := filepath.Join(root, "release"), filepath.Join(root, "bin"), filepath.Join(root, "krit")
			for _, dir := range []string{release, bin} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeExecutable(t, filepath.Join(bin, "curl"), fakeCurl)
			if err := os.WriteFile(filepath.Join(release, asset), archive, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.checksums != "" {
				if err := os.WriteFile(filepath.Join(release, "checksums.txt"), []byte(tc.checksums), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", script, "install")
			cmd.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FAKE_RELEASE_DIR="+release, "KRIT_TAG=v1.2.3", "KRIT_ASSET="+asset, "KRIT_DIR="+dest)
			out, err := cmd.CombinedOutput()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("install failed: %v\n%s", err, out)
				}
				if _, err := os.Stat(filepath.Join(dest, "krit")); err != nil {
					t.Fatalf("krit not installed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(string(out), tc.wantErr) {
				t.Fatalf("err=%v output=%s, want failure containing %q", err, out, tc.wantErr)
			}
			if _, statErr := os.Stat(filepath.Join(dest, "krit")); statErr == nil {
				t.Fatal("an unverified archive was installed")
			}
		})
	}
}

func TestActionRunCountsSARIFResults(t *testing.T) {
	script := actionScript(t, "run-krit.sh")
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not available")
	}
	sarif := `{"runs":[{"results":[{"ruleId":"A"},{"ruleId":"B"}]},{"results":[]}]}`
	for _, tc := range []struct {
		name         string
		kritExit     int
		failOn       string
		wantExit     int
		wantCount    string
		wantArgsFrom string
	}{
		{"findings fail the step", 1, "true", 1, "findings-count=2", "--baseline base.xml"},
		{"findings tolerated", 1, "false", 0, "findings-count=2", "--config krit.yml"},
		{"krit error", 2, "false", 2, "findings-count=2", "--report=sarif"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			argsFile := filepath.Join(root, "args")
			writeExecutable(t, filepath.Join(bin, "krit"), fmt.Sprintf(`#!/usr/bin/env bash
echo "$@" > %q
while [ $# -gt 0 ]; do [ "$1" = -o ] && out="$2"; shift; done
printf '%%s' %q > "$out"
exit %d
`, argsFile, sarif, tc.kritExit))
			output := filepath.Join(root, "github_output")
			cmd := exec.Command("bash", script)
			cmd.Dir = root
			cmd.Env = append(os.Environ(),
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"GITHUB_OUTPUT="+output, "GITHUB_WORKSPACE="+root,
				"KRIT_SARIF=out/krit.sarif", "KRIT_CONFIG=krit.yml", "KRIT_BASELINE=base.xml",
				"KRIT_PATHS=app lib", "KRIT_FAIL_ON_FINDINGS="+tc.failOn)
			out, err := cmd.CombinedOutput()
			exit := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exit = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if exit != tc.wantExit {
				t.Fatalf("exit=%d, want %d\n%s", exit, tc.wantExit, out)
			}
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.wantCount) || !strings.Contains(string(got), "sarif-file=out/krit.sarif") {
				t.Fatalf("outputs = %q", got)
			}
			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), tc.wantArgsFrom) || !strings.Contains(string(args), "--base-path "+root) || !strings.HasSuffix(strings.TrimSpace(string(args)), "app lib") {
				t.Fatalf("krit args = %q", args)
			}
		})
	}
}
