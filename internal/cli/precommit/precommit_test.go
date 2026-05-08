package precommit

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// gitInit prepares a fresh git repo at root with a default identity.
func gitInit(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "init.defaultBranch=main", "config", "user.email", "test@example.com"},
		{"-c", "init.defaultBranch=main", "config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitAdd stages the given paths.
func gitAdd(t *testing.T, root string, paths ...string) {
	t.Helper()
	args := append([]string{"-C", root, "add", "--"}, paths...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
}

func TestStagedKotlinFiles_FiltersAndAbsolutifies(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-pc-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	gitInit(t, root)

	if err := os.WriteFile(filepath.Join(root, "Foo.kt"), []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Bar.java"), []byte("class Bar {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "Baz.kts"), []byte("println(\"hi\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, root, "Foo.kt", "Bar.java", "sub/Baz.kts")

	staged, err := stagedKotlinFiles(root)
	if err != nil {
		t.Fatalf("stagedKotlinFiles: %v", err)
	}
	expected := map[string]bool{
		filepath.Join(root, "Foo.kt"):         true,
		filepath.Join(root, "sub", "Baz.kts"): true,
	}
	if len(staged) != len(expected) {
		t.Fatalf("got %d staged, want %d: %v", len(staged), len(expected), staged)
	}
	for _, p := range staged {
		if !expected[p] {
			t.Errorf("unexpected staged path: %q", p)
		}
	}
}

func TestStagedKotlinFiles_EmptyWhenNothingStaged(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-pc-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	gitInit(t, root)

	staged, err := stagedKotlinFiles(root)
	if err != nil {
		t.Fatalf("stagedKotlinFiles: %v", err)
	}
	if len(staged) != 0 {
		t.Errorf("expected no staged files, got %v", staged)
	}
}

// buildEchoFindingColumns returns a single-row FindingColumns that
// round-trips through scanner.FindingColumns' JSON codec — easier
// than hand-rolling matching slice lengths.
func buildEchoFindingColumns() scanner.FindingColumns {
	c := scanner.NewFindingCollector(1)
	c.Append(scanner.Finding{
		File: "echo.kt", Line: 1, Col: 1,
		RuleSet: "test", Rule: "EchoRule",
		Severity: "warning", Message: "echoed",
	})
	return *c.Columns()
}

// startEchoDaemon stands up a tiny daemon at root/.krit/daemon.sock so
// the analyzer's Discover path can find it. The daemon's
// analyze-buffer handler always returns a single canned finding so we
// can assert the dispatch happened.
func startEchoDaemon(t *testing.T, root string) (*daemon.Server, string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".krit"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	socketDir, err := os.MkdirTemp("/tmp", "krit-pc-srv-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	expected := daemon.DefaultSocketPath(root)
	if err := os.Symlink(socket, expected); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	echoColumns := buildEchoFindingColumns()
	echoBody, err := json.Marshal(echoColumns)
	if err != nil {
		t.Fatalf("marshal echo cols: %v", err)
	}
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbAnalyzeBuffer, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.AnalyzeBufferResult{Findings: json.RawMessage(echoBody), CacheHit: false}, nil
	})
	srv.Register(daemon.VerbAnalyzeBuffers, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AnalyzeBuffersArgs
		_ = json.Unmarshal(raw, &args)
		results := make([]daemon.AnalyzeBufferEntry, len(args.Buffers))
		for i := range args.Buffers {
			results[i] = daemon.AnalyzeBufferEntry{Findings: json.RawMessage(echoBody)}
		}
		return daemon.AnalyzeBuffersResult{Results: results}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return srv, socket
}

func TestAnalyzer_UsesDaemonWhenAvailable(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-pc-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	startEchoDaemon(t, root)

	a := newAnalyzer(root, false /* noDaemon */, false /* autoSpawn */)
	defer a.Close()

	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := a.Analyze([]string{path})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if a.client == nil {
		t.Error("expected analyzer to retain the daemon client when one is reachable")
	}
	if !anyFindings(results) {
		t.Error("expected daemon's echoed finding to surface")
	}
}

func TestAnalyzer_FallsBackToInProcessWithoutDaemon(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-pc-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	a := newAnalyzer(root, false, false)
	defer a.Close()

	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := a.Analyze([]string{path})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if a.client != nil {
		t.Error("expected client to remain nil when no daemon is running")
	}
	if a.fallback == nil {
		t.Error("expected in-process fallback to be initialised")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestAnalyzer_NoDaemonFlagSkipsClient(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-pc-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	startEchoDaemon(t, root)

	a := newAnalyzer(root, true /* noDaemon */, false)
	defer a.Close()

	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Analyze([]string{path}); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if a.client != nil {
		t.Errorf("--no-daemon should skip client, got %v", a.client)
	}
}

func TestAnalyzer_AnyFindings(t *testing.T) {
	if anyFindings(nil) {
		t.Error("anyFindings(nil) should be false")
	}
	if anyFindings([]scanner.FindingColumns{{}}) {
		t.Error("anyFindings on a zero-row column set should be false")
	}
	withRow := scanner.FindingColumns{Files: []string{"x"}, Line: []uint32{1}}
	if !anyFindings([]scanner.FindingColumns{withRow}) {
		t.Error("anyFindings should be true when at least one column carries a row")
	}
}
