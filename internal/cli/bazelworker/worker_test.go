package bazelworker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// fakeScan records each scan and exits with the code its first argument names.
type fakeScan struct {
	mu     sync.Mutex
	states []string
	args   [][]string
}

func (f *fakeScan) run(args []string, _ string, env []string) (int, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.args = append(f.args, args)
	for _, kv := range env {
		if state, ok := strings.CutPrefix(kv, "KRIT_STATE_DIR="); ok {
			if _, err := os.Stat(state); err != nil {
				return 2, "state dir missing during the scan"
			}
			f.states = append(f.states, state)
		}
	}
	for _, a := range args {
		switch a {
		case "findings":
			return 1, "1 finding\n"
		case "crash":
			return 2, "internal error\n"
		}
	}
	return 0, "clean\n"
}

func requests(t *testing.T, reqs ...WorkRequest) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	for _, r := range reqs {
		if err := json.NewEncoder(&buf).Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	return &buf
}

func responses(t *testing.T, out *bytes.Buffer) map[int]WorkResponse {
	t.Helper()
	got := map[int]WorkResponse{}
	dec := json.NewDecoder(out)
	for dec.More() {
		var r WorkResponse
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		got[r.RequestID] = r
	}
	return got
}

func TestServeWritesStatusAndIsolatesState(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.status")
	findings := filepath.Join(dir, "findings.status")
	scan := &fakeScan{}
	var out, logs bytes.Buffer
	code := serve(requests(t,
		WorkRequest{RequestID: 1, Arguments: []string{"--status-file", clean, "--", "a.kt"}},
		WorkRequest{RequestID: 2, Arguments: []string{"--status-file=" + findings, "findings"}},
		WorkRequest{RequestID: 3, Arguments: []string{"--status-file", filepath.Join(dir, "x"), "crash"}},
		WorkRequest{RequestID: 4, Arguments: []string{"no-status"}},
	), &out, &logs, scan.run)
	if code != 0 {
		t.Fatalf("serve = %d, logs %s", code, logs.String())
	}
	got := responses(t, &out)
	if len(got) != 4 {
		t.Fatalf("responses: %+v", got)
	}
	if got[1].ExitCode != 0 || got[2].ExitCode != 0 {
		t.Fatalf("clean and findings requests must succeed: %+v", got)
	}
	if got[3].ExitCode != 2 || !strings.Contains(got[3].Output, "internal error") {
		t.Fatalf("a krit failure must fail the request: %+v", got[3])
	}
	if got[4].ExitCode != 2 || !strings.Contains(got[4].Output, "--status-file") {
		t.Fatalf("a request without --status-file must fail: %+v", got[4])
	}
	for path, want := range map[string]string{clean: "0\n", findings: "1\n"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v; want %q", path, data, err, want)
		}
	}
	// Every request had a state directory of its own, gone afterwards.
	sort.Strings(scan.states)
	for i, state := range scan.states {
		if i > 0 && state == scan.states[i-1] {
			t.Fatalf("two requests shared state dir %s", state)
		}
		if _, err := os.Stat(state); !os.IsNotExist(err) {
			t.Fatalf("state dir %s outlived its request", state)
		}
	}
	// A persistent worker keeps the krit-fir daemon between requests.
	for _, args := range scan.args {
		for _, a := range args {
			if a == "--no-fir-daemon" {
				t.Fatalf("persistent request ran with --no-fir-daemon: %v", args)
			}
		}
	}
}

func TestServeSingleplexAnswersInOrder(t *testing.T) {
	dir := t.TempDir()
	scan := &fakeScan{}
	var out, logs bytes.Buffer
	serve(requests(t,
		WorkRequest{Arguments: []string{"--status-file", filepath.Join(dir, "a"), "a.kt"}},
		WorkRequest{Arguments: []string{"--status-file", filepath.Join(dir, "b"), "findings"}},
	), &out, &logs, scan.run)
	dec := json.NewDecoder(&out)
	for i := 0; i < 2; i++ {
		var r WorkResponse
		if err := dec.Decode(&r); err != nil || r.ExitCode != 0 || r.RequestID != 0 {
			t.Fatalf("response %d = %+v, %v", i, r, err)
		}
	}
}

func TestHandleSandboxDirStatus(t *testing.T) {
	sandbox := t.TempDir()
	s, err := newScratch()
	if err != nil {
		t.Fatal(err)
	}
	defer s.remove()
	scan := &fakeScan{}
	code, _ := handle([]string{"--status-file", "out/x.status", "a.kt"}, sandbox, s, true, scan.run)
	if code != 2 {
		// out/ does not exist in the sandbox: the write must fail the request.
		t.Fatalf("missing output dir: code %d", code)
	}
	if err := os.MkdirAll(filepath.Join(sandbox, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, out := handle([]string{"--status-file", "out/x.status", "a.kt"}, sandbox, s, true, scan.run); code != 0 {
		t.Fatalf("code %d: %s", code, out)
	}
	if data, err := os.ReadFile(filepath.Join(sandbox, "out", "x.status")); err != nil || string(data) != "0\n" {
		t.Fatalf("status in sandbox = %q, %v", data, err)
	}
}

func TestOneShotDisablesDaemon(t *testing.T) {
	s, err := newScratch()
	if err != nil {
		t.Fatal(err)
	}
	defer s.remove()
	scan := &fakeScan{}
	status := filepath.Join(t.TempDir(), "s")
	if code, out := handle([]string{"--status-file", status, "a.kt"}, "", s, false, scan.run); code != 0 {
		t.Fatalf("code %d: %s", code, out)
	}
	if len(scan.args) != 1 || scan.args[0][0] != "--no-fir-daemon" {
		t.Fatalf("one-shot args = %v, want --no-fir-daemon first", scan.args)
	}
}

func TestExpandFlagfiles(t *testing.T) {
	file := filepath.Join(t.TempDir(), "params")
	if err := os.WriteFile(file, []byte("--status-file\nout/s\n--\na b.kt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := expandFlagfiles([]string{"--x", "@" + file, "@@literal"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--x", "--status-file", "out/s", "--", "a b.kt", "@literal"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := expandFlagfiles([]string{"@" + file + ".missing"}); err == nil {
		t.Fatal("a missing flagfile must be an error")
	}
}

func TestScanEnvPointsStateIntoScratch(t *testing.T) {
	t.Setenv("JAVA_HOME", "rel/jdk")
	t.Setenv("KRIT_FIR_JAR", "rel/krit-fir.jar")
	t.Setenv("KRIT_STATE_DIR", "/elsewhere")
	s, err := newScratch()
	if err != nil {
		t.Fatal(err)
	}
	defer s.remove()
	env := map[string]string{}
	for _, kv := range s.env("/state") {
		name, value, _ := strings.Cut(kv, "=")
		if _, dup := env[name]; dup {
			t.Fatalf("%s set twice", name)
		}
		env[name] = value
	}
	if env["KRIT_STATE_DIR"] != "/state" || env["KRIT_FIR_INCREMENTAL"] != "0" || env["KRIT_NO_JAR_DOWNLOAD"] != "1" ||
		env["KRIT_DAEMON_PARENT_PID"] != strconv.Itoa(os.Getpid()) {
		t.Fatalf("env = %v", env)
	}
	for _, name := range []string{"HOME", "TMPDIR", "KRIT_HOME"} {
		if !strings.HasPrefix(env[name], s.dir) {
			t.Fatalf("%s = %q, want it under %s", name, env[name], s.dir)
		}
	}
	if !filepath.IsAbs(env["JAVA_HOME"]) || !filepath.IsAbs(env["KRIT_FIR_JAR"]) {
		t.Fatalf("JAVA_HOME/KRIT_FIR_JAR must be absolute: %q %q", env["JAVA_HOME"], env["KRIT_FIR_JAR"])
	}
}

// sleeper starts a process that stands in for a krit-fir daemon.
func sleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	return cmd
}

func registerDaemon(t *testing.T, s *scratch, name string, pid int) {
	t.Helper()
	dir := filepath.Join(s.dir, "krit-home", "cache", "daemons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestKeepNewestDaemonBoundsTheWorkerToOneJVM(t *testing.T) {
	s, err := newScratch()
	if err != nil {
		t.Fatal(err)
	}
	defer s.remove()
	old := sleeper(t)
	registerDaemon(t, s, "check-old", old.Process.Pid)

	// A request that reused the daemon stops nothing.
	before := s.daemons()
	s.keepNewestDaemon(before, io.Discard)
	if err := old.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("a reused daemon was stopped: %v", err)
	}

	// A request that started a daemon for another target stops the old one.
	before = s.daemons()
	newer := sleeper(t)
	registerDaemon(t, s, "check-new", newer.Process.Pid)
	s.keepNewestDaemon(before, io.Discard)
	if _, err := old.Process.Wait(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("wait old daemon: %v", err)
		}
	}
	if err := newer.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("the newest daemon was stopped: %v", err)
	}
}
