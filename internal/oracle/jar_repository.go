package oracle

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Legacy Maven checksum fallback for download integrity only.
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

const mavenCentralBase = "https://repo1.maven.org/maven2"
const jarRepositoryEnv = "KRIT_JAR_REPOSITORY"

// redactURL removes credentials from a URL before it is shown in logs or errors.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparseable URL>"
	}
	if u.User == nil {
		return raw
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return u.Redacted()
	}
	u.User = url.User("xxxxx")
	return u.String()
}

// jarSource describes one download endpoint; mavenChecksum selects sibling
// checksum verification, while GitHub uses release checksums.txt instead.
type jarSource struct {
	url           string
	mavenChecksum bool
}

// mavenVersion removes the release tag's leading v for Maven coordinates.
func mavenVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
}

// mavenJarURL builds the conventional Maven repository path for a helper jar.
func mavenJarURL(repoBase, artifactID, version string) string {
	return strings.TrimRight(repoBase, "/") + "/dev/jasonpearson/krit/" + artifactID + "/" + version + "/" + artifactID + "-" + version + ".jar"
}

// jarSources chooses a mirror alone when configured, otherwise GitHub then Maven Central.
func jarSources(b Backend, tag string) []jarSource {
	artifact := b.jarBaseName()
	version := mavenVersion(tag)
	if base := strings.TrimSpace(os.Getenv(jarRepositoryEnv)); base != "" {
		return []jarSource{{url: mavenJarURL(base, artifact, version), mavenChecksum: true}}
	}
	return []jarSource{
		{url: jarReleaseURL(b)},
		{url: mavenJarURL(mavenCentralBase, artifact, version), mavenChecksum: true},
	}
}

// downloadVerifiedJar streams a Maven jar atomically and checks its sibling digest.
func downloadVerifiedJar(ctx context.Context, jarURL, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	resp, err := httpGet(ctx, jarURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", redactURL(jarURL), err)
	}
	defer resp.Body.Close()
	return fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		hashes := map[string]hash.Hash{
			".sha256": sha256.New(),
			".sha512": sha512.New(),
			".sha1":   sha1.New(), //nolint:gosec // Legacy Maven checksum fallback for download integrity only.
		}
		writers := []io.Writer{w, hashes[".sha256"], hashes[".sha512"], hashes[".sha1"]}
		if _, err := io.Copy(io.MultiWriter(writers...), resp.Body); err != nil {
			return err
		}
		return verifyJarChecksum(ctx, jarURL, hashes)
	})
}

// verifyJarChecksum accepts the first available Maven checksum, strongest first.
// A malformed or mismatched published checksum is authoritative and fails the install.
func verifyJarChecksum(ctx context.Context, jarURL string, hashes map[string]hash.Hash) error {
	var unavailable []error
	for _, suffix := range []string{".sha256", ".sha512", ".sha1"} {
		content, err := fetchJarChecksum(ctx, jarURL+suffix)
		if err != nil {
			unavailable = append(unavailable, fmt.Errorf("%s: %w", redactURL(jarURL+suffix), err))
			continue
		}
		fields := strings.Fields(string(content))
		if len(fields) == 0 {
			return fmt.Errorf("malformed %s checksum: empty", suffix)
		}
		want, err := hex.DecodeString(fields[0])
		if err != nil || len(want) != hashes[suffix].Size() {
			return fmt.Errorf("malformed %s checksum", suffix)
		}
		if got := hashes[suffix].Sum(nil); !bytes.Equal(got, want) {
			return fmt.Errorf("checksum mismatch for %s: got %x, want %x", suffix, got, want)
		}
		return nil
	}
	return fmt.Errorf("no Maven checksum available: %w", errors.Join(unavailable...))
}

// fetchJarChecksum reads a bounded Maven checksum body with its own short deadline.
func fetchJarChecksum(ctx context.Context, checksumURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := httpGet(ctx, checksumURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", redactURL(checksumURL), err)
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1025))
	if err != nil {
		return nil, err
	}
	if len(content) > 1024 {
		return nil, errors.New("checksum exceeds 1 KiB")
	}
	return content, nil
}
