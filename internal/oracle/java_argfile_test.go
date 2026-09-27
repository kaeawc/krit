package oracle

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	cases := []struct {
		arg  string
		want string
	}{
		{"/some path/krit.jar", `"/some path/krit.jar"`},
		{"/work/project#1/lib.jar", `"/work/project#1/lib.jar"`},
		{"#lead.jar", `"#lead.jar"`},
		{`C:\work\lib.jar`, `"C:\\work\\lib.jar"`},
		{`C:\work dir\lib.jar`, `"C:\\work dir\\lib.jar"`},
		{`a"b.jar`, `"a\"b.jar"`},
		{"it's.jar", `"it's.jar"`},
		{`a"b'c`, `"a\"b'c"`},
		{"a\fb.jar", "\"a\fb.jar\""},
		{"plain.jar", "plain.jar"},
	}
	var args, wantLines []string
	for _, tc := range cases {
		args = append(args, tc.arg)
		wantLines = append(wantLines, tc.want)
	}
	padding := strings.Repeat("x", javaArgfileThreshold)
	args = append(args, padding)
	wantLines = append(wantLines, padding)
	got, cleanup, err := prepareJavaArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") {
		t.Fatalf("expected argfile, got %v", got)
	}
	body, err := os.ReadFile(strings.TrimPrefix(got[0], "@"))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Join(wantLines, "\n") + "\n"; string(body) != want {
		t.Fatalf("argfile bytes differ:\n got %q\nwant %q", body, want)
	}
}

func TestPrepareJavaArgsJavaRoundTrip(t *testing.T) {
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java is not on PATH; skipping argfile launcher round trip")
	}
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not on PATH; skipping argfile launcher round trip")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "ArgDump.java")
	if err := os.WriteFile(source, []byte(`public class ArgDump {
    public static void main(String[] args) {
        for (String arg : args) System.out.println(arg);
    }
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(javac, "-d", dir, source).CombinedOutput(); err != nil {
		t.Fatalf("compile ArgDump: %v\n%s", err, output)
	}
	want := []string{"/work/project#1/lib.jar", "#lead.jar", `C:\work dir\lib.jar`, `a"b.jar`, "it's.jar", "plain.jar"}
	args := []string{"-Dkrit.argfile.padding=" + strings.Repeat("x", javaArgfileThreshold), "-cp", dir, "ArgDump"}
	args = append(args, want...)
	got, cleanup, err := prepareJavaArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(got) != 1 || !strings.HasPrefix(got[0], "@") {
		t.Fatalf("expected argfile, got %v", got)
	}
	output, err := exec.Command(java, got...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("run ArgDump: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("run ArgDump: %v", err)
	}
	if wantOutput := strings.Join(want, "\n") + "\n"; string(output) != wantOutput {
		t.Fatalf("Java arguments differ:\n got %q\nwant %q", output, wantOutput)
	}
}
