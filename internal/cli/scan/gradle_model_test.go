package scan

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
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
	body := `{"schema":1,"projects":[{"path":":","dir":"` + root + `","sourceSets":[{"classpath":["` + jar + `"]}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "_root.json"), []byte(body), 0644); err != nil {
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
