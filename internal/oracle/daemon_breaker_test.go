package oracle

import (
	"errors"
	"testing"
	"time"
)

// TestDaemonBreakerPolicy verifies the inlined breaker on Daemon: after
// breakerThreshold consecutive failures send admits no further calls
// until breakerCooldown elapses, after which a successful probe clears
// the failure count.
func TestDaemonBreakerPolicy(t *testing.T) {
	now := time.Unix(0, 0)
	prev := daemonNow
	daemonNow = func() time.Time { return now }
	t.Cleanup(func() { daemonNow = prev })

	d := &Daemon{}
	wantErr := errors.New("transient daemon error")

	for i := 1; i <= breakerThreshold; i++ {
		if !d.breakerAdmit() {
			t.Fatalf("attempt %d: breaker should admit while closed", i)
		}
		d.breakerRecord(wantErr)
	}

	if d.breakerAdmit() {
		t.Fatal("breaker should be open after threshold failures")
	}

	now = now.Add(breakerCooldown - time.Second)
	if d.breakerAdmit() {
		t.Fatal("breaker should still be open before cooldown elapses")
	}

	now = now.Add(2 * time.Second)
	if !d.breakerAdmit() {
		t.Fatal("breaker should admit a probe past cooldown")
	}
	d.breakerRecord(nil)

	if !d.breakerAdmit() {
		t.Fatal("breaker should be closed after a successful probe")
	}
}

// TestDaemonBreakerProbeFailureReopens verifies that a failed probe
// past cooldown re-arms the open state without needing another full
// threshold run.
func TestDaemonBreakerProbeFailureReopens(t *testing.T) {
	now := time.Unix(0, 0)
	prev := daemonNow
	daemonNow = func() time.Time { return now }
	t.Cleanup(func() { daemonNow = prev })

	d := &Daemon{}
	wantErr := errors.New("daemon still bad")

	for i := 0; i < breakerThreshold; i++ {
		d.breakerRecord(wantErr)
	}
	if d.breakerAdmit() {
		t.Fatal("expected breaker open after threshold failures")
	}

	now = now.Add(breakerCooldown + time.Second)
	if !d.breakerAdmit() {
		t.Fatal("expected breaker to admit probe past cooldown")
	}
	d.breakerRecord(wantErr)

	if d.breakerAdmit() {
		t.Fatal("failed probe should re-open the breaker")
	}
}
