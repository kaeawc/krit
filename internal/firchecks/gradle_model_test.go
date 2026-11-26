package firchecks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/testutil"
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
	// A rebuild that keeps the size and lands in the same mtime tick.
	testutil.RewriteKeepingStat(t, jar, "SECOND LONGER")
	rebuilt := FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{})
	if before == rebuilt {
		t.Fatal("same-size, same-mtime jar rebuild retained FIR fingerprint")
	}
	// The same bytes with a new mtime (a touch, or a re-downloaded jar) are
	// the same input and keep their cache entries.
	stat, err := os.Stat(jar)
	if err != nil {
		t.Fatal(err)
	}
	newTime := stat.ModTime().Add(time.Second)
	if err := os.Chtimes(jar, newTime, newTime); err != nil {
		t.Fatal(err)
	}
	if rebuilt != FirInvocationFingerprint([]string{jar}, "", nil, nil, FileFacts{}) {
		t.Fatal("an mtime-only change of identical bytes changed the FIR fingerprint")
	}
}

func TestFirFingerprintChangesWithJvmTarget(t *testing.T) {
	a := FirInvocationFingerprint(nil, "", nil, nil, FileFacts{}, CompileTarget{JvmTarget: "11"})
	b := FirInvocationFingerprint(nil, "", nil, nil, FileFacts{}, CompileTarget{JvmTarget: "17"})
	if a == b {
		t.Fatal("JVM target did not affect invocation fingerprint")
	}
}

func TestGeneratedSymbolGatedCount(t *testing.T) {
	stats := VerdictStats{GatedFiles: map[string]string{"a.kt": "Unresolved reference 'R'", "b.kt": "Unresolved reference: Other", "c.kt": "unresolved reference: ViewBinding", "d.kt": "Unresolved reference 'foo' while inferring <R> fun <R> foo(): R"}}
	var out bytes.Buffer
	writeVerdictSummary(&out, stats)
	if !strings.Contains(out.String(), "2 gated (compiler error or crash), 0 excluded (scripts or not in a JVM source set), 0 rule errors (checker threw; Go kept for that rule and file), 2 gated (generated sources)") {
		t.Fatalf("summary: %s", out.String())
	}
	if generatedSymbolError("Unresolved reference 'foo' while inferring <R> fun <R> foo(): R") || !generatedSymbolError("Unresolved reference 'R'") {
		t.Fatal("R should count only when it is the unresolved symbol")
	}
}
