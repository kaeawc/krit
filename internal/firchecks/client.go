package firchecks

// client.go — transport layer for the krit-fir daemon.
//
// Mirrors internal/oracle/daemon.go: spawns java -jar krit-fir.jar
// --daemon --port 0, reads the JSON readiness handshake, keeps the TCP
// connection open for reuse. Daemon PID/port files live at
//   ~/.krit/cache/daemons/{sourcesHash}-{pathTag}@{identity}.krit-fir.{pid,port}
// so multiple repos can keep two warm daemons without colliding.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/fslock"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/jvmaot"
	"github.com/kaeawc/krit/internal/oracle"
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
	// aotCachePath is persisted with the PID registry for reconnecting clients.
	aotCachePath string
	aotCheckOnce sync.Once
	// aotVerified records that the AOT canary passed for this daemon's JVM,
	// here or in the krit run that started it (persisted with the registry).
	aotVerified bool
	aotPID      int
	// unpublished marks an AOT-launched daemon kept out of the registry until
	// its canary passes. Release closes it, and Close leaves the registry
	// alone, while it is set.
	unpublished bool
	// sourcesHash is the registry key (role, jar, sourceDirs, classpath)
	// this daemon serves.
	sourcesHash string
	// role separates the checker daemon from the oracle-backend daemon; see
	// firCheckRole.
	role      string
	jvmTarget string
}

// MatchesRepo returns true if this daemon uses the current jar, sourceDirs,
// and classpath.
func (d *FirDaemon) MatchesRepo(jarPath string, sourceDirs []string, classpath ...string) bool {
	if d.sourcesHash == "" {
		return false
	}
	return d.sourcesHash == firRegistryKeyFor(d.role, jarPath, sourceDirs, classpath, d.jvmTarget)
}

// firDaemonRequest is the JSON shape sent to the krit-fir daemon.
type firDaemonRequest struct {
	ID         int64     `json:"id"`
	Command    string    `json:"command"`
	Files      []fileRef `json:"files,omitempty"`
	SourceDirs []string  `json:"sourceDirs,omitempty"`
	Classpath  []string  `json:"classpath,omitempty"`
	JvmTarget  string    `json:"jvmTarget,omitempty"`
	Rules      []string  `json:"rules,omitempty"`
	// TestFiles is the subset of Files (spelled exactly as in Files) that
	// krit classifies as test sources (scanner.IsTestFile, honoring the
	// configured test paths); checkers read it through FirRule.isTestFile.
	TestFiles []string `json:"testFiles,omitempty"`
	// ScanPaths maps a file (spelled exactly as in Files) to the scan's own
	// spelling of it, the path string the Go rules test, when the two
	// differ; checkers read it through FirRule.scanPath.
	ScanPaths map[string]string `json:"scanPaths,omitempty"`
	// SDKLevels maps a file (spelled exactly as in Files) to the minSdk /
	// targetSdk Go resolved for it, for the files with a known level;
	// checkers read it through FirRule.minSdkFor / FirRule.targetSdkFor.
	SDKLevels map[string]android.SDKLevels `json:"sdkLevels,omitempty"`
	// RuleConfigs is rule ID -> options, read by FirRule.config(). It stays
	// after the fixed fields; krit-fir also blanks it out before its
	// nest-blind field extraction so option names cannot shadow them.
	RuleConfigs map[string]any `json:"ruleConfigs,omitempty"`
}

// FileFacts is what Go knows about the requested files that the checkers
// need to decide like the Go rules do. Every path is spelled exactly as in
// the request's files.
type FileFacts struct {
	// TestFiles are the requested files krit classifies as test sources.
	TestFiles []string
	// ScanPaths maps a requested file to the scan's own spelling of it
	// (usually relative to the working directory), for files whose scan
	// spelling differs from the requested (absolute) path.
	ScanPaths map[string]string
	// SDKLevels maps a requested file to its minSdk / targetSdk, resolved
	// with android.ResolveSDKLevels (the lookup the Go rules use) from the
	// scan's spelling of the file. Files with no known level have no entry.
	SDKLevels map[string]android.SDKLevels
}

// forFiles keeps the facts about files, so a check request only describes
// the files it actually asks krit-fir to check.
func (f FileFacts) forFiles(files []string) FileFacts {
	if len(f.TestFiles) == 0 && len(f.ScanPaths) == 0 && len(f.SDKLevels) == 0 {
		return FileFacts{}
	}
	want := make(map[string]bool, len(files))
	for _, p := range files {
		want[p] = true
	}
	var out FileFacts
	for _, p := range f.TestFiles {
		if want[p] {
			out.TestFiles = append(out.TestFiles, p)
		}
	}
	for p, spelling := range f.ScanPaths {
		if want[p] {
			if out.ScanPaths == nil {
				out.ScanPaths = map[string]string{}
			}
			out.ScanPaths[p] = spelling
		}
	}
	for p, levels := range f.SDKLevels {
		if want[p] {
			if out.SDKLevels == nil {
				out.SDKLevels = map[string]android.SDKLevels{}
			}
			out.SDKLevels[p] = levels
		}
	}
	return out
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

// StartFirDaemonWithPort launches krit-fir.jar in TCP daemon mode. The
// daemon runs in oracle.DaemonWorkDir: it may outlive this invocation and
// serve krit runs from other directories, so every path it is sent is
// absolute (see Check) and nothing may resolve against its working directory.
func StartFirDaemonWithPort(jarPath string, verbose bool, jvmTarget ...string) (*FirDaemon, error) {
	// A valid Leyden cache can restore FIR with registered checkers that emit
	// zero findings (#768). The cause is not found, so launch FIR without AOT
	// for both one-shot and shared daemons. checkFirWithRecovery's canary
	// (aot_canary.go) is the guard to rely on when this is re-enabled.
	return startFirDaemonWithAOT(jarPath, verbose, false, jvmTarget...)
}

func startFirDaemonWithAOT(jarPath string, verbose, allowAOT bool, jvmTarget ...string) (*FirDaemon, error) {
	javaPath, err := oracle.JavaPath()
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := buildFirJVMArgsWithAOT(jarPath, javaPath, oracle.CachedJDKMajorVersion(), verbose, allowAOT)
	if len(jvmTarget) > 0 && jvmTarget[0] != "" {
		args = append(args, "--jvm-target", jvmTarget[0])
	}
	args = oracle.EphemeralDaemonArgs(args...)

	if verbose {
		reporter().Verbosef("verbose: Starting krit-fir daemon: %s %s\n", javaPath, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(context.Background(), javaPath, args...)
	cmd.Dir = oracle.DaemonWorkDir()
	logFile, logPath, err := fsutil.CreateUserKritFile("krit-fir-daemon.log")
	if err != nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = logFile
		if verbose {
			reporter().Verbosef("verbose: FIR daemon log: %s\n", logPath)
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start fir daemon: %w", err)
	}
	oracle.RecordTestDaemonPID(cmd.Process.Pid)

	readyCh := make(chan firReadyResult, 1)
	go func() {
		ready, err := readFirReady(stdoutPipe, verbose)
		readyCh <- firReadyResult{ready: ready, err: err}
	}()

	const startupTimeout = 30 * time.Second
	var ready firReadyMessage
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, res.err
		}
		ready = res.ready
	case <-time.After(startupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("fir daemon startup timed out after %s", startupTimeout)
	}

	if verbose {
		reporter().Verbosef("verbose: krit-fir daemon started on port %d (PID %d)\n", ready.Port, cmd.Process.Pid)
	}

	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", ready.Port))
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("connect to fir daemon port %d: %w", ready.Port, err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	d := &FirDaemon{
		cmd:          cmd,
		conn:         conn,
		reader:       reader,
		logFile:      logFile,
		port:         ready.Port,
		nextID:       1,
		started:      true,
		shared:       false,
		slot:         0,
		aotCachePath: firAOTCacheArg(args),
	}
	return d, nil
}

func firAOTCacheArg(args []string) string {
	for _, arg := range args {
		if path, ok := strings.CutPrefix(arg, "-XX:AOTCache="); ok {
			return path
		}
	}
	return ""
}

func buildFirJVMArgs(jarPath, javaPath string, jdkMajor int) []string {
	return buildFirJVMArgsWithAOT(jarPath, javaPath, jdkMajor, false, false)
}

func buildFirJVMArgsWithAOT(jarPath, javaPath string, jdkMajor int, verbose, allowAOT bool) []string {
	args := []string{
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms512m",
		"-Xmx1g",
	}
	if allowAOT {
		args, _ = jvmaot.AppendArgs(args, javaPath, oracle.AbsolutePath(jarPath), "fir", jdkMajor, verbose, reporter().Verbosef)
	}
	return append(args, "-jar", oracle.AbsolutePath(jarPath), "--daemon", "--port", "0")
}

type firReadyResult struct {
	ready firReadyMessage
	err   error
}

const (
	firReadyMaxSkippedLines = 50
	firReadyMaxSkippedBytes = 64 * 1024
)

func readFirReady(r io.Reader, verbose bool) (firReadyMessage, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), firReadyMaxSkippedBytes+1)
	skippedLines, skippedBytes := 0, 0
	for sc.Scan() {
		line := sc.Text()
		var ready firReadyMessage
		if err := json.Unmarshal([]byte(line), &ready); err == nil {
			if !ready.Ready || ready.Port == 0 {
				return firReadyMessage{}, fmt.Errorf("fir daemon did not report ready with port (got: %s)", line)
			}
			return ready, nil
		}
		if skippedLines >= firReadyMaxSkippedLines || skippedBytes+len(line)+1 > firReadyMaxSkippedBytes {
			return firReadyMessage{}, fmt.Errorf("fir daemon ready message not found within stdout noise limit (%d lines or %d bytes)", firReadyMaxSkippedLines, firReadyMaxSkippedBytes)
		}
		skippedLines++
		skippedBytes += len(line) + 1
		if verbose {
			reporter().Verbosef("verbose: skipped fir daemon stdout before ready (%d): %s\n", skippedLines, line)
		}
	}
	if err := sc.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return firReadyMessage{}, fmt.Errorf("fir daemon ready message not found within stdout noise limit (%d lines or %d bytes)", firReadyMaxSkippedLines, firReadyMaxSkippedBytes)
		}
		return firReadyMessage{}, fmt.Errorf("fir daemon startup: %w", err)
	}
	if skippedLines > 0 {
		return firReadyMessage{}, fmt.Errorf("fir daemon closed stdout before ready after %d non-JSON lines", skippedLines)
	}
	return firReadyMessage{}, fmt.Errorf("fir daemon closed stdout before ready")
}

// firCheckRole namespaces the daemon that serves `check` requests (the --fir
// pass) apart from the oracle-backend daemon. Both can serve the same
// sourceDirs and classpath, but the oracle path keeps its connection open for
// the whole scan and a daemon serves one client at a time, so sharing one
// would stall the checker's ping and kill the oracle's daemon.
const firCheckRole = "check"

// ConnectOrStartFirDaemon tries to reuse an existing daemon for the given
// sourceDirs and classpath (via PID file), or starts a new one.
func ConnectOrStartFirDaemon(jarPath string, sourceDirs, classpath []string, verbose bool, jvmTarget ...string) (*FirDaemon, error) {
	return connectOrStartFirDaemon("", jarPath, sourceDirs, classpath, verbose, jvmTarget...)
}

func connectOrStartFirCheckDaemon(jarPath string, sourceDirs, classpath []string, verbose bool, jvmTarget ...string) (*FirDaemon, error) {
	return connectOrStartFirDaemon(firCheckRole, jarPath, sourceDirs, classpath, verbose, jvmTarget...)
}

func connectOrStartFirDaemon(role, jarPath string, sourceDirs, classpath []string, verbose bool, jvmTarget ...string) (*FirDaemon, error) {
	// Capture the jar identity once, before any JVM opens the jar. If the jar
	// is replaced while a new daemon starts, it stays registered under the
	// identity observed first, so the next caller restarts it instead of
	// trusting a daemon that may be running the old artifact.
	target := ""
	if len(jvmTarget) > 0 {
		target = jvmTarget[0]
	}
	srcHash := firRegistryKeyFor(role, jarPath, sourceDirs, classpath, target)
	// rejected is the PID of the record this call decided against. The stop
	// below re-validates it under the registry lock, so a record another
	// process published in the meantime is never killed.
	rejected, _ := registeredFirPID(srcHash)
	if d, err := connectExistingFirDaemon(srcHash, verbose); err == nil {
		if d.aotVerified {
			d.role = role
			d.jvmTarget = target
			return d, nil
		}
		// An AOT-backed daemon whose canary never passed may be serving
		// silent false negatives. Retire it before sending any analysis request.
		_ = d.conn.Close()
		rejected = d.aotPID
	}
	retireSupersededFirDaemons(firRegistryFamilyPrefix(role, jarPath, sourceDirs, classpath, target), sourceDirs, srcHash, len(classpath) > 0, verbose)
	if rejected != 0 {
		stopFirDaemonIf(srcHash, rejected, verbose)
	}
	d, err := StartFirDaemonWithPort(jarPath, verbose, target)
	if err != nil {
		return nil, fmt.Errorf("start persistent fir daemon: %w", err)
	}
	d.sourcesHash = srcHash
	d.role = role
	d.jvmTarget = target
	if err := publishFirDaemon(d, d.cmd.Process.Pid, srcHash, verbose); err != nil {
		d.conn.Close()
		d.cmd.Process.Kill()
		return nil, fmt.Errorf("write fir PID file: %w", err)
	}
	return d, nil
}

// publishFirDaemon registers a freshly started daemon so later krit runs can
// reuse it. A daemon launched from an AOT cache is registered by
// markAOTVerified once its canary passes. Otherwise it is registered here,
// but only while holding the per-key lock: when the lock cannot be taken the
// registry is left untouched and the daemon stays private to this process
// (closed on Release), never written over a concurrent publisher.
func publishFirDaemon(d *FirDaemon, pid int, srcHash string, verbose bool) error {
	if d.aotCachePath != "" {
		d.unpublished = true
		return nil
	}
	err := writeFirPIDFile(pid, d.port, srcHash)
	if errors.Is(err, errFirRegistryBusy) || errors.Is(err, errFirRegistryOwned) {
		if verbose {
			reporter().Verbosef("verbose: krit-fir registry for %s is locked or owned by another process (%v); keeping the daemon private\n", srcHash, err)
		}
		d.unpublished = true
		return nil
	}
	return err
}

// absoluteKeys keys a per-file fact (ScanPaths, SDKLevels) by the absolute
// spelling Check sends in Files, since krit-fir matches the two exactly.
func absoluteKeys[V any](byPath map[string]V) map[string]V {
	if len(byPath) == 0 {
		return nil
	}
	out := make(map[string]V, len(byPath))
	for path, value := range byPath {
		out[oracle.AbsolutePath(path)] = value
	}
	return out
}

// Check sends a check request to the daemon and returns the response.
func (d *FirDaemon) Check(files []fileRef, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts, jvmTarget ...string) (*CheckResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil, fmt.Errorf("fir daemon not started")
	}

	id := d.nextID
	d.nextID++

	target := d.jvmTarget
	if len(jvmTarget) > 0 {
		target = jvmTarget[0]
	}
	data, spelling, err := encodeCheckRequest(id, files, sourceDirs, classpath, rules, ruleConfigs, facts, target)
	if err != nil {
		return nil, err
	}
	line, err := d.exchangeLocked(data)
	if err != nil {
		return nil, err
	}
	return decodeCheckResponse([]byte(line), id, spelling)
}

// Canary sends the daemon's built-in canary request: it compiles a tiny
// source with one known finding in an isolated session, so its cost does not
// depend on the project the daemon serves. A jar that predates the request
// answers with an error object and no findings, which reads as unhealthy.
func (d *FirDaemon) Canary() (*CheckResponse, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return nil, fmt.Errorf("fir daemon not started")
	}
	id := d.nextID
	d.nextID++
	data, err := json.Marshal(firDaemonRequest{ID: id, Command: "canary"})
	if err != nil {
		return nil, err
	}
	line, err := d.exchangeLocked(data)
	if err != nil {
		return nil, err
	}
	return decodeCheckResponse([]byte(line), id, oracle.PathSpelling{})
}

// exchangeLocked writes one request line and reads one response line,
// killing the daemon when it does not answer in time. Caller holds d.mu.
func (d *FirDaemon) exchangeLocked(data []byte) (string, error) {
	data = append(data, '\n')
	if _, err := d.conn.Write(data); err != nil {
		return "", fmt.Errorf("write to fir daemon: %w", err)
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
				return "", fmt.Errorf("read from fir daemon: %w", res.err)
			}
			return "", fmt.Errorf("fir daemon closed stdout unexpectedly")
		}
		line = res.line
	case <-time.After(timeout):
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		d.started = false
		return "", fmt.Errorf("fir daemon request timed out after %s; daemon killed", timeout)
	}
	return line, nil
}

// encodeCheckRequest returns the JSON check request for files and the
// mapping back to the caller's spelling of each file. The daemon echoes
// request paths back; they are sent absolute, and decodeCheckResponse
// restores the caller's spelling, which Go indexes the response by.
func encodeCheckRequest(id int64, files []fileRef, sourceDirs, classpath, rules []string, ruleConfigs RuleConfigs, facts FileFacts, jvmTarget string) ([]byte, oracle.PathSpelling, error) {
	requested, spelling := absoluteFileRefs(files)
	req := firDaemonRequest{
		ID:          id,
		Command:     "check",
		Files:       requested,
		SourceDirs:  oracle.AbsolutePaths(sourceDirs),
		Classpath:   oracle.AbsolutePaths(classpath),
		JvmTarget:   jvmTarget,
		Rules:       rules,
		TestFiles:   oracle.AbsolutePaths(facts.TestFiles),
		ScanPaths:   absoluteKeys(facts.ScanPaths),
		SDKLevels:   absoluteKeys(facts.SDKLevels),
		RuleConfigs: wireRuleConfigs(ruleConfigs),
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, oracle.PathSpelling{}, fmt.Errorf("marshal fir request: %w", err)
	}
	return data, spelling, nil
}

// decodeCheckResponse parses a check response to the request with id,
// spelled back as the caller spelled the request (see encodeCheckRequest).
func decodeCheckResponse(data []byte, id int64, spelling oracle.PathSpelling) (*CheckResponse, error) {
	var resp CheckResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal fir response: %w (got: %s)", err, data)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("inspect fir response fields: %w", err)
	}
	_, resp.rulesPresent = fields["rules"]
	if resp.ID != id {
		return nil, fmt.Errorf("fir response ID mismatch: expected %d, got %d", id, resp.ID)
	}
	resp.toCallerSpelling(spelling)
	return &resp, nil
}

// absoluteFileRefs returns refs with absolute paths and the mapping back to
// the caller's spelling.
func absoluteFileRefs(refs []fileRef) ([]fileRef, oracle.PathSpelling) {
	paths := make([]string, len(refs))
	for i, ref := range refs {
		paths[i] = ref.Path
	}
	abs, spelling := oracle.AbsoluteRequestPaths(paths)
	out := make([]fileRef, len(refs))
	for i, ref := range refs {
		out[i] = fileRef{Path: abs[i], ContentHash: ref.ContentHash}
	}
	return out, spelling
}

// toCallerSpelling re-keys every path in r from the absolute form sent to
// the daemon to the caller's spelling.
func (r *CheckResponse) toCallerSpelling(spelling oracle.PathSpelling) {
	for i := range r.Findings {
		r.Findings[i].Path = spelling.Caller(r.Findings[i].Path)
	}
	r.Crashed = oracle.CallerKeys(spelling, r.Crashed)
	r.ErrorFiles = oracle.CallerKeys(spelling, r.ErrorFiles)
	for rule, byPath := range r.RuleErrors {
		r.RuleErrors[rule] = oracle.CallerKeys(spelling, byPath)
	}
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
	if d.cmd != nil && !d.shared && (d.unpublished || jvmaot.IsRecording(d.cmd.Args)) {
		return d.Close()
	}
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
		case err := <-done:
			jvmaot.FinalizeRecording(d.cmd.Args, err == nil)
		case <-time.After(10 * time.Second):
			if d.cmd != nil && d.cmd.Process != nil {
				d.cmd.Process.Kill()
			}
			<-done //nolint:errcheck // killed above, so the exit error is expected
			jvmaot.FinalizeRecording(d.cmd.Args, false)
		}
	}
	if d.port != 0 && d.sourcesHash != "" && !d.unpublished && d.cmd != nil && d.cmd.Process != nil {
		removeFirRegistryIfOwned(d.sourcesHash, d.cmd.Process.Pid)
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
	if dir := os.Getenv("KRIT_DAEMON_REGISTRY_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("create daemon registry dir: %w", err)
		}
		return dir, nil
	}
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

func firAOTPath(sourcesHash string) string {
	dir, err := firDaemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-fir-"+sourcesHash+".aot")
	}
	return filepath.Join(dir, sourcesHash+".krit-fir.aot")
}

func writeFirPIDFile(pid, port int, sourcesHash string, aotPath ...string) error {
	// A daemon without a cache needs no canary; one launched from a cache is
	// registered unverified until the canary passes (see markAOTVerified).
	cache := ""
	if len(aotPath) > 0 {
		cache = aotPath[0]
	}
	return writeFirRegistry(pid, port, sourcesHash, firAOTMeta{PID: pid, Cache: cache, Verified: cache == ""})
}

// writeFirRegistry publishes a daemon: its metadata first, then the PID and
// port files that make it discoverable. The launch PID is stored with the
// metadata so stale metadata cannot be attributed to a newer daemon
// registered under the same key.
func writeFirRegistry(pid, port int, sourcesHash string, meta firAOTMeta) error {
	// Never write without the per-key lock: another publisher's metadata,
	// PID and port files would interleave with ours. When the lock cannot be
	// taken the registry is left untouched and the caller keeps its daemon
	// private.
	release, ok := claimFirRegistry(sourcesHash)
	if !ok {
		return errFirRegistryBusy
	}
	defer release()
	return writeFirRegistryClaimed(pid, port, sourcesHash, meta)
}

// errFirRegistryBusy reports that the per-key publication lock was not
// obtained (contention timeout, lock-file failure, or an unsupported
// platform), so nothing was written.
var errFirRegistryBusy = errors.New("fir registry publication lock not obtained")

// errFirRegistryOwned reports that another live daemon is already registered
// for the key, so nothing was written. Callers keep their daemon private, as
// for errFirRegistryBusy.
var errFirRegistryOwned = errors.New("fir registry key already owned by another live daemon")

// writeFirRegistryClaimed writes the record; the caller holds the claim.
// The PID file is the discoverability marker and is written last, after the
// metadata and port: a reader that sees a live PID therefore sees a complete
// record. Any previous PID file is removed first so a half-replaced record
// is never discoverable.
func writeFirRegistryClaimed(pid, port int, sourcesHash string, meta firAOTMeta) error {
	// Re-read the registered PID now that the lock is held: another live
	// daemon already registered for this key is never displaced.
	if registered, found := registeredFirPID(sourcesHash); found && registered != pid && firPIDAlive(registered) {
		return errFirRegistryOwned
	}
	_ = os.Remove(firPIDPath(sourcesHash))
	if err := writeFirAOTMeta(sourcesHash, meta); err != nil {
		return err
	}
	if err := writeFileAtomic(firPortPath(sourcesHash), []byte(strconv.Itoa(port)+"\n")); err != nil {
		return fmt.Errorf("write fir port: %w", err)
	}
	if err := writeFileAtomic(firPIDPath(sourcesHash), []byte(strconv.Itoa(pid)+"\n")); err != nil {
		return fmt.Errorf("write fir pid: %w", err)
	}
	return nil
}

func firLockPath(sourcesHash string) string { return firPIDPath(sourcesHash) + ".lock" }

// firClaimWait bounds how long a publisher waits for another one.
var firClaimWait = 3 * time.Second

// claimFirRegistry takes the per-key publication lock: an OS advisory lock,
// so exactly one process checks-and-publishes at a time and the kernel
// releases it if the holder dies. It gives up after firClaimWait; the caller
// then stays private.
func claimFirRegistry(sourcesHash string) (release func(), ok bool) {
	return fslock.Acquire(firLockPath(sourcesHash), firClaimWait)
}

// writeFileAtomic replaces path with data via temp file + rename.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		return errors.Join(werr, cerr)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// firAOTMeta is the launch metadata persisted beside every registered
// daemon. A daemon whose metadata is missing or unreadable is treated as an
// unverified AOT launch, never as a non-AOT one.
type firAOTMeta struct {
	PID int `json:"pid"`
	// Cache is the Leyden AOT cache the JVM was launched with, "" when none.
	Cache string `json:"cache"`
	// Verified is set for non-AOT launches and, for AOT ones, once the AOT
	// canary passed on this PID's JVM, so reconnecting krit runs can reuse
	// the daemon without repeating it.
	Verified bool `json:"verified,omitempty"`
}

// unknownAOTState stands in for the cache path of a reconnected daemon whose
// metadata could not be read.
const unknownAOTState = "unknown: registry metadata unreadable"

// writeFirAOTMeta replaces the metadata atomically (temp file + rename), so a
// concurrent reader sees the old or the new record, never a torn one.
func writeFirAOTMeta(sourcesHash string, meta firAOTMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(firAOTPath(sourcesHash), data); err != nil {
		return fmt.Errorf("write fir AOT metadata: %w", err)
	}
	return nil
}

func registeredFirPID(sourcesHash string) (int, bool) {
	data, err := os.ReadFile(firPIDPath(sourcesHash))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil
}

func firPIDAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	return err == nil && proc.Signal(syscall.Signal(0)) == nil
}

// markAOTVerified records that the canary passed.
//
// A daemon launched from an AOT cache is not registered until this point
// (d.unpublished), so no other krit process can reuse or retire it
// mid-verification. Publishing is skipped when another live daemon already
// holds the registry key; this one then stays private and is closed on
// Release. A daemon that is already registered only has its metadata
// updated, and only while the PID file still names it.
func (d *FirDaemon) markAOTVerified() {
	d.aotVerified = true
	pid := d.aotPID
	if d.cmd != nil && d.cmd.Process != nil {
		pid = d.cmd.Process.Pid
	}
	if d.sourcesHash == "" || pid == 0 || d.aotCachePath == "" {
		return
	}
	meta := firAOTMeta{PID: pid, Cache: d.aotCachePath, Verified: true}
	registered, ok := registeredFirPID(d.sourcesHash)
	if d.unpublished {
		// Check-and-publish runs under the per-key claim, so of several
		// concurrent verifiers exactly one publishes; the rest stay private
		// and are closed on Release.
		release, claimed := claimFirRegistry(d.sourcesHash)
		if !claimed {
			return
		}
		defer release()
		if writeFirRegistryClaimed(pid, d.port, d.sourcesHash, meta) == nil {
			d.unpublished = false
		}
		return
	}
	// Registered already: update the metadata under the lock, and only while
	// the PID file still names this daemon.
	if ok && registered == pid {
		if release, claimed := claimFirRegistry(d.sourcesHash); claimed {
			defer release()
			if current, found := registeredFirPID(d.sourcesHash); found && current == pid {
				_ = writeFirAOTMeta(d.sourcesHash, meta)
			}
		}
	}
}

// removeFirRegistryFiles deletes a record, the PID marker first so the
// record is never discoverable while half deleted. The caller holds the lock.
func removeFirRegistryFiles(sourcesHash string) {
	os.Remove(firPIDPath(sourcesHash))
	os.Remove(firPortPath(sourcesHash))
	os.Remove(firAOTPath(sourcesHash))
}

// removeFirRegistryIfOwned deletes the record under the lock, and only if the
// PID file still names pid. If the lock cannot be taken the record is left
// alone: it is being published or replaced by someone else, and a record of a
// dead daemon is cleaned up by the next stopFirDaemonIf.
func removeFirRegistryIfOwned(sourcesHash string, pid int) {
	release, ok := claimFirRegistry(sourcesHash)
	if !ok {
		return
	}
	defer release()
	if current, found := registeredFirPID(sourcesHash); found && current == pid {
		removeFirRegistryFiles(sourcesHash)
	}
}

// connectExistingFirDaemon reuses the daemon registered under hash, a
// firRegistryKey.
func connectExistingFirDaemon(hash string, verbose bool) (*FirDaemon, error) {
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

	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
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
	var aotMeta firAOTMeta
	d.aotPID = pid
	if data, err := os.ReadFile(firAOTPath(hash)); err == nil && json.Unmarshal(data, &aotMeta) == nil && aotMeta.PID == pid {
		d.aotCachePath = aotMeta.Cache
		d.aotVerified = aotMeta.Cache == "" || aotMeta.Verified
	} else {
		// Without readable metadata for this PID we cannot tell that the JVM
		// was launched without AOT, so it must not be trusted.
		d.aotCachePath = unknownAOTState
	}
	if err := d.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("fir daemon not responsive: %w", err)
	}
	if verbose {
		reporter().Verbosef("verbose: Reusing existing krit-fir daemon (PID %d, port %d, sources %s)\n", pid, port, hash)
	}
	return d, nil
}

// stopFirDaemon kills whatever daemon is registered under hash and removes
// its record. See stopFirDaemonIf.
func stopFirDaemon(hash string, verbose bool) bool { return stopFirDaemonIf(hash, 0, verbose) }

// stopFirDaemonIf stops the daemon registered under hash and removes its
// record, but only under the per-key lock and, when expect is non-zero, only
// if the record still names expect. It reports whether it acted.
//
// Every destructive registry path goes through here (or
// removeFirRegistryIfOwned) so a publisher holding the lock is never raced:
// when the lock cannot be taken the record is treated as being published or
// replaced and nothing is killed or deleted. The caller then starts its own
// daemon, which publishes under the same lock or stays private.
func stopFirDaemonIf(hash string, expect int, verbose bool) bool {
	release, ok := claimFirRegistry(hash)
	if !ok {
		if verbose {
			reporter().Verbosef("verbose: krit-fir registry %s is locked by another process; not stopping its daemon\n", hash)
		}
		return false
	}
	defer release()
	pidData, err := os.ReadFile(firPIDPath(hash))
	if err != nil {
		// No PID marker means no discoverable daemon; clear any leftovers of
		// a publication that never completed.
		removeFirRegistryFiles(hash)
		return true
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		removeFirRegistryFiles(hash)
		return true
	}
	if expect != 0 && pid != expect {
		if verbose {
			reporter().Verbosef("verbose: krit-fir registry %s now names PID %d, not %d; leaving it\n", hash, pid, expect)
		}
		return false
	}
	proc, ferr := os.FindProcess(pid)
	if ferr != nil || proc.Signal(syscall.Signal(0)) != nil {
		if verbose {
			reporter().Verbosef("verbose: Cleaning stale krit-fir daemon PID file (PID %d not alive)\n", pid)
		}
		removeFirRegistryFiles(hash)
		return true
	}
	if verbose {
		reporter().Verbosef("verbose: Killing unresponsive krit-fir daemon (PID %d)\n", pid)
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
	removeFirRegistryFiles(hash)
	return true
}

// Retiring an old daemon can interrupt another krit mid-request, but a jar
// used by that daemon has already been replaced on disk. family is the
// path-only prefix for the caller's role, jar, sources, and classpath.
func retireSupersededFirDaemons(family string, sourceDirs []string, current string, hasClasspath, verbose bool) {
	dir, err := firDaemonsDir()
	if err != nil {
		return
	}
	paths, err := filepath.Glob(filepath.Join(dir, family+"*.krit-fir.pid"))
	if err == nil {
		for _, path := range paths {
			stem := strings.TrimSuffix(filepath.Base(path), ".krit-fir.pid")
			candidatePrefix, identity, found := strings.Cut(stem, "@")
			if !found || candidatePrefix != family && (!hasClasspath || !strings.HasPrefix(candidatePrefix, family+"-")) || !validFirIdentity(identity) || stem == current {
				continue
			}
			stopFirDaemon(stem, verbose)
		}
	}
	legacy := hashFirSources(sourceDirs)
	if _, err := os.Stat(firPIDPath(legacy)); err == nil {
		stopFirDaemon(legacy, verbose)
	}
}

func validFirIdentity(identity string) bool {
	if identity == "missing" {
		return true
	}
	if len(identity) != 8 {
		return false
	}
	for _, c := range identity {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

// firRegistryKey is the oracle-backend daemon's registry key for a daemon
// started without a classpath.
func firRegistryKey(jarPath string, sourceDirs []string) string {
	return firRegistryKeyFor("", jarPath, sourceDirs, nil)
}

// firRegistryKeyFor identifies a daemon by role, source dirs, classpath, jar
// path, and jar identity, mirroring the oracle daemon key: a daemon started
// for one classpath never answers for another, since its checker verdicts
// would be computed against the wrong libraries.
func firRegistryKeyFor(role, jarPath string, sourceDirs, classpath []string, jvmTarget ...string) string {
	return firRegistryPrefix(role, jarPath, sourceDirs, classpath, jvmTarget...) + oracle.JarIdentity(jarPath)
}

func firRegistryPrefix(role, jarPath string, sourceDirs, classpath []string, jvmTarget ...string) string {
	key := firRegistryFamilyPrefix(role, jarPath, sourceDirs, classpath, jvmTarget...)
	if len(classpath) > 0 {
		key += "-" + gradlemodel.ClasspathFingerprint(oracle.AbsolutePaths(classpath))[:8]
	}
	return key + "@"
}

func firRegistryFamilyPrefix(role, jarPath string, sourceDirs, classpath []string, jvmTarget ...string) string {
	key := hashFirSources(sourceDirs)
	if len(jvmTarget) > 0 && jvmTarget[0] != "" {
		key += "-jvm" + jvmTarget[0]
	}
	if len(classpath) > 0 {
		// Order matters on a classpath, so hash it in the given order.
		absolute := oracle.AbsolutePaths(classpath)
		key += "-" + hashutil.HashHex([]byte(strings.Join(absolute, "\n")))[:8]
	}
	if role != "" {
		key = role + "-" + key
	}
	return key + "-" + oracle.JarPathTag(jarPath)
}

// hashFirSources returns a 16-hex-char fingerprint of sorted sourceDirs in
// absolute form. The registry is shared by every working directory, so the
// relative "src/main/kotlin" of two projects must not name one daemon.
func hashFirSources(sourceDirs []string) string {
	sorted := oracle.AbsolutePaths(sourceDirs)
	sort.Strings(sorted)
	return hashutil.HashHex([]byte(strings.Join(sorted, "\n")))[:16]
}
