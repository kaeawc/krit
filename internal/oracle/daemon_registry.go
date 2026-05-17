package oracle

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/hashutil"
)

// daemonCacheDir returns ~/.krit/cache/, creating it if needed.
func daemonCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".krit", "cache")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

// daemonsDir returns ~/.krit/cache/daemons/, creating it if needed.
// This is the directory that holds one PID file pair per distinct
// sourcesHash, enabling multiple daemons (one per repo) to coexist
// under the same user cache hierarchy.
func daemonsDir() (string, error) {
	base, err := daemonCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "daemons")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create daemons dir: %w", err)
	}
	return dir, nil
}

// daemonPIDPathForSlot returns the path to the PID file for the daemon
// serving the given sourcesHash and slot. Each repo (identified by the
// hashSources() of its source directories) has its own PID file under
// ~/.krit/cache/daemons/{hash}.pid, so multiple daemons can coexist —
// one per repo the user is actively working on.
func daemonPIDPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonPIDFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonPIDFileName(sourcesHash, slot))
}

// daemonPortPathForSlot returns the path to the port file for the daemon
// serving the given sourcesHash and slot. Sibling of daemonPIDPathForSlot.
func daemonPortPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonPortFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonPortFileName(sourcesHash, slot))
}

func daemonPIDFileName(sourcesHash string, slot int) string {
	if slot <= 0 {
		return sourcesHash + ".pid"
	}
	return fmt.Sprintf("%s.%d.pid", sourcesHash, slot)
}

func daemonPortFileName(sourcesHash string, slot int) string {
	if slot <= 0 {
		return sourcesHash + ".port"
	}
	return fmt.Sprintf("%s.%d.port", sourcesHash, slot)
}

// hashSources returns the 16-hex-char content-hash prefix of the
// sorted, newline-joined sourceDirs list. Deterministic across
// platforms — the sort makes the order of sourceDirs irrelevant to
// the hash, so "sourcesA\nsourcesB" and "sourcesB\nsourcesA" are the
// same repo.
func hashSources(sourceDirs []string) string {
	sorted := make([]string, len(sourceDirs))
	copy(sorted, sourceDirs)
	sort.Strings(sorted)
	return hashutil.HashHex([]byte(strings.Join(sorted, "\n")))[:16]
}

// pidFileInfo holds the contents of the PID file pair for one daemon.
type pidFileInfo struct {
	PID  int
	Port int
	// SourcesHash is the 16-hex-char hashSources() encoded in the
	// PID file's filename. Populated by readPIDFile from the hash
	// the caller looked up, so it's always set for successful reads.
	SourcesHash string
}

// readPIDFile reads the PID and port files for the daemon serving
// the given sourcesHash. Returns an error if either file is missing
// or unparseable. SourcesHash on the returned info is populated from
// the lookup key, not from disk.
func readPIDFile(sourcesHash string) (*pidFileInfo, error) {
	return readPIDFileSlot(sourcesHash, 0)
}

func readPIDFileSlot(sourcesHash string, slot int) (*pidFileInfo, error) {
	pidPath := daemonPIDPathForSlot(sourcesHash, slot)
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return nil, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return nil, fmt.Errorf("parse pid: %w", err)
	}

	portPath := daemonPortPathForSlot(sourcesHash, slot)
	portData, err := os.ReadFile(portPath)
	if err != nil {
		return nil, fmt.Errorf("read port file: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portData)))
	if err != nil {
		return nil, fmt.Errorf("parse port: %w", err)
	}

	return &pidFileInfo{PID: pid, Port: port, SourcesHash: sourcesHash}, nil
}

// writePIDFile records the PID and port for the daemon serving the
// given sourcesHash. Creates the daemons/ directory if needed.
func writePIDFile(pid, port int, sourcesHash string) error {
	return writePIDFileSlot(pid, port, sourcesHash, 0)
}

func writePIDFileSlot(pid, port int, sourcesHash string, slot int) error {
	pidPath := daemonPIDPathForSlot(sourcesHash, slot)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	portPath := daemonPortPathForSlot(sourcesHash, slot)
	if err := os.WriteFile(portPath, []byte(strconv.Itoa(port)+"\n"), 0644); err != nil {
		return fmt.Errorf("write port file: %w", err)
	}
	return nil
}

// removePIDFile removes the PID and port files for the daemon
// serving the given sourcesHash. Silent on missing files.
func removePIDFile(sourcesHash string) {
	removePIDFileSlot(sourcesHash, 0)
}

func removePIDFileSlot(sourcesHash string, slot int) {
	os.Remove(daemonPIDPathForSlot(sourcesHash, slot))
	os.Remove(daemonPortPathForSlot(sourcesHash, slot))
}

// isProcessAlive checks whether a process with the given PID is running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 tests process existence without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// The dial retry loop smooths over transient socket races (TIME_WAIT,
// port-reuse) when reconnecting to a recently-restarted daemon. It does
// not mask a truly down daemon — the caller falls through to kill-stale
// + start-new on exhaustion.
const (
	dialDaemonAttempts  = 3
	dialDaemonBaseDelay = 100 * time.Millisecond
)

// dialDaemonSleep is the test seam for the retry loop's backoff.
var dialDaemonSleep = func(d time.Duration) { time.Sleep(d) }

func dialDaemonAddr(addr string, timeout time.Duration) (net.Conn, error) {
	var lastErr error
	for attempt := 1; attempt <= dialDaemonAttempts; attempt++ {
		c, err := (&net.Dialer{Timeout: timeout}).DialContext(context.Background(), "tcp", addr)
		if err == nil {
			return c, nil
		}
		lastErr = err
		if attempt == dialDaemonAttempts {
			break
		}
		dialDaemonSleep(dialDaemonBaseDelay << (attempt - 1))
	}
	return nil, lastErr
}

// connectDaemonTCP dials the daemon's TCP port and returns a buffered scanner over it.
// Cleans up pidFile on connect failure.
func connectDaemonTCP(cmd *exec.Cmd, port int, srcHash string, slot int) (net.Conn, *bufio.Scanner, error) {
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		cmd.Process.Kill()
		removePIDFileSlot(srcHash, slot)
		return nil, nil, fmt.Errorf("connect to new daemon: %w", err)
	}
	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)
	return conn, reader, nil
}

// StartDaemon launches the krit-types JVM process in daemon mode.
//
// If the JVM corrupts the JSON ready line with `[cds]`/`[aot]` log chatter
// (almost always because an AppCDS/Leyden archive in the per-jar cache was
// trained against a now-missing absolute classpath), the AppCDS+Leyden
// caches for this jar are purged and the spawn is retried once. The
// second attempt sees no cache files, so the JVM takes the training path
// and starts cleanly.
func StartDaemon(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	return retryOnHandshakeNoise(jarPath, verbose, func() (*Daemon, error) {
		return startDaemonOnce(jarPath, sourceDirs, classpath, verbose)
	})
}

// retryOnHandshakeNoise runs spawn once, and if the daemon's handshake
// was overwritten by JVM unified-log chatter (a `handshakeNoiseError`),
// purges the per-jar AppCDS/Leyden caches and retries exactly once. Other
// errors (or success) are returned directly.
func retryOnHandshakeNoise(jarPath string, verbose bool, spawn func() (*Daemon, error)) (*Daemon, error) {
	d, err := spawn()
	var noise *handshakeNoiseError
	if !errors.As(err, &noise) {
		return d, err
	}
	if verbose {
		reporter().Verbosef("verbose: daemon handshake polluted by JVM log line; purging JVM caches and retrying: %s\n", noise.line)
	}
	purgeJVMCachesForJar(jarPath, verbose)
	return spawn()
}

func startDaemonOnce(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = append(args, "-jar", jarPath, "--daemon")
	if len(sourceDirs) > 0 {
		args = append(args, "--sources", strings.Join(sourceDirs, ","))
	}
	if len(classpath) > 0 {
		args = append(args, "--classpath", strings.Join(classpath, string(os.PathListSeparator)))
	}

	if verbose {
		reporter().Verbosef("verbose: Starting krit-types daemon: %s %s\n", javaPath, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	logFile := openDaemonLogFile(cmd, verbose)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

	d := &Daemon{
		cmd:     cmd,
		stdin:   stdinPipe,
		stdout:  scanner,
		logFile: logFile,
		nextID:  1,
	}

	if _, err := waitPipeReady(cmd, scanner); err != nil {
		return nil, err
	}

	d.started = true
	if verbose {
		reporter().Verbosef("verbose: krit-types daemon ready\n")
	}

	return d, nil
}

// connectExistingDaemon attempts to connect to an already-running daemon
// for the given sourceDirs. Each repo has its own PID file under
// ~/.krit/cache/daemons/{hash}.{pid,port}, so multiple daemons (one
// per repo) can coexist under the same user cache hierarchy.
func connectExistingDaemon(sourceDirs []string, verbose bool) (*Daemon, error) {
	return connectExistingDaemonSlot(sourceDirs, verbose, 0)
}

func connectExistingDaemonSlot(sourceDirs []string, verbose bool, slot int) (*Daemon, error) {
	hash := hashSources(sourceDirs)
	info, err := readPIDFileSlot(hash, slot)
	if err != nil {
		return nil, fmt.Errorf("no existing daemon: %w", err)
	}

	if !isProcessAlive(info.PID) {
		return nil, fmt.Errorf("daemon PID %d is not alive", info.PID)
	}

	// Try connecting to the TCP port. Brief retry-with-backoff smooths
	// over transient socket races; on exhaustion the caller falls through
	// to kill-stale + start-new.
	conn, err := dialDaemonAddr(fmt.Sprintf("127.0.0.1:%d", info.Port), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon port %d: %w", info.Port, err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

	d := &Daemon{
		stdin:       conn,
		stdout:      reader,
		conn:        conn,
		port:        info.Port,
		nextID:      1,
		started:     true,
		shared:      true,
		slot:        slot,
		sourcesHash: info.SourcesHash,
	}

	// Verify the daemon is responsive with a ping
	if err := d.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("daemon not responsive: %w", err)
	}

	if verbose {
		reporter().Verbosef("verbose: Reusing existing daemon slot %d (PID %d, port %d, sources %s)\n", slot, info.PID, info.Port, info.SourcesHash)
	}

	return d, nil
}

// cleanStaleDaemon detects and cleans up a stale daemon process for
// the given sourceDirs. Only touches the PID file entries belonging
// to this repo's hash — other repos' daemons are left alone.
func cleanStaleDaemon(sourceDirs []string, verbose bool) {
	cleanStaleDaemonSlot(sourceDirs, verbose, 0)
}

func cleanStaleDaemonSlot(sourceDirs []string, verbose bool, slot int) {
	hash := hashSources(sourceDirs)
	info, err := readPIDFileSlot(hash, slot)
	if err != nil {
		// No PID file or unreadable — nothing to clean
		removePIDFileSlot(hash, slot)
		return
	}

	if !isProcessAlive(info.PID) {
		if verbose {
			reporter().Verbosef("verbose: Cleaning up stale daemon slot %d PID file (PID %d no longer alive)\n", slot, info.PID)
		}
		removePIDFileSlot(hash, slot)
		return
	}

	// Process is alive but we couldn't connect — it's stuck. Kill it.
	if verbose {
		reporter().Verbosef("verbose: Killing unresponsive daemon slot %d (PID %d)\n", slot, info.PID)
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		removePIDFileSlot(hash, slot)
		return
	}

	// Try SIGTERM first, then SIGKILL after timeout
	proc.Signal(syscall.SIGTERM)

	// Wait up to 5 seconds for graceful exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(info.PID) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Force kill if still alive
	if isProcessAlive(info.PID) {
		proc.Signal(syscall.SIGKILL)
		time.Sleep(500 * time.Millisecond)
	}

	removePIDFileSlot(hash, slot)
}

// StartDaemonWithPort launches the krit-types JVM process in daemon mode with
// a TCP listener. The daemon auto-assigns a port and reports it on stdout.
func StartDaemonWithPort(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	return StartDaemonWithPortSlot(jarPath, sourceDirs, classpath, verbose, 0)
}

func StartDaemonWithPortSlot(jarPath string, sourceDirs []string, classpath []string, verbose bool, slot int) (*Daemon, error) {
	return retryOnHandshakeNoise(jarPath, verbose, func() (*Daemon, error) {
		return startDaemonWithPortSlotOnce(jarPath, sourceDirs, classpath, verbose, slot)
	})
}

func startDaemonWithPortSlotOnce(jarPath string, sourceDirs []string, classpath []string, verbose bool, slot int) (*Daemon, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = append(args, "-jar", jarPath, "--daemon", "--port", "0")
	if len(sourceDirs) > 0 {
		args = append(args, "--sources", strings.Join(sourceDirs, ","))
	}
	if len(classpath) > 0 {
		args = append(args, "--classpath", strings.Join(classpath, string(os.PathListSeparator)))
	}

	if verbose {
		reporter().Verbosef("verbose: Starting persistent krit-types daemon slot %d: %s %s\n", slot, javaPath, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	logFile := openDaemonLogFile(cmd, verbose)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	ready, err := waitPortReady(cmd, stdoutPipe)
	if err != nil {
		return nil, err
	}

	if verbose {
		reporter().Verbosef("verbose: Daemon started on port %d (PID %d)\n", ready.Port, cmd.Process.Pid)
	}

	srcHash := hashSources(sourceDirs)

	if err := writePIDFileSlot(cmd.Process.Pid, ready.Port, srcHash, slot); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("write PID file: %w", err)
	}

	conn, reader, err := connectDaemonTCP(cmd, ready.Port, srcHash, slot)
	if err != nil {
		return nil, err
	}

	d := &Daemon{
		cmd:         cmd,
		stdin:       conn,
		stdout:      reader,
		conn:        conn,
		logFile:     logFile,
		port:        ready.Port,
		nextID:      1,
		started:     true,
		shared:      false,
		slot:        slot,
		sourcesHash: srcHash,
	}

	return d, nil
}

// ConnectOrStartDaemon tries to reuse an existing persistent daemon
// for the given sourceDirs. Each repo has its own PID file under
// ~/.krit/cache/daemons/{hash}.{pid,port} so multiple daemons (one per
// repo) can coexist; krit invocations targeting different repos don't
// fight over a shared PID file. If no daemon is running (or the
// existing one is unresponsive), cleanStaleDaemon wipes just this
// repo's entry and StartDaemonWithPort starts a fresh one.
func ConnectOrStartDaemon(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	// Try connecting to an existing daemon for this source tree.
	if d, err := connectExistingDaemon(sourceDirs, verbose); err == nil {
		return d, nil
	}

	// Clean up this repo's stale daemon entry (if any). Leaves other
	// repos' entries under daemons/ untouched.
	cleanStaleDaemon(sourceDirs, verbose)

	// Start a new persistent daemon for this source tree.
	d, err := StartDaemonWithPort(jarPath, sourceDirs, classpath, verbose)
	if err != nil {
		return nil, fmt.Errorf("start persistent daemon: %w", err)
	}

	return d, nil
}
