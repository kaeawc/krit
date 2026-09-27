package gradlemodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "app", "src")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(model, 0755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "settings.gradle.kts"), "")
	if got := Discover(child); got != model {
		t.Fatalf("discover = %q, want %q", got, model)
	}
	other := t.TempDir()
	write(t, filepath.Join(other, ".git", "HEAD"), "")
	if got := Discover(other); got != "" {
		t.Fatalf("git boundary: %s", got)
	}
	worktree := filepath.Join(t.TempDir(), "worktree")
	write(t, filepath.Join(worktree, ".git"), "gitdir: /tmp/worktree\n")
	if got := Discover(worktree); got != "" {
		t.Fatalf("worktree pointer boundary: %s", got)
	}
	if err := os.Remove(filepath.Join(root, "settings.gradle.kts")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "settings.gradle"), "")
	if got := Discover(child); got != model {
		t.Fatalf("groovy settings: %s", got)
	}
}

func TestLoadClasspathAndWarnings(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "model")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	boot := filepath.Join(root, "android.jar")
	lib := filepath.Join(root, "lib.jar")
	write(t, boot, "boot")
	write(t, lib, "lib")
	write(t, filepath.Join(dir, "bad.json"), "{")
	write(t, filepath.Join(dir, "schema.json"), `{"schema":2}`)
	write(t, filepath.Join(dir, "missing.json"), `{"schema":1,"projects":[{"path":":missing","dir":"/definitely/not/here"}]}`)
	body, err := json.Marshal(map[string]interface{}{
		"schema": 1,
		"projects": []interface{}{
			map[string]interface{}{"path": ":z", "dir": root, "sourceSets": []interface{}{}},
			map[string]interface{}{"path": ":a", "dir": root, "sourceSets": []interface{}{map[string]interface{}{
				"name":          "debug",
				"bootClasspath": []string{boot, boot},
				"classpath":     []string{lib, boot, filepath.Join(root, "gone.jar")},
			}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "root.json"), string(body))
	write(t, filepath.Join(dir, "ignored.txt"), "ignored")
	m, warnings, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings: %v", warnings)
	}
	if len(m.Projects) != 2 || m.Projects[0].Path != ":a" {
		t.Fatalf("projects: %+v", m.Projects)
	}
	cp, missing := m.Classpath()
	if !reflect.DeepEqual(cp, []string{boot, lib}) || missing != 1 {
		t.Fatalf("classpath=%v missing=%d", cp, missing)
	}
	if got := m.BootClasspath(); !reflect.DeepEqual(got, []string{boot}) {
		t.Fatalf("boot classpath=%v", got)
	}
	if got := m.CompileClasspath(); !reflect.DeepEqual(got, []string{lib, boot}) {
		t.Fatalf("compile classpath=%v", got)
	}
	before := m.Fingerprint()
	time.Sleep(time.Millisecond)
	write(t, lib, "longer library")
	if before == m.Fingerprint() {
		t.Fatal("jar replacement did not change fingerprint")
	}
	if _, _, err := Load(filepath.Join(root, "absent")); err == nil || !strings.Contains(err.Error(), "read gradle model") {
		t.Fatalf("missing directory: %v", err)
	}
}

func TestExpandedSourceSets(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "model")
	if err := os.Mkdir(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mainDir := filepath.Join(root, "src", "main", "kotlin")
	generatedDir := filepath.Join(root, "build", "generated", "source")
	for _, dir := range []string{mainDir, generatedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	boot, compile, generated := filepath.Join(root, "boot.jar"), filepath.Join(root, "compile.jar"), filepath.Join(root, "r.jar")
	for _, path := range []string{boot, compile, generated} {
		write(t, path, "jar")
	}
	body, err := json.Marshal(map[string]any{
		"schema": 1,
		"projects": []Project{{Path: ":app", Dir: root, SourceSets: []SourceSet{
			{Kind: "main", SourceDirs: []string{mainDir}, BootClasspath: []string{boot}, ClasspathEntries: []string{compile}, JvmTarget: "1.8"},
			{Kind: "test", SourceDirs: []string{mainDir}, GeneratedSourceDirs: []string{generatedDir, filepath.Join(root, "missing")}, GeneratedClasspath: []string{generated, compile, filepath.Join(root, "missing.jar")}, JvmTarget: "17"},
			{Kind: "androidTest", JvmTarget: "11"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modelDir, "model.json"), string(body))
	m, warnings, err := Load(modelDir)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("load: %v, %v", err, warnings)
	}
	if got, missing := m.Classpath(); !reflect.DeepEqual(got, []string{boot, compile, generated}) || missing != 1 {
		t.Fatalf("classpath=%v missing=%d", got, missing)
	}
	if got := m.SourceDirs(); !reflect.DeepEqual(got, []string{mainDir, generatedDir}) {
		t.Fatalf("source dirs=%v", got)
	}
	if got := m.MaxJvmTarget(); got != "17" {
		t.Fatalf("jvm target=%q", got)
	}
	if got := m.CompileClasspath(); !reflect.DeepEqual(got, []string{compile}) {
		t.Fatalf("compile classpath=%v", got)
	}
	if got := m.GeneratedClasspath(); !reflect.DeepEqual(got, []string{generated, compile}) {
		t.Fatalf("generated classpath=%v", got)
	}
}
