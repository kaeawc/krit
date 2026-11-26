package hashutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stampedFile writes a file whose mtime is at, as a filesystem whose clock
// reads at would stamp it.
func stampedFile(t *testing.T, at time.Time) (string, os.FileInfo) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "A.kt")
	if err := os.WriteFile(p, []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, info
}

// calibrate pretends CalibrateClock measured offset for the filesystem
// holding dir.
func calibrate(t *testing.T, dir string, offset time.Duration) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	dev, ok := deviceOf(info)
	if !ok {
		t.Skip("no device IDs on this platform")
	}
	fsClock.mu.Lock()
	fsClock.ok, fsClock.dev, fsClock.offset = true, dev, offset
	fsClock.mu.Unlock()
	t.Cleanup(resetClockCalibration)
}

func TestCalibrateClockMeasuresALocalFilesystemAsUnskewed(t *testing.T) {
	t.Cleanup(resetClockCalibration)
	dir := t.TempDir()
	CalibrateClock(dir)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	offset, ok := calibratedOffset(info)
	if _, hasDev := deviceOf(info); hasDev && (!ok || offset != 0) {
		t.Fatalf("local filesystem calibrated to offset %v (ok=%v), want 0", offset, ok)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".krit-clock-*")); len(leftovers) != 0 {
		t.Fatalf("calibration left files behind: %v", leftovers)
	}
	CalibrateClock(filepath.Join(dir, "missing")) // never created, never fatal
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatal("CalibrateClock created its directory")
	}
}

// A server whose clock is 10 s behind stamps a fresh write 10 s in the
// local past; only its own clock shows the stat is racy.
func TestStatTrustedNowUsesTheCalibratedClockOfASkewedServer(t *testing.T) {
	skew := -10 * time.Second
	p, info := stampedFile(t, time.Now().Add(skew))
	if !StatTrusted(info.ModTime(), time.Now()) {
		t.Fatal("precondition: the local clock should be fooled")
	}
	calibrate(t, filepath.Dir(p), skew)
	if StatTrustedNow(info) {
		t.Fatal("a fresh write on a server 10 s behind was trusted")
	}
}

func TestStatTrustedNowTrustsFutureMtimesOnlyOnACalibratedFilesystem(t *testing.T) {
	p, info := stampedFile(t, time.Now().Add(time.Hour))
	if StatTrustedNow(info) {
		t.Fatal("a future mtime on an uncalibrated filesystem must not be trusted")
	}
	calibrate(t, filepath.Dir(p), 0)
	if !StatTrustedNow(info) {
		t.Fatal("a future mtime on a calibrated filesystem cannot be reproduced by a write now")
	}
	// The memo can then serve it instead of rehashing on every call.
	m := NewMemo()
	for i := 0; i < 3; i++ {
		if _, err := m.HashFile(p, nil); err != nil {
			t.Fatal(err)
		}
	}
	if hits, misses := m.Stats(); hits != 2 || misses != 1 {
		t.Fatalf("stats = (%d,%d); want (2,1)", hits, misses)
	}
}
