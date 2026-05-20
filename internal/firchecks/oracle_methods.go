package firchecks

// oracle_methods.go — analyze/analyzeAll/analyzeWithDeps RPCs on the
// long-lived krit-fir daemon. The Kotlin side already speaks these
// commands (see tools/krit-fir/.../Main.kt's analyze branches). This
// file is the Go-side wrapper so `--daemon --oracle-backend=fir` can
// reuse a warm JVM instead of paying the cold-start cost per analyze.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

// firAnalyzeResponse is the envelope krit-fir's OracleResponse.buildAnalyze
// produces. Result mirrors oracle.Data so a Go-side consumer parses
// the same fields the krit-types daemon ships.
type firAnalyzeResponse struct {
	ID     int64        `json:"id"`
	Result *oracle.Data `json:"result"`
	Error  string       `json:"error,omitempty"`
}

// firAnalyzeWithDepsResponse is the flat envelope krit-fir produces for
// the analyzeWithDeps command — result, errors, and cacheDeps are
// siblings (not nested), matching krit-types' buildDaemonResponseWithDeps.
type firAnalyzeWithDepsResponse struct {
	ID        int64                 `json:"id"`
	Result    *oracle.Data          `json:"result"`
	Errors    map[string]string     `json:"errors,omitempty"`
	CacheDeps *oracle.CacheDepsFile `json:"cacheDeps,omitempty"`
	Error     string                `json:"error,omitempty"`
}

// Analyze sends an `analyze` request to the krit-fir daemon and
// returns the parsed oracle data. The daemon analyzes the listed
// [files] against the session's sourceDirs/classpath (set at startup
// via ConnectOrStartFirDaemon).
//
// `files` may be nil/empty — pass nil to run a full project sweep
// (equivalent to [AnalyzeAll]).
func (d *FirDaemon) Analyze(files, sourceDirs, classpath []string) (*oracle.Data, error) {
	command := "analyze"
	if len(files) == 0 {
		command = "analyzeAll"
	}
	return d.runAnalyze(command, toFileRefs(files), sourceDirs, classpath)
}

// AnalyzeAll is the explicit form of [Analyze] with no per-file slice
// — sends `analyzeAll` and returns the full-project oracle data.
func (d *FirDaemon) AnalyzeAll(sourceDirs, classpath []string) (*oracle.Data, error) {
	return d.runAnalyze("analyzeAll", nil, sourceDirs, classpath)
}

// AnalyzeWithDeps sends an `analyzeWithDeps` request and parses out
// both the oracle data and the cache-deps closure the Go-side cache
// layer needs. Mirrors `oracle.Daemon.AnalyzeWithDeps`.
func (d *FirDaemon) AnalyzeWithDeps(files, sourceDirs, classpath []string) (*oracle.Data, *oracle.CacheDepsFile, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return nil, nil, fmt.Errorf("fir daemon not started")
	}
	id := d.nextID
	d.nextID++
	req := firDaemonRequest{
		ID:         id,
		Command:    "analyzeWithDeps",
		Files:      toFileRefs(files),
		SourceDirs: sourceDirs,
		Classpath:  classpath,
	}
	line, err := d.sendAndReceive(req)
	if err != nil {
		return nil, nil, err
	}
	var resp firAnalyzeWithDepsResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal fir analyzeWithDeps response: %w (got: %s)", err, line)
	}
	if resp.Error != "" {
		return nil, nil, fmt.Errorf("fir daemon error: %s", resp.Error)
	}
	if resp.ID != id {
		return nil, nil, fmt.Errorf("fir response ID mismatch: expected %d, got %d", id, resp.ID)
	}
	if resp.Result != nil && len(resp.Errors) > 0 {
		// Surface fatal per-file errors by returning a non-nil Data
		// alongside the err so the caller can decide whether to
		// proceed with partial results. Matches oracle.Daemon's
		// behavior where errors live inside `result` for the legacy
		// shape but are surfaced separately here.
		return resp.Result, resp.CacheDeps, fmt.Errorf("fir daemon analyzeWithDeps errors: %v", resp.Errors)
	}
	return resp.Result, resp.CacheDeps, nil
}

// runAnalyze is the shared body of Analyze / AnalyzeAll. Held lock,
// id alloc, TCP send, response parse all live here so the public
// methods stay declarative.
func (d *FirDaemon) runAnalyze(command string, files []fileRef, sourceDirs, classpath []string) (*oracle.Data, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started {
		return nil, fmt.Errorf("fir daemon not started")
	}
	id := d.nextID
	d.nextID++
	req := firDaemonRequest{
		ID:         id,
		Command:    command,
		Files:      files,
		SourceDirs: sourceDirs,
		Classpath:  classpath,
	}
	line, err := d.sendAndReceive(req)
	if err != nil {
		return nil, err
	}
	var resp firAnalyzeResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal fir %s response: %w (got: %s)", command, err, line)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("fir daemon %s error: %s", command, resp.Error)
	}
	if resp.ID != id {
		return nil, fmt.Errorf("fir response ID mismatch: expected %d, got %d", id, resp.ID)
	}
	return resp.Result, nil
}

// sendAndReceive writes [req] as newline-delimited JSON and reads one
// response line. Mirrors the scan-with-timeout pattern in [Check] but
// factored out so the analyze methods don't duplicate it.
//
// Caller must hold d.mu.
func (d *FirDaemon) sendAndReceive(req firDaemonRequest) (string, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal fir request: %w", err)
	}
	data = append(data, '\n')
	if _, err := d.conn.Write(data); err != nil {
		return "", fmt.Errorf("write to fir daemon: %w", err)
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
	timeout := daemonRequestTimeout()
	select {
	case res := <-ch:
		if !res.ok {
			if res.err != nil {
				return "", fmt.Errorf("read from fir daemon: %w", res.err)
			}
			return "", fmt.Errorf("fir daemon closed stdout unexpectedly")
		}
		return res.line, nil
	case <-time.After(timeout):
		if d.cmd != nil && d.cmd.Process != nil {
			_ = d.cmd.Process.Kill()
		}
		d.started = false
		return "", fmt.Errorf("fir daemon request timed out after %s; daemon killed", timeout)
	}
}

func toFileRefs(paths []string) []fileRef {
	if len(paths) == 0 {
		return nil
	}
	out := make([]fileRef, len(paths))
	for i, p := range paths {
		out[i] = fileRef{Path: p}
	}
	return out
}
