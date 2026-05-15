package sessdaemon

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAnalyzeMatchesInProcessRunProject is the contract called out in
// issue #201: spin up the daemon, send one analyze request, and assert
// the streamed findings are row-equal to an in-process pipeline analysis
// against the same paths and flags. Both paths skip OutputPhase to keep
// the comparison apples-to-apples; the daemon emits per-row directly
// off the dispatcher.
func TestAnalyzeMatchesInProcessRunProject(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, repo, "Foo.kt",
		"package demo\n\nclass Foo {\n    fun greet(): String { return \"hi\" }\n}\n")
	writeFile(t, repo, "Bar.kt",
		"package demo\n\nfun bar(unused: Int): Int { return 42 }\n")

	socket := startTestServer(t, repo)

	dRes, err := Analyze(socket, AnalyzeParams{Paths: []string{repo}})
	if err != nil {
		t.Fatalf("daemon analyze: %v", err)
	}

	pc, err := scanner.NewParseCacheWithCap(repo, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		t.Fatalf("direct ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	active := rules.ActiveRulesV2(nil, nil, false, false, false)
	directRes, err := pipeline.RunProjectAnalysis(context.Background(), pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:      cfg,
			Paths:       []string{repo},
			ActiveRules: active,
			Format:      "json",
			Version:     "daemon",
		},
		Host: pipeline.ProjectHostState{ParseCache: pc},
	})
	if err != nil {
		t.Fatalf("direct RunProjectAnalysis: %v", err)
	}

	directRows := flattenFindings(&directRes.CrossFileResult.Findings)
	daemonRows := append([]Finding(nil), dRes.Findings...)

	if got, want := dRes.Summary.FindingsCount, len(directRows); got != want {
		t.Errorf("FindingsCount mismatch: daemon=%d direct=%d", got, want)
	}
	if got, want := len(daemonRows), dRes.Summary.FindingsCount; got != want {
		t.Errorf("streamed finding count != summary: %d vs %d", got, want)
	}

	sortFindings(directRows)
	sortFindings(daemonRows)
	if len(directRows) != len(daemonRows) {
		t.Fatalf("row count mismatch: direct=%d daemon=%d", len(directRows), len(daemonRows))
	}
	for i := range directRows {
		if directRows[i] != daemonRows[i] {
			t.Errorf("row %d differs:\n direct=%+v\n daemon=%+v",
				i, directRows[i], daemonRows[i])
		}
	}
	if got, want := dRes.Summary.FilesScanned, directRes.FilesScanned; got != want {
		t.Errorf("FilesScanned mismatch: daemon=%d direct=%d", got, want)
	}
}

// TestHealthReturnsQuickly validates the health verb path and the
// idle-SLA target called out in the issue (5 ms steady state; the 50 ms
// cap here is CI-tolerant).
func TestHealthReturnsQuickly(t *testing.T) {
	repo := t.TempDir()
	socket := startTestServer(t, repo)

	start := time.Now()
	res, err := Health(socket)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	elapsed := time.Since(start)
	if !res.OK {
		t.Errorf("health.OK = false")
	}
	if res.PID != os.Getpid() {
		t.Errorf("health.PID = %d, want %d", res.PID, os.Getpid())
	}
	if elapsed > 50*time.Millisecond {
		t.Errorf("health round-trip took %v (cap 50ms)", elapsed)
	}
}

func TestShutdownClosesSession(t *testing.T) {
	repo := t.TempDir()
	srv, socket := buildTestServer(t, repo)

	if err := Shutdown(socket); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case <-srv.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop within 2s of shutdown")
	}
}

func TestUnknownMethod(t *testing.T) {
	repo := t.TempDir()
	socket := startTestServer(t, repo)

	conn, err := Dial(socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := writeRequest(conn, "bogus", nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := readResponse(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("expected method-not-found, got %+v", resp)
	}
}

// --- helpers ------------------------------------------------------------

func buildTestServer(t *testing.T, repo string) (*Server, string) {
	t.Helper()
	// macOS caps Unix socket paths at 104 bytes; t.TempDir paths under
	// /var/folders/.../TestVeryLongName/NNN/ can blow past that. Use a
	// short tempdir name instead.
	sockDir, err := os.MkdirTemp("", "kritd")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	socket := filepath.Join(sockDir, "d.sock")
	srv, err := NewServer(context.Background(), Options{RepoDir: repo, SocketPath: socket})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		srv.Stop()
		srv.Wait()
	})
	return srv, socket
}

func startTestServer(t *testing.T, repo string) string {
	t.Helper()
	_, socket := buildTestServer(t, repo)
	return socket
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func flattenFindings(cols *scanner.FindingColumns) []Finding {
	if cols == nil {
		return nil
	}
	rows := cols.Findings()
	out := make([]Finding, len(rows))
	for i, f := range rows {
		out[i] = wireFinding(f)
	}
	return out
}

func sortFindings(rows []Finding) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].File != rows[j].File {
			return rows[i].File < rows[j].File
		}
		if rows[i].Line != rows[j].Line {
			return rows[i].Line < rows[j].Line
		}
		if rows[i].Col != rows[j].Col {
			return rows[i].Col < rows[j].Col
		}
		if rows[i].Rule != rows[j].Rule {
			return rows[i].Rule < rows[j].Rule
		}
		return rows[i].Message < rows[j].Message
	})
}
