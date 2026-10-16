package oracle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// fakeOracleJava puts a `java` on PATH that records its arguments, writes
// the --check-out response (when asked) before the --output facts, and
// exits 0, the order krit-fir's one-shot CLI writes them in.
func fakeOracleJava(t *testing.T, checkResponse string) (argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fake")
	}
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := `#!/bin/sh
case " $* " in *" -version "*) echo 'openjdk version "25"' >&2; exit 0;; esac
printf '%s\n' "$@" > "` + argsFile + `"
out=""; check=""; deps=""
while [ $# -gt 0 ]; do
  case "$1" in
    --output) out="$2"; shift;;
    --check-out) check="$2"; shift;;
    --cache-deps-out) deps="$2"; shift;;
  esac
  shift
done
[ -n "$check" ] && [ -n "` + checkResponse + `" ] && printf '%s' '` + checkResponse + `' > "$check"
[ -n "$deps" ] && printf '{}' > "$deps"
[ -n "$out" ] && printf '{"version":1,"files":{}}' > "$out"
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "java"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("JAVA_HOME", "")
	return argsFile
}

func runFakeMissRun(t *testing.T, opts InvocationOptions) {
	t.Helper()
	dir := t.TempDir()
	jar := filepath.Join(dir, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	misses := filepath.Join(dir, "misses.txt")
	if err := os.WriteFile(misses, []byte(filepath.Join(dir, "A.kt")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh, deps := filepath.Join(dir, "fresh.json"), filepath.Join(dir, "deps.json")
	if err := runKritTypesCached(jar, []string{dir}, misses, fresh, deps, false, perf.New(false), opts); err != nil {
		t.Fatalf("runKritTypesCached: %v", err)
	}
}

func recordedArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fake java was not run: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func argValue(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

// The cached one-shot miss run is the default for a cold oracle; it used to
// drop the configured classpath and JVM target that the uncached run passes.
func TestCachedMissRunPassesCompileContext(t *testing.T) {
	argsFile := fakeOracleJava(t, "")
	runFakeMissRun(t, InvocationOptions{Backend: BackendFIR, Classpath: []string{"/libs/a.jar", "/libs/b.jar"}, JvmTarget: "17"})
	args := recordedArgs(t, argsFile)
	if got, _ := argValue(args, "--classpath"); got != "/libs/a.jar"+string(os.PathListSeparator)+"/libs/b.jar" {
		t.Fatalf("--classpath = %q in %v", got, args)
	}
	if got, _ := argValue(args, "--jvm-target"); got != "17" {
		t.Fatalf("--jvm-target = %q in %v", got, args)
	}
}

func TestCheckRiderDeliversTheSharedResponse(t *testing.T) {
	const response = `{"id":1,"rules":["R"]}`
	argsFile := fakeOracleJava(t, response)
	var delivered []string
	rider := riderFor([]byte(`{"id":1,"command":"check"}`), func(b []byte) { delivered = append(delivered, string(b)) })

	runFakeMissRun(t, InvocationOptions{Backend: BackendFIR, CheckRider: rider})

	args := recordedArgs(t, argsFile)
	requestPath, ok := argValue(args, "--check-request")
	if !ok {
		t.Fatalf("no --check-request in %v", args)
	}
	if len(delivered) != 1 || delivered[0] != response {
		t.Fatalf("delivered = %q", delivered)
	}
	outPath, _ := argValue(args, "--check-out")
	for _, path := range []string{requestPath, outPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("rider file %s left behind (err=%v)", path, err)
		}
	}
}

func TestCheckRiderStaysQuietWhenNotShared(t *testing.T) {
	fakeOracleJava(t, "")
	called := false
	rider := riderFor([]byte(`{}`), func([]byte) { called = true })
	runFakeMissRun(t, InvocationOptions{Backend: BackendFIR, CheckRider: rider})
	if called {
		t.Fatal("an empty --check-out was delivered")
	}
}

func TestCheckRiderOnlyRidesKritFir(t *testing.T) {
	prepared := 0
	rider := &CheckRider{Prepare: func() ([]byte, func([]byte)) { prepared++; return []byte(`{}`), func([]byte) {} }}
	for _, opts := range []InvocationOptions{
		{Backend: BackendKAA, CheckRider: rider},
		{Backend: BackendFIR},
		{Backend: BackendFIR, CheckRider: riderFor(nil, func([]byte) {})},
	} {
		run, args, err := startCheckRider(opts)
		if run != nil || args != nil || err != nil {
			t.Fatalf("startCheckRider(%+v) = %v, %v, %v", opts, run, args, err)
		}
	}
	if prepared != 0 {
		t.Fatal("prepared a rider for krit-types")
	}
}

func riderFor(request []byte, deliver func([]byte)) *CheckRider {
	return &CheckRider{Prepare: func() ([]byte, func([]byte)) { return request, deliver }}
}

func TestCheckRiderSkipsDeliveryAfterAFailedProcess(t *testing.T) {
	called := false
	run, _, err := startCheckRider(InvocationOptions{Backend: BackendFIR, CheckRider: riderFor([]byte(`{}`), func([]byte) { called = true })})
	if err != nil {
		t.Fatal(err)
	}
	defer run.cleanup()
	if err := os.WriteFile(run.outPath, []byte(`{"id":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run.finish(perf.New(false), os.ErrDeadlineExceeded)
	if called {
		t.Fatal("delivered a response from a failed oracle process")
	}
}
