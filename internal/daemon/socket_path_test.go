package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSocketPath_ShortRootUsesKritDir(t *testing.T) {
	root := "/tmp/proj"
	if got, want := DefaultSocketPath(root), filepath.Join(root, ".krit", "daemon.sock"); got != want {
		t.Fatalf("DefaultSocketPath(%q) = %q, want %q", root, got, want)
	}
}

// A project nested deep enough that <root>/.krit/daemon.sock exceeds
// sun_path used to make bind fail with EINVAL, so the spawned daemon exited
// and clients waited out their whole startup timeout.
func TestDefaultSocketPath_LongRootFallsBackAndBinds(t *testing.T) {
	root := filepath.Join(shortTempDir(t), strings.Repeat("deeply-nested-checkout-", 6), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	sock := DefaultSocketPath(root)
	if len(sock) > maxSocketPath {
		t.Fatalf("socket path is %d bytes, want <= %d: %s", len(sock), maxSocketPath, sock)
	}
	if sock != DefaultSocketPath(root) {
		t.Fatalf("fallback path is not stable for the same root")
	}
	if other := DefaultSocketPath(root + "2"); other == sock {
		t.Fatalf("distinct roots share fallback socket %s", sock)
	}
	srv := NewServer(sock)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start on fallback socket: %v", err)
	}
	t.Cleanup(srv.Stop)
	if !Available(sock) {
		t.Fatalf("daemon not reachable at %s", sock)
	}
}
