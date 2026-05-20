package firchecks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeFirOracleServer is a small TCP listener that fields one
// firDaemonRequest per connection and returns a canned response keyed
// by command. Used to exercise the analyze / analyzeAll /
// analyzeWithDeps round-trip without a real JVM in the loop.
type fakeFirOracleServer struct {
	listener  net.Listener
	responses map[string]string
	requests  []firDaemonRequest
	mu        sync.Mutex
}

func newFakeFirOracleServer(t *testing.T) *fakeFirOracleServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	s := &fakeFirOracleServer{
		listener:  ln,
		responses: map[string]string{},
	}
	go s.serve()
	return s
}

func (s *fakeFirOracleServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeFirOracleServer) handle(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req firDaemonRequest
		if err := json.Unmarshal([]byte(sc.Text()), &req); err != nil {
			fmt.Fprintf(conn, "{\"id\":0,\"error\":\"invalid json: %s\"}\n", err)
			continue
		}
		s.mu.Lock()
		s.requests = append(s.requests, req)
		body, ok := s.responses[req.Command]
		s.mu.Unlock()
		if !ok {
			fmt.Fprintf(conn, "{\"id\":%d,\"error\":\"no canned response for %s\"}\n", req.ID, req.Command)
			continue
		}
		// Inject the request ID into the canned body via a simple
		// `%d` placeholder so the fake server tracks the correlation
		// rules expect.
		response := strings.ReplaceAll(body, "{{ID}}", fmt.Sprintf("%d", req.ID))
		fmt.Fprintln(conn, response)
	}
}

// connect dials the fake server and constructs a FirDaemon ready for
// analyze calls. Callers t.Cleanup the returned daemon.
func (s *fakeFirOracleServer) connect(t *testing.T) *FirDaemon {
	t.Helper()
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	d := &FirDaemon{
		conn:    conn,
		reader:  bufio.NewScanner(conn),
		nextID:  1,
		started: true,
	}
	t.Cleanup(func() { _ = conn.Close() })
	return d
}

func TestFirDaemonAnalyzeAllRoundTrip(t *testing.T) {
	server := newFakeFirOracleServer(t)
	server.responses["analyzeAll"] = `{"id":{{ID}},"result":{"version":1,"kotlinVersion":"2.3.21","files":{"/x.kt":{"package":"x","declarations":[{"fqn":"x.Y","kind":"class"}]}},"dependencies":{}}}`
	d := server.connect(t)

	data, err := d.AnalyzeAll([]string{"/src"}, nil)
	if err != nil {
		t.Fatalf("AnalyzeAll: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil oracle.Data")
	}
	if data.Version != 1 || data.KotlinVersion != "2.3.21" {
		t.Errorf("envelope fields wrong: %+v", data)
	}
	if got := len(data.Files); got != 1 {
		t.Fatalf("expected 1 file, got %d", got)
	}
	f := data.Files["/x.kt"]
	if f == nil || f.Package != "x" {
		t.Fatalf("file payload wrong: %+v", f)
	}
	if len(f.Declarations) != 1 || f.Declarations[0].FQN != "x.Y" {
		t.Fatalf("declarations wrong: %+v", f.Declarations)
	}
}

func TestFirDaemonAnalyzeRouteShiftsCommandWhenFilesEmpty(t *testing.T) {
	// An empty file list should route to `analyzeAll` (not `analyze`),
	// matching krit-types' behavior. The fake server records the
	// command we sent so we can assert routing without relying on
	// response shape.
	server := newFakeFirOracleServer(t)
	server.responses["analyzeAll"] = `{"id":{{ID}},"result":{"version":1,"kotlinVersion":"","files":{},"dependencies":{}}}`
	d := server.connect(t)
	if _, err := d.Analyze(nil, []string{"/src"}, nil); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.requests) != 1 || server.requests[0].Command != "analyzeAll" {
		t.Fatalf("expected analyzeAll routing for empty files, got %+v", server.requests)
	}
}

func TestFirDaemonAnalyzeFilesShipsExplicitCommand(t *testing.T) {
	server := newFakeFirOracleServer(t)
	server.responses["analyze"] = `{"id":{{ID}},"result":{"version":1,"kotlinVersion":"","files":{},"dependencies":{}}}`
	d := server.connect(t)
	if _, err := d.Analyze([]string{"/src/x.kt"}, []string{"/src"}, nil); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.requests) != 1 || server.requests[0].Command != "analyze" {
		t.Fatalf("expected analyze command, got %+v", server.requests)
	}
	if len(server.requests[0].Files) != 1 || server.requests[0].Files[0].Path != "/src/x.kt" {
		t.Fatalf("files not forwarded: %+v", server.requests[0])
	}
}

func TestFirDaemonAnalyzeWithDepsParsesFlatEnvelope(t *testing.T) {
	server := newFakeFirOracleServer(t)
	server.responses["analyzeWithDeps"] = `{"id":{{ID}},"result":{"version":1,"kotlinVersion":"2.3.21","files":{},"dependencies":{}},"cacheDeps":{"version":1,"approximation":"symbol-resolved-sources","files":{"/leaf.kt":{"depPaths":["/base.kt"],"perFileDeps":{}}},"crashed":{}}}`
	d := server.connect(t)
	data, deps, err := d.AnalyzeWithDeps(nil, []string{"/src"}, nil)
	if err != nil {
		t.Fatalf("AnalyzeWithDeps: %v", err)
	}
	if data == nil || data.Version != 1 {
		t.Fatalf("data missing or wrong: %+v", data)
	}
	if deps == nil {
		t.Fatal("expected non-nil cacheDeps")
	}
	if deps.Approximation != "symbol-resolved-sources" {
		t.Errorf("cacheDeps.approximation = %q, want %q", deps.Approximation, "symbol-resolved-sources")
	}
	if entry := deps.Files["/leaf.kt"]; entry == nil || len(entry.DepPaths) != 1 || entry.DepPaths[0] != "/base.kt" {
		t.Errorf("cacheDeps entry wrong: %+v", entry)
	}
}

func TestFirDaemonAnalyzeSurfacesErrorEnvelope(t *testing.T) {
	server := newFakeFirOracleServer(t)
	server.responses["analyzeAll"] = `{"id":{{ID}},"error":"boom"}`
	d := server.connect(t)
	if _, err := d.AnalyzeAll(nil, nil); err == nil {
		t.Fatal("expected error from envelope")
	} else if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error message lost: %v", err)
	}
}

func TestFirDaemonAnalyzeChecksIdMatch(t *testing.T) {
	server := newFakeFirOracleServer(t)
	// Hard-code an id of 999 — the actual nextID starts at 1, so the
	// mismatch should surface as a clear error rather than silently
	// returning the wrong response.
	server.responses["analyzeAll"] = `{"id":999,"result":{"version":1,"kotlinVersion":"","files":{},"dependencies":{}}}`
	d := server.connect(t)
	_, err := d.AnalyzeAll(nil, nil)
	if err == nil {
		t.Fatal("expected ID-mismatch error")
	}
	if !strings.Contains(err.Error(), "ID mismatch") {
		t.Errorf("error should mention ID mismatch: %v", err)
	}
}
