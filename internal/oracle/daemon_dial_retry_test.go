package oracle

import (
	"testing"
	"time"
)

// TestDialDaemonAddrRetries verifies the inlined backoff loop in
// dialDaemonAddr: against a closed port it tries dialDaemonAttempts
// times and sleeps dialDaemonAttempts-1 times with exponential delays.
func TestDialDaemonAddrRetries(t *testing.T) {
	var delays []time.Duration
	prev := dialDaemonSleep
	dialDaemonSleep = func(d time.Duration) { delays = append(delays, d) }
	t.Cleanup(func() { dialDaemonSleep = prev })

	// Port 1 (tcpmux) is reserved and never listening, so DialContext
	// fails fast on every attempt.
	if _, err := dialDaemonAddr("127.0.0.1:1", 50*time.Millisecond); err == nil {
		t.Fatal("expected error dialing closed port")
	}

	if got, want := len(delays), dialDaemonAttempts-1; got != want {
		t.Fatalf("recorded %d sleeps, want %d", got, want)
	}
	for i, d := range delays {
		want := dialDaemonBaseDelay << i
		if d != want {
			t.Errorf("delay[%d] = %v, want %v", i, d, want)
		}
	}
}
