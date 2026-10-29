package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestOracleResolvesDefaultStdlib requires the FIR oracle to resolve the
// Kotlin stdlib with no explicit classpath: the stdlib-typed diagnostics below
// are only reported when listOf and uppercase resolve.
func TestOracleResolvesDefaultStdlib(t *testing.T) {
	root := repoRoot(t)
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `make fir-jar`")
	}

	tmp := t.TempDir()
	srcDir, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Stdlib.kt"), []byte("fun f(): Int = listOf(1)?.size ?: 0\nfun g(s: String) = s.uppercase()!!\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out := filepath.Join(tmp, "krit-fir.json")
	if _, err := oracle.InvokeWithFilesWithOptions(firJar, []string{srcDir}, out, "", false, oracle.InvocationOptions{}); err != nil {
		t.Fatalf("invoke krit-fir: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse output: %v\n%s", err, raw)
	}
	file := indexByBasename(data.Files)["Stdlib.kt"]
	if file == nil {
		t.Fatalf("Stdlib.kt missing from oracle output: %v", data.Files)
	}
	foundSafeCall, foundNotNull := false, false
	for _, diagnostic := range file.Diagnostics {
		foundSafeCall = foundSafeCall || diagnostic.FactoryName == "UNNECESSARY_SAFE_CALL"
		foundNotNull = foundNotNull || diagnostic.FactoryName == "UNNECESSARY_NOT_NULL_ASSERTION"
	}
	if !foundSafeCall || !foundNotNull {
		t.Errorf("diagnostics missing: safeCall=%t notNull=%t; diagnostics=%+v", foundSafeCall, foundNotNull, file.Diagnostics)
	}
}
