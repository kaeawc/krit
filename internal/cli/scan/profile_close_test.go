package scan

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

func TestWriteMemProfileToSurfacesCloseError(t *testing.T) {
	var errOut bytes.Buffer
	w := &closingBuf{closeErr: errors.New("nfs writeback")}
	writeMemProfileTo(w, "/tmp/mem.pprof", &errOut)
	if !strings.Contains(errOut.String(), "could not close memory profile") {
		t.Errorf("close error must reach errOut; got %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "nfs writeback") {
		t.Errorf("close error must include underlying cause; got %q", errOut.String())
	}
}

func TestWriteMemProfileToCleanCloseReportsNothing(t *testing.T) {
	var errOut bytes.Buffer
	w := &closingBuf{}
	writeMemProfileTo(w, "/tmp/mem.pprof", &errOut)
	if errOut.Len() != 0 {
		t.Errorf("clean run must not write to errOut; got %q", errOut.String())
	}
}
