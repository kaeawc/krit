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

// heldRelease pins the helper process's lock; see TestMain.
var heldRelease func()

// TestMain doubles as the lock-holding helper process: when helperEnv is set
// the test binary takes the lock, reports it on stdout, and blocks until
// killed. It never reaches m.Run, so the suite is not re-executed.
func TestMain(m *testing.M) {
	if path := os.Getenv(helperEnv); path != "" {
		release, ok := Acquire(path, time.Second)
		if !ok {
			os.Exit(3)
		}
		// Precaution: release owns the *os.File, whose finalizer closes the
		// descriptor (dropping the flock) if it is ever garbage collected,
		// so keep it reachable for the life of the process.
		heldRelease = release
		os.Stdout.WriteString("locked\n")
		// Block on a timer, not "select {}": with no other goroutine alive
		// the runtime reports "all goroutines are asleep - deadlock!" and
		// exits, which silently dropped the lock the parent expects held.
		// (-race starts extra goroutines, which hid this.)
		time.Sleep(time.Hour)
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
	// The helper reports readiness only after taking the lock; wait for
	// that line with a timeout so a wedged helper cannot hang the suite.
	type ready struct {
		line string
		err  error
	}
	readyCh := make(chan ready, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		readyCh <- ready{line, err}
	}()
	select {
	case r := <-readyCh:
		if r.err != nil || r.line != "locked\n" {
			t.Fatalf("helper did not lock: %q, %v", r.line, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("helper did not report the lock within 10s")
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
