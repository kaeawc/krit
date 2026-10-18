package serve

import (
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
	// The daemon was started under a different TMPDIR; its pointer says where.
	elsewhere := filepath.Join(short, "other-env", "krit", "0123456789abcdef.sock")
	if err := daemon.WriteSocketPointer(daemon.PointerPath(root), elsewhere); err != nil {
		t.Fatal(err)
	}
	if got := stopTarget(root, ""); got != elsewhere {
		t.Fatalf("stopTarget = %q, want pointer socket %q", got, elsewhere)
	}
	if got := stopTarget(root, "/explicit.sock"); got != "/explicit.sock" {
		t.Fatalf("explicit --socket must win, got %q", got)
	}
	// Without a pointer it falls back to derivation.
	_ = os.Remove(daemon.PointerPath(root))
	if got := stopTarget(root, ""); got == elsewhere || got == "" {
		t.Fatalf("without pointer stopTarget = %q", got)
	}
}
