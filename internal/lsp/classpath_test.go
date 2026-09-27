package lsp

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/logger"
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
	got := lspClasspath(root, cfg, []string{initJar}, nil)
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

func TestLSPClasspathUsesModelThenHeuristicFallback(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLASSPATH", "")
	modelJar := filepath.Join(root, "model.jar")
	configJar := filepath.Join(root, "config.jar")
	heuristicJar := filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", "example", "library", "1.0", "checksum", "library-1.0.jar")
	for _, path := range []string{modelJar, configJar, heuristicJar} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("jar"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build.gradle.kts"), []byte(`dependencies { implementation("example:library:1.0") }`), 0644); err != nil {
		t.Fatal(err)
	}
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	body := `{"schema":1,"projects":[{"path":":","dir":"` + root + `","sourceSets":[{"classpath":["` + modelJar + `"]}]}]}`
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewConfigFromData(map[string]interface{}{"lsp": map[string]interface{}{"classpath": []interface{}{configJar}}})
	modelLog := logger.NewCapture(slog.LevelInfo)
	if got := lspClasspath(root, cfg, nil, modelLog); !reflect.DeepEqual(got, existingUniquePaths([]string{configJar, modelJar})) {
		t.Fatalf("model classpath = %v", got)
	}
	if !modelLog.HasMessage("LSP classpath: gradle model") {
		t.Fatalf("model source not logged: %+v", modelLog.Records())
	}
	if err := os.RemoveAll(modelDir); err != nil {
		t.Fatal(err)
	}
	heuristicLog := logger.NewCapture(slog.LevelInfo)
	if got := lspClasspath(root, cfg, nil, heuristicLog); !reflect.DeepEqual(got, existingUniquePaths([]string{configJar, heuristicJar})) {
		t.Fatalf("heuristic classpath = %v", got)
	}
	if !heuristicLog.HasMessage("LSP classpath: heuristic discovery") {
		t.Fatalf("heuristic source not logged: %+v", heuristicLog.Records())
	}
}
