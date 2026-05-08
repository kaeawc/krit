package scan

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFinalScanExitCode(t *testing.T) {
	cases := []struct {
		name     string
		findings int
		want     int
	}{
		{"zero findings exits 0", 0, 0},
		{"one finding exits 1", 1, 1},
		{"many findings exits 1", 9999, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := finalScanExit(&bytes.Buffer{}, tc.findings, 0, false)
			if got != tc.want {
				t.Fatalf("findings=%d -> exit %d; want %d", tc.findings, got, tc.want)
			}
		})
	}
}

func TestFinalScanExitQuietSuppressesSummary(t *testing.T) {
	var buf bytes.Buffer
	finalScanExit(&buf, 5, 250*time.Millisecond, true)
	if buf.Len() != 0 {
		t.Fatalf("quiet=true should suppress summary; got %q", buf.String())
	}
}

func TestFinalScanExitVerboseEmitsSummary(t *testing.T) {
	var buf bytes.Buffer
	finalScanExit(&buf, 5, 250*time.Millisecond, false)
	out := buf.String()
	if !strings.Contains(out, "info: Found 5 issue(s)") {
		t.Fatalf("missing finding count: %q", out)
	}
	if !strings.Contains(out, "250ms") {
		t.Fatalf("missing elapsed: %q", out)
	}
}

func TestFinalScanExitElapsedIsRoundedToMillis(t *testing.T) {
	var buf bytes.Buffer
	// Pick a value with sub-millisecond precision; the print format must
	// round it to the nearest millisecond.
	finalScanExit(&buf, 0, 1234567*time.Microsecond, false) // 1.234567s
	out := buf.String()
	// 1.234567s rounded to ms = 1.235s
	if !strings.Contains(out, "1.235s") {
		t.Fatalf("expected rounded duration 1.235s in %q", out)
	}
}
