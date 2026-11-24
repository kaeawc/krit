package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

const tag = "v1.2.3"

func jarBytes(t *testing.T, withManifest bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	name := "dev/Main.class"
	if withManifest {
		name = "META-INF/MANIFEST.MF"
	}
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("Manifest-Version: 1.0\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// releaseDir writes a dist directory holding jar and bazelArchive (each when
// non-nil) and sums.
func releaseDir(t *testing.T, jar, bazelArchive []byte, sums string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string][]byte{firJarName(tag): jar, bazelArchiveName(tag): bazelArchive} {
		if data == nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func sumLine(data []byte, name string) string {
	return fmt.Sprintf("%x  %s\n", sha256.Sum256(data), name)
}

func TestCheckDir(t *testing.T) {
	jar := jarBytes(t, true)
	archive := []byte("archive")
	bazel := tarGz(t, map[string]string{bazelModuleFile(tag): "module(name = \"krit\")\n"})
	wrongPrefix := tarGz(t, map[string]string{"krit-v1.2.3/bazel/MODULE.bazel": "module(name = \"krit\")\n"})
	jarSum := sumLine(jar, firJarName(tag))
	bazelSum := sumLine(bazel, bazelArchiveName(tag))
	good := sumLine(archive, "krit_1.2.3_linux_amd64.tar.gz") + jarSum + bazelSum

	cases := []struct {
		name    string
		jar     []byte
		bazel   []byte
		sums    string
		wantErr string
	}{
		{"valid", jar, bazel, good, ""},
		{"missing jar", nil, bazel, good, "missing krit-fir-v1.2.3.jar"},
		{"not a jar", []byte("not a zip"), bazel, sumLine([]byte("not a zip"), firJarName(tag)) + bazelSum, "not a jar"},
		{"no manifest", jarBytes(t, false), bazel, sumLine(jarBytes(t, false), firJarName(tag)) + bazelSum, "MANIFEST"},
		{"jar not in checksums", jar, bazel, sumLine(archive, "krit_1.2.3_linux_amd64.tar.gz") + bazelSum, "no line for krit-fir-v1.2.3.jar"},
		{"checksum mismatch", jar, bazel, sumLine(archive, firJarName(tag)) + bazelSum, "hashes to"},
		{"missing bazel archive", jar, nil, good, "missing krit-bazel-v1.2.3.tar.gz"},
		{"bazel archive not gzip", jar, []byte("plain"), jarSum + sumLine([]byte("plain"), bazelArchiveName(tag)), "not a gzip archive"},
		{"bazel archive without module", jar, wrongPrefix, jarSum + sumLine(wrongPrefix, bazelArchiveName(tag)), "no krit-bazel-v1.2.3/bazel/MODULE.bazel"},
		{"bazel archive not in checksums", jar, bazel, jarSum, "no line for krit-bazel-v1.2.3.tar.gz"},
		{"bazel archive checksum mismatch", jar, bazel, jarSum + sumLine(archive, bazelArchiveName(tag)), "hashes to"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkDir(releaseDir(t, c.jar, c.bazel, c.sums), tag)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDir: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("checkDir error = %v, want one containing %q", err, c.wantErr)
			}
		})
	}
}

func TestCheckPublished(t *testing.T) {
	all := "krit_1.2.3_linux_amd64.tar.gz\nkrit-fir-v1.2.3.jar\nkrit-bazel-v1.2.3.tar.gz\nchecksums.txt\nchecksums.txt.sig\n"
	if err := checkPublished(strings.NewReader(all), tag, nil); err != nil {
		t.Fatalf("complete release: %v", err)
	}
	for _, asset := range []string{"krit-fir-v1.2.3.jar", "krit-bazel-v1.2.3.tar.gz"} {
		without := strings.Replace(all, asset+"\n", "", 1)
		if err := checkPublished(strings.NewReader(without), tag, nil); err == nil || !strings.Contains(err.Error(), asset) {
			t.Fatalf("release without %s: error = %v", asset, err)
		}
	}
	// Every uploaded name must be present, and a duplicated expected name is
	// reported once.
	expected := []string{"krit_1.2.3_linux_amd64.tar.gz", "krit_1.2.3_windows_amd64.zip", "krit_1.2.3_windows_amd64.zip"}
	err := checkPublished(strings.NewReader(all), tag, expected)
	if err == nil || !strings.HasSuffix(err.Error(), "missing krit_1.2.3_windows_amd64.zip") {
		t.Fatalf("release without an uploaded archive: error = %v", err)
	}
}

// The checked name must be the one released CLIs download.
func TestJarNameMatchesDownloadedAsset(t *testing.T) {
	if got, want := firJarName(tag), oracle.BackendFIR.ReleaseAssetName(tag); got != want {
		t.Fatalf("releaseassets checks %q, CLIs download %q", got, want)
	}
}
