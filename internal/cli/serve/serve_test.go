package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// startServerForTest spins up a serve.Server with the standard verbs
// registered. warm() is intentionally skipped: analyze-buffer doesn't
// need the module graph, and warm() would try to scan t.TempDir() as
// a project.
//
// macOS limits Unix-socket paths to 104 bytes; with long test names
// even t.TempDir() overruns. Place the socket under a short MkdirTemp
// rooted at /tmp so the path stays well under the cap.
func startServerForTest(t *testing.T) (string, *daemonState) {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-srv-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")

	state := newDaemonState(t.TempDir())
	srv := daemon.NewServer(socket)
	registerVerbs(srv, state)

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
	return socket, state
}

func TestAnalyzeBuffer_RoundTrip(t *testing.T) {
	socket, _ := startServerForTest(t)

	args := daemon.AnalyzeBufferArgs{
		Path:    "Foo.kt",
		Content: "fun main() {\n    println(\"hello\")\n}\n",
	}
	var got daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer, args, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Findings) == 0 {
		t.Errorf("expected findings JSON, got empty")
	}
	if got.CacheHit {
		t.Errorf("first call should be a cache miss")
	}
}

func TestAnalyzeBuffer_RepeatedCallHitsCache(t *testing.T) {
	socket, _ := startServerForTest(t)

	args := daemon.AnalyzeBufferArgs{
		Path:    "Foo.kt",
		Content: "fun main() {\n    println(\"hello\")\n}\n",
	}
	var first daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer, args, &first); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.CacheHit {
		t.Fatalf("first call should miss")
	}

	var second daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer, args, &second); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !second.CacheHit {
		t.Fatalf("second call with identical content should hit the cache")
	}
	if string(first.Findings) != string(second.Findings) {
		t.Errorf("hit findings should byte-equal miss findings\nmiss: %s\nhit:  %s",
			string(first.Findings), string(second.Findings))
	}
}

func TestAnalyzeBuffer_ContentChangeMisses(t *testing.T) {
	socket, _ := startServerForTest(t)

	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer,
		daemon.AnalyzeBufferArgs{Path: "Foo.kt", Content: "fun a() {}\n"},
		&daemon.AnalyzeBufferResult{}); err != nil {
		t.Fatalf("first: %v", err)
	}
	var second daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer,
		daemon.AnalyzeBufferArgs{Path: "Foo.kt", Content: "fun b() {}\n"},
		&second); err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.CacheHit {
		t.Fatalf("changed content should miss the cache")
	}
}

func TestAnalyzeBuffer_TrailingWhitespaceProducesFinding(t *testing.T) {
	socket, _ := startServerForTest(t)

	args := daemon.AnalyzeBufferArgs{
		// Trailing whitespace triggers a built-in rule, so we know
		// we get at least one finding back without depending on
		// rule registration order.
		Path:    "Bad.kt",
		Content: "fun main() {   \n}\n",
	}
	var got daemon.AnalyzeBufferResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer, args, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	// Findings is a FindingColumns JSON. Row count is the length of
	// any per-row slice (rules / line / etc.).
	var cols struct {
		Rules []string `json:"rules"`
		Line  []int    `json:"line"`
	}
	if err := json.Unmarshal(got.Findings, &cols); err != nil {
		t.Fatalf("decode findings: %v\n%s", err, string(got.Findings))
	}
	if len(cols.Line) == 0 && len(cols.Rules) == 0 {
		t.Errorf("expected at least one finding for trailing-whitespace input, got: %s",
			string(got.Findings))
	}
}

func TestAnalyzeBuffers_BatchOf3(t *testing.T) {
	socket, _ := startServerForTest(t)

	args := daemon.AnalyzeBuffersArgs{Buffers: []daemon.AnalyzeBufferArgs{
		{Path: "A.kt", Content: "fun a() {}\n"},
		{Path: "B.kt", Content: "fun b() {}\n"},
		{Path: "C.kt", Content: "fun c() {}\n"},
	}}
	var got daemon.AnalyzeBuffersResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers, args, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got.Results))
	}
	for i, r := range got.Results {
		if r.Error != "" {
			t.Errorf("result %d had error: %s", i, r.Error)
		}
		if len(r.Findings) == 0 {
			t.Errorf("result %d empty findings", i)
		}
		if r.CacheHit {
			t.Errorf("result %d should miss on first batch call, got hit", i)
		}
	}
}

func TestAnalyzeBuffers_RepeatedBatchHitsCacheOnSecondCall(t *testing.T) {
	socket, _ := startServerForTest(t)

	args := daemon.AnalyzeBuffersArgs{Buffers: []daemon.AnalyzeBufferArgs{
		{Path: "X.kt", Content: "fun x() {}\n"},
		{Path: "Y.kt", Content: "fun y() {}\n"},
	}}
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers, args, &daemon.AnalyzeBuffersResult{}); err != nil {
		t.Fatalf("first: %v", err)
	}
	var second daemon.AnalyzeBuffersResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers, args, &second); err != nil {
		t.Fatalf("second: %v", err)
	}
	for i, r := range second.Results {
		if !r.CacheHit {
			t.Errorf("result %d should hit on the second batch with identical buffers", i)
		}
	}
}

func TestAnalyzeBuffers_EmptyBatchReturnsEmptyResults(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.AnalyzeBuffersResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffers,
		daemon.AnalyzeBuffersArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(got.Results))
	}
}

func TestAnalyzeBuffer_EmptyContentDoesNotPanic(t *testing.T) {
	socket, _ := startServerForTest(t)

	if err := daemon.Call(socket, daemon.VerbAnalyzeBuffer,
		daemon.AnalyzeBufferArgs{Path: "Empty.kt", Content: ""},
		&daemon.AnalyzeBufferResult{}); err != nil {
		t.Fatalf("call: %v", err)
	}
}
