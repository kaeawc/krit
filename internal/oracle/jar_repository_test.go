package oracle

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
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

func TestRedactURL(t *testing.T) {
	for _, tc := range []struct {
		input, want string
		unchanged   bool
	}{
		{"https://user:pass@host/path", "https://user:xxxxx@host/path", false},
		{"https://token@host/path", "https://xxxxx@host/path", false},
		{"https://host/path", "https://host/path", true},
		{"https://host/path\n", "<unparseable URL>", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got := redactURL(tc.input)
			if got != tc.want {
				t.Fatalf("redactURL() = %q, want %q", got, tc.want)
			}
			if tc.unchanged && got != tc.input {
				t.Fatal("URL without userinfo changed")
			}
			if strings.Contains(got, "pass") || strings.Contains(got, "token") {
				t.Fatalf("credential leaked: %q", got)
			}
		})
	}
}

func TestConfiguredRepositoryRedactsErrorsAndKeepsBasicAuth(t *testing.T) {
	isolateJarLookup(t)
	Version = "1.2.3"
	authorized := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.User != nil {
			t.Errorf("request path contained userinfo: %v", r.URL.User)
		}
		if r.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("user:s3cret")) {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		} else {
			authorized = true
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	priorClient, priorBase, priorJava := jarHTTPClient, releaseDownloadBase, javaAvailable
	jarHTTPClient = http.DefaultClient
	releaseDownloadBase = server.URL + "/github"
	javaAvailable = func() bool { return true }
	t.Cleanup(func() { jarHTTPClient, releaseDownloadBase, javaAvailable = priorClient, priorBase, priorJava })
	base := strings.TrimPrefix(server.URL, "http://")
	t.Setenv(jarRepositoryEnv, "http://user:s3cret@"+base+"/repo")
	_, err := EnsureBackendJar(context.Background(), BackendFIR, []string{t.TempDir()}, false)
	if err == nil {
		t.Fatal("expected 404 download error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("credential leaked in error: %v", err)
	}
	if !authorized {
		t.Fatal("server did not receive Basic Auth credentials")
	}
}

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
		sources, err := jarSources(b, "v1.2.3")
		if err != nil {
			t.Fatal(err)
		}
		if len(sources) != 2 || sources[0].mavenChecksum || !sources[1].mavenChecksum || !strings.Contains(sources[1].url, b.jarBaseName()) {
			t.Fatalf("%s: %+v", b, sources)
		}
	}
	t.Setenv(jarRepositoryEnv, " https://mirror/ ")
	sources, err := jarSources(BackendFIR, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || !sources[0].mavenChecksum || sources[0].url != "https://mirror/dev/jasonpearson/krit/krit-fir/1.2.3/krit-fir-1.2.3.jar" {
		t.Fatalf("%+v", sources)
	}
}

func TestJarSourcesValidatesConfiguredRepository(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		wantErr     bool
		redacted    bool
	}{
		{"https", "https://mirror.example/maven", false, false},
		{"ipv4 loopback", "http://127.0.0.1:1234/repo", false, false},
		{"localhost", "http://localhost/repo", false, false},
		{"ipv6 loopback", "http://[::1]/repo", false, false},
		{"plain http", "http://mirror.example/maven", true, false},
		{"ftp", "ftp://x", true, false},
		{"not url", "not a url", true, false},
		{"bad scheme", "://bad", true, false},
		{"credentials", "http://user:secret@mirror.example/", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(jarRepositoryEnv, tc.value)
			sources, err := jarSources(BackendFIR, "v1.2.3")
			if (err != nil) != tc.wantErr {
				t.Fatalf("jarSources() error = %v", err)
			}
			if tc.wantErr && sources != nil {
				t.Fatalf("sources = %+v, want nil", sources)
			}
			if tc.redacted && strings.Contains(err.Error(), "secret") {
				t.Fatalf("credential leaked in error: %v", err)
			}
		})
	}
}

func TestEnsureBackendJarRejectsInsecureRepositoryBeforeRequest(t *testing.T) {
	isolateJarLookup(t)
	Version = "1.2.3"
	t.Setenv(jarRepositoryEnv, "http://not-a-loopback-hostname.example/repo")
	priorClient, priorJava := jarHTTPClient, javaAvailable
	hits := 0
	jarHTTPClient = &http.Client{Transport: jarRoundTripFunc(func(*http.Request) (*http.Response, error) {
		hits++
		return nil, fmt.Errorf("unexpected HTTP request")
	})}
	javaAvailable = func() bool { return true }
	t.Cleanup(func() { jarHTTPClient, javaAvailable = priorClient, priorJava })
	_, err := EnsureBackendJar(context.Background(), BackendFIR, []string{t.TempDir()}, false)
	if err == nil {
		t.Fatal("expected insecure repository error")
	}
	if hits != 0 {
		t.Fatalf("made %d HTTP requests before rejecting repository", hits)
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
		{"sha1 only", "legacy checksum", http.StatusNotFound, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := useJarTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/jar":
					_, _ = w.Write(jar)
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
				case strings.HasSuffix(r.URL.Path, ".sha1") && tc.name == "sha1 only":
					_, _ = io.WriteString(w, "legacy checksum")
				default:
					http.NotFound(w, r)
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
