package daemoncmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/sessdaemon"
)

// ExitForceKill is the exit code stop returns when SIGTERM did not
// take effect within the grace window and SIGKILL had to take over.
const ExitForceKill = 75

var (
	startWaitTimeout = 30 * time.Second
	startPollEvery   = 50 * time.Millisecond
)

func runStart(args []string) int {
	fs := flag.NewFlagSet("daemon start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := addRepoFlag(fs)
	socketFlag := fs.String("socket", "", "socket path (defaults to <repo>/.krit/daemon.sock)")
	binaryFlag := fs.String("binary", "", "krit-daemon binary path (defaults to one next to krit)")
	timeoutFlag := fs.Duration("timeout", startWaitTimeout, "how long to wait for the daemon to become ready")
	idleTimeoutFlag := fs.Duration("idle-timeout", 0,
		"daemon exits after this duration of no requests (e.g. 30m); 0 disables auto-shutdown")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *idleTimeoutFlag < 0 {
		fmt.Fprintf(os.Stderr, "krit daemon start: --idle-timeout must be >= 0 (got %s)\n", *idleTimeoutFlag)
		return 2
	}

	repo, err := resolveRepo(*repoFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: %v\n", err)
		return 2
	}
	socket := *socketFlag
	if socket == "" {
		socket = sessdaemon.DefaultSocketPath(repo)
	}

	if st := collectStatus(repo); st.Running {
		fmt.Fprintf(os.Stdout, "krit-daemon already running, PID %d\n", st.PID)
		return 0
	}

	pidPath := pidFilePath(repo)
	if err := removePIDFile(pidPath); err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: remove stale pid file: %v\n", err)
		return 1
	}

	binary, err := resolveDaemonBinary(*binaryFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: %v\n", err)
		return 1
	}

	pid, err := spawnDaemon(binary, repo, socket, *idleTimeoutFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: spawn: %v\n", err)
		return 1
	}
	if err := writePIDFile(pidPath, pid); err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: write pid file: %v\n", err)
		return 1
	}
	if err := waitForReady(socket, pid, *timeoutFlag); err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon start: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "krit-daemon started, PID %d, socket %s\n", pid, socket)
	return 0
}

func resolveDaemonBinary(explicit string) (string, error) {
	if explicit != "" {
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve --binary: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("--binary %s: %w", abs, err)
		}
		return abs, nil
	}
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "krit-daemon")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}
	found, err := exec.LookPath("krit-daemon")
	if err != nil {
		return "", errors.New("krit-daemon binary not found next to krit or on PATH; pass --binary")
	}
	return found, nil
}

// spawnDaemon launches krit-daemon detached via Setsid so a SIGHUP on
// the parent terminal (e.g. closing the shell) doesn't take the
// daemon down with it.
func spawnDaemon(binary, repo, socket string, idleTimeout time.Duration) (int, error) {
	dotKrit := filepath.Join(repo, ".krit")
	if err := os.MkdirAll(dotKrit, 0o755); err != nil {
		return 0, fmt.Errorf("prepare .krit dir: %w", err)
	}
	log, err := os.OpenFile(filepath.Join(dotKrit, "daemon.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open daemon log: %w", err)
	}
	// The child inherits its own fd to the log; close ours after Start.
	defer log.Close()

	args := []string{"--repo", repo}
	if socket != "" {
		args = append(args, "--socket", socket)
	}
	if idleTimeout > 0 {
		args = append(args, "--idle-timeout", idleTimeout.String())
	}
	cmd := exec.CommandContext(context.Background(), binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// Reap in the background so a fast-failing spawn doesn't leak a zombie.
	go func() { _ = cmd.Wait() }()
	return cmd.Process.Pid, nil
}

// waitForReady polls the health verb until the daemon answers or the
// timeout expires. If the spawned process dies first, we surface that
// directly so operators look at the daemon log instead of waiting out
// the timeout.
func waitForReady(socket string, pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return fmt.Errorf("daemon process %d exited before becoming ready (see .krit/daemon.log)", pid)
		}
		if _, err := sessdaemon.Health(socket); err == nil {
			return nil
		}
		time.Sleep(startPollEvery)
	}
	return fmt.Errorf("daemon did not become ready within %s", timeout)
}
