package pipeline

import (
	"context"
	"testing"
)

func TestEarlyAndroidNilHandleIsNoop(t *testing.T) {
	var e *earlyAndroid
	if e.started() {
		t.Fatal("nil handle must not report started")
	}
	if _, ran, err := e.wait(); ran || err != nil {
		t.Fatalf("nil wait = ran %v err %v", ran, err)
	}
}

func TestEarlyAndroidWithoutProjectDoesNotRun(t *testing.T) {
	e := startEarlyAndroid(context.Background(), ProjectArgs{}, ProjectHostState{}, IndexResult{})
	if !e.started() {
		t.Fatal("started handle must report started")
	}
	if _, ran, err := e.wait(); ran || err != nil {
		t.Fatalf("empty project wait = ran %v err %v", ran, err)
	}
	// A second wait returns the same result.
	if _, ran, _ := e.wait(); ran {
		t.Fatal("second wait changed result")
	}
}
