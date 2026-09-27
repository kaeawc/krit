package mcp

import (
	"bufio"
	"log/slog"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
)

func TestFIRPreflightLogOncePerSession(t *testing.T) {
	t.Setenv("JAVA_HOME", t.TempDir())
	s := NewServer(bufio.NewReader(strings.NewReader("")), &strings.Builder{})
	cap := logger.NewCapture(slog.LevelInfo)
	s.SetLogger(cap)
	s.buildDispatcher()
	s.analyzeCode(analyzeArgs{Code: "fun one() = 1\n", Path: "/tmp/one.kt"})
	s.analyzeCode(analyzeArgs{Code: "fun two() = 2\n", Path: "/tmp/two.kt"})
	if got := len(cap.Records()); got != 1 {
		t.Fatalf("notice count = %d, want 1: %+v", got, cap.Records())
	}
}
