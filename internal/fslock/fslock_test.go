package fslock

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const helperEnv = "KRIT_FSLOCK_HELPER_PATH"

// TestMain doubles as the lock-holding helper process: when helperEnv is set
// the test binary takes the lock, reports it on stdout, and blocks until
// killed. It never reaches m.Run, so the suite is not re-executed.
func TestMain(m *testing.M) {
	if path := os.Getenv(helperEnv); path != "" {
		release, ok := Acquire(path, time.Second)
		if !ok {
			os.Exit(3)
		}
		_ = release
		os.Stdout.WriteString("locked\n")
		select {}
	}
	os.Exit(m.Run())
}

// Goroutines of one process exclude each other: every Acquire has its own
// descriptor, which flock(2) needs to tell them apart.
func TestGoroutinesInOneProcessExcludeEachOther(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.lock")
	var holders, maxHolders, acquired int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, ok := Acquire(path, 5*time.Second)
			if !ok {
				return
			}
			atomic.AddInt32(&acquired, 1)
			n := atomic.AddInt32(&holders, 1)
			for {
				m := atomic.LoadInt32(&maxHolders)
				if n <= m || atomic.CompareAndSwapInt32(&maxHolders, m, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(&holders, -1)
			release()
		}()
	}
	wg.Wait()
	if acquired != 16 || maxHolders != 1 {
		t.Fatalf("acquired=%d maxHolders=%d, want 16 and 1", acquired, maxHolders)
	}
}

func TestHeldLockTimesOutThenReleaseFreesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.lock")
	release, ok := Acquire(path, time.Second)
	if !ok {
		t.Fatal("first acquire failed")
	}
	if _, ok := Acquire(path, 50*time.Millisecond); ok {
		t.Fatal("second acquire succeeded while the lock was held")
	}
	release()
	again, ok := Acquire(path, time.Second)
	if !ok {
		t.Fatal("lock not acquirable after release")
	}
	again()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file must persist: %v", err)
	}
}

// The kernel drops a dead holder's lock, so no stale-lock recovery exists.
func TestLockHeldByKilledProcessIsReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "k.lock")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(exe)
	child.Env = append(os.Environ(), helperEnv+"="+path)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper did not lock: %q, %v", line, err)
	}
	if _, ok := Acquire(path, 50*time.Millisecond); ok {
		t.Fatal("acquired a lock held by a live child")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	killed = true
	release, ok := Acquire(path, 2*time.Second)
	if !ok {
		t.Fatal("lock still held after the holder was killed")
	}
	release()
}
