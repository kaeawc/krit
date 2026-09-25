package oracle

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureJar_DevBuildReturnsHelpfulError(t *testing.T) {
	isolateJarLookup(t)
	// Version stays "" → versionTag() returns "" → no release URL → error.
	_, err := EnsureJar(context.Background(), nil, false)
	if err == nil {
		t.Fatal("expected error for dev build with no jar")
	}
	msg := err.Error()
	for _, want := range []string{"krit-types.jar not found", "KRIT_TYPES_JAR", "./gradlew shadowJar"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}
	if !errors.Is(err, ErrJarDownloadSkipped) {
		t.Errorf("dev build error should wrap ErrJarDownloadSkipped: %v", err)
	}
}

func TestEnsureBackendJar_FIRDevBuildNamesFIRRecovery(t *testing.T) {
	isolateJarLookup(t)
	_, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false)
	if err == nil {
		t.Fatal("expected error for dev build with no krit-fir jar")
	}
	for _, want := range []string{"krit-fir.jar not found", "KRIT_FIR_JAR", "tools/krit-fir"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q: %v", want, err)
		}
	}
}

func TestJarReleaseURL_NormalisesVersion(t *testing.T) {
	prev := Version
	t.Cleanup(func() { Version = prev })

	Version = "1.2.3"
	if got, want := jarReleaseURL(BackendKAA), "https://github.com/kaeawc/krit/releases/download/v1.2.3/krit-types-v1.2.3.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := jarReleaseURL(BackendFIR), "https://github.com/kaeawc/krit/releases/download/v1.2.3/krit-fir-v1.2.3.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	Version = "v1.2.3"
	if got, want := jarReleaseURL(BackendKAA), "https://github.com/kaeawc/krit/releases/download/v1.2.3/krit-types-v1.2.3.jar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	Version = "dev"
	if got := jarReleaseURL(BackendKAA); got != "" {
		t.Errorf("dev build should not have a release URL, got %q", got)
	}
	Version = ""
	if got := jarReleaseURL(BackendKAA); got != "" {
		t.Errorf("empty version should not have a release URL, got %q", got)
	}
}

func TestFindBackendJar_FIREnvOverride(t *testing.T) {
	isolateJarLookup(t)
	jarPath := filepath.Join(t.TempDir(), "custom-fir.jar")
	if err := os.WriteFile(jarPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jarPath)
	if got := FindBackendJar(BackendFIR, nil); got != jarPath {
		t.Errorf("KRIT_FIR_JAR override: got %q, want %q", got, jarPath)
	}
	// The FIR override never satisfies a krit-types lookup.
	if got := FindJar(nil); got != "" {
		t.Errorf("KRIT_FIR_JAR leaked into the krit-types lookup: %q", got)
	}
}

func TestFindBackendJar_FIRInstalledUnderKritJars(t *testing.T) {
	home := isolateJarLookup(t)
	Version = "1.2.3"
	pinned := filepath.Join(home, ".krit", "jars", "krit-fir-v1.2.3.jar")
	if err := os.MkdirAll(filepath.Dir(pinned), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pinned, []byte("pin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindBackendJar(BackendFIR, nil); got != pinned {
		t.Errorf("got %q, want %q", got, pinned)
	}
}

func TestFindBackendJar_FIRInProjectDir(t *testing.T) {
	isolateJarLookup(t)
	project := t.TempDir()
	jarPath := filepath.Join(project, ".krit", "krit-fir.jar")
	if err := os.MkdirAll(filepath.Dir(jarPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jarPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindBackendJar(BackendFIR, []string{project}); got != jarPath {
		t.Errorf("got %q, want %q", got, jarPath)
	}
}

// releaseServer serves a fake GitHub release: tag v1.2.3 with the given
// assets and a checksums.txt body. It repoints downloads at itself and
// reports java as available.
func releaseServer(t *testing.T, assets map[string][]byte, checksums string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/v1.2.3/")
		if name == "checksums.txt" && checksums != "" {
			_, _ = w.Write([]byte(checksums))
			return
		}
		if body, ok := assets[name]; ok {
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	prevBase, prevJava := releaseDownloadBase, javaAvailable
	releaseDownloadBase = srv.URL
	javaAvailable = func() bool { return true }
	t.Cleanup(func() { releaseDownloadBase, javaAvailable = prevBase, prevJava })
	Version = "1.2.3"
}

func checksumLine(name string, body []byte) string {
	return fmt.Sprintf("%x  %s\n", sha256.Sum256(body), name)
}

func TestEnsureBackendJar_DownloadsAndVerifiesFIRJar(t *testing.T) {
	home := isolateJarLookup(t)
	jar := []byte("krit-fir jar bytes")
	releaseServer(t, map[string][]byte{"krit-fir-v1.2.3.jar": jar},
		checksumLine("krit_1.2.3_darwin_arm64.tar.gz", []byte("x"))+checksumLine("krit-fir-v1.2.3.jar", jar))

	got, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false)
	if err != nil {
		t.Fatalf("EnsureBackendJar: %v", err)
	}
	want := filepath.Join(home, ".krit", "jars", "krit-fir-v1.2.3.jar")
	if got != want {
		t.Errorf("installed at %q, want %q", got, want)
	}
	if data, err := os.ReadFile(got); err != nil || string(data) != string(jar) {
		t.Errorf("installed jar content = %q, %v", data, err)
	}
	// A second call finds the installed jar without the server.
	releaseDownloadBase = "http://127.0.0.1:0"
	if again, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false); err != nil || again != want {
		t.Errorf("second EnsureBackendJar = %q, %v", again, err)
	}
}

func TestEnsureBackendJar_ChecksumMismatchInstallsNothing(t *testing.T) {
	home := isolateJarLookup(t)
	releaseServer(t, map[string][]byte{"krit-fir-v1.2.3.jar": []byte("tampered")},
		checksumLine("krit-fir-v1.2.3.jar", []byte("genuine")))

	_, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if errors.Is(err, ErrJarDownloadSkipped) {
		t.Errorf("a failed download is not a skip: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(home, ".krit", "jars"))
	if len(entries) != 0 {
		t.Errorf("mismatched download left files behind: %v", entries)
	}
}

func TestEnsureBackendJar_RequiresChecksumEntry(t *testing.T) {
	isolateJarLookup(t)
	jar := []byte("jar")
	releaseServer(t, map[string][]byte{"krit-fir-v1.2.3.jar": jar}, checksumLine("krit-types-v1.2.3.jar", jar))
	if _, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false); err == nil || !strings.Contains(err.Error(), "no checksum listed") {
		t.Fatalf("expected missing-checksum error, got %v", err)
	}
}

func TestEnsureBackendJar_MissingChecksumsFileFails(t *testing.T) {
	isolateJarLookup(t)
	releaseServer(t, map[string][]byte{"krit-fir-v1.2.3.jar": []byte("jar")}, "")
	if _, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false); err == nil || !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("expected checksums.txt fetch error, got %v", err)
	}
}

func TestEnsureBackendJar_SkipsDownload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T)
		want  string
	}{
		{"opt-out env", func(t *testing.T) { t.Setenv(NoJarDownloadEnv, "1") }, NoJarDownloadEnv},
		{"no java", func(t *testing.T) { javaAvailable = func() bool { return false } }, "java not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateJarLookup(t)
			requested := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested = true
				http.NotFound(w, r)
			}))
			t.Cleanup(srv.Close)
			releaseServer(t, nil, "")
			releaseDownloadBase = srv.URL
			tc.setup(t)

			_, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false)
			if !errors.Is(err, ErrJarDownloadSkipped) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected skipped error mentioning %q, got %v", tc.want, err)
			}
			if requested {
				t.Error("a skipped download must not hit the network")
			}
		})
	}
}

func TestResolveOracleJar_FallsBackToInstalledKAAJar(t *testing.T) {
	isolateJarLookup(t)
	releaseServer(t, nil, "") // krit-fir asset unavailable
	kaa := filepath.Join(t.TempDir(), "krit-types.jar")
	if err := os.WriteFile(kaa, []byte("kaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_TYPES_JAR", kaa)

	jarPath, used, warning, err := ResolveOracleJar(context.Background(), BackendFIR, nil, false)
	if err != nil {
		t.Fatalf("ResolveOracleJar: %v", err)
	}
	if jarPath != kaa || used != BackendKAA {
		t.Errorf("got (%q, %s), want (%q, kaa)", jarPath, used, kaa)
	}
	if !strings.Contains(warning, "krit-fir.jar") || !strings.Contains(warning, "KAA") {
		t.Errorf("warning should name the missing jar and the fallback: %q", warning)
	}
}

func TestResolveOracleJar_DevBuildNeverFallsBack(t *testing.T) {
	isolateJarLookup(t)
	kaa := filepath.Join(t.TempDir(), "krit-types.jar")
	if err := os.WriteFile(kaa, []byte("kaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_TYPES_JAR", kaa)

	jarPath, used, warning, err := ResolveOracleJar(context.Background(), BackendFIR, nil, false)
	if jarPath != "" || used != BackendFIR || warning != "" {
		t.Errorf("dev build fell back: (%q, %s, %q)", jarPath, used, warning)
	}
	if !errors.Is(err, ErrJarDownloadSkipped) {
		t.Errorf("expected a skipped error, got %v", err)
	}
}

func TestParseChecksum(t *testing.T) {
	sum := sha256.Sum256([]byte("x"))
	content := []byte(fmt.Sprintf("%x  other.jar\n%x *krit-fir-v1.jar\n", sha256.Sum256([]byte("y")), sum))
	got, err := parseChecksum(content, "krit-fir-v1.jar")
	if err != nil || string(got) != string(sum[:]) {
		t.Errorf("binary-mode entry: got %x, %v", got, err)
	}
	if _, err := parseChecksum([]byte("zz  a.jar\n"), "a.jar"); err == nil {
		t.Error("expected malformed checksum error")
	}
	if _, err := parseChecksum(content, "missing.jar"); err == nil {
		t.Error("expected missing entry error")
	}
}

func TestVersionTag_OnlyTaggedReleases(t *testing.T) {
	prev := Version
	t.Cleanup(func() { Version = prev })
	for _, tc := range []struct{ version, want string }{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"1.5.0-nightly.20260516", "v1.5.0-nightly.20260516"},
		{"v1.5.0-rc1", "v1.5.0-rc1"},
		// `make build` stamps `git describe --tags --always --dirty`; none of
		// these have release assets to download.
		{"v1.4.2-12-gabc1234", ""},
		{"v1.4.2-12-gabc1234-dirty", ""},
		{"v1.4.2-dirty", ""},
		{"abc1234", ""},
		{"abc1234-dirty", ""},
		{"dev", ""},
		{"", ""},
	} {
		Version = tc.version
		if got := versionTag(); got != tc.want {
			t.Errorf("versionTag(%q) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

func TestEnsureBackendJar_FailedDownloadNotRetriedInProcess(t *testing.T) {
	isolateJarLookup(t)
	releaseServer(t, nil, "")
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	releaseDownloadBase = srv.URL

	for i := 0; i < 3; i++ {
		if _, err := EnsureBackendJar(context.Background(), BackendFIR, nil, false); err == nil {
			t.Fatal("expected download failure")
		}
	}
	if requests != 1 {
		t.Errorf("expected one download attempt, got %d requests", requests)
	}
}
