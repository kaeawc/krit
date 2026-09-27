package scan

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
)

func fakeJava21(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "bin", "java"), []byte("#!/bin/sh\necho 'openjdk version \"21.0.1\"' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JAVA_HOME", home)
}

func fakeFIRJar(t *testing.T) string {
	t.Helper()
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	return jar
}

func TestFIRPreflightRequirements(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	t.Run("Java", func(t *testing.T) {
		t.Setenv("JAVA_HOME", filepath.Join(t.TempDir(), "missing"))
		_, err := PreflightFIR(ctx, []string{root}, config.NewConfig(), "", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "Java 21") || !strings.Contains(err.Error(), "JAVA_HOME") || !strings.Contains(err.Error(), "--no-fir") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("jar", func(t *testing.T) {
		fakeJava21(t)
		t.Setenv("KRIT_FIR_JAR", filepath.Join(t.TempDir(), "missing.jar"))
		_, err := PreflightFIR(ctx, []string{root}, config.NewConfig(), "", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "KRIT_FIR_JAR") || !strings.Contains(err.Error(), "--no-fir") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("classpath", func(t *testing.T) {
		fakeJava21(t)
		fakeFIRJar(t)
		_, err := PreflightFIR(ctx, []string{root}, config.NewConfig(), "", false, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "kritExportModel") || !strings.Contains(err.Error(), "oracle.classpath") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("empty declared classpath", func(t *testing.T) {
		fakeJava21(t)
		fakeFIRJar(t)
		cfg := config.NewConfigFromData(map[string]interface{}{"oracle": map[string]interface{}{"classpath": []interface{}{}}})
		if _, err := PreflightFIR(ctx, []string{root}, cfg, "", false, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("CLASSPATH", func(t *testing.T) {
		fakeJava21(t)
		fakeFIRJar(t)
		t.Setenv("CLASSPATH", string(os.PathListSeparator)+filepath.Join(root, "library.jar"))
		if _, err := PreflightFIR(ctx, []string{root}, config.NewConfig(), "", true, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestFIRPreflightStaleModelIgnoresUndeclaredDirectories(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "root.json")
	if err := os.WriteFile(model, []byte(`{"schema":1,"projects":[{"path":":","dir":"`+root+`","sourceSets":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(model, old, old); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(root, "unrelated")
	if err := os.MkdirAll(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unrelated, "build.gradle.kts"), []byte("plugins {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warnings bytes.Buffer
	warnStaleGradleModel(dir, []string{root}, &warnings)
	if strings.Contains(warnings.String(), "stale") {
		t.Fatalf("unrelated subtree warned: %s", warnings.String())
	}
}

func TestFIRPreflightStaleModelWarns(t *testing.T) {
	fakeJava21(t)
	fakeFIRJar(t)
	root := t.TempDir()
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "_root.json")
	if err := os.WriteFile(model, []byte(`{"schema":1,"projects":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(model, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "build.gradle.kts"), []byte("plugins {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warnings bytes.Buffer
	if _, err := PreflightFIR(context.Background(), []string{root}, config.NewConfig(), "", false, &warnings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warnings.String(), "stale") {
		t.Fatalf("warnings = %q", warnings.String())
	}
}

func TestFIRPreflightMissingModelJarsWarns(t *testing.T) {
	fakeJava21(t)
	fakeFIRJar(t)
	root := t.TempDir()
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	model := `{"schema":1,"projects":[{"path":":","dir":` + strconv.Quote(root) + `,"sourceSets":[{"name":"main","classpath":["/nonexistent/krit-missing.jar"]}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "_root.json"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	var warnings bytes.Buffer
	if _, err := PreflightFIR(context.Background(), []string{root}, config.NewConfig(), "", false, &warnings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warnings.String(), "1 classpath jars missing") {
		t.Fatalf("warnings = %q", warnings.String())
	}
}
