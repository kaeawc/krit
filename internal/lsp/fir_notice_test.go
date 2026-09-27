package lsp

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestFIRPreflightNoticeOncePerSession(t *testing.T) {
	t.Setenv("JAVA_HOME", t.TempDir())
	var output bytes.Buffer
	s := NewServer(bufio.NewReader(strings.NewReader("")), &output)
	s.loadConfigAndBuildDispatcher()
	s.analyzeAndPublish("file:///tmp/one.kt", []byte("fun one() = 1\n"))
	s.analyzeAndPublish("file:///tmp/two.kt", []byte("fun two() = 2\n"))
	if got := strings.Count(output.String(), "window/showMessage"); got != 1 {
		t.Fatalf("showMessage count = %d, want 1: %s", got, output.String())
	}
	if !strings.Contains(output.String(), "JAVA_HOME") {
		t.Fatalf("missing actionable notice: %s", output.String())
	}
}
