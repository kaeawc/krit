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
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/perf"
)

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

	// Breaker state layers a soft-open under the started=false hard-fail.
	// The hard-fail is permanent (set on process death); the breaker
	// catches softer errors — JVM "Analysis API exception" responses, ID
	// mismatches, transient write/read hiccups — and short-circuits
	// subsequent calls until cooldown lets a probe through.
	breakerMu       sync.Mutex
	breakerFailures int
	breakerOpenedAt time.Time
}

// daemonNow is the time source for breaker cooldown; tests override it.
var daemonNow = time.Now

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

// Analyze sends an incremental analysis request for specific files.
func (d *Daemon) Analyze(files []string) (*Data, error) {
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
func (d *Daemon) AnalyzeAll() (*Data, error) {
	return d.AnalyzeAllWithCallFilter(nil)
}

// AnalyzeAllWithCallFilter sends a full analysis request for all files,
// optionally narrowing call-target resolution in the JVM oracle.
func (d *Daemon) AnalyzeAllWithCallFilter(callFilter *CallTargetFilterSummary) (*Data, error) {
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

// DecompileJar asks krit-types to produce Kotlin source for a class resolved
// from the daemon's library module.
func (d *Daemon) DecompileJar(jarPath, fqn string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.sendResult("decompileJar", map[string]interface{}{
		"jarPath": jarPath,
		"fqn":     fqn,
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Text string `json:"text"`
	}
	if result == nil {
		return "", fmt.Errorf("decompileJar returned nil result")
	}
	if err := json.Unmarshal(*result, &resp); err != nil {
		return "", fmt.Errorf("unmarshal decompileJar response: %w", err)
	}
	if resp.Text == "" {
		return "", fmt.Errorf("decompileJar returned empty text")
	}
	return resp.Text, nil
}

// PluginRuleDescriptor is the daemon-reported metadata for one Kotlin
// custom rule loaded from a plugin jar.
type PluginRuleDescriptor struct {
	RuleID     string   `json:"ruleId"`
	Category   string   `json:"category"`
	Severity   string   `json:"severity"`
	Maturity   string   `json:"maturity"`
	Languages  []string `json:"languages"`
	Needs      []string `json:"needs"`
	SDKVersion string   `json:"sdkVersion,omitempty"`
}

// ListPluginsResult is the result payload for the krit-types listPlugins verb.
type ListPluginsResult struct {
	Rules []PluginRuleDescriptor `json:"rules"`
}

// AnalyzePluginFileResult is the result payload for the krit-types analyzeFile verb.
type AnalyzePluginFileResult struct {
	Findings []PluginFinding   `json:"findings"`
	Errors   map[string]string `json:"errors,omitempty"`
}

// PluginFinding is a custom Kotlin rule finding before conversion into
// scanner.FindingColumns by the Go pipeline.
type PluginFinding struct {
	File       string     `json:"file"`
	Line       int        `json:"line"`
	Column     int        `json:"column"`
	StartByte  int        `json:"startByte,omitempty"`
	EndByte    int        `json:"endByte,omitempty"`
	RuleSet    string     `json:"ruleSet"`
	RuleID     string     `json:"ruleId"`
	Severity   string     `json:"severity"`
	Message    string     `json:"message"`
	Confidence float64    `json:"confidence,omitempty"`
	Fix        *PluginFix `json:"fix,omitempty"`
}

// PluginFix is a line-oriented text edit produced by a Kotlin custom rule.
type PluginFix struct {
	StartLine   int    `json:"startLine"`
	EndLine     int    `json:"endLine"`
	Replacement string `json:"replacement"`
	Safety      string `json:"safety,omitempty"`
}

// ListPlugins loads plugin jars into the daemon and returns their descriptors.
func (d *Daemon) ListPlugins(jars []string) (ListPluginsResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.sendResult("listPlugins", map[string]interface{}{
		"jars": jars,
	})
	if err != nil {
		return ListPluginsResult{}, err
	}
	if result == nil {
		return ListPluginsResult{}, fmt.Errorf("listPlugins returned nil result")
	}
	var out ListPluginsResult
	if err := json.Unmarshal(*result, &out); err != nil {
		return ListPluginsResult{}, fmt.Errorf("unmarshal listPlugins response: %w", err)
	}
	return out, nil
}

// AnalyzePluginFile runs selected Kotlin custom rules against one source file.
func (d *Daemon) AnalyzePluginFile(jars []string, path string, source []byte, ruleIDs []string) (AnalyzePluginFileResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	params := map[string]interface{}{
		"jars":    jars,
		"path":    path,
		"source":  string(source),
		"ruleIds": ruleIDs,
	}
	result, err := d.sendResult("analyzeFile", params)
	if err != nil {
		return AnalyzePluginFileResult{}, err
	}
	if result == nil {
		return AnalyzePluginFileResult{}, fmt.Errorf("analyzeFile returned nil result")
	}
	var out AnalyzePluginFileResult
	if err := json.Unmarshal(*result, &out); err != nil {
		return AnalyzePluginFileResult{}, fmt.Errorf("unmarshal analyzeFile response: %w", err)
	}
	return out, nil
}

// AnalyzeWithDeps is the cache-aware variant of Analyze. It asks the
// daemon to run analysis with a DepTracker instrumented per file and
// returns both the Data AND the per-file dep closure the Go-side
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
func (d *Daemon) AnalyzeWithDeps(files []string) (*Data, *CacheDepsFile, error) {
	data, deps, _, err := d.AnalyzeWithDepsWithTimings(files, false, nil, nil)
	return data, deps, err
}

// AnalyzeWithDepsWithTimings is AnalyzeWithDeps plus optional Kotlin-side
// timing entries returned by newer daemon processes.
func (d *Daemon) AnalyzeWithDepsWithTimings(files []string, collectTimings bool, callFilter *CallTargetFilterSummary, declarationProfile *DeclarationProfileSummary) (*Data, *CacheDepsFile, []perf.TimingEntry, error) {
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

// Close shuts down and cleans up. For shared (reused) daemons, only the TCP
// connection is closed — the daemon process keeps running for future clients.
// For owned daemons, the process is shut down and PID files are removed.
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

// unmarshalOracleData parses a JSON result into Data.
func unmarshalOracleData(result *json.RawMessage) (*Data, error) {
	if result == nil {
		return nil, fmt.Errorf("daemon returned empty result")
	}

	var data Data
	if err := json.Unmarshal(*result, &data); err != nil {
		return nil, fmt.Errorf("unmarshal oracle data: %w", err)
	}

	return &data, nil
}
