package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestLSPClasspathMergesInitConfigAndExistingOnly(t *testing.T) {
	root := t.TempDir()
	initJar := filepath.Join(root, "init.jar")
	cfgJar := filepath.Join(root, "cfg.jar")
	envJar := filepath.Join(root, "env.jar")
	for _, p := range []string{initJar, cfgJar, envJar} {
		if err := os.WriteFile(p, []byte("jar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLASSPATH", envJar+string(os.PathListSeparator)+filepath.Join(root, "missing.jar"))

	cfgPath := filepath.Join(root, "krit.yml")
	if err := os.WriteFile(cfgPath, []byte("oracle:\n  classpath:\n    - "+cfgJar+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := lspClasspath(root, cfg, []string{initJar})
	want := map[string]bool{initJar: true, cfgJar: true, envJar: true}
	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	for p := range want {
		if !seen[p] {
			t.Fatalf("missing classpath entry %q in %v", p, got)
		}
	}
	if seen[filepath.Join(root, "missing.jar")] {
		t.Fatalf("missing jar should not be included: %v", got)
	}
}
