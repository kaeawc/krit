package oracle

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Version is the krit CLI semantic version, set by cmd/krit/main.go from the
// same goreleaser ldflag that drives scan.Version and serve.Version. EnsureJar
// uses it to pick the matching <jar>-<version>.jar release asset and the
// per-version cache file under ~/.krit/jars/.
//
// Empty or "dev" disables auto-download: developer builds compile the jars
// in-tree via `./gradlew shadowJar` under tools/krit-types or tools/krit-fir.
var Version = ""

// NoJarDownloadEnv disables jar auto-download when set to "1". Air-gapped
// CI can pre-seed ~/.krit/jars or set KRIT_TYPES_JAR / KRIT_FIR_JAR instead.
const NoJarDownloadEnv = "KRIT_NO_JAR_DOWNLOAD"

// releaseDownloadBase is the GitHub release download root. A package var so
// tests can point downloads at an httptest server.
var releaseDownloadBase = "https://github.com/kaeawc/krit/releases/download"

// javaAvailable reports whether a java launcher is on PATH. A package var so
// download tests don't depend on the host having a JDK.
var javaAvailable = func() bool {
	_, err := exec.LookPath("java")
	return err == nil
}

// jarDownloadTimeout caps one download attempt: the checksums.txt fetch plus
// the jar. The shaded jars are tens of megabytes, well within reach over a
// slow connection in 5 minutes.
const jarDownloadTimeout = 5 * time.Minute

var ensureJarMu sync.Mutex

// ErrJarDownloadSkipped marks a missing jar that auto-download deliberately
// did not fetch: a dev build, KRIT_NO_JAR_DOWNLOAD=1, or no java on PATH.
// Nothing failed, so callers log it verbosely rather than warning.
var ErrJarDownloadSkipped = errors.New("auto-download skipped")

func downloadSkipped(reason string) error {
	return fmt.Errorf("%w: %s", ErrJarDownloadSkipped, reason)
}

// jarBaseName is the jar's file stem: "krit-types" for KAA, "krit-fir" for
// FIR. The release asset, install cache file, and in-tree build output all
// derive from it.
func (b Backend) jarBaseName() string {
	return strings.TrimSuffix(b.JarName(), ".jar")
}

// JarEnvVar names the environment variable that overrides the backend's jar
// location: KRIT_TYPES_JAR for KAA, KRIT_FIR_JAR for FIR.
func (b Backend) JarEnvVar() string {
	if b == BackendFIR {
		return "KRIT_FIR_JAR"
	}
	return "KRIT_TYPES_JAR"
}

// FindBackendJar locates the backend's shadow jar without downloading.
// Checked in order:
//  1. The backend's env override (KRIT_TYPES_JAR / KRIT_FIR_JAR) when the
//     file exists
//  2. Installed jars under ~/.krit/jars/ — version-pinned then unversioned
//  3. ~/.krit/<jar> (legacy, pre-#300 install location)
//  4. Next to the krit binary or under exe-dir/tools/<jar>/build/libs/
//  5. In the project being scanned (.krit/<jar> or
//     tools/<jar>/build/libs/<jar>)
//  6. Under the current working directory's tools/<jar>/build/libs/
//
// Returns "" when no jar is found. Use EnsureBackendJar instead when the
// caller should auto-download a missing jar for the current krit release.
func FindBackendJar(b Backend, scanPaths []string) string {
	if v := strings.TrimSpace(os.Getenv(b.JarEnvVar())); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}

	base := b.jarBaseName()
	name := b.JarName()
	buildRel := filepath.Join("tools", base, "build", "libs", name)
	candidates := []string{}

	// Installed locations under ~/.krit. krit binary releases download
	// the matching jar here on first use; see EnsureBackendJar.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		jarsDir := filepath.Join(home, ".krit", "jars")
		if tag := versionTag(); tag != "" {
			candidates = append(candidates, filepath.Join(jarsDir, base+"-"+tag+".jar"))
		}
		candidates = append(candidates,
			filepath.Join(jarsDir, name),
			filepath.Join(home, ".krit", name),
		)
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, name),
			filepath.Join(exeDir, buildRel),
			filepath.Join(exeDir, "..", buildRel),
		)
	}

	if len(scanPaths) > 0 {
		projectDir := scanPaths[0]
		if fi, err := os.Stat(projectDir); err == nil && !fi.IsDir() {
			projectDir = filepath.Dir(projectDir)
		}
		candidates = append(candidates,
			filepath.Join(projectDir, ".krit", name),
			filepath.Join(projectDir, buildRel),
		)
	}

	cwd, _ := os.Getwd()
	candidates = append(candidates,
		filepath.Join(cwd, buildRel),
		filepath.Join(cwd, name),
	)

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// EnsureJar returns the krit-types jar path, downloading the matching release
// asset when needed. See EnsureBackendJar.
func EnsureJar(ctx context.Context, scanPaths []string, verbose bool) (string, error) {
	return EnsureBackendJar(ctx, BackendKAA, scanPaths, verbose)
}

// EnsureBackendJar returns the backend's jar path, downloading the matching
// release asset to ~/.krit/jars/<jar>-<version>.jar when no jar is found
// locally and the krit binary is a tagged release. Downloads are verified
// against the release's checksums.txt before they are installed.
//
// Returns an actionable error when no jar is locatable and auto-download is
// not available (dev builds, KRIT_NO_JAR_DOWNLOAD=1, no java on PATH, missing
// HOME, no network, or a checksum mismatch).
func EnsureBackendJar(ctx context.Context, b Backend, scanPaths []string, verbose bool) (string, error) {
	if path := FindBackendJar(b, scanPaths); path != "" {
		return path, nil
	}

	ensureJarMu.Lock()
	defer ensureJarMu.Unlock()
	// Re-check under the lock: a concurrent EnsureBackendJar may have
	// downloaded.
	if path := FindBackendJar(b, scanPaths); path != "" {
		return path, nil
	}

	tag := versionTag()
	if tag == "" {
		return "", missingJarError(b, downloadSkipped("dev build"))
	}
	if os.Getenv(NoJarDownloadEnv) == "1" {
		return "", missingJarError(b, downloadSkipped(NoJarDownloadEnv+"=1"))
	}
	// The jar is useless without a JVM; don't spend the download on it.
	if !javaAvailable() {
		return "", missingJarError(b, downloadSkipped("java not found in PATH"))
	}
	target, err := installedJarPath(b)
	if err != nil {
		return "", missingJarError(b, err)
	}
	asset := jarAssetName(b, tag)
	// One attempt per process: a long-lived daemon or LSP server must not
	// re-fetch (and re-warn) on every request after a failure.
	if err, failed := failedDownloads[asset]; failed {
		return "", err
	}
	path, err := downloadReleaseJar(ctx, b, tag, asset, target, verbose)
	if err != nil {
		failedDownloads[asset] = err
		return "", err
	}
	return path, nil
}

// failedDownloads memoizes download failures by asset name for the life of
// the process. Guarded by ensureJarMu.
var failedDownloads = map[string]error{}

func downloadReleaseJar(ctx context.Context, b Backend, tag, asset, target string, verbose bool) (string, error) {
	url := jarReleaseURL(b)
	if verbose {
		reporter().Verbosef("verbose: downloading %s from %s\n", b.JarName(), url)
	}
	// Bounds the checksums fetch and the jar download together; neither
	// the callers' contexts nor http.DefaultClient carry a deadline.
	ctx, cancel := context.WithTimeout(ctx, jarDownloadTimeout)
	defer cancel()
	want, err := releaseChecksum(ctx, tag, asset)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", b.JarName(), err)
	}
	if err := downloadVerified(ctx, url, target, want); err != nil {
		return "", fmt.Errorf("download %s from %s: %w", b.JarName(), url, err)
	}
	return target, nil
}

// ResolveOracleJar returns the jar for the requested oracle backend,
// downloading it on tagged releases (see EnsureBackendJar). When the krit-fir
// jar cannot be had on a tagged release but a krit-types jar is already
// installed, it falls back to the KAA backend and returns a warning saying so;
// callers must use the returned backend, since cache entries are scoped to it.
// Dev builds never fall back: they select the backend whose jar was built.
//
// On failure jarPath is "" and err explains why; err wraps
// ErrJarDownloadSkipped when no download was attempted.
func ResolveOracleJar(ctx context.Context, b Backend, scanPaths []string, verbose bool) (jarPath string, used Backend, warning string, err error) {
	if b == "" {
		b = DefaultBackend
	}
	jarPath, err = EnsureBackendJar(ctx, b, scanPaths, verbose)
	if err == nil || b != BackendFIR || !IsReleaseBuild() {
		return jarPath, b, "", err
	}
	if kaa := FindJar(scanPaths); kaa != "" {
		return kaa, BackendKAA, fmt.Sprintf("%v; falling back to the krit-types (KAA) oracle backend at %s", err, kaa), nil
	}
	return "", b, "", err
}

// IsReleaseBuild reports whether this binary is a tagged release, the only
// kind that auto-downloads missing jars.
func IsReleaseBuild() bool {
	return versionTag() != ""
}

// releaseVersionRE matches the versions goreleaser stamps into tagged
// releases: X.Y.Z with an optional prerelease suffix (-rc1, -nightly.2026…),
// with or without a leading v.
var releaseVersionRE = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

// gitDescribeSuffixRE matches what `git describe --tags --always --dirty`
// (the Makefile's VERSION) appends past a tag: -<commits>-g<sha> and/or
// -dirty. Such builds have no release assets to download.
var gitDescribeSuffixRE = regexp.MustCompile(`(-\d+-g[0-9a-f]+)?(-dirty)?$`)

// versionTag normalises the package-level Version into a release tag form
// ("vX.Y.Z"). Returns "" unless Version is a tagged release: dev builds,
// bare commit hashes, and git-describe versions past a tag or with a dirty
// tree all return "".
func versionTag() string {
	v := strings.TrimSpace(Version)
	if !releaseVersionRE.MatchString(v) || gitDescribeSuffixRE.FindString(v) != "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func installedJarPath(b Backend) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("no HOME directory; cannot install jar under ~/.krit/jars")
	}
	tag := versionTag()
	if tag == "" {
		return filepath.Join(home, ".krit", "jars", b.JarName()), nil
	}
	return filepath.Join(home, ".krit", "jars", jarAssetName(b, tag)), nil
}

// jarAssetName is the release asset filename, e.g. krit-fir-v1.2.3.jar.
func jarAssetName(b Backend, tag string) string {
	return b.jarBaseName() + "-" + tag + ".jar"
}

func jarReleaseURL(b Backend) string {
	tag := versionTag()
	if tag == "" {
		return ""
	}
	return releaseDownloadBase + "/" + tag + "/" + jarAssetName(b, tag)
}

// maxChecksumsSize bounds the checksums.txt read; the real file is a few KB.
const maxChecksumsSize = 1 << 20

// releaseChecksum fetches the release's checksums.txt and returns the
// SHA-256 recorded for asset.
func releaseChecksum(ctx context.Context, tag, asset string) ([]byte, error) {
	url := releaseDownloadBase + "/" + tag + "/checksums.txt"
	resp, err := httpGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumsSize))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	sum, err := parseChecksum(body, asset)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return sum, nil
}

// parseChecksum finds asset in sha256sum-format content ("<hex>  <name>",
// with an optional '*' binary-mode marker before the name).
func parseChecksum(content []byte, asset string) ([]byte, error) {
	s := bufio.NewScanner(bytes.NewReader(content))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != asset {
			continue
		}
		sum, err := hex.DecodeString(fields[0])
		if err != nil || len(sum) != sha256.Size {
			return nil, fmt.Errorf("malformed checksum for %s", asset)
		}
		return sum, nil
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("no checksum listed for %s", asset)
}

// downloadVerified streams url into target, installing it only when its
// SHA-256 matches want. A mismatch leaves no file behind.
func downloadVerified(ctx context.Context, url, target string, want []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	resp, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		h := sha256.New()
		if _, err := io.Copy(io.MultiWriter(w, h), resp.Body); err != nil {
			return err
		}
		if got := h.Sum(nil); !bytes.Equal(got, want) {
			return fmt.Errorf("checksum mismatch: got sha256 %x, want %x", got, want)
		}
		return nil
	})
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return resp, nil
}

// missingJarError builds the helpful error users see when no jar is found and
// auto-download cannot fill the gap. Keeps the install / env-var / dev-build
// options visible together.
func missingJarError(b Backend, reason error) error {
	base := b.jarBaseName()
	return fmt.Errorf("%s not found (%w). Install a tagged krit release to enable auto-download, set %s to an existing jar, or build with: cd tools/%s && ./gradlew shadowJar", b.JarName(), reason, b.JarEnvVar(), base)
}
