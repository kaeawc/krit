package oracle

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Protocol marshaling/unmarshaling tests
// ---------------------------------------------------------------------------

func TestDaemonRequest_Marshal(t *testing.T) {
	req := daemonRequest{
		ID:     1,
		Method: "analyze",
		Params: map[string]interface{}{
			"files": []string{"a.kt", "b.kt"},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if int(parsed["id"].(float64)) != 1 {
		t.Errorf("expected id 1, got %v", parsed["id"])
	}
	if parsed["method"] != "analyze" {
		t.Errorf("expected method analyze, got %v", parsed["method"])
	}
	params := parsed["params"].(map[string]interface{})
	files := params["files"].([]interface{})
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
}

func TestDaemonRequest_MarshalNoParams(t *testing.T) {
	req := daemonRequest{
		ID:     5,
		Method: "analyzeAll",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// params should be omitted when nil
	if strings.Contains(string(data), `"params"`) {
		t.Errorf("expected params to be omitted, got: %s", data)
	}
}

func TestDaemonResponse_Unmarshal_Result(t *testing.T) {
	input := `{"id": 1, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`

	var resp daemonResponse
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.ID != 1 {
		t.Errorf("expected id 1, got %d", resp.ID)
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got %q", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}

	data, err := unmarshalOracleData(resp.Result)
	if err != nil {
		t.Fatalf("unmarshal oracle data: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}
	if data.KotlinVersion != "2.1.0" {
		t.Errorf("expected kotlinVersion 2.1.0, got %s", data.KotlinVersion)
	}
}

func TestDaemonResponse_Unmarshal_Error(t *testing.T) {
	input := `{"id": 2, "error": "analysis failed"}`

	var resp daemonResponse
	if err := json.Unmarshal([]byte(input), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.ID != 2 {
		t.Errorf("expected id 2, got %d", resp.ID)
	}
	if resp.Error != "analysis failed" {
		t.Errorf("expected error message, got %q", resp.Error)
	}
	if resp.Result != nil {
		t.Error("expected nil result for error response")
	}
}

func TestUnmarshalOracleData_NilResult(t *testing.T) {
	_, err := unmarshalOracleData(nil)
	if err == nil {
		t.Error("expected error for nil result")
	}
}

func TestUnmarshalOracleData_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	_, err := unmarshalOracleData(&raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalOracleData_WithDeps(t *testing.T) {
	raw := json.RawMessage(`{
		"version": 1,
		"kotlinVersion": "2.1.0",
		"files": {
			"src/App.kt": {
				"package": "com.example",
				"declarations": [{"fqn": "com.example.App", "kind": "class"}]
			}
		},
		"dependencies": {
			"kotlin.String": {"fqn": "kotlin.String", "kind": "class"}
		}
	}`)

	data, err := unmarshalOracleData(&raw)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(data.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(data.Files))
	}
	if len(data.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(data.Dependencies))
	}
	if data.Dependencies["kotlin.String"].Kind != "class" {
		t.Errorf("expected class kind for String")
	}
}

// ---------------------------------------------------------------------------
// Mock daemon tests using pipes
// ---------------------------------------------------------------------------

// newMockDaemon creates a Daemon that communicates through in-memory pipes
// instead of a real JVM process. The returned reader/writer are the "daemon side"
// of the connection (read requests from reader, write responses to writer).
func newMockDaemon(t *testing.T) (*Daemon, io.Reader, io.Writer) {
	t.Helper()

	// Go client writes to clientWriter -> daemon reads from clientReader
	clientReader, clientWriter := io.Pipe()
	// Daemon writes to daemonWriter -> Go client reads from daemonReader
	daemonReader, daemonWriter := io.Pipe()

	sc := bufio.NewScanner(daemonReader)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	d := &Daemon{
		cmd:     &exec.Cmd{}, // placeholder, not used in mock
		stdin:   clientWriter,
		stdout:  sc,
		mu:      sync.Mutex{},
		nextID:  1,
		started: true,
	}

	return d, clientReader, daemonWriter
}

func TestDaemon_AnalyzeAll_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "analyzeAll" {
			t.Errorf("expected analyzeAll, got %q", req.Method)
		}

		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}
}

func TestDaemon_Analyze_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "analyze" {
			t.Errorf("expected analyze, got %q", req.Method)
		}

		// Verify files param
		files, ok := req.Params["files"].([]interface{})
		if !ok || len(files) != 2 {
			t.Errorf("expected 2 files in params, got %v", req.Params["files"])
		}

		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {"a.kt": {"package": "com.example", "declarations": []}}, "dependencies": {}}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	data, err := d.Analyze([]string{"a.kt", "b.kt"})
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(data.Files) != 1 {
		t.Errorf("expected 1 file in result, got %d", len(data.Files))
	}
}

func TestDaemon_AnalyzeWithDeps_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "analyzeWithDeps" {
			t.Errorf("expected analyzeWithDeps, got %q", req.Method)
		}

		// Flat envelope: result, errors, cacheDeps as siblings.
		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {"a.kt": {"package": "com.example", "declarations": []}}, "dependencies": {}}, "cacheDeps": {"version": 1, "approximation": "symbol-resolved-sources", "files": {"a.kt": {"depPaths": ["b.kt"], "perFileDeps": {}}}, "crashed": {}}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	data, deps, err := d.AnalyzeWithDeps([]string{"a.kt", "b.kt"})
	if err != nil {
		t.Fatalf("AnalyzeWithDeps error: %v", err)
	}
	if len(data.Files) != 1 {
		t.Errorf("expected 1 file in result, got %d", len(data.Files))
	}
	if deps == nil {
		t.Fatal("expected non-nil CacheDepsFile")
	}
	if deps.Version != 1 {
		t.Errorf("expected deps version 1, got %d", deps.Version)
	}
	if deps.Approximation != "symbol-resolved-sources" {
		t.Errorf("unexpected approximation: %q", deps.Approximation)
	}
	if entry, ok := deps.Files["a.kt"]; !ok {
		t.Error("expected a.kt entry in cacheDeps")
	} else if len(entry.DepPaths) != 1 || entry.DepPaths[0] != "b.kt" {
		t.Errorf("expected DepPaths=[b.kt], got %v", entry.DepPaths)
	}
}

func TestDaemon_AnalyzeWithDeps_MissingCacheDeps_Error(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		// Old-protocol daemon: result only, no cacheDeps.
		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	_, _, err := d.AnalyzeWithDeps([]string{"a.kt"})
	if err == nil {
		t.Fatal("expected error for missing cacheDeps, got nil")
	}
	if !strings.Contains(err.Error(), "missing cacheDeps") {
		t.Errorf("expected 'missing cacheDeps' error, got: %v", err)
	}
}

func TestDaemon_AnalyzeWithDeps_FileNotInSession_FoldedToCrashed(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		// Response includes errors map with file-not-in-session entries.
		// Both result and cacheDeps are still populated (possibly for
		// the files that DID resolve).
		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}, "errors": {"new.kt": "File not found in source module", "also-new.kt": "File not found in source module"}, "cacheDeps": {"version": 1, "approximation": "symbol-resolved-sources", "files": {}, "crashed": {}}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	oracleData, deps, err := d.AnalyzeWithDeps([]string{"new.kt", "also-new.kt"})
	if err != nil {
		t.Fatalf("AnalyzeWithDeps should return nil error for file-not-in-session; got %v", err)
	}
	if oracleData == nil {
		t.Fatal("expected non-nil oracleData")
	}
	if deps == nil {
		t.Fatal("expected non-nil cacheDeps")
	}
	// File-not-in-session errors must have been folded into deps.Crashed
	// so the caller writes poison markers for them via the normal
	// WriteFreshEntries pipeline. Next invocation's ClassifyFiles will
	// treat those files as hits and skip them — eliminating the waste
	// of re-requesting structurally-excluded files every run.
	if len(deps.Crashed) != 2 {
		t.Errorf("expected 2 crashed entries, got %d", len(deps.Crashed))
	}
	for _, want := range []string{"new.kt", "also-new.kt"} {
		if _, ok := deps.Crashed[want]; !ok {
			t.Errorf("expected %q in deps.Crashed, got keys %v", want, keysOf(deps.Crashed))
		}
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestHashSources_Deterministic(t *testing.T) {
	h1 := hashSources([]string{"/a", "/b", "/c"})
	h2 := hashSources([]string{"/a", "/b", "/c"})
	if h1 != h2 {
		t.Errorf("hashSources not deterministic: %q vs %q", h1, h2)
	}
}

func TestHashSources_OrderIndependent(t *testing.T) {
	h1 := hashSources([]string{"/a", "/b", "/c"})
	h2 := hashSources([]string{"/c", "/a", "/b"})
	if h1 != h2 {
		t.Errorf("hashSources should be order-independent: %q vs %q", h1, h2)
	}
}

func TestHashSources_DifferentInputsDifferentHashes(t *testing.T) {
	h1 := hashSources([]string{"/a", "/b"})
	h2 := hashSources([]string{"/a", "/c"})
	if h1 == h2 {
		t.Errorf("different sources should hash differently, both = %q", h1)
	}
}

func TestHashSources_LengthIs16(t *testing.T) {
	h := hashSources([]string{"/some/path"})
	if len(h) != 16 {
		t.Errorf("expected 16-char hash, got %d chars: %q", len(h), h)
	}
}

func TestDaemon_MatchesRepo_SameHash(t *testing.T) {
	d := &Daemon{sourcesHash: hashSources([]string{"/repo/a", "/repo/b"})}
	if !d.MatchesRepo([]string{"/repo/a", "/repo/b"}) {
		t.Error("MatchesRepo should return true for identical sources")
	}
	if !d.MatchesRepo([]string{"/repo/b", "/repo/a"}) {
		t.Error("MatchesRepo should return true for reordered sources")
	}
}

func TestDaemon_MatchesRepo_DifferentHash(t *testing.T) {
	d := &Daemon{sourcesHash: hashSources([]string{"/repo/a"})}
	if d.MatchesRepo([]string{"/repo/b"}) {
		t.Error("MatchesRepo should return false for different sources")
	}
}

func TestDaemon_MatchesRepo_EmptyHash(t *testing.T) {
	// An older daemon that didn't write daemon.sources has an empty
	// sourcesHash. MatchesRepo returns false so callers fall back.
	d := &Daemon{sourcesHash: ""}
	if d.MatchesRepo([]string{"/repo/a"}) {
		t.Error("MatchesRepo should return false when sourcesHash is empty")
	}
}

func TestWriteReadPIDFile_RoundTripWithSources(t *testing.T) {
	// Use a temp cache dir so we don't clobber a real daemon.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	srcHash := hashSources([]string{"/fake/repo"})
	if err := writePIDFile(12345, 6789, srcHash); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}
	defer removePIDFile(srcHash)

	info, err := readPIDFile(srcHash)
	if err != nil {
		t.Fatalf("readPIDFile: %v", err)
	}
	if info.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", info.PID)
	}
	if info.Port != 6789 {
		t.Errorf("expected port 6789, got %d", info.Port)
	}
	if info.SourcesHash != srcHash {
		t.Errorf("expected sources hash %q, got %q", srcHash, info.SourcesHash)
	}
}

func TestDaemon_Rebuild_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "rebuild" {
			t.Errorf("expected rebuild, got %q", req.Method)
		}

		resp := fmt.Sprintf(`{"id": %d, "result": {}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	err := d.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild error: %v", err)
	}
}

func TestDaemon_ErrorResponse_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		resp := fmt.Sprintf(`{"id": %d, "error": "compilation failed"}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	_, err := d.AnalyzeAll()
	if err == nil {
		t.Fatal("expected error from daemon")
	}
	if !strings.Contains(err.Error(), "compilation failed") {
		t.Errorf("expected error message containing 'compilation failed', got: %v", err)
	}
}

func TestDaemon_IDMismatch_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		// Respond with wrong ID
		resp := `{"id": 999, "result": {}}` + "\n"
		respWriter.Write([]byte(resp))
	}()

	_, err := d.AnalyzeAll()
	if err == nil {
		t.Fatal("expected error for ID mismatch")
	}
	if !strings.Contains(err.Error(), "ID mismatch") {
		t.Errorf("expected ID mismatch error, got: %v", err)
	}
}

func TestDaemon_SendNotStarted(t *testing.T) {
	d := &Daemon{started: false}

	_, err := d.AnalyzeAll()
	if err == nil {
		t.Fatal("expected error when daemon not started")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("expected 'not started' error, got: %v", err)
	}
}

func TestDaemon_ShutdownNotStarted(t *testing.T) {
	d := &Daemon{started: false}
	err := d.Shutdown()
	if err != nil {
		t.Errorf("expected nil error for shutdown of non-started daemon, got: %v", err)
	}
}

func TestDaemon_RequestTimeout_KillsAndMarksUnstarted(t *testing.T) {
	// Simulate a daemon that hangs indefinitely after receiving a request.
	// We drain the request from the fake daemon side but never write a
	// response — the Daemon.send() goroutine's Scan() should block, the
	// timeout should fire, and send() should return an error without
	// deadlocking.
	d, reqReader, _ := newMockDaemon(t)

	// Drain the request so the client-side Write doesn't block. The
	// daemon-side pipe stays open (never closed) so Scan() will hang until
	// the timeout fires.
	go func() {
		sc := bufio.NewScanner(reqReader)
		sc.Scan() // consume the request; never write a response
	}()

	// Shrink the timeout to 300ms so the test runs fast.
	t.Setenv("KRIT_TYPES_REQUEST_TIMEOUT", "300ms")

	start := time.Now()
	_, err := d.AnalyzeAll()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected 'timed out' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "method=analyzeAll") {
		t.Errorf("expected error to include method=analyzeAll, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected timeout to fire within ~300ms, took %s", elapsed)
	}
	if d.started {
		t.Errorf("expected daemon to be marked unstarted after timeout")
	}

	// Follow-up call should short-circuit with "not started" instead of
	// deadlocking on a second timeout.
	_, err = d.AnalyzeAll()
	if err == nil || !strings.Contains(err.Error(), "not started") {
		t.Errorf("expected second call to fail fast with 'not started', got: %v", err)
	}
}

func TestDaemon_CloseNotStarted(t *testing.T) {
	d := &Daemon{started: false}
	err := d.Close()
	if err != nil {
		t.Errorf("expected nil error for close of non-started daemon, got: %v", err)
	}
}

func TestDaemon_SequentialRequests_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)

		// Handle first request (analyzeAll)
		if !sc.Scan() {
			return
		}
		var req1 daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req1)
		resp1 := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`, req1.ID) + "\n"
		respWriter.Write([]byte(resp1))

		// Handle second request (rebuild)
		if !sc.Scan() {
			return
		}
		var req2 daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req2)
		if req2.Method != "rebuild" {
			t.Errorf("expected rebuild, got %q", req2.Method)
		}
		resp2 := fmt.Sprintf(`{"id": %d, "result": {}}`, req2.ID) + "\n"
		respWriter.Write([]byte(resp2))

		// Handle third request (analyze)
		if !sc.Scan() {
			return
		}
		var req3 daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req3)
		if req3.Method != "analyze" {
			t.Errorf("expected analyze, got %q", req3.Method)
		}
		resp3 := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {"x.kt": {"package": "x", "declarations": []}}, "dependencies": {}}}`, req3.ID) + "\n"
		respWriter.Write([]byte(resp3))
	}()

	// First: analyzeAll
	data1, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if data1.Version != 1 {
		t.Errorf("expected version 1")
	}

	// Second: rebuild
	if err := d.Rebuild(); err != nil {
		t.Fatalf("Rebuild error: %v", err)
	}

	// Third: analyze
	data3, err := d.Analyze([]string{"x.kt"})
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if len(data3.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(data3.Files))
	}

	// Verify IDs incremented
	if d.nextID != 4 {
		t.Errorf("expected nextID=4, got %d", d.nextID)
	}
}

// ---------------------------------------------------------------------------
// AppCDS archive path tests
// ---------------------------------------------------------------------------

func TestCdsArchivePath_ValidJar(t *testing.T) {
	// Create a temporary JAR file
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	if err := os.WriteFile(jarPath, []byte("fake jar content"), 0644); err != nil {
		t.Fatalf("write temp jar: %v", err)
	}

	path, err := cdsArchivePath(jarPath)
	if err != nil {
		t.Fatalf("cdsArchivePath error: %v", err)
	}

	if !strings.HasSuffix(path, ".jsa") {
		t.Errorf("expected .jsa suffix, got %q", path)
	}
	if !strings.Contains(path, "krit-types-") {
		t.Errorf("expected krit-types- prefix in filename, got %q", path)
	}
}

func TestCdsArchivePath_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()
	jar1 := filepath.Join(tmpDir, "a.jar")
	jar2 := filepath.Join(tmpDir, "b.jar")
	os.WriteFile(jar1, []byte("content A"), 0644)
	os.WriteFile(jar2, []byte("content B"), 0644)

	path1, _ := cdsArchivePath(jar1)
	path2, _ := cdsArchivePath(jar2)

	if path1 == path2 {
		t.Error("expected different archive paths for different JAR content")
	}
}

func TestCdsArchivePath_SameContent(t *testing.T) {
	tmpDir := t.TempDir()
	jar1 := filepath.Join(tmpDir, "a.jar")
	jar2 := filepath.Join(tmpDir, "b.jar")
	os.WriteFile(jar1, []byte("same content"), 0644)
	os.WriteFile(jar2, []byte("same content"), 0644)

	path1, _ := cdsArchivePath(jar1)
	path2, _ := cdsArchivePath(jar2)

	if path1 != path2 {
		t.Error("expected same archive paths for identical JAR content")
	}
}

func TestCdsArchivePath_MissingJar(t *testing.T) {
	_, err := cdsArchivePath("/nonexistent/path.jar")
	if err == nil {
		t.Error("expected error for nonexistent JAR")
	}
}

func TestCracCheckpointPath_ValidJar(t *testing.T) {
	tmpDir := t.TempDir()
	jarPath := filepath.Join(tmpDir, "test.jar")
	os.WriteFile(jarPath, []byte("fake jar content"), 0644)

	path, err := cracCheckpointPath(jarPath)
	if err != nil {
		t.Fatalf("cracCheckpointPath error: %v", err)
	}

	if !strings.HasSuffix(path, ".crac") {
		t.Errorf("expected .crac suffix, got %q", path)
	}
}

// ---------------------------------------------------------------------------
// Checkpoint mock test
// ---------------------------------------------------------------------------

func TestDaemon_Checkpoint_CRaCNotAvailable_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "checkpoint" {
			t.Errorf("expected checkpoint, got %q", req.Method)
		}

		// Simulate CRaC not available (typical response from non-CRaC JDK)
		resp := fmt.Sprintf(`{"id": %d, "error": "CRaC not available"}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	// Checkpoint should return nil (graceful degradation) when CRaC is not available
	err := d.Checkpoint()
	if err != nil {
		t.Fatalf("Checkpoint should degrade gracefully, got error: %v", err)
	}
}

func TestDaemon_Checkpoint_Success_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		resp := fmt.Sprintf(`{"id": %d, "result": {"ok": true, "restored": true}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	err := d.Checkpoint()
	if err != nil {
		t.Fatalf("Checkpoint error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Ping mock tests
// ---------------------------------------------------------------------------

func TestDaemon_Ping_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "ping" {
			t.Errorf("expected ping, got %q", req.Method)
		}

		resp := fmt.Sprintf(`{"id": %d, "result": {"ok": true, "uptime": 12345}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	err := d.Ping()
	if err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestDaemon_Ping_NotOK_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		resp := fmt.Sprintf(`{"id": %d, "result": {"ok": false, "uptime": 0}}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	err := d.Ping()
	if err == nil {
		t.Fatal("expected error for ok=false ping")
	}
	if !strings.Contains(err.Error(), "ok=false") {
		t.Errorf("expected ok=false error, got: %v", err)
	}
}

func TestDaemon_Ping_Error_Mock(t *testing.T) {
	d, reqReader, respWriter := newMockDaemon(t)

	go func() {
		sc := bufio.NewScanner(reqReader)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		resp := fmt.Sprintf(`{"id": %d, "error": "not ready"}`, req.ID) + "\n"
		respWriter.Write([]byte(resp))
	}()

	err := d.Ping()
	if err == nil {
		t.Fatal("expected error for daemon error response")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("expected 'not ready' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PID file tests
// ---------------------------------------------------------------------------

func TestWriteAndReadPIDFile(t *testing.T) {
	// Use temp dir to avoid clobbering real PID files
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "daemon.pid")
	portPath := filepath.Join(tmpDir, "daemon.port")

	// Write
	os.WriteFile(pidPath, []byte("12345\n"), 0644)
	os.WriteFile(portPath, []byte("54321\n"), 0644)

	// Read
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	if pid != 12345 {
		t.Errorf("expected PID 12345, got %d", pid)
	}

	portData, err := os.ReadFile(portPath)
	if err != nil {
		t.Fatalf("read port: %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(portData)))
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	if port != 54321 {
		t.Errorf("expected port 54321, got %d", port)
	}
}

func TestIsProcessAlive_Self(t *testing.T) {
	// Our own process should be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("expected own process to be alive")
	}
}

func TestIsProcessAlive_NonExistent(t *testing.T) {
	// PID 99999999 should not exist
	if isProcessAlive(99999999) {
		t.Error("expected PID 99999999 to not be alive")
	}
}

// ---------------------------------------------------------------------------
// Shared daemon Close behavior tests
// ---------------------------------------------------------------------------

func TestDaemon_Close_Shared(t *testing.T) {
	// A shared daemon should just close the connection, not shut down the process
	d := &Daemon{
		started: true,
		shared:  true,
	}

	err := d.Close()
	if err != nil {
		t.Errorf("expected no error for shared daemon close, got: %v", err)
	}
	if d.started {
		t.Error("expected started=false after close")
	}
}

func TestDaemon_Close_Owned(t *testing.T) {
	// An owned, non-started daemon should just clean up without error
	d := &Daemon{
		started: false,
		shared:  false,
	}

	err := d.Close()
	if err != nil {
		t.Errorf("expected no error for non-started owned daemon close, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TCP-based mock daemon for persistent daemon tests
// ---------------------------------------------------------------------------

// newTCPMockDaemon creates a TCP listener that acts as a mock daemon,
// returning the Daemon connected to it and the listener for cleanup.
func newTCPMockDaemon(t *testing.T, handler func(conn net.Conn)) (*Daemon, net.Listener) {
	t.Helper()

	listener := listenLocalTCP(t)

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		handler(conn)
	}()

	// Connect to the mock daemon
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		listener.Close()
		t.Fatalf("connect: %v", err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	d := &Daemon{
		stdin:   conn,
		stdout:  reader,
		conn:    conn,
		port:    port,
		nextID:  1,
		started: true,
		shared:  true,
	}

	return d, listener
}

func TestDaemon_Ping_TCP_Mock(t *testing.T) {
	d, listener := newTCPMockDaemon(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		resp := fmt.Sprintf(`{"id": %d, "result": {"ok": true, "uptime": 5000}}`, req.ID) + "\n"
		conn.Write([]byte(resp))
	})
	defer listener.Close()
	defer d.Close()

	err := d.Ping()
	if err != nil {
		t.Fatalf("TCP Ping error: %v", err)
	}
}

func TestDaemon_AnalyzeAll_TCP_Mock(t *testing.T) {
	d, listener := newTCPMockDaemon(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		if !sc.Scan() {
			return
		}
		var req daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req)

		if req.Method != "analyzeAll" {
			t.Errorf("expected analyzeAll, got %q", req.Method)
		}

		resp := fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`, req.ID) + "\n"
		conn.Write([]byte(resp))
	})
	defer listener.Close()
	defer d.Close()

	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("TCP AnalyzeAll error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}
}

func TestDaemon_MultipleRequests_TCP_Mock(t *testing.T) {
	d, listener := newTCPMockDaemon(t, func(conn net.Conn) {
		defer conn.Close()
		sc := bufio.NewScanner(conn)

		// Handle ping
		if !sc.Scan() {
			return
		}
		var req1 daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req1)
		conn.Write([]byte(fmt.Sprintf(`{"id": %d, "result": {"ok": true, "uptime": 100}}`, req1.ID) + "\n"))

		// Handle analyzeAll
		if !sc.Scan() {
			return
		}
		var req2 daemonRequest
		json.Unmarshal([]byte(sc.Text()), &req2)
		conn.Write([]byte(fmt.Sprintf(`{"id": %d, "result": {"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}}`, req2.ID) + "\n"))
	})
	defer listener.Close()
	defer d.Close()

	// Ping first
	if err := d.Ping(); err != nil {
		t.Fatalf("Ping error: %v", err)
	}

	// Then analyze
	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("AnalyzeAll error: %v", err)
	}
	if data.Version != 1 {
		t.Errorf("expected version 1, got %d", data.Version)
	}

	// Verify IDs incremented
	if d.nextID != 3 {
		t.Errorf("expected nextID=3, got %d", d.nextID)
	}
}

// ---------------------------------------------------------------------------
// connectExistingDaemon integration test with real TCP
// ---------------------------------------------------------------------------

func TestConnectExistingDaemon_NoFiles(t *testing.T) {
	// With no PID file, connectExistingDaemon should fail
	// Ensure no PID file exists by using a custom HOME
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	_, err := connectExistingDaemon(testSourceDirs, false)
	if err == nil {
		t.Fatal("expected error when no PID file exists")
	}
}

func TestConnectExistingDaemon_DeadPID(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create daemons dir and write a PID file with a dead PID under the
	// test repo's sources hash
	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".pid"), []byte("99999999\n"), 0644)
	os.WriteFile(filepath.Join(daemonsDir, testSourcesHash+".port"), []byte("12345\n"), 0644)

	_, err := connectExistingDaemon(testSourceDirs, false)
	if err == nil {
		t.Fatal("expected error for dead PID")
	}
	if !strings.Contains(err.Error(), "not alive") {
		t.Errorf("expected 'not alive' error, got: %v", err)
	}
}

func TestCleanStaleDaemon_NoFiles(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Should not panic with no PID file
	cleanStaleDaemon(testSourceDirs, false)
}

func TestCleanStaleDaemon_DeadPID(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	daemonsDir := filepath.Join(tmpHome, ".krit", "cache", "daemons")
	os.MkdirAll(daemonsDir, 0755)
	pidFile := filepath.Join(daemonsDir, testSourcesHash+".pid")
	portFile := filepath.Join(daemonsDir, testSourcesHash+".port")
	os.WriteFile(pidFile, []byte("99999999\n"), 0644)
	os.WriteFile(portFile, []byte("12345\n"), 0644)

	cleanStaleDaemon(testSourceDirs, false)

	// PID files should be removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed after cleanup")
	}
	if _, err := os.Stat(portFile); !os.IsNotExist(err) {
		t.Error("expected port file to be removed after cleanup")
	}
}

func TestDaemonCacheDir(t *testing.T) {
	dir, err := daemonCacheDir()
	if err != nil {
		t.Fatalf("daemonCacheDir error: %v", err)
	}
	if !strings.Contains(dir, ".krit") || !strings.HasSuffix(dir, "cache") {
		t.Errorf("unexpected cache dir: %q", dir)
	}
	// Directory should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat cache dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected cache dir to be a directory")
	}
}

func TestStartDaemonReady_Marshal(t *testing.T) {
	ready := startDaemonReady{Ready: true, Port: 8080}
	data, err := json.Marshal(ready)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"ready":true`) {
		t.Errorf("expected ready:true in JSON, got: %s", s)
	}
	if !strings.Contains(s, `"port":8080`) {
		t.Errorf("expected port:8080 in JSON, got: %s", s)
	}
}

func TestStartDaemonReady_Unmarshal(t *testing.T) {
	input := `{"ready":true,"port":54321}`
	var ready startDaemonReady
	if err := json.Unmarshal([]byte(input), &ready); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !ready.Ready {
		t.Error("expected ready=true")
	}
	if ready.Port != 54321 {
		t.Errorf("expected port 54321, got %d", ready.Port)
	}
}

// ---------------------------------------------------------------------------
// Project Leyden AOT helper tests
// ---------------------------------------------------------------------------

func TestJdkMajorVersion_Modern(t *testing.T) {
	cases := []struct {
		output string
		want   int
	}{
		{`openjdk version "25.0.2" 2025-01-21`, 25},
		{`openjdk version "21.0.3" 2024-04-16`, 21},
		{`java version "17.0.9" 2023-10-17`, 17},
		{`java version "11.0.21" 2023-10-17`, 11},
		{`java version "1.8.0_392" 2023-10-17`, 8},
		{`openjdk version "24-ea" 2025-03-18`, 24},
		{``, 0},
		{`garbage output no quotes`, 0},
	}

	for _, tc := range cases {
		// Write a fake "java" script that outputs the test string to stderr.
		dir := t.TempDir()
		fakeJava := filepath.Join(dir, "java")
		script := "#!/bin/sh\necho '" + tc.output + "' >&2\n"
		if err := os.WriteFile(fakeJava, []byte(script), 0755); err != nil {
			t.Fatalf("write fake java: %v", err)
		}
		got := jdkMajorVersion(fakeJava)
		if got != tc.want {
			t.Errorf("jdkMajorVersion(%q) = %d, want %d", tc.output, got, tc.want)
		}
	}
}

func TestAotConfigPath_KeyedByHash(t *testing.T) {
	// Two different JAR contents must produce different config paths.
	dir := t.TempDir()
	jar1 := filepath.Join(dir, "a.jar")
	jar2 := filepath.Join(dir, "b.jar")
	if err := os.WriteFile(jar1, []byte("content-a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jar2, []byte("content-b"), 0644); err != nil {
		t.Fatal(err)
	}

	p1, err := aotConfigPath(jar1)
	if err != nil {
		t.Fatalf("aotConfigPath jar1: %v", err)
	}
	p2, err := aotConfigPath(jar2)
	if err != nil {
		t.Fatalf("aotConfigPath jar2: %v", err)
	}
	if p1 == p2 {
		t.Errorf("expected different paths for different JARs, both got %q", p1)
	}
	if !strings.HasSuffix(p1, ".aotconf") {
		t.Errorf("expected .aotconf suffix, got %q", p1)
	}
}

func TestAotCachePath_KeyedByHash(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "test.jar")
	if err := os.WriteFile(jar, []byte("jar-content"), 0644); err != nil {
		t.Fatal(err)
	}

	p, err := aotCachePath(jar)
	if err != nil {
		t.Fatalf("aotCachePath: %v", err)
	}
	if !strings.HasSuffix(p, ".aot") {
		t.Errorf("expected .aot suffix, got %q", p)
	}

	// Config and cache paths must differ for the same JAR.
	c, err := aotConfigPath(jar)
	if err != nil {
		t.Fatalf("aotConfigPath: %v", err)
	}
	if p == c {
		t.Errorf("aotCachePath and aotConfigPath should differ, both %q", p)
	}
}
