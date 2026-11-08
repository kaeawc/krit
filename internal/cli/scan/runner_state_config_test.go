package scan

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
)

func TestFatalConfigErrorExplicitIsFatal(t *testing.T) {
	var buf bytes.Buffer
	if !fatalConfigError("missing.yml", &buf, errors.New("boom")) {
		t.Fatal("explicit --config failure must be fatal")
	}
	if !strings.HasPrefix(buf.String(), "error: config:") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestFatalConfigErrorAutoDetectedWarns(t *testing.T) {
	var buf bytes.Buffer
	if fatalConfigError("", &buf, errors.New("boom")) {
		t.Fatal("auto-detected config failure must not be fatal")
	}
	if !strings.HasPrefix(buf.String(), "warning: config:") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestExplicitConfigLoadFailures(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(bad, []byte("a: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "missing.yml"), bad} {
		if _, err := config.LoadAndMergeDefaultsWith(config.LoadOptions{}, p, dir); err == nil {
			t.Errorf("expected load error for %s", p)
		}
	}
}
