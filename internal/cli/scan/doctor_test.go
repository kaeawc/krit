package scan

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

func TestPrintDoctorReportsOracleJarsAndDefaultConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fir := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(fir, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", fir)

	var out bytes.Buffer
	printDoctor(&out, "dev")
	got := out.String()
	if strings.Contains(got, "krit-types") {
		t.Errorf("doctor output must not list the removed krit-types jar:\n%s", got)
	}
	for _, want := range []string{
		"krit-fir: " + fir + " (default oracle backend)",
		"default config: ",
		"javac: ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor output missing %q:\n%s", want, got)
		}
	}
}

func TestOracleJarStatusMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_FIR_JAR", "")
	t.Chdir(t.TempDir())
	prev := oracle.Version
	t.Cleanup(func() { oracle.Version = prev })

	oracle.Version = "1.2.3"
	if got := oracleJarStatus(oracle.BackendFIR); !strings.Contains(got, "not found") || !strings.Contains(got, "auto-downloaded") || !strings.Contains(got, "KRIT_FIR_JAR") {
		t.Errorf("release status = %q", got)
	}
	oracle.Version = "dev"
	if got := oracleJarStatus(oracle.BackendFIR); !strings.Contains(got, "cd tools/krit-fir && ./gradlew shadowJar") {
		t.Errorf("dev status = %q", got)
	}
}

func TestPrintDoctorReportsJavaHomeTools(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	names := map[string]string{"java": "java", "javac": "javac"}
	if runtime.GOOS == "windows" {
		names = map[string]string{"java": "java.exe", "javac": "javac.exe"}
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("JAVA_HOME", home)

	var out bytes.Buffer
	printDoctor(&out, "dev")
	got := out.String()
	for _, want := range []string{
		"java: " + filepath.Join(bin, names["java"]) + "\n",
		"javac: " + filepath.Join(bin, names["javac"]) + " ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor output missing %q:\n%s", want, got)
		}
	}
}
