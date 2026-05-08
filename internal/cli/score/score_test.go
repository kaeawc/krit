package score

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "xml"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(stderr.String(), "unsupported format") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
