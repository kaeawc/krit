package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

func TestOracleBackendsResolveDefaultStdlib(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar` to enable oracle parity")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable oracle parity")
	}

	tmp := t.TempDir()
	srcDir, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Stdlib.kt"), []byte("fun f(): Int = listOf(1)?.size ?: 0\nfun g(s: String) = s.uppercase()!!\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	for _, tc := range []struct {
		name string
		jar  string
	}{
		{name: "krit-types", jar: kaaJar},
		{name: "krit-fir", jar: firJar},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(tmp, tc.name+".json")
			if _, err := oracle.InvokeWithFilesWithOptions(tc.jar, []string{srcDir}, out, "", false, oracle.InvocationOptions{}); err != nil {
				t.Fatalf("invoke %s: %v", tc.name, err)
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
		})
	}
}
