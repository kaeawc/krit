package daemonclient

import (
	"os"
	"testing"
)

// spawnedTestChildEnv marks a copy of this test binary that
// EnsureRunning launched as a "daemon". Such a copy exits at once
// instead of rerunning the suite, so a regression in the self-exec
// guard fails the guard test rather than forking without bound.
const spawnedTestChildEnv = "KRIT_DAEMONCLIENT_SPAWNED_TEST_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(spawnedTestChildEnv) == "1" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}
