package firchecks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/testutil"
)

func TestFirInvocationFingerprintSeesSameTickJarRewrite(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar-one"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{})
	testutil.RewriteKeepingStat(t, jar, "jar-two")
	if FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{}) == before {
		t.Fatal("a same-size krit-fir jar rewrite within one mtime tick kept the fingerprint")
	}
}

func TestFirInvocationFingerprintIsStableAcrossProcesses(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar-one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{})
	hashutil.ResetDefault()
	testutil.Age(t, jar)
	if got := FirInvocationFingerprint(nil, jar, []string{"A"}, nil, FileFacts{}); got != first {
		t.Fatal("fingerprint of an unchanged krit-fir jar changed between processes")
	}
}
