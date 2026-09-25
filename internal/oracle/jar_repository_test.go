package oracle

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type jarRoundTripFunc func(*http.Request) (*http.Response, error)

func (f jarRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func useJarTestServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	priorClient, priorBase, priorJava := jarHTTPClient, releaseDownloadBase, javaAvailable
	jarHTTPClient = &http.Client{Transport: jarRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		if req.URL.Host == "repo1.maven.org" {
			clone.URL.Path = "/central" + req.URL.Path
		}
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(server.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})}
	releaseDownloadBase = server.URL + "/github"
	javaAvailable = func() bool { return true }
	t.Cleanup(func() { jarHTTPClient, releaseDownloadBase, javaAvailable = priorClient, priorBase, priorJava })
	return server.URL
}

func TestMavenCoordinatesAndSources(t *testing.T) {
	isolateJarLookup(t)
	Version = "1.2.3"
	if got := mavenVersion("v1.2.3"); got != "1.2.3" {
		t.Fatal(got)
	}
	if got := mavenJarURL("https://repo/", "krit-fir", "1.2.3"); got != "https://repo/dev/jasonpearson/krit/krit-fir/1.2.3/krit-fir-1.2.3.jar" {
		t.Fatal(got)
	}
	for _, b := range []Backend{BackendKAA, BackendFIR} {
		sources := jarSources(b, "v1.2.3")
		if len(sources) != 2 || sources[0].mavenChecksum || !sources[1].mavenChecksum || !strings.Contains(sources[1].url, b.jarBaseName()) {
			t.Fatalf("%s: %+v", b, sources)
		}
	}
	t.Setenv(jarRepositoryEnv, " https://mirror/ ")
	sources := jarSources(BackendFIR, "v1.2.3")
	if len(sources) != 1 || !sources[0].mavenChecksum || sources[0].url != "https://mirror/dev/jasonpearson/krit/krit-fir/1.2.3/krit-fir-1.2.3.jar" {
		t.Fatalf("%+v", sources)
	}
}

func TestDownloadVerifiedJar(t *testing.T) {
	isolateJarLookup(t)
	jar := []byte("jar contents")
	sha256Sum := sha256.Sum256(jar)
	sha512Sum := sha512.Sum512(jar)
	for _, tc := range []struct {
		name, checksum string
		status         int
		wantError      bool
	}{
		{"sha256", hex.EncodeToString(sha256Sum[:]), http.StatusOK, false},
		{"sha512 fallback", hex.EncodeToString(sha512Sum[:]), http.StatusNotFound, false},
		{"mismatch", strings.Repeat("0", 64), http.StatusOK, true},
		{"none", "", http.StatusNotFound, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := useJarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, ".sha256"):
					if tc.status != http.StatusOK {
						http.NotFound(w, r)
						return
					}
					fmt.Fprintln(w, tc.checksum)
				case strings.HasSuffix(r.URL.Path, ".sha512"):
					if tc.name != "sha512 fallback" {
						http.NotFound(w, r)
						return
					}
					fmt.Fprintln(w, tc.checksum)
				case strings.HasSuffix(r.URL.Path, ".sha1"):
					http.NotFound(w, r)
				default:
					_, _ = w.Write(jar)
				}
			})
			target := filepath.Join(t.TempDir(), "jar")
			err := downloadVerifiedJar(context.Background(), base+"/jar", target)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			_, statErr := os.Stat(target)
			if (statErr == nil) == tc.wantError {
				t.Fatalf("target status = %v", statErr)
			}
		})
	}
}

func TestEnsureBackendJarSources(t *testing.T) {
	for _, tc := range []struct {
		name            string
		backend         Backend
		mirror, failAll bool
	}{
		{"central fallback fir", BackendFIR, false, false},
		{"mirror only kaa", BackendKAA, true, false},
		{"all fail fir", BackendFIR, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateJarLookup(t)
			Version = "1.2.3"
			githubHits, mavenHits := 0, 0
			jar := []byte("verified jar")
			sum := sha256.Sum256(jar)
			base := useJarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/github/") {
					githubHits++
					http.NotFound(w, r)
					return
				}
				mavenHits++
				if tc.failAll {
					http.NotFound(w, r)
					return
				}
				if strings.HasSuffix(r.URL.Path, ".sha256") {
					_, _ = io.WriteString(w, hex.EncodeToString(sum[:]))
					return
				}
				_, _ = w.Write(jar)
			})
			if tc.mirror {
				t.Setenv(jarRepositoryEnv, base+"/mirror")
			}
			path, err := EnsureBackendJar(context.Background(), tc.backend, []string{t.TempDir()}, false)
			if tc.failAll {
				if err == nil || path != "" {
					t.Fatalf("path=%q err=%v", path, err)
				}
				message := err.Error()
				for _, want := range []string{jarReleaseURL(tc.backend), mavenJarURL(mavenCentralBase, tc.backend.jarBaseName(), "1.2.3"), jarRepositoryEnv, tc.backend.JarEnvVar()} {
					if !strings.Contains(message, want) {
						t.Errorf("error missing %s: %s", want, message)
					}
				}
				before := githubHits + mavenHits
				_, _ = EnsureBackendJar(context.Background(), tc.backend, []string{t.TempDir()}, false)
				if githubHits+mavenHits != before {
					t.Fatal("memoized failure retried")
				}
			} else if err != nil || path == "" {
				t.Fatalf("path=%q err=%v", path, err)
			}
			if tc.mirror && (githubHits != 0 || mavenHits == 0) {
				t.Fatalf("hits: github=%d maven=%d", githubHits, mavenHits)
			}
			if !tc.mirror && (githubHits == 0 || mavenHits == 0) {
				t.Fatalf("hits: github=%d maven=%d", githubHits, mavenHits)
			}
		})
	}
}
