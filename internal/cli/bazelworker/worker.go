// Package bazelworker implements `krit bazel-worker`, the tool of the krit
// Bazel module's Krit actions (bazel/private/krit_aspect.bzl).
//
// Each action is a krit scan of one target. Run with --persistent_worker,
// the verb speaks Bazel's JSON persistent-worker protocol
// (https://bazel.build/remote/persistent): it reads WorkRequests from stdin
// and answers each with a WorkResponse on stdout, one at a time. Every
// request is a fresh `krit` process with an empty state directory of its
// own, so no Go-side cache or result outlives a request, but consecutive
// requests share the worker's krit-fir daemon (with incremental FIR state
// off), which keeps the JVM and the compiler warm. A daemon serves one
// compile target, so the warm path is a target analyzed again with an
// unchanged classpath (an edit to its own sources); the worker keeps only
// its newest daemon, so it never holds more than one JVM. The daemon
// watches the worker's PID (KRIT_DAEMON_PARENT_PID) and stops when it
// exits, however it exits; the worker also stops it on EOF, SIGTERM and
// SIGINT.
//
// Without --persistent_worker (workers disabled, remote execution), the
// verb runs one request from its arguments, which end in Bazel's @flagfile,
// with a one-shot krit-fir JVM that does not outlive the action.
//
// A request's arguments are `--status-file PATH` followed by krit's scan
// arguments. krit exits 1 when it reports findings; that still succeeds
// (the reports are the action's outputs) and is written to the status file
// for the action's validation. Any other non-zero exit fails the action.
package bazelworker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/kaeawc/krit/internal/selfexec"
)

// WorkRequest is the JSON form of Bazel's worker_protocol.proto WorkRequest.
type WorkRequest struct {
	Arguments  []string `json:"arguments"`
	RequestID  int      `json:"requestId"`
	Cancel     bool     `json:"cancel"`
	Verbosity  int      `json:"verbosity"`
	SandboxDir string   `json:"sandboxDir"`
}

// WorkResponse is the JSON form of Bazel's WorkResponse.
type WorkResponse struct {
	ExitCode  int    `json:"exitCode"`
	Output    string `json:"output"`
	RequestID int    `json:"requestId"`
}

// scanFunc runs one krit scan: args are krit's arguments, dir its working
// directory, env its environment. It returns krit's exit code and output.
type scanFunc func(args []string, dir string, env []string) (int, string)

// Run is the verb's entry point.
func Run(args []string) int {
	persistent := false
	var rest []string
	for _, a := range args {
		if a == "--persistent_worker" {
			persistent = true
			continue
		}
		rest = append(rest, a)
	}
	krit, err := selfexec.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit bazel-worker: %v\n", err)
		return 2
	}
	scan := execScan(krit)
	if persistent {
		return serve(os.Stdin, os.Stdout, os.Stderr, scan)
	}
	argv, err := expandFlagfiles(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit bazel-worker: %v\n", err)
		return 2
	}
	scratch, err := newScratch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit bazel-worker: %v\n", err)
		return 2
	}
	defer scratch.remove()
	code, out := handle(argv, "", scratch, false, scan)
	fmt.Fprint(os.Stderr, out)
	return code
}

// serve answers WorkRequests from in until EOF.
func serve(in io.Reader, out, logs io.Writer, scan scanFunc) int {
	scratch, err := newScratch()
	if err != nil {
		fmt.Fprintf(logs, "krit bazel-worker: %v\n", err)
		return 2
	}
	defer scratch.remove()
	defer scratch.stopDaemons(logs)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(signals)
	go func() {
		if _, ok := <-signals; ok {
			scratch.stopDaemons(logs)
			scratch.remove()
			os.Exit(1)
		}
	}()

	enc := json.NewEncoder(out)
	dec := json.NewDecoder(bufio.NewReader(in))
	for {
		var req WorkRequest
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Fprintf(logs, "krit bazel-worker: read request: %v\n", err)
			}
			break
		}
		if req.Cancel {
			// Cancellation is not supported (no supports-worker-cancellation);
			// the request finishes normally.
			continue
		}
		// One request at a time, so the daemon pruning below never stops a
		// daemon a running scan uses. Bazel runs one worker per concurrent
		// action instead.
		before := scratch.daemons()
		code, output := handle(req.Arguments, req.SandboxDir, scratch, true, scan)
		scratch.keepNewestDaemon(before, logs)
		if err := enc.Encode(WorkResponse{ExitCode: code, Output: output, RequestID: req.RequestID}); err != nil {
			fmt.Fprintf(logs, "krit bazel-worker: write response: %v\n", err)
			return 2
		}
	}
	return 0
}

// handle runs one request.
func handle(args []string, sandboxDir string, scratch *scratch, persistent bool, scan scanFunc) (int, string) {
	status, kritArgs, err := parseRequest(args)
	if err != nil {
		return 2, "krit bazel-worker: " + err.Error() + "\n"
	}
	state, err := os.MkdirTemp(scratch.dir, "state-")
	if err != nil {
		return 2, "krit bazel-worker: " + err.Error() + "\n"
	}
	defer os.RemoveAll(state)
	if !persistent {
		// No krit-fir daemon may outlive the action.
		kritArgs = append([]string{"--no-fir-daemon"}, kritArgs...)
	}
	dir := sandboxDir
	code, output := scan(kritArgs, dir, scratch.env(state))
	if code != 0 && code != 1 {
		return code, output
	}
	if dir != "" && !filepath.IsAbs(status) {
		status = filepath.Join(dir, status)
	}
	if err := os.WriteFile(status, []byte(strconv.Itoa(code)+"\n"), 0o644); err != nil {
		return 2, output + "krit bazel-worker: " + err.Error() + "\n"
	}
	return 0, output
}

// parseRequest splits `--status-file PATH [--] krit-args...`.
func parseRequest(args []string) (status string, kritArgs []string, err error) {
	if len(args) >= 2 && args[0] == "--status-file" {
		return args[1], args[2:], nil
	}
	if len(args) >= 1 && strings.HasPrefix(args[0], "--status-file=") {
		return strings.TrimPrefix(args[0], "--status-file="), args[1:], nil
	}
	return "", nil, errors.New("a request starts with --status-file PATH")
}

// expandFlagfiles replaces each @file argument with the file's lines, the
// way Bazel writes a param file (one argument per line); @@x is a literal @x.
func expandFlagfiles(args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "@@"):
			out = append(out, a[1:])
		case strings.HasPrefix(a, "@"):
			data, err := os.ReadFile(a[1:])
			if err != nil {
				return nil, fmt.Errorf("read flagfile: %w", err)
			}
			out = append(out, strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")...)
		default:
			out = append(out, a)
		}
	}
	return out, nil
}

// scratch holds a worker's per-user state: HOME, TMPDIR and KRIT_HOME (where
// the shared krit-fir daemon registers) live as long as the worker.
type scratch struct {
	dir string
	// JAVA_HOME and KRIT_FIR_JAR made absolute against the worker's start
	// directory, so a request that runs in a sandbox directory (and the
	// daemon it starts) still finds them.
	javaHome, firJar string
}

func newScratch() (*scratch, error) {
	dir, err := os.MkdirTemp("", "krit-bazel-worker-")
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"home", "tmp", "krit-home", "buildid", "no-jdk"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	s := &scratch{dir: dir}
	s.javaHome = absPath(os.Getenv("JAVA_HOME"))
	if s.javaHome == "" {
		// Go-only analysis: a JAVA_HOME without a JDK keeps krit from
		// running any JVM.
		s.javaHome = filepath.Join(dir, "no-jdk")
	}
	s.firJar = absPath(os.Getenv("KRIT_FIR_JAR"))
	return s, nil
}

func absPath(p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// env is the environment of one krit scan: the worker's environment without
// krit's per-user and per-run state variables, which point into the scratch
// directory instead, plus state, the request's own state directory.
func (s *scratch) env(state string) []string {
	drop := map[string]bool{
		"HOME": true, "TMPDIR": true, "KRIT_HOME": true, "KRIT_STATE_DIR": true,
		"KRIT_BUILDID_CACHE_BASE": true, "JAVA_HOME": true, "KRIT_FIR_JAR": true,
		"KRIT_FIR_INCREMENTAL": true, "KRIT_DAEMON_PARENT_PID": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !drop[name] {
			env = append(env, kv)
		}
	}
	env = append(env,
		"HOME="+filepath.Join(s.dir, "home"),
		"TMPDIR="+filepath.Join(s.dir, "tmp"),
		"KRIT_HOME="+filepath.Join(s.dir, "krit-home"),
		"KRIT_STATE_DIR="+state,
		"KRIT_BUILDID_CACHE_BASE="+filepath.Join(s.dir, "buildid"),
		"KRIT_NO_DAEMON_AUTOSTART=1",
		"KRIT_NO_JAR_DOWNLOAD=1",
		// The shared daemon keeps no incremental FIR state between
		// requests: each target is compiled from its own inputs only.
		"KRIT_FIR_INCREMENTAL=0",
		// The daemons stop when the worker does, even when Bazel kills it.
		"KRIT_DAEMON_PARENT_PID="+strconv.Itoa(os.Getpid()),
		"JAVA_HOME="+s.javaHome,
	)
	if s.firJar != "" {
		env = append(env, "KRIT_FIR_JAR="+s.firJar)
	}
	return env
}

// daemons lists the worker's krit-fir daemons by PID file, from their
// registry under KRIT_HOME/cache/daemons.
func (s *scratch) daemons() map[string]int {
	files, _ := filepath.Glob(filepath.Join(s.dir, "krit-home", "cache", "daemons", "*.pid"))
	out := map[string]int{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(bytes.SplitN(data, []byte("\n"), 2)[0])))
		if err == nil && pid > 0 {
			out[file] = pid
		}
	}
	return out
}

// keepNewestDaemon bounds the worker to one krit-fir JVM. A daemon serves
// one compile target (sources, classpath, Kotlin settings), so a request
// for another target starts a new one; when the request did, the older
// daemons are stopped. A request that started none reused the one daemon
// there is.
func (s *scratch) keepNewestDaemon(before map[string]int, logs io.Writer) {
	after := s.daemons()
	started := false
	for file, pid := range after {
		// A new PID file, or a daemon restarted under the same key.
		if before[file] != pid {
			started = true
		}
	}
	if !started {
		return
	}
	newest := map[int]bool{}
	for file, pid := range after {
		if before[file] != pid {
			newest[pid] = true
		}
	}
	for file, pid := range before {
		if after[file] == pid && !newest[pid] {
			stopDaemon(pid, logs)
		}
	}
}

// stopDaemons terminates all of the worker's krit-fir daemons.
func (s *scratch) stopDaemons(logs io.Writer) {
	for _, pid := range s.daemons() {
		stopDaemon(pid, logs)
	}
}

func stopDaemon(pid int, logs io.Writer) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := p.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		fmt.Fprintf(logs, "krit bazel-worker: stop krit-fir daemon %d: %v\n", pid, err)
	}
}

func (s *scratch) remove() { _ = os.RemoveAll(s.dir) }

// execScan runs krit as a subprocess.
func execScan(krit string) scanFunc {
	return func(args []string, dir string, env []string) (int, string) {
		cmd := exec.CommandContext(context.Background(), krit, args...)
		cmd.Dir = dir
		cmd.Env = env
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		var exit *exec.ExitError
		switch {
		case err == nil:
			return 0, out.String()
		case errors.As(err, &exit):
			return exit.ExitCode(), out.String()
		default:
			return 2, out.String() + "krit bazel-worker: " + err.Error() + "\n"
		}
	}
}
