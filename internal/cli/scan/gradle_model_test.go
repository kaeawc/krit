package scan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestGradleClasspathPrecedenceAndDisable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(root, "lib.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0644); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]interface{}{
		"schema": 1,
		"projects": []interface{}{map[string]interface{}{
			"path": ":", "dir": root,
			"sourceSets": []interface{}{map[string]interface{}{"classpath": []string{jar}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_root.json"), body, 0644); err != nil {
		t.Fatal(err)
	}
	model, err := loadGradleClasspath([]string{root}, "", false, true, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.NewConfigFromData(map[string]interface{}{"oracle": map[string]interface{}{"classpath": []interface{}{jar, "/config.jar"}}})
	t.Setenv("CLASSPATH", "/env.jar")
	if got := effectiveOracleClasspath(model, cfg); !reflect.DeepEqual(got, []string{jar, "/config.jar", "/env.jar"}) {
		t.Fatalf("effective classpath: %v", got)
	}
	disabled, err := loadGradleClasspath([]string{root}, "", true, false, &bytes.Buffer{})
	if err != nil || len(disabled) != 0 {
		t.Fatalf("disabled: %v %v", disabled, err)
	}
	if _, err := loadGradleClasspath([]string{root}, filepath.Join(root, "missing"), true, false, &bytes.Buffer{}); err == nil {
		t.Fatal("explicit missing model accepted")
	}
}

func TestDaemonForwardsResolvedClasspath(t *testing.T) {
	f := freshScanFlags(t)
	f.modelClasspath = []string{"/model.jar"}
	t.Setenv("CLASSPATH", "/env.jar")
	args := buildDaemonAnalyzeArgs(f, []string{t.TempDir()})
	if len(args.OracleClasspath) < 2 || args.OracleClasspath[0] != "/model.jar" || args.OracleClasspath[len(args.OracleClasspath)-1] != "/env.jar" {
		t.Fatalf("daemon classpath: %v", args.OracleClasspath)
	}
}

func TestGradleClasspathFromMultipleRoots(t *testing.T) {
	var roots, modelDirs, bootJars, compileJars []string
	for i, name := range []string{"first", "second"} {
		root := filepath.Join(t.TempDir(), name)
		modelDir := filepath.Join(root, ".krit", "gradle-model")
		child := filepath.Join(root, "app")
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(child, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0644); err != nil {
			t.Fatal(err)
		}
		bootJar := filepath.Join(root, name+"-boot.jar")
		compileJar := filepath.Join(root, name+"-compile.jar")
		for _, jar := range []string{bootJar, compileJar} {
			if err := os.WriteFile(jar, []byte("jar"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		bootClasspath := []string{bootJar}
		compileClasspath := []string{compileJar}
		if i > 0 {
			bootClasspath = append(bootClasspath, bootJars[0])
			compileClasspath = append(compileClasspath, compileJars[0])
		}
		body, err := json.Marshal(map[string]interface{}{
			"schema": 1,
			"projects": []interface{}{map[string]interface{}{
				"path": ":", "dir": root,
				"sourceSets": []interface{}{map[string]interface{}{
					"bootClasspath": bootClasspath,
					"classpath":     compileClasspath,
				}},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modelDir, "root.json"), body, 0644); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, child)
		modelDirs = append(modelDirs, modelDir)
		bootJars = append(bootJars, bootJar)
		compileJars = append(compileJars, compileJar)
	}
	var out bytes.Buffer
	got, err := loadGradleClasspath([]string{roots[0], filepath.Dir(roots[0]), roots[1]}, "", false, true, &out)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{bootJars[0], bootJars[1], compileJars[0], compileJars[1]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classpath order = %v, want %v", got, want)
	}
	for _, dir := range modelDirs {
		if count := strings.Count(out.String(), "gradle model: "+dir+" ("); count != 1 {
			t.Fatalf("model %s reported %d times: %s", dir, count, out.String())
		}
	}
}

func TestGradleClasspathWarningsRegardlessOfVerbose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(path, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "quiet", true: "verbose"}[verbose], func(t *testing.T) {
			var out bytes.Buffer
			if _, err := loadGradleClasspath(nil, dir, false, verbose, &out); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "warning: gradle model: "+path+": malformed JSON:") {
				t.Fatalf("missing warning: %q", out.String())
			}
			summary := "gradle model: " + dir + " (0 projects, 0 classpath entries, 0 missing dropped)\n"
			if strings.Contains(out.String(), summary) != verbose {
				t.Fatalf("summary presence for verbose=%t: %q", verbose, out.String())
			}
		})
	}
}
