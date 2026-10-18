package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

func TestStopTarget_UsesPointerFile(t *testing.T) {
	short, err := os.MkdirTemp("/tmp", "krit-stop-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	t.Setenv("TMPDIR", short)
	t.Setenv("XDG_RUNTIME_DIR", "")
	root := filepath.Join(short, strings.Repeat("deeply-nested-checkout-", 6), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// The daemon was started under a different TMPDIR; its pointer says where
	// and it is alive there.
	elsewhere := filepath.Join(short, "other-env", "krit", daemon.FallbackSocketName(root))
	srv := daemon.NewServer(elsewhere)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	if err := daemon.WriteSocketPointer(daemon.PointerPath(root), elsewhere); err != nil {
		t.Fatal(err)
	}
	if got := stopTarget(root, ""); got != elsewhere {
		t.Fatalf("stopTarget = %q, want pointer socket %q", got, elsewhere)
	}
	if got := stopTarget(root, "/explicit.sock"); got != "/explicit.sock" {
		t.Fatalf("explicit --socket must win, got %q", got)
	}
	// With the pointer's daemon gone, stop falls back to the derived path.
	srv.Stop()
	if got := stopTarget(root, ""); got == elsewhere || got == "" {
		t.Fatalf("with a dead pointer stopTarget = %q", got)
	}
}

// A second `krit serve` for a repo that already has a daemon (whatever
// socket path its environment would derive) must refuse to start instead of
// unlinking the first one's socket.
func TestRun_RefusesWhenRepoLockHeld(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-lock-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	lock, err := daemon.AcquireRepoLock(daemon.NormalizeRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })

	// Different socket per attempt, as with differing TMPDIR/XDG env.
	for _, sock := range []string{"", filepath.Join(root, "other.sock")} {
		args := []string{"--root", root}
		if sock != "" {
			args = append(args, "--socket", sock)
		}
		if code := Run(args); code != daemon.ExitAlreadyRunning {
			t.Fatalf("Run(%v) = %d, want %d (already running)", args, code, daemon.ExitAlreadyRunning)
		}
		if sock != "" {
			if _, err := os.Stat(sock); err == nil {
				t.Fatalf("second serve bound %s despite the lock", sock)
			}
		}
	}
}
