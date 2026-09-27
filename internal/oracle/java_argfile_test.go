package oracle

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPrepareJavaArgsLargeClasspath(t *testing.T) {
	var entries []string
	for i := 1; i <= 5000; i++ {
		entries = append(entries, fmt.Sprintf("/fake/path/entry-%04d.jar", i))
	}
	joined := strings.Join(entries, string(os.PathListSeparator))
	args := []string{"-Xms1g", "-jar", "/fake/krit-fir.jar", "--classpath", joined}
	got, cleanup, err := prepareJavaArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cmd := exec.Command("java", got...)
	if len(cmd.Args) != 2 || !strings.HasPrefix(cmd.Args[1], "@") || len(cmd.Args[1]) >= javaArgfileThreshold {
		t.Fatalf("long classpath leaked into exec argv: %v", cmd.Args)
	}
	body, err := os.ReadFile(strings.TrimPrefix(got[0], "@"))
	if err != nil || !strings.Contains(string(body), "--classpath\n"+joined+"\n") {
		t.Fatalf("argfile did not preserve classpath: %v", err)
	}
	short := []string{"-jar", "a.jar", "--classpath", "/one.jar"}
	unchanged, shortCleanup, err := prepareJavaArgs(short)
	if err != nil {
		t.Fatal(err)
	}
	defer shortCleanup()
	if strings.Join(unchanged, "\x00") != strings.Join(short, "\x00") {
		t.Fatalf("short argv changed: %v", unchanged)
	}
}

func TestPrepareJavaArgsQuotesTokens(t *testing.T) {
	args := []string{"-jar", "/some path/krit.jar", "--name", `a"b'c`}
	args = append(args, strings.Repeat("x", javaArgfileThreshold))
	got, cleanup, err := prepareJavaArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	body, err := os.ReadFile(strings.TrimPrefix(got[0], "@"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"/some path/krit.jar"`) || !strings.Contains(string(body), `"a\"b'c"`) {
		t.Fatalf("quoted argfile tokens missing: %q", body[:100])
	}
}
