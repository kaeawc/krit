package daemoncmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

var stopGrace = 5 * time.Second

func runStop(args []string) int {
	fs := flag.NewFlagSet("daemon stop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := addRepoFlag(fs)
	timeoutFlag := fs.Duration("timeout", stopGrace, "SIGTERM grace before SIGKILL")
	allFlag := fs.Bool("all", false, "also stop every krit-fir and oracle JVM helper in the daemon registry (all projects)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *allFlag {
		code := stopRepoDaemon(*repoFlag, *timeoutFlag)
		stopHelpers(os.Stdout)
		return code
	}
	return stopRepoDaemon(*repoFlag, *timeoutFlag)
}

// stopHelpers stops the JVM helpers recorded in the daemon registry. They are
// shared across projects and outlive the CLI by design (see docs/perf.md);
// each package's stop path signals a PID only when it proves the process is
// the recorded helper.
func stopHelpers(w io.Writer) {
	firStopped, firLeft := firchecks.StopRegisteredDaemons(false)
	oracleStopped, oracleLeft := oracle.StopRegisteredDaemons(false)
	fmt.Fprintf(w, "krit-fir helpers: %d stopped or cleared, %d left (busy or unproven)\n", firStopped, firLeft)
	fmt.Fprintf(w, "oracle helpers: %d stopped or cleared, %d left (busy or unproven)\n", oracleStopped, oracleLeft)
}

// stopRepoDaemon stops the Go project daemon serving repoFlag (or the cwd).
func stopRepoDaemon(repoFlag string, timeout time.Duration) int {

	repo, err := resolveRepo(repoFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon stop: %v\n", err)
		return 2
	}

	pidPath := pidFilePath(repo)
	pid, err := readPIDFile(pidPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stdout, "krit-daemon: not running")
			return 0
		}
		fmt.Fprintf(os.Stderr, "krit daemon stop: %v\n", err)
		return 1
	}

	if !processAlive(pid) {
		_ = removePIDFile(pidPath)
		fmt.Fprintln(os.Stdout, "krit-daemon: not running (stale pid removed)")
		return 0
	}

	if ok, reason := verifyDaemonPID(repo, pid); !ok {
		fmt.Fprintf(os.Stderr, "krit daemon stop: %s names a process that is not this repo's krit daemon: %s\n"+
			"  Refusing to signal pid %d. If the pid file is stale or foreign, remove it with: rm %s\n",
			pidPath, reason, pid, pidPath)
		return 1
	}

	code := stopProcess(pid, timeout)
	_ = removePIDFile(pidPath)
	return code
}

// stopProcess sends SIGTERM, waits up to grace, then SIGKILLs if the
// process is still alive. Returns 0 for a graceful exit, ExitForceKill
// when SIGKILL was required.
func stopProcess(pid int, grace time.Duration) int {
	p, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon stop: find pid %d: %v\n", pid, err)
		return 1
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "krit daemon stop: SIGTERM pid %d: %v\n", pid, err)
		return 1
	}
	if waitForExit(pid, grace) {
		return 0
	}
	fmt.Fprintf(os.Stderr, "krit daemon stop: pid %d did not exit within %s, sending SIGKILL\n", pid, grace)
	_ = p.Signal(syscall.SIGKILL)
	waitForExit(pid, 2*time.Second)
	return ExitForceKill
}

func waitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return !processAlive(pid)
}
