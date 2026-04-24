package firchecks

// client.go — transport layer for the krit-fir daemon.
//
// Mirrors internal/oracle/daemon.go: spawns java -jar krit-fir.jar
// --daemon --port 0, reads the JSON readiness handshake, keeps the TCP
// connection open for reuse. Daemon PID/port files live at
//   ~/.krit/cache/daemons/{sourcesHash}.krit-fir.{pid,port}
// so multiple repos can keep two warm daemons without colliding.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/hashutil"
)

// FirDaemon manages a long-lived krit-fir JVM process.
type FirDaemon struct {
	cmd     *exec.Cmd
	conn    net.Conn
	reader  *bufio.Scanner
	logFile *os.File
	mu      sync.Mutex
	port    int
	nextID  int64
	started bool
	shared  bool
	slot    int
	// sourcesHash is the 16-hex-char fingerprint of sourceDirs this daemon serves.
	sourcesHash string
}

// MatchesRepo returns true if this daemon was started for the given sourceDirs.
func (d *FirDaemon) MatchesRepo(sourceDirs []string) bool {
	if d.sourcesHash == "" {
		return false
	}
	return d.sourcesHash == hashFirSources(sourceDirs)
}

// firDaemonRequest is the JSON shape sent to the krit-fir daemon.
type firDaemonRequest struct {
	ID         int64      `json:"id"`
	Command    string     `json:"command"`
	Files      []fileRef  `json:"files,omitempty"`
	SourceDirs []string   `json:"sourceDirs,omitempty"`
	Classpath  []string   `json:"classpath,omitempty"`
	Rules      []string   `json:"rules,omitempty"`
}

// fileRef is a file path + content hash sent in check requests.
type fileRef struct {
	Path        string `json:"path"`
	ContentHash string `json:"contentHash,omitempty"`
}

// firReadyMessage is the JSON sent by the daemon on startup.
type firReadyMessage struct {
	Ready bool `json:"ready"`
	Port  int  `json:"port"`
}

// daemonRequestTimeout returns the max duration for a single round-trip.
func daemonRequestTimeout() time.Duration {
	if v := os.Getenv("KRIT_FIR_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Minute
}

// StartFirDaemonWithPort launches krit-fir.jar in TCP daemon mode.
func StartFirDaemonWithPort(jarPath string, verbose bool) (*FirDaemon, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := []string{
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms512m",
		"-jar", jarPath,
		"--daemon", "--port", "0",
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Starting krit-fir daemon: %s %s\n", javaPath, strings.Join(args, " "))
	}

	cmd := exec.Command(javaPath, args...)
	logPath := filepath.Join(os.TempDir(), "krit-fir-daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = logFile
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: FIR daemon log: %s\n", logPath)
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start fir daemon: %w", err)
	}

	type scanResult struct {
		line string
		err  error
	}
	readyCh := make(chan scanResult, 1)
	go func() {
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		if sc.Scan() {
			readyCh <- scanResult{line: sc.Text()}
		} else {
			readyCh <- scanResult{err: sc.Err()}
		}
	}()

	const startupTimeout = 30 * time.Second
	var line string
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, fmt.Errorf("fir daemon startup: %w", res.err)
		}
		if res.line == "" {
			cmd.Process.Kill()
			return nil, fmt.Errorf("fir daemon closed stdout before ready")
		}
		line = res.line
	case <-time.After(startupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("fir daemon startup timed out after %s", startupTimeout)
	}

	var ready firReadyMessage
	if err := json.Unmarshal([]byte(line), &ready); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("fir daemon ready message: invalid JSON: %w (got: %s)", err, line)
	}
	if !ready.Ready || ready.Port == 0 {
		cmd.Process.Kill()
		return nil, fmt.Errorf("fir daemon did not report ready with port (got: %s)", line)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: krit-fir daemon started on port %d (PID %d)\n", ready.Port, cmd.Process.Pid)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ready.Port), 5*time.Second)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("connect to fir daemon port %d: %w", ready.Port, err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	d := &FirDaemon{
		cmd:     cmd,
		conn:    conn,
		reader:  reader,
		logFile: logFile,
		port:    ready.Port,
		nextID:  1,
		started: true,
		shared:  false,
		slot:    0,
	}
	return d, nil
}

// ConnectOrStartFirDaemon tries to reuse an existing daemon for the given
// sourceDirs (via PID file), or starts a new one.
func ConnectOrStartFirDaemon(jarPath string, sourceDirs []string, verbose bool) (*FirDaemon, error) {
	if d, err := connectExistingFirDaemon(sourceDirs, verbose); err == nil {
		return d, nil
	}
	cleanStaleFirDaemon(sourceDirs, verbose)
	d, err := StartFirDaemonWithPort(jarPath, verbose)
	if err != nil {
		return nil, fmt.Errorf("start persistent fir daemon: %w", err)
	}
	srcHash := hashFirSources(sourceDirs)
	d.sourcesHash = srcHash
	if err := writeFirPIDFile(d.cmd.Process.Pid, d.port, srcHash); err != nil {
		d.conn.Close()
		d.cmd.Process.Kill()
		return nil, fmt.Errorf("write fir PID file: %w", err)
	}
	return d, nil
}

// Check sends a check request to the daemon and returns the response.
func (d *FirDaemon) Check(files []fileRef, sourceDirs, classpath, rules []string) (*CheckResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil, fmt.Errorf("fir daemon not started")
	}

	id := d.nextID
	d.nextID++

	req := firDaemonRequest{
		ID:         id,
		Command:    "check",
		Files:      files,
		SourceDirs: sourceDirs,
		Classpath:  classpath,
		Rules:      rules,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal fir request: %w", err)
	}
	data = append(data, '\n')
	if _, err := d.conn.Write(data); err != nil {
		return nil, fmt.Errorf("write to fir daemon: %w", err)
	}

	type scanResult struct {
		line string
		ok   bool
		err  error
	}
	resultCh := make(chan scanResult, 1)
	go func() {
		if d.reader.Scan() {
			resultCh <- scanResult{line: d.reader.Text(), ok: true}
			return
		}
		resultCh <- scanResult{err: d.reader.Err()}
	}()

	timeout := daemonRequestTimeout()
	var line string
	select {
	case res := <-resultCh:
		if !res.ok {
			if res.err != nil {
				return nil, fmt.Errorf("read from fir daemon: %w", res.err)
			}
			return nil, fmt.Errorf("fir daemon closed stdout unexpectedly")
		}
		line = res.line
	case <-time.After(timeout):
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		d.started = false
		return nil, fmt.Errorf("fir daemon request timed out after %s; daemon killed", timeout)
	}

	var resp CheckResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal fir response: %w (got: %s)", err, line)
	}
	if resp.ID != id {
		return nil, fmt.Errorf("fir response ID mismatch: expected %d, got %d", id, resp.ID)
	}
	return &resp, nil
}

// Ping verifies the daemon is responsive.
func (d *FirDaemon) Ping() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return fmt.Errorf("fir daemon not started")
	}
	id := d.nextID
	d.nextID++
	req := firDaemonRequest{ID: id, Command: "ping"}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := d.conn.Write(data); err != nil {
		return fmt.Errorf("write ping: %w", err)
	}
	type scanResult struct {
		line string
		ok   bool
		err  error
	}
	ch := make(chan scanResult, 1)
	go func() {
		if d.reader.Scan() {
			ch <- scanResult{line: d.reader.Text(), ok: true}
			return
		}
		ch <- scanResult{err: d.reader.Err()}
	}()
	select {
	case res := <-ch:
		if !res.ok {
			return fmt.Errorf("fir daemon not responsive")
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("fir daemon ping timed out")
	}
}

// Release drops this Go-side handle but leaves the daemon process alive.
func (d *FirDaemon) Release() error {
	d.mu.Lock()
	d.started = false
	d.mu.Unlock()
	if d.conn != nil {
		d.conn.Close()
	}
	if d.logFile != nil {
		d.logFile.Close()
	}
	return nil
}

// Close shuts the daemon down completely.
func (d *FirDaemon) Close() error {
	if d.shared {
		d.mu.Lock()
		d.started = false
		d.mu.Unlock()
		if d.conn != nil {
			d.conn.Close()
		}
		return nil
	}
	if d.started {
		d.mu.Lock()
		if d.started {
			req := firDaemonRequest{ID: d.nextID, Command: "shutdown"}
			d.nextID++
			if data, err := json.Marshal(req); err == nil {
				data = append(data, '\n')
				d.conn.Write(data) //nolint:errcheck
			}
			d.started = false
		}
		d.mu.Unlock()

		done := make(chan error, 1)
		go func() { done <- d.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			if d.cmd != nil && d.cmd.Process != nil {
				d.cmd.Process.Kill()
			}
		}
	}
	if d.port != 0 && d.sourcesHash != "" {
		removeFirPIDFile(d.sourcesHash)
	}
	if d.conn != nil {
		d.conn.Close()
	}
	if d.logFile != nil {
		d.logFile.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// PID file helpers (mirrors oracle/daemon.go pattern, namespaced as .krit-fir)
// ---------------------------------------------------------------------------

func firDaemonsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".krit", "cache", "daemons")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create daemons dir: %w", err)
	}
	return dir, nil
}

func firPIDPath(sourcesHash string) string {
	dir, err := firDaemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-fir-"+sourcesHash+".pid")
	}
	return filepath.Join(dir, sourcesHash+".krit-fir.pid")
}

func firPortPath(sourcesHash string) string {
	dir, err := firDaemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-fir-"+sourcesHash+".port")
	}
	return filepath.Join(dir, sourcesHash+".krit-fir.port")
}

func writeFirPIDFile(pid, port int, sourcesHash string) error {
	if err := os.WriteFile(firPIDPath(sourcesHash), []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		return fmt.Errorf("write fir pid: %w", err)
	}
	if err := os.WriteFile(firPortPath(sourcesHash), []byte(strconv.Itoa(port)+"\n"), 0644); err != nil {
		return fmt.Errorf("write fir port: %w", err)
	}
	return nil
}

func removeFirPIDFile(sourcesHash string) {
	os.Remove(firPIDPath(sourcesHash))
	os.Remove(firPortPath(sourcesHash))
}

func connectExistingFirDaemon(sourceDirs []string, verbose bool) (*FirDaemon, error) {
	hash := hashFirSources(sourceDirs)
	pidData, err := os.ReadFile(firPIDPath(hash))
	if err != nil {
		return nil, fmt.Errorf("no existing fir daemon: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return nil, fmt.Errorf("parse fir pid: %w", err)
	}
	portData, err := os.ReadFile(firPortPath(hash))
	if err != nil {
		return nil, fmt.Errorf("read fir port: %w", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portData)))
	if err != nil {
		return nil, fmt.Errorf("parse fir port: %w", err)
	}

	proc, ferr := os.FindProcess(pid)
	if ferr != nil || proc.Signal(syscall.Signal(0)) != nil {
		return nil, fmt.Errorf("fir daemon PID %d not alive", pid)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to fir daemon port %d: %w", port, err)
	}
	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	d := &FirDaemon{
		conn:        conn,
		reader:      reader,
		port:        port,
		nextID:      1,
		started:     true,
		shared:      true,
		sourcesHash: hash,
	}
	if err := d.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("fir daemon not responsive: %w", err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Reusing existing krit-fir daemon (PID %d, port %d, sources %s)\n", pid, port, hash)
	}
	return d, nil
}

func cleanStaleFirDaemon(sourceDirs []string, verbose bool) {
	hash := hashFirSources(sourceDirs)
	pidData, err := os.ReadFile(firPIDPath(hash))
	if err != nil {
		removeFirPIDFile(hash)
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		removeFirPIDFile(hash)
		return
	}
	proc, ferr := os.FindProcess(pid)
	if ferr != nil || proc.Signal(syscall.Signal(0)) != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: Cleaning stale krit-fir daemon PID file (PID %d not alive)\n", pid)
		}
		removeFirPIDFile(hash)
		return
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Killing unresponsive krit-fir daemon (PID %d)\n", pid)
	}
	proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if proc.Signal(syscall.Signal(0)) == nil {
		proc.Signal(syscall.SIGKILL)
		time.Sleep(500 * time.Millisecond)
	}
	removeFirPIDFile(hash)
}

// hashFirSources returns a 16-hex-char fingerprint of sorted sourceDirs.
func hashFirSources(sourceDirs []string) string {
	sorted := make([]string, len(sourceDirs))
	copy(sorted, sourceDirs)
	sort.Strings(sorted)
	return hashutil.HashHex([]byte(strings.Join(sorted, "\n")))[:16]
}
