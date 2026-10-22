package daemoncmd

import "testing"

func TestStartAndRestartUnsupportedExitImmediately(t *testing.T) {
	orig := daemonSupported
	daemonSupported = func() bool { return false }
	t.Cleanup(func() { daemonSupported = orig })
	repo := t.TempDir()

	if code := runStart([]string{"--repo", repo}); code != 1 {
		t.Fatalf("runStart = %d, want 1", code)
	}
	if code := runRestart([]string{"--repo", repo}); code != 1 {
		t.Fatalf("runRestart = %d, want 1", code)
	}
}
