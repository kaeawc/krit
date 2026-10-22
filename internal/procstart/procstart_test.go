package procstart

import (
	"os"
	"runtime"
	"testing"
)

func TestOfIsStableForALiveProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no start time on windows")
	}
	first := Of(os.Getpid())
	if first == "" {
		t.Fatal("no start time for the running process")
	}
	if second := Of(os.Getpid()); second != first {
		t.Fatalf("start time changed between reads: %q vs %q", first, second)
	}
}

func TestOfUnknownProcessIsEmpty(t *testing.T) {
	if got := Of(0); got != "" {
		t.Fatalf("Of(0) = %q, want empty", got)
	}
	if got := Of(-1); got != "" {
		t.Fatalf("Of(-1) = %q, want empty", got)
	}
}

func TestStartTimeFromStatHandlesParensInCommand(t *testing.T) {
	// pid (comm) state ppid pgrp session tty tpgid flags minflt cminflt majflt
	// cmajflt utime stime cutime cstime priority nice threads itrealvalue
	// starttime ...
	stat := []byte("42 (a) b (c)) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 12345 20 21\n")
	if got := startTimeFromStat(stat); got != "12345" {
		t.Fatalf("startTimeFromStat = %q, want 12345", got)
	}
	if got := startTimeFromStat([]byte("42 (short) S 1")); got != "" {
		t.Fatalf("short stat = %q, want empty", got)
	}
	if got := startTimeFromStat([]byte("garbage")); got != "" {
		t.Fatalf("garbage stat = %q, want empty", got)
	}
}
