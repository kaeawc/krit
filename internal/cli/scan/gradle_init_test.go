package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureKritIgnored(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".gitignore")
	if err := ensureKritIgnored(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("created missing .gitignore: %v", err)
	}
	if err := os.WriteFile(path, []byte("build/"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := ensureKritIgnored(path); err != nil {
			t.Fatal(err)
		}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "build/\n.krit/\n" {
		t.Fatalf("gitignore: %q", body)
	}
	for _, rule := range []string{".krit", ".krit/", "/.krit/"} {
		if err := os.WriteFile(path, []byte(rule+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := ensureKritIgnored(path); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(body), ".krit") != 1 {
			t.Fatalf("duplicate rule: %q", body)
		}
	}
	if err := os.WriteFile(path, []byte("build/\r\nout/"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ensureKritIgnored(path); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(path)
	if err != nil || string(body) != "build/\r\nout/\r\n.krit/\r\n" {
		t.Fatalf("CRLF gitignore: %q %v", body, err)
	}
	for _, rule := range []string{".krit", ".krit/", "/.krit", "/.krit/"} {
		original := "build/\r\n" + rule + "\r\n"
		if err := os.WriteFile(path, []byte(original), 0644); err != nil {
			t.Fatal(err)
		}
		if err := ensureKritIgnored(path); err != nil {
			t.Fatal(err)
		}
		body, err = os.ReadFile(path)
		if err != nil || string(body) != original {
			t.Fatalf("duplicated CRLF rule: %q %v", body, err)
		}
	}
}
