package firchecks

import (
	"archive/zip"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

func TestKnownFIRRulesMatchJar(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	jar := FindFirJar([]string{root})
	if jar == "" || !executableFirJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	java, err := oracle.JavaPath()
	if err != nil {
		t.Skipf("java unavailable for krit-fir --list-rules: %v", err)
	}
	out, err := exec.Command(java, "-jar", jar, "--list-rules").CombinedOutput()
	if err != nil {
		t.Fatalf("krit-fir --list-rules failed (%v): %s", err, out)
	}
	actual := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			actual[id] = true
		}
	}
	var onlyJar, onlyGo []string
	for id := range actual {
		if !knownFIRRules[id] {
			onlyJar = append(onlyJar, id)
		}
	}
	for id := range knownFIRRules {
		if !actual[id] {
			onlyGo = append(onlyGo, id)
		}
	}
	slices.Sort(onlyJar)
	slices.Sort(onlyGo)
	if len(actual) == 0 || len(onlyJar) != 0 || len(onlyGo) != 0 {
		t.Fatalf("knownFIRRules drift: only in jar: %v; only in Go: %v (jar returned %d IDs)", onlyJar, onlyGo, len(actual))
	}
}

func executableFirJar(path string) bool {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, file := range zr.File {
		if file.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return false
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		return err == nil && strings.Contains(string(data), "Main-Class: dev.jasonpearson.krit.fir.MainKt")
	}
	return false
}
