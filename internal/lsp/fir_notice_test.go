package lsp

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFIRPreflightNoticeOncePerSession(t *testing.T) {
	t.Setenv("JAVA_HOME", t.TempDir())
	output := newSyncBuffer()
	s := NewServer(bufio.NewReader(strings.NewReader("")), output)
	s.loadConfigAndBuildDispatcher()
	s.analyzeAndPublish("file:///tmp/one.kt", []byte("fun one() = 1\n"))
	s.analyzeAndPublish("file:///tmp/two.kt", []byte("fun two() = 2\n"))

	// The notice is sent from a goroutine off the request path.
	notice := []byte("window/showMessage")
	if !output.waitUntil(10*time.Second, func(b []byte) bool { return bytes.Contains(b, notice) }) {
		t.Fatalf("no showMessage notice: %s", output.Bytes())
	}
	// Give a duplicate notice a chance to appear before counting.
	time.Sleep(100 * time.Millisecond)
	got := output.Bytes()
	if n := bytes.Count(got, notice); n != 1 {
		t.Fatalf("showMessage count = %d, want 1: %s", n, got)
	}
	if !bytes.Contains(got, []byte("JAVA_HOME")) {
		t.Fatalf("missing actionable notice: %s", got)
	}
}
