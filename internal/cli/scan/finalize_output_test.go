package scan

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type errCloser struct{ err error }

func (e errCloser) Close() error { return e.err }

func TestFinalizeOutputCloseSuccessPreservesExitCode(t *testing.T) {
	var stderr bytes.Buffer
	got := finalizeOutputClose(errCloser{}, &stderr, 0)
	if got != 0 {
		t.Errorf("clean close: want exit 0, got %d", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("clean close: want no stderr, got %q", stderr.String())
	}
}

func TestFinalizeOutputCloseFailureEscalatesToExitTwo(t *testing.T) {
	var stderr bytes.Buffer
	got := finalizeOutputClose(errCloser{err: errors.New("disk full")}, &stderr, 0)
	if got != 2 {
		t.Errorf("close error from clean run: want exit 2, got %d", got)
	}
	if !strings.Contains(stderr.String(), "disk full") {
		t.Errorf("close error must reach stderr, got %q", stderr.String())
	}
}

func TestFinalizeOutputCloseFailurePreservesNonzeroExit(t *testing.T) {
	var stderr bytes.Buffer
	got := finalizeOutputClose(errCloser{err: errors.New("late")}, &stderr, 1)
	if got != 1 {
		t.Errorf("close error after prior failure must preserve exit 1, got %d", got)
	}
	if !strings.Contains(stderr.String(), "late") {
		t.Errorf("close error must still reach stderr even when exit was already nonzero, got %q", stderr.String())
	}
}

func TestFinalizeOutputNilCloserIsNoop(t *testing.T) {
	var stderr bytes.Buffer
	got := finalizeOutputClose(nil, &stderr, 0)
	if got != 0 {
		t.Errorf("nil writer: want exit 0, got %d", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("nil writer: want no stderr, got %q", stderr.String())
	}
}
