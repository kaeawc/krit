package firchecks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirFingerprintChangesWhenClasspathJarChanges(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "dep.jar")
	if err := os.WriteFile(jar, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	before := FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{})
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(jar, []byte("second longer"), 0644); err != nil {
		t.Fatal(err)
	}
	if before == FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{}) {
		t.Fatal("jar replacement retained FIR fingerprint")
	}
	before = FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{})
	stat, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	newTime := stat.ModTime().Add(time.Second)
	if err := os.Chtimes(jar, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	if before == FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{}) {
		t.Fatal("mtime-only change retained FIR fingerprint")
	}
}

func TestGeneratedSymbolGatedCount(t *testing.T) {
	stats := VerdictStats{GatedFiles: map[string]string{"a.kt": "Unresolved reference: R", "b.kt": "Unresolved reference: Other", "c.kt": "unresolved reference: ViewBinding"}}
	var out bytes.Buffer
	writeVerdictSummary(&out, stats)
	if !strings.Contains(out.String(), "1 gated (compiler error or crash), 0 excluded (scripts or not in a JVM source set), 0 rule errors (checker threw; Go kept for that rule and file), 2 gated (generated sources)") {
		t.Fatalf("summary: %s", out.String())
	}
}
