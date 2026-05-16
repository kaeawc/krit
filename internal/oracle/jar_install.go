package oracle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Version is the krit CLI semantic version, set by cmd/krit/main.go from the
// same goreleaser ldflag that drives scan.Version and serve.Version. EnsureJar
// uses it to pick the matching krit-types-<version>.jar release asset and the
// per-version cache file under ~/.krit/jars/.
//
// Empty or "dev" disables auto-download: developer builds compile the jar
// in-tree via `cd tools/krit-types && ./gradlew shadowJar`.
var Version = ""

// jarDownloadTimeout caps a single jar download. The shaded jar is ~50 MB,
// well within reach over a slow connection in 5 minutes.
const jarDownloadTimeout = 5 * time.Minute

var ensureJarMu sync.Mutex

// EnsureJar returns the krit-types jar path, downloading the matching release
// asset under ~/.krit/jars/krit-types-<version>.jar when no jar is found
// locally and the krit binary is a tagged release.
//
// Returns an actionable error when no jar is locatable and auto-download is
// not available (dev builds, missing HOME, or no network).
func EnsureJar(ctx context.Context, scanPaths []string, verbose bool) (string, error) {
	if path := FindJar(scanPaths); path != "" {
		return path, nil
	}

	ensureJarMu.Lock()
	defer ensureJarMu.Unlock()
	// Re-check under the lock: a concurrent EnsureJar may have downloaded.
	if path := FindJar(scanPaths); path != "" {
		return path, nil
	}

	target, err := installedJarPath()
	if err != nil {
		return "", missingJarError(err)
	}
	url := jarReleaseURL()
	if url == "" {
		return "", missingJarError(errors.New("dev build; auto-download is disabled"))
	}
	if verbose {
		reporter().Verbosef("verbose: downloading krit-types.jar from %s\n", url)
	}
	if err := downloadJar(ctx, url, target); err != nil {
		return "", fmt.Errorf("download krit-types.jar from %s: %w", url, err)
	}
	return target, nil
}

// versionTag normalises the package-level Version into a release tag form
// ("vX.Y.Z"). Returns "" when Version is unset or a dev build.
func versionTag() string {
	v := strings.TrimSpace(Version)
	if v == "" || v == "dev" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

func installedJarPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("no HOME directory; cannot install jar under ~/.krit/jars")
	}
	tag := versionTag()
	if tag == "" {
		return filepath.Join(home, ".krit", "jars", "krit-types.jar"), nil
	}
	return filepath.Join(home, ".krit", "jars", "krit-types-"+tag+".jar"), nil
}

func jarReleaseURL() string {
	tag := versionTag()
	if tag == "" {
		return ""
	}
	return "https://github.com/kaeawc/krit/releases/download/" + tag + "/krit-types-" + tag + ".jar"
}

func downloadJar(ctx context.Context, url, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	dlCtx, cancel := context.WithTimeout(ctx, jarDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		_, copyErr := io.Copy(w, resp.Body)
		return copyErr
	})
}

// missingJarError builds the helpful error users see when no jar is found and
// auto-download cannot fill the gap. Keeps the install / env-var / dev-build
// options visible together.
func missingJarError(reason error) error {
	return fmt.Errorf("krit-types.jar not found (%w). Install via 'brew install krit' to enable auto-download, set KRIT_TYPES_JAR to an existing jar, or build with: cd tools/krit-types && ./gradlew shadowJar", reason)
}
