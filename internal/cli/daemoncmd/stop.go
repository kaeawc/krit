package daemoncmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"syscall"
	"time"
)

var stopGrace = 5 * time.Second

func runStop(args []string) int {
	fs := flag.NewFlagSet("daemon stop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := addRepoFlag(fs)
	timeoutFlag := fs.Duration("timeout", stopGrace, "SIGTERM grace before SIGKILL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	repo, err := resolveRepo(*repoFlag)
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

	code := stopProcess(pid, *timeoutFlag)
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
