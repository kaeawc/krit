package oracle

import (
	"bufio"
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

	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/perf"
)

// jarCachePath returns a cache path keyed by the JAR's content hash, with the
// given suffix appended.  The file lives under $TMPDIR/krit-cache/ (or
// ~/.krit/cache/ if HOME is set) and is named krit-types-<hash><suffix>.
func jarCachePath(jarPath, suffix string) (string, error) {
	full, err := hashutil.HashFile(jarPath)
	if err != nil {
		return "", err
	}
	hash := full[:12]

	cacheDir := filepath.Join(os.TempDir(), "krit-cache")
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".krit", "cache")
		if err := os.MkdirAll(candidate, 0755); err == nil {
			cacheDir = candidate
		}
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "krit-types-"+hash+suffix), nil
}

// cdsArchivePath returns the path for an AppCDS shared archive keyed by the
// JAR's content hash.
func cdsArchivePath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".jsa")
}

// cracCheckpointPath returns the directory for a CRaC checkpoint keyed by the
// JAR's content hash.  Only meaningful on CRaC-enabled JDKs (Azul Zulu, Liberica NIK).
func cracCheckpointPath(jarPath string) (string, error) {
	return jarCachePath(jarPath, ".crac")
}

// Daemon manages a long-lived krit-types JVM process for on-demand type resolution.
type Daemon struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	conn    net.Conn // TCP connection when using persistent daemon mode
	logFile *os.File // daemon log file, closed on Close()
	mu      sync.Mutex
	port    int // TCP port for persistent daemon mode (0 = pipe mode)
	nextID  int
	started bool
	shared  bool // true if this daemon was connected to (not started by us)
	slot    int  // daemon-pool slot; 0 is the legacy single-daemon slot
	// sourcesHash is the 16-hex-char fingerprint of the sourceDirs this
	// Daemon was built for (or connected to). Used by MatchesRepo to
	// detect cross-repo daemon reuse. For freshly-started-by-us daemons
	// the field is set from the actual sourceDirs at startup. For
	// shared/connected-to daemons, it's populated from daemon.sources
	// on the filesystem. Empty string means unknown — see MatchesRepo.
	sourcesHash string
}

// MatchesRepo returns true if this Daemon was started for (or is
// currently connected to) a daemon whose sourceDirs fingerprint
// matches the given set. Used by runMissAnalysis to reject daemon
// reuse across krit invocations that target different repos — when
// the fingerprints disagree the caller falls back to one-shot for
// the current invocation rather than trusting a daemon whose
// sourceModule walks a different file set.
//
// An empty sourcesHash (older daemon that predates Phase 3 sources
// tagging) always returns false so callers conservatively fall back.
// The old daemon keeps running for its original consumer.
func (d *Daemon) MatchesRepo(sourceDirs []string) bool {
	if d.sourcesHash == "" {
		return false
	}
	return d.sourcesHash == hashSources(sourceDirs)
}

// daemonRequest is the JSON request sent to the daemon process.
type daemonRequest struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// daemonResponse is the JSON response read from the daemon process.
type daemonResponse struct {
	ID     int              `json:"id"`
	Result *json.RawMessage `json:"result,omitempty"`
	Error  string           `json:"error,omitempty"`
	// Errors is the per-file error map surfaced by the new analyzeWithDeps
	// method's flat envelope (errors sibling of result). The legacy analyze
	// and analyzeAll methods nest errors inside result and don't populate
	// this top-level field — it stays nil for those callers. The cache path
	// uses this field to detect file-not-in-session conditions and trigger
	// a daemon rebuild + retry.
	Errors map[string]string `json:"errors,omitempty"`
	// CacheDeps is the per-file dependency closure block emitted only by
	// the analyzeWithDeps method. Nil for legacy methods.
	CacheDeps *json.RawMessage `json:"cacheDeps,omitempty"`
	// Timings is an optional perf.TimingEntry array emitted by newer
	// krit-types daemon methods for request-level deep dives.
	Timings *json.RawMessage `json:"timings,omitempty"`
}

// ErrDaemonFileNotInSession is returned by Daemon.AnalyzeWithDeps when
// the daemon's response errors map contains any "File not found in source
// module" entries. Callers use errors.Is to recognize the condition and
// trigger a daemon Rebuild + retry before falling through to one-shot.
var ErrDaemonFileNotInSession = errors.New("daemon: file not in source module")

// StartDaemon launches the krit-types JVM process in daemon mode.
func StartDaemon(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := []string{
		// JVM tuning for long-running analyzer workloads. See
		// StartDaemonWithPort for rationale on why each of the old
		// "fast startup" flags was removed — the short version is that
		// this is a daemon, not a one-shot, and optimizing for cold
		// startup penalizes the steady-state throughput the daemon
		// actually needs.
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms1g",
		"-XX:ReservedCodeCacheSize=256m",
		"-Djava.awt.headless=true",
	}

	// AppCDS: use a shared class-data archive to speed up class loading.
	// Available on JDK 13+.  Uses -Xshare:auto so unsupported JDKs ignore it.
	if archivePath, err := cdsArchivePath(jarPath); err == nil {
		if _, statErr := os.Stat(archivePath); statErr == nil {
			// Archive exists — use it
			args = append(args, "-XX:SharedArchiveFile="+archivePath, "-Xshare:auto")
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: AppCDS: using archive %s\n", archivePath)
			}
		} else {
			// No archive yet — train on this run (JVM writes the archive at exit)
			args = append(args, "-XX:ArchiveClassesAtExit="+archivePath, "-Xshare:auto")
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: AppCDS: training archive %s\n", archivePath)
			}
		}
	}

	// CRaC: attempt restore from a checkpoint directory.
	// Only works on CRaC-enabled JDKs (Azul Zulu, Liberica NIK); ignored otherwise.
	cracPath, cracErr := cracCheckpointPath(jarPath)
	useCRaCRestore := false
	if cracErr == nil {
		if info, statErr := os.Stat(cracPath); statErr == nil && info.IsDir() {
			// Checkpoint directory exists — try restoring.
			// We launch a separate attempt; if it fails we fall back to cold start.
			restoreArgs := []string{"-XX:CRaCRestoreFrom=" + cracPath}
			restoreCmd := exec.Command(javaPath, restoreArgs...)
			if restoreCmd.Start() == nil {
				// If the restore process starts, we use it instead.
				useCRaCRestore = true
				if verbose {
					fmt.Fprintf(os.Stderr, "verbose: CRaC: restoring from %s\n", cracPath)
				}
				// But we cannot easily reuse exec.Cmd after Start — abandon this
				// path and fall through to cold start.  CRaC restore replaces the
				// process image, so the child IS the daemon.  However, Go cannot
				// attach pipes to an already-started process.  Kill it and fall back.
				restoreCmd.Process.Kill()
				restoreCmd.Wait() //nolint:errcheck
				useCRaCRestore = false
			}
		}
	}
	_ = useCRaCRestore // reserved for future pipe-based CRaC restore

	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = append(args, "-jar", jarPath, "--daemon")
	if len(sourceDirs) > 0 {
		args = append(args, "--sources", strings.Join(sourceDirs, ","))
	}
	if len(classpath) > 0 {
		args = append(args, "--classpath", strings.Join(classpath, ","))
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Starting krit-types daemon: %s %s\n", javaPath, strings.Join(args, " "))
	}

	cmd := exec.Command(javaPath, args...)

	// Write daemon logs to a file for inspection
	logPath := filepath.Join(os.TempDir(), "krit-types-daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		cmd.Stderr = os.Stderr // fallback to stderr
	} else {
		cmd.Stderr = logFile
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: Daemon log: %s\n", logPath)
		}
	}

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
	// Allow up to 64MB lines for large oracle responses
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

	d := &Daemon{
		cmd:     cmd,
		stdin:   stdinPipe,
		stdout:  scanner,
		logFile: logFile,
		nextID:  1,
	}

	// Wait for the "ready" message from the daemon with timeout.
	// The JVM can take 2-27s for cold start; timeout prevents blocking forever.
	type scanResult struct {
		line string
		err  error
	}
	readyCh := make(chan scanResult, 1)
	go func() {
		if scanner.Scan() {
			readyCh <- scanResult{line: scanner.Text()}
		} else {
			readyCh <- scanResult{err: scanner.Err()}
		}
	}()

	var line string
	const startupTimeout = 30 * time.Second
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon startup: %w", res.err)
		}
		if res.line == "" {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon closed stdout before sending ready message")
		}
		line = res.line
	case <-time.After(startupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup timed out after %s", startupTimeout)
	}
	var readyResp daemonResponse
	if err := json.Unmarshal([]byte(line), &readyResp); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon ready message: invalid JSON: %w (got: %s)", err, line)
	}
	if readyResp.Error != "" {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup error: %s", readyResp.Error)
	}

	d.started = true
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: krit-types daemon ready\n")
	}

	return d, nil
}

// daemonRequestTimeout returns the max wall-clock duration allowed for a
// single daemon request/response round-trip (env KRIT_TYPES_REQUEST_TIMEOUT,
// default 10 minutes). This is separate from the startup timeout: a single
// analyze call can legitimately take minutes on a large repo, but it should
// never take hours. A hang past this limit is the failure mode the FIR-crash
// resilience concept is trying to unstick — kotlin/kotlin was observed to
// hang indefinitely in Analysis API when it hit an unhandled internal error.
func daemonRequestTimeout() time.Duration {
	if v := os.Getenv("KRIT_TYPES_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Minute
}

// send writes a request and reads the corresponding response, returning
// the full decoded envelope. Caller must hold d.mu.
//
// If the daemon doesn't produce a response within KRIT_TYPES_REQUEST_TIMEOUT
// (default 10m), the daemon process is force-killed, the Daemon is marked
// not-started so subsequent calls fail fast, and an error is returned. The
// caller is expected to fall back to tree-sitter-only analysis — the oracle
// was unreliable and any retry on the same Daemon will fail immediately.
//
// Most callers only need the Result field; they project via sendResult.
// The analyzeWithDeps path needs Errors + CacheDeps siblings of Result so
// it calls send directly.
func (d *Daemon) send(method string, params map[string]interface{}) (*daemonResponse, error) {
	if !d.started {
		return nil, fmt.Errorf("daemon not started")
	}

	id := d.nextID
	d.nextID++

	req := daemonRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write newline-delimited JSON
	data = append(data, '\n')
	if _, err := d.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to daemon: %w", err)
	}

	// Read response. The Scan() call would block forever if the daemon
	// hangs mid-request (e.g. Analysis API FIR crash with no thrown
	// exception), so run it in a goroutine and select against a timer.
	type scanResult struct {
		line string
		ok   bool
		err  error
	}
	resultCh := make(chan scanResult, 1)
	go func() {
		if d.stdout.Scan() {
			resultCh <- scanResult{line: d.stdout.Text(), ok: true}
			return
		}
		resultCh <- scanResult{err: d.stdout.Err()}
	}()

	timeout := daemonRequestTimeout()
	var line string
	select {
	case res := <-resultCh:
		if !res.ok {
			if res.err != nil {
				return nil, fmt.Errorf("read from daemon: %w", res.err)
			}
			return nil, fmt.Errorf("daemon closed stdout unexpectedly")
		}
		line = res.line
	case <-time.After(timeout):
		// Kill the daemon so the hung Scan() goroutine can exit (closing
		// the stdout pipe makes Scan() return false). Mark started=false
		// so any subsequent call short-circuits without reading stale
		// bytes from a dead pipe.
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		d.started = false
		return nil, fmt.Errorf("daemon request timed out after %s (method=%s); daemon killed, falling back to tree-sitter", timeout, method)
	}

	var resp daemonResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (got: %s)", err, line)
	}

	if resp.ID != id {
		return nil, fmt.Errorf("response ID mismatch: expected %d, got %d", id, resp.ID)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	return &resp, nil
}

// sendResult is the legacy wrapper that projects a daemon response to just
// its Result field. Used by Analyze, AnalyzeAll, Rebuild, Ping, Checkpoint —
// methods that don't consume the Errors / CacheDeps sibling fields.
func (d *Daemon) sendResult(method string, params map[string]interface{}) (*json.RawMessage, error) {
	resp, err := d.send(method, params)
	if err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// Analyze sends an incremental analysis request for specific files.
func (d *Daemon) Analyze(files []string) (*OracleData, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	params := map[string]interface{}{
		"files": files,
	}

	result, err := d.sendResult("analyze", params)
	if err != nil {
		return nil, err
	}

	return unmarshalOracleData(result)
}

// AnalyzeAll sends a full analysis request for all files.
func (d *Daemon) AnalyzeAll() (*OracleData, error) {
	return d.AnalyzeAllWithCallFilter(nil)
}

// AnalyzeAllWithCallFilter sends a full analysis request for all files,
// optionally narrowing call-target resolution in the JVM oracle.
func (d *Daemon) AnalyzeAllWithCallFilter(callFilter *CallTargetFilterSummary) (*OracleData, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var params map[string]interface{}
	if callFilter != nil && callFilter.Enabled {
		params = map[string]interface{}{
			"callFilterCalleeNames":          callFilter.CalleeNames,
			"callFilterLexicalHintsByCallee": callFilter.LexicalHintsByCallee,
			"callFilterLexicalSkipByCallee":  callFilter.LexicalSkipByCallee,
			"callFilterRuleProfiles":         callFilter.RuleProfiles,
		}
	}
	result, err := d.sendResult("analyzeAll", params)
	if err != nil {
		return nil, err
	}

	return unmarshalOracleData(result)
}

// AnalyzeWithDeps is the cache-aware variant of Analyze. It asks the
// daemon to run analysis with a DepTracker instrumented per file and
// returns both the OracleData AND the per-file dep closure the Go-side
// cache layer needs to write fresh entries.
//
// The response envelope has a flat shape with `result`, `errors`, and
// `cacheDeps` as siblings (unlike the legacy `analyze` which nests
// errors inside result). A missing `cacheDeps` field on a successful
// response is a protocol-version error — the caller should fall back
// to the one-shot path.
//
// If the daemon's errors map contains any file-not-in-session entries,
// this method returns ErrDaemonFileNotInSession wrapping the count.
// The caller (typically runMissAnalysis) uses errors.Is to recognize
// the condition and trigger a daemon Rebuild + retry before falling
// through to one-shot.
func (d *Daemon) AnalyzeWithDeps(files []string) (*OracleData, *CacheDepsFile, error) {
	data, deps, _, err := d.AnalyzeWithDepsWithTimings(files, false, nil, nil)
	return data, deps, err
}

// AnalyzeWithDepsWithTimings is AnalyzeWithDeps plus optional Kotlin-side
// timing entries returned by newer daemon processes.
func (d *Daemon) AnalyzeWithDepsWithTimings(files []string, collectTimings bool, callFilter *CallTargetFilterSummary, declarationProfile *DeclarationProfileSummary) (*OracleData, *CacheDepsFile, []perf.TimingEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	params := map[string]interface{}{
		"files":   files,
		"timings": collectTimings,
	}
	if callFilter != nil && callFilter.Enabled {
		params["callFilterCalleeNames"] = callFilter.CalleeNames
		params["callFilterLexicalHintsByCallee"] = callFilter.LexicalHintsByCallee
		params["callFilterLexicalSkipByCallee"] = callFilter.LexicalSkipByCallee
		params["callFilterRuleProfiles"] = callFilter.RuleProfiles
	}
	if declarationProfile != nil {
		if cliVal := declarationProfile.Profile.CLIValue(); cliVal != "" {
			params["declarationProfile"] = cliVal
		}
	}

	resp, err := d.send("analyzeWithDeps", params)
	if err != nil {
		return nil, nil, nil, err
	}

	if resp.CacheDeps == nil {
		return nil, nil, nil, fmt.Errorf("daemon response missing cacheDeps field (old daemon version?)")
	}

	oracleData, err := unmarshalOracleData(resp.Result)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal oracle data: %w", err)
	}

	var cacheDeps CacheDepsFile
	if err := json.Unmarshal([]byte(*resp.CacheDeps), &cacheDeps); err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal cacheDeps: %w", err)
	}

	var timings []perf.TimingEntry
	if resp.Timings != nil {
		if err := json.Unmarshal([]byte(*resp.Timings), &timings); err != nil {
			return nil, nil, nil, fmt.Errorf("unmarshal timings: %w", err)
		}
	}

	// If the daemon reported any files it couldn't find in its source
	// module (e.g. files Go's CollectKtFiles walker includes but the
	// Analysis API's session walker excludes for structural reasons like
	// oversized data fixtures), fold them into cacheDeps.Crashed so they
	// flow through the existing poison-entry writer on the caller side.
	// Next invocation's ClassifyFiles will treat them as hits and skip
	// them, eliminating the waste of sending them to the daemon every
	// time. This replaces the original Rebuild+retry heuristic which
	// was hostile to cold runs: a structural mismatch of just 1 file
	// would trigger a full session rebuild and then still fail, forcing
	// fallback to a one-shot — all while the first daemon analysis pass
	// had already produced a valid partial result.
	if len(resp.Errors) > 0 {
		if cacheDeps.Crashed == nil {
			cacheDeps.Crashed = map[string]string{}
		}
		for path, msg := range resp.Errors {
			if strings.Contains(msg, "not found in source module") {
				cacheDeps.Crashed[path] = "daemon: " + msg
			}
			// Other per-file analysis errors (FIR crashes inside
			// analyzeKtFile) are already handled by the daemon-side
			// DepTracker.recordCrash path; they'll appear in
			// cacheDeps.Crashed directly from the Kotlin side.
		}
	}

	return oracleData, &cacheDeps, timings, nil
}

// Rebuild tells the daemon to rebuild its Analysis API session (after file changes).
func (d *Daemon) Rebuild() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.sendResult("rebuild", nil)
	return err
}

// Shutdown gracefully stops the daemon.
func (d *Daemon) Shutdown() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	// Send shutdown request — ignore errors since the process may close immediately
	d.sendResult("shutdown", nil) //nolint:errcheck

	d.started = false

	// Wait for process to exit with a timeout
	done := make(chan error, 1)
	go func() {
		done <- d.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		d.cmd.Process.Kill()
		return fmt.Errorf("daemon did not exit within timeout, killed")
	}
}

// Checkpoint asks the daemon to create a CRaC checkpoint.  If the JDK does not
// support CRaC the daemon returns an error which we silently ignore.
func (d *Daemon) Checkpoint() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.sendResult("checkpoint", nil)
	if err != nil {
		// CRaC not available is expected on most JDKs — degrade silently.
		if strings.Contains(err.Error(), "CRaC not available") {
			return nil
		}
		return err
	}
	return nil
}

// Close shuts down and cleans up. For shared (reused) daemons, only the TCP
// connection is closed — the daemon process keeps running for future clients.
// For owned daemons, the process is shut down and PID files are removed.
// Release drops this Go-side handle to the daemon but leaves the
// daemon process alive. Closes the TCP connection (releasing the
// socket) and the log file handle. Does NOT call Shutdown, does NOT
// remove the PID file. The daemon stays discoverable by the next
// krit invocation via the per-repo PID files under ~/.krit/cache/daemons/,
// and eventually self-terminates on its 30-minute idle timeout
// (serverSocket.soTimeout in Main.kt:117-124).
//
// This is the correct cleanup for short-lived CLI invocations that
// want to benefit from daemon reuse across sequential runs: Close()
// shuts the daemon down on exit (losing all the warmup benefit),
// Release() leaves it alive.
func (d *Daemon) Release() error {
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

func (d *Daemon) Close() error {
	if d.shared {
		// We connected to an existing daemon — just close the TCP connection.
		// The daemon keeps running for the next client.
		d.mu.Lock()
		d.started = false
		d.mu.Unlock()
		if d.conn != nil {
			d.conn.Close()
		}
		return nil
	}

	if d.started {
		if err := d.Shutdown(); err != nil {
			// Force kill as fallback
			if d.cmd != nil && d.cmd.Process != nil {
				d.cmd.Process.Kill()
			}
		}
	}
	// Clean up PID files if this was a persistent daemon we started.
	// Use the Daemon's own sourcesHash so we only touch this repo's
	// PID file entries — other repos' daemons under daemons/ are
	// left alone.
	if d.port != 0 && d.sourcesHash != "" {
		removePIDFileSlot(d.sourcesHash, d.slot)
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
// Persistent daemon support — PID file + TCP port reuse
// ---------------------------------------------------------------------------

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

// daemonPIDPath returns the path to the PID file for the daemon
// serving the given sourcesHash. Each repo (identified by the
// hashSources() of its source directories) has its own PID file
// under ~/.krit/cache/daemons/{hash}.pid, so multiple daemons
// can coexist — one per repo the user is actively working on.
func daemonPIDPath(sourcesHash string) string {
	return daemonPIDPathForSlot(sourcesHash, 0)
}

func daemonPIDPathForSlot(sourcesHash string, slot int) string {
	dir, err := daemonsDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "krit-cache", "daemons", daemonPIDFileName(sourcesHash, slot))
	}
	return filepath.Join(dir, daemonPIDFileName(sourcesHash, slot))
}

// daemonPortPath returns the path to the port file for the daemon
// serving the given sourcesHash. Sibling of daemonPIDPath.
func daemonPortPath(sourcesHash string) string {
	return daemonPortPathForSlot(sourcesHash, 0)
}

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

	// Try connecting to the TCP port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", info.Port), 3*time.Second)
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
		fmt.Fprintf(os.Stderr, "verbose: Reusing existing daemon slot %d (PID %d, port %d, sources %s)\n", slot, info.PID, info.Port, info.SourcesHash)
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
			fmt.Fprintf(os.Stderr, "verbose: Cleaning up stale daemon slot %d PID file (PID %d no longer alive)\n", slot, info.PID)
		}
		removePIDFileSlot(hash, slot)
		return
	}

	// Process is alive but we couldn't connect — it's stuck. Kill it.
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Killing unresponsive daemon slot %d (PID %d)\n", slot, info.PID)
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

// startDaemonReady is the JSON message the daemon sends when --port is used.
type startDaemonReady struct {
	Ready bool `json:"ready"`
	Port  int  `json:"port"`
}

// StartDaemonWithPort launches the krit-types JVM process in daemon mode with
// a TCP listener. The daemon auto-assigns a port and reports it on stdout.
func StartDaemonWithPort(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*Daemon, error) {
	return StartDaemonWithPortSlot(jarPath, sourceDirs, classpath, verbose, 0)
}

func StartDaemonWithPortSlot(jarPath string, sourceDirs []string, classpath []string, verbose bool, slot int) (*Daemon, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("java not found in PATH: %w", err)
	}

	args := []string{
		// G1 is the default on JDK 21; explicit here so tuning is visible.
		"-XX:+UseG1GC",
		// String deduplication cuts FQN churn in the FIR dependencies map.
		// Measured -23% peak RSS on the one-shot path; same kind of
		// repetitive string data lives in the daemon's long-lived session
		// so the same win applies. Missing from the original daemon flag
		// set — added here alongside the one-shot path.
		"-XX:+UseStringDeduplication",
		// Generous initial heap so the analyze loop doesn't grind through
		// many heap-grow GC cycles on the cold first run. 1 GB matches
		// the one-shot path.
		"-Xms1g",
		// NO -Xmx cap. A long-lived analyzer for large repos (kotlin
		// peaks at ~32 GB) should be able to use whatever the JVM
		// defaults allow, typically 25% of physical memory. The previous
		// -Xmx2g was causing constant GC thrash on Signal-Android's
		// ~5-6 GB working set and probably accounted for most of the
		// ~20 s gap between daemon cold and one-shot cold wall times.
		//
		// NO -XX:SoftRefLRUPolicyMSPerMB=1. That flag was collecting
		// soft references aggressively (1 ms per free MB), which wiped
		// the Analysis API's lazy FIR resolver cache constantly. FIR
		// is the single biggest contributor to a warm daemon's
		// throughput — wiping its cache every few hundred ms defeats
		// the whole point of keeping the daemon alive.
		//
		// NO -XX:TieredStopAtLevel=1. The old C1-only setting was
		// targeted at fast startup of a short-lived process, which is
		// the opposite of a daemon's workload. Removing it restores the
		// default tiered compilation (C1 for startup, C2 for hot
		// methods), so the per-file analyze loop can get C2-optimized
		// after the first few hundred invocations.
		//
		// Keeping ReservedCodeCacheSize=256m because Analysis API's
		// per-class bytecode is large and the 240m default can run
		// out on larger repos, which triggers JIT code-cache flushing
		// and visible latency spikes in the analyze loop.
		"-XX:ReservedCodeCacheSize=256m",
		"-Djava.awt.headless=true",
	}

	// AppCDS support (same as StartDaemon)
	if archivePath, err := cdsArchivePath(jarPath); err == nil {
		if _, statErr := os.Stat(archivePath); statErr == nil {
			args = append(args, "-XX:SharedArchiveFile="+archivePath, "-Xshare:auto")
		} else {
			args = append(args, "-XX:ArchiveClassesAtExit="+archivePath, "-Xshare:auto")
		}
	}

	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgsFromEnv())
	args = append(args, "-jar", jarPath, "--daemon", "--port", "0")
	if len(sourceDirs) > 0 {
		args = append(args, "--sources", strings.Join(sourceDirs, ","))
	}
	if len(classpath) > 0 {
		args = append(args, "--classpath", strings.Join(classpath, ","))
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Starting persistent krit-types daemon slot %d: %s %s\n", slot, javaPath, strings.Join(args, " "))
	}

	cmd := exec.Command(javaPath, args...)

	// Write daemon logs to a file
	logPath := filepath.Join(os.TempDir(), "krit-types-daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = logFile
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: Daemon log: %s\n", logPath)
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start daemon: %w", err)
	}

	// Read the ready message which includes the assigned port.
	// Use a goroutine + channel with timeout to avoid blocking forever
	// if the JVM is slow to start or the JAR doesn't exist.
	type scanResult struct {
		line string
		err  error
	}
	readyCh := make(chan scanResult, 1)
	go func() {
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)
		if sc.Scan() {
			readyCh <- scanResult{line: sc.Text()}
		} else {
			readyCh <- scanResult{err: sc.Err()}
		}
	}()

	var line string
	const daemonStartupTimeout = 30 * time.Second
	select {
	case res := <-readyCh:
		if res.err != nil {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon startup: %w", res.err)
		}
		if res.line == "" {
			cmd.Process.Kill()
			return nil, fmt.Errorf("daemon closed stdout before sending ready message")
		}
		line = res.line
	case <-time.After(daemonStartupTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon startup timed out after %s (JVM may be slow to initialize)", daemonStartupTimeout)
	}

	var ready startDaemonReady
	if err := json.Unmarshal([]byte(line), &ready); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon ready message: invalid JSON: %w (got: %s)", err, line)
	}
	if !ready.Ready || ready.Port == 0 {
		cmd.Process.Kill()
		return nil, fmt.Errorf("daemon did not report ready with port (got: %s)", line)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Daemon started on port %d (PID %d)\n", ready.Port, cmd.Process.Pid)
	}

	// Compute the sources hash once so we can both write it to the PID
	// file (for future connect-and-check) and stash it on the Daemon
	// struct (so MatchesRepo returns true for this freshly-started daemon).
	srcHash := hashSources(sourceDirs)

	// Write PID file so future invocations can find this daemon
	if err := writePIDFileSlot(cmd.Process.Pid, ready.Port, srcHash, slot); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("write PID file: %w", err)
	}

	// Connect to the daemon's TCP port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ready.Port), 5*time.Second)
	if err != nil {
		cmd.Process.Kill()
		removePIDFileSlot(srcHash, slot)
		return nil, fmt.Errorf("connect to new daemon: %w", err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 512*1024*1024)

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

// Ping sends a ping request to verify the daemon is responsive.
func (d *Daemon) Ping() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.sendResult("ping", nil)
	if err != nil {
		return err
	}

	if result == nil {
		return fmt.Errorf("ping returned nil result")
	}

	var pingResp struct {
		OK     bool  `json:"ok"`
		Uptime int64 `json:"uptime"`
	}
	if err := json.Unmarshal(*result, &pingResp); err != nil {
		return fmt.Errorf("unmarshal ping response: %w", err)
	}
	if !pingResp.OK {
		return fmt.Errorf("daemon ping returned ok=false")
	}

	return nil
}

// unmarshalOracleData parses a JSON result into OracleData.
func unmarshalOracleData(result *json.RawMessage) (*OracleData, error) {
	if result == nil {
		return nil, fmt.Errorf("daemon returned empty result")
	}

	var data OracleData
	if err := json.Unmarshal(*result, &data); err != nil {
		return nil, fmt.Errorf("unmarshal oracle data: %w", err)
	}

	return &data, nil
}
