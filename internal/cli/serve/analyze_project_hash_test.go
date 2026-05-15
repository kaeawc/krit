package serve

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_BinaryHashMismatchRejected pins the daemon
// handshake: a non-empty client hash that disagrees with the running
// binary's hash makes the verb refuse the request with the documented
// ErrBinaryHashMismatchPrefix prefix. The CLI side detects this prefix
// to fall back to in-process without prompting after a stale-binary
// `go install`.
func TestAnalyzeProject_BinaryHashMismatchRejected(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	var got daemon.AnalyzeProjectResult
	err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{ClientBinaryHash: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
		&got)
	if err == nil {
		t.Fatalf("expected mismatch rejection, got result=%+v", got)
	}
	if !strings.Contains(err.Error(), daemon.ErrBinaryHashMismatchPrefix) {
		t.Errorf("expected error containing %q; got %v", daemon.ErrBinaryHashMismatchPrefix, err)
	}
}

// TestAnalyzeProject_EmptyClientHashSkipsHandshake confirms that
// callers that haven't computed their binary hash (e.g. older tools)
// still get a successful response — the handshake is opt-in via a
// non-empty ClientBinaryHash.
func TestAnalyzeProject_EmptyClientHashSkipsHandshake(t *testing.T) {
	socket, state := startServerForTest(t)
	writeKotlinFile(t, state.root, "Foo.kt", "package demo\n\nclass Foo\n")

	var got daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(got.Findings) == 0 {
		t.Errorf("expected non-empty Findings on empty-hash call")
	}
}
