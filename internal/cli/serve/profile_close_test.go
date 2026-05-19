package serve

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type closingBuf struct {
	bytes.Buffer
	closeErr error
}

func (c *closingBuf) Close() error { return c.closeErr }

func TestWriteDaemonMemProfileToReportsCloseAsWarning(t *testing.T) {
	w := &closingBuf{closeErr: errors.New("nfs writeback")}
	warnings := writeDaemonMemProfileTo(w, "/tmp/mem.pprof")
	if len(warnings) == 0 {
		t.Fatal("close error must surface as a warning entry")
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "daemon mem profile close") || !strings.Contains(joined, "nfs writeback") {
		t.Errorf("close warning must name the phase and underlying cause; got %v", warnings)
	}
}

func TestWriteDaemonMemProfileToCleanRunHasNoWarnings(t *testing.T) {
	w := &closingBuf{}
	warnings := writeDaemonMemProfileTo(w, "/tmp/mem.pprof")
	if len(warnings) != 0 {
		t.Errorf("clean run must produce no warnings; got %v", warnings)
	}
}
