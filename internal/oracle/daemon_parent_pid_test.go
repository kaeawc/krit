package oracle

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestEphemeralDaemonArgsParentPID(t *testing.T) {
	t.Setenv("KRIT_EPHEMERAL_DAEMONS", "")
	t.Setenv("KRIT_DAEMON_PARENT_PID", "")
	if got := EphemeralDaemonArgs("--daemon"); strings.Join(got, " ") != "--daemon" {
		t.Fatalf("no switch: %v", got)
	}

	t.Setenv("KRIT_EPHEMERAL_DAEMONS", "1")
	want := "--daemon --parent-pid " + strconv.Itoa(os.Getpid())
	if got := EphemeralDaemonArgs("--daemon"); strings.Join(got, " ") != want {
		t.Fatalf("ephemeral: %v, want %s", got, want)
	}

	// The worker's PID wins: the daemon outlives each scan.
	t.Setenv("KRIT_DAEMON_PARENT_PID", "4242")
	if got := EphemeralDaemonArgs("--daemon"); strings.Join(got, " ") != "--daemon --parent-pid 4242" {
		t.Fatalf("parent pid: %v", got)
	}
	for _, bad := range []string{"0", "-3", "x"} {
		t.Setenv("KRIT_DAEMON_PARENT_PID", bad)
		if got := EphemeralDaemonArgs("--daemon"); strings.Join(got, " ") != want {
			t.Fatalf("KRIT_DAEMON_PARENT_PID=%s: %v, want the ephemeral fallback", bad, got)
		}
	}
}
