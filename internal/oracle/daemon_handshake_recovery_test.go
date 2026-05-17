package oracle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLooksLikeJVMUnifiedLog locks in the discriminator the retry path
// hinges on: real JVM `-Xlog` output must trip the detector while genuine
// daemon JSON / surface-level error text must not.
func TestLooksLikeJVMUnifiedLog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "cds warning with required-classpath message",
			line: `[0.014s][warning][cds] Required classpath entry does not exist: /old/path/krit-types.jar`,
			want: true,
		},
		{
			name: "info-level cds log",
			line: `[0.020s][info ][cds] full module graph: enabled`,
			want: true,
		},
		{
			name: "aot tag",
			line: `[0.030s][warning][aot] AOT cache disabled: classpath mismatch`,
			want: true,
		},
		{
			name: "happy path json",
			line: `{"ready":true,"port":51823}`,
			want: false,
		},
		{
			name: "json array",
			line: `[1, 2, 3]`,
			want: false,
		},
		{
			name: "single bracketed token then text",
			line: `[ok] daemon ready`,
			want: false,
		},
		{
			name: "empty",
			line: ``,
			want: false,
		},
		{
			name: "plain text",
			line: `Error occurred during initialization of VM`,
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := looksLikeJVMUnifiedLog(tc.line); got != tc.want {
				t.Fatalf("looksLikeJVMUnifiedLog(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// TestHandshakeNoiseErrorTypeAssertion ensures the wrapped error is
// recoverable via errors.As — that's the only contract callers rely on.
func TestHandshakeNoiseErrorTypeAssertion(t *testing.T) {
	t.Parallel()
	wrapped := &handshakeNoiseError{line: "[0.01s][warning][cds] x"}
	var target *handshakeNoiseError
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As did not recognise *handshakeNoiseError")
	}
	if target.line == "" {
		t.Fatalf("noise error should preserve the offending line")
	}
	if !strings.Contains(target.Error(), "JVM log line") {
		t.Fatalf("error message should mention JVM log line, got: %q", target.Error())
	}
}

// TestPurgeJVMCachesForJarRemovesArchives stages every cache-suffix path
// that purgeJVMCachesForJar knows about, then asserts each is gone and
// the helper is robust when called twice in a row.
func TestPurgeJVMCachesForJarRemovesArchives(t *testing.T) {
	// Not Parallel: t.Setenv is incompatible with t.Parallel.

	// Force the cache dir into the test's tempdir by overriding HOME.
	// jarCachePath honours os.UserHomeDir(); on darwin/linux that reads HOME.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	jarDir := t.TempDir()
	jarPath := filepath.Join(jarDir, "fake-krit-types.jar")
	if err := os.WriteFile(jarPath, []byte("not a real jar but stable bytes"), 0o644); err != nil {
		t.Fatalf("write fake jar: %v", err)
	}

	// Pre-populate each suffix the helper purges so we can verify each is
	// actually deleted, not just one branch.
	suffixes := []func(string) (string, error){
		cdsArchivePath,
		cracCheckpointPath,
		aotConfigPath,
		aotCachePath,
	}
	staged := make([]string, 0, len(suffixes))
	for _, fn := range suffixes {
		p, err := fn(jarPath)
		if err != nil {
			t.Fatalf("derive cache path: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
			t.Fatalf("stage %s: %v", p, err)
		}
		staged = append(staged, p)
	}

	purgeJVMCachesForJar(jarPath, false)

	for _, p := range staged {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", p, err)
		}
	}

	// Calling twice must not panic and must remain a no-op.
	purgeJVMCachesForJar(jarPath, false)
}
