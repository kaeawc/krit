package selfexec

import (
	"errors"
	"testing"
)

func TestExecutableRefusesTestBinary(t *testing.T) {
	exe, err := Executable()
	if !errors.Is(err, ErrTestBinary) {
		t.Fatalf("Executable() = %q, %v; want ErrTestBinary inside go test", exe, err)
	}
}

func TestIsTestBinary(t *testing.T) {
	cases := map[string]bool{
		"/tmp/go-build123/b001/scan.test":  true,
		`C:\tmp\b001\scan.test.exe`:        true,
		"rules.test":                       true,
		"/usr/local/bin/krit":              false,
		"/usr/local/bin/krit.exe":          false,
		"/opt/krit-daemon":                 false,
		"/tmp/testdata/krit":               false,
		"/tmp/latest/krit-contest-edition": false,
	}
	for path, want := range cases {
		if got := IsTestBinary(path); got != want {
			t.Errorf("IsTestBinary(%q) = %v, want %v", path, got, want)
		}
	}
}
