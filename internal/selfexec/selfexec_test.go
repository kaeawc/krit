package selfexec

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestExecutableRefusesTestBinary(t *testing.T) {
	exe, err := Executable()
	if !errors.Is(err, ErrTestBinary) {
		t.Fatalf("Executable() = %q, %v; want ErrTestBinary inside go test", exe, err)
	}
}

func TestCustomNamedTestBinary(t *testing.T) {
	name := "custom-name"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "test", "-c", "-o", binary, "./internal/selfexec/testprobe")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build custom-named test binary: %v\n%s", err, output)
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "go test flags", args: []string{"-test.run=TestSelfExecProbe", "-test.v"}},
		{name: "no test flags"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, tc.args...)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("custom-named test binary timed out: %v\n%s", ctx.Err(), output)
			}
			if err != nil {
				t.Fatalf("custom-named test binary: %v\n%s", err, output)
			}
		})
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

func TestDetectTestBinaryKeepsArgsCheckWhenLookupFails(t *testing.T) {
	tb := detectTestBinary(true, func() (string, error) {
		return "", errors.New("lookup failed")
	})
	if !tb.matches(os.Args[0]) {
		t.Errorf("matches(os.Args[0]=%q) = false after a failed lookup, want true", os.Args[0])
	}
	if tb.matches("/usr/local/bin/krit") {
		t.Error("matches(/usr/local/bin/krit) = true, want false")
	}
	link := filepath.Join(t.TempDir(), "krit")
	if err := os.Symlink(os.Args[0], link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if !tb.matches(link) {
		t.Errorf("matches(symlink to os.Args[0]) = false after a failed lookup, want true")
	}
}

func TestDetectTestBinaryOutsideTests(t *testing.T) {
	tb := detectTestBinary(false, os.Executable)
	if tb.matches(os.Args[0]) {
		t.Error("a non-test process must not match its own executable")
	}
}
