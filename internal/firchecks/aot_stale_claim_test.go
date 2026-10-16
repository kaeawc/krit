package firchecks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Two claimants classify the same claim stale. The first recovers it and
// creates its own live claim before the second executes its already-decided
// removal. The second must not delete the live claim, so only one holds it.
func TestStaleClaimRemovalNeverDeletesLiveClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", t.TempDir())
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(firClaimPath("interleave"), []byte(strconv.Itoa(dead.Process.Pid)+"\n"), 0o644))

	oldWait, oldHook := firClaimWait, staleClaimHook
	firClaimWait = 300 * time.Millisecond
	t.Cleanup(func() { firClaimWait, staleClaimHook = oldWait, oldHook })

	var firstRelease func()
	var firstOK bool
	inHook := false
	staleClaimHook = func() {
		if inHook {
			return
		}
		inHook = true
		// The other claimant runs to completion here: it recovers the stale
		// claim and now holds a live one.
		firstRelease, firstOK = claimFirRegistry("interleave")
	}
	release, ok := claimFirRegistry("interleave")
	if ok {
		release()
		t.Fatal("second claimant also obtained the claim")
	}
	if !firstOK {
		t.Fatal("first claimant did not obtain the recovered claim")
	}
	data, err := os.ReadFile(firClaimPath("interleave"))
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("live claim was deleted or replaced: %q, %v", data, err)
	}
	if leftovers, _ := filepath.Glob(firClaimPath("interleave") + ".stale.*"); len(leftovers) != 0 {
		t.Fatalf("tombstones left behind: %v", leftovers)
	}
	firstRelease()
	if _, err := os.Stat(firClaimPath("interleave")); !os.IsNotExist(err) {
		t.Fatalf("claim not released: %v", err)
	}
}
