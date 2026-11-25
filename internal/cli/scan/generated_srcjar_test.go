package scan

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithGeneratedSrcjarsAddsCompileAndGeneratedRoots(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KRIT_HOME", filepath.Join(root, "krit-home"))
	jar := filepath.Join(root, "wire.srcjar")
	f, err := os.Create(jar)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("gen/Msg.java")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("package gen; public final class Msg {}\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sources, generated := withGeneratedSrcjars([]string{"/src/main/kotlin"}, []string{"/build/generated"},
		[]string{jar, jar, filepath.Join(root, "absent.srcjar")}, true, &out)
	if len(sources) != 2 || sources[0] != "/src/main/kotlin" {
		t.Fatalf("sources = %v, want the existing root then one srcjar root", sources)
	}
	if len(generated) != 2 || generated[1] != sources[1] {
		t.Fatalf("generated = %v, want the srcjar root marked generated", generated)
	}
	if !strings.HasPrefix(sources[1], filepath.Join(root, "krit-home", "srcjars")) {
		t.Fatalf("srcjar root %s is not under $KRIT_HOME/srcjars", sources[1])
	}
	log := out.String()
	if !strings.Contains(log, "warning: generated srcjar: srcjar "+filepath.Join(root, "absent.srcjar")) {
		t.Fatalf("no warning for the absent srcjar:\n%s", log)
	}
	if !strings.Contains(log, "generated srcjars: 1 of 3 unpacked") {
		t.Fatalf("no verbose summary:\n%s", log)
	}

	same, gen := withGeneratedSrcjars([]string{"a"}, nil, nil, true, &out)
	if len(same) != 1 || gen != nil {
		t.Fatalf("no srcjars changed the roots: %v %v", same, gen)
	}
}
