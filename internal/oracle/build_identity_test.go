package oracle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOracleVersionHashChangesWithJarBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "krit-fir.jar")
	t.Setenv("KRIT_FIR_JAR", path)
	if err := os.WriteFile(path, []byte("first jar"), 0600); err != nil {
		t.Fatal(err)
	}
	first := oracleVersionHash(BackendFIR, nil)
	if err := os.WriteFile(path, []byte("different jar bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if second := oracleVersionHash(BackendFIR, nil); first == second {
		t.Fatal("jar content change kept the same oracle version hash")
	}
}

func TestOracleVersionHashChangesWithProjectLocalJar(t *testing.T) {
	old, set := os.LookupEnv("KRIT_FIR_JAR")
	if err := os.Unsetenv("KRIT_FIR_JAR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if set {
			_ = os.Setenv("KRIT_FIR_JAR", old)
		} else {
			_ = os.Unsetenv("KRIT_FIR_JAR")
		}
	})
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	jar := filepath.Join(project, ".krit", BackendFIR.JarName())
	if err := os.MkdirAll(filepath.Dir(jar), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar, []byte("first local jar"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := FindBackendJar(BackendFIR, []string{project}); got != jar {
		t.Fatalf("FindBackendJar = %q, want %q", got, jar)
	}
	first := oracleVersionHash(BackendFIR, []string{project})
	if err := os.WriteFile(jar, []byte("second local jar bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if second := oracleVersionHash(BackendFIR, []string{project}); first == second {
		t.Fatal("project-local jar content change kept the same version hash")
	}
}

func TestOracleStoreKeyPartitionsBackends(t *testing.T) {
	t.Setenv("KRIT_FIR_JAR", "")
	t.Setenv("KRIT_TYPES_JAR", "")
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	for _, backend := range []Backend{BackendFIR, BackendKAA} {
		path := filepath.Join(project, ".krit", backend.JarName())
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("same jar bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scanPaths := []string{project}
	fir := oracleVersionHash(BackendFIR, scanPaths)
	kaa := oracleVersionHash(BackendKAA, scanPaths)
	if fir == kaa {
		t.Fatal("FIR and KAA share an oracle store namespace")
	}
	kaaPath := filepath.Join(project, ".krit", BackendKAA.JarName())
	if err := os.WriteFile(kaaPath, []byte("different KAA bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := oracleVersionHash(BackendFIR, scanPaths); got != fir {
		t.Fatal("changing the unused KAA jar invalidated the FIR namespace")
	}
}
