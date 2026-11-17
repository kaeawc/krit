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

// releaseDir writes a dist directory holding jar (when non-nil) and sums.
func releaseDir(t *testing.T, jar []byte, sums string) string {
	t.Helper()
	dir := t.TempDir()
	if jar != nil {
		if err := os.WriteFile(filepath.Join(dir, firJarName(tag)), jar, 0o644); err != nil {
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
	good := sumLine(archive, "krit_1.2.3_linux_amd64.tar.gz") + sumLine(jar, firJarName(tag))

	cases := []struct {
		name    string
		jar     []byte
		sums    string
		wantErr string
	}{
		{"valid", jar, good, ""},
		{"missing jar", nil, good, "missing krit-fir-v1.2.3.jar"},
		{"not a jar", []byte("not a zip"), sumLine([]byte("not a zip"), firJarName(tag)), "not a jar"},
		{"no manifest", jarBytes(t, false), sumLine(jarBytes(t, false), firJarName(tag)), "MANIFEST"},
		{"jar not in checksums", jar, sumLine(archive, "krit_1.2.3_linux_amd64.tar.gz"), "no line for krit-fir-v1.2.3.jar"},
		{"checksum mismatch", jar, sumLine(archive, firJarName(tag)), "hashes to"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkDir(releaseDir(t, c.jar, c.sums), tag)
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
	all := "krit_1.2.3_linux_amd64.tar.gz\nkrit-fir-v1.2.3.jar\nchecksums.txt\nchecksums.txt.sig\n"
	if err := checkPublished(strings.NewReader(all), tag); err != nil {
		t.Fatalf("complete release: %v", err)
	}
	withoutJar := strings.Replace(all, "krit-fir-v1.2.3.jar\n", "", 1)
	if err := checkPublished(strings.NewReader(withoutJar), tag); err == nil || !strings.Contains(err.Error(), "krit-fir-v1.2.3.jar") {
		t.Fatalf("release without the jar: error = %v", err)
	}
}

// The checked name must be the one released CLIs download.
func TestJarNameMatchesDownloadedAsset(t *testing.T) {
	if got, want := firJarName(tag), oracle.BackendFIR.ReleaseAssetName(tag); got != want {
		t.Fatalf("releaseassets checks %q, CLIs download %q", got, want)
	}
}
