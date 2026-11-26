package hashutil

import (
	"os"
	"sync"
	"time"
)

// RacyWindow is how much older than the moment a file was stat'd its
// modification time must be before (size, mtime) can stand in for its
// content. Filesystems stamp mtimes from a coarse clock (a 4 ms kernel tick
// on ext4, whole seconds on HFS+ and some network filesystems, 2 s on FAT),
// so a same-size rewrite within one tick of the stat keeps the old mtime.
// Git calls such index entries "racily clean"; cmd/go's module index uses
// the same 2 s cutoff for the same reason.
const RacyWindow = 2 * time.Second

// StatTrusted reports whether a stat taken at at with modification time
// mtime proves the content: any later write would carry a different mtime.
// An mtime within RacyWindow of at, or in the future, is not proof.
func StatTrusted(mtime, at time.Time) bool {
	return mtime.Before(at.Add(-RacyWindow))
}

// StatTrustedNow is StatTrusted for a stat taken just now, judged against
// the clock of the filesystem holding the file when CalibrateClock measured
// it. Git compares against the index file's mtime for the same reason: a
// network or VM mount whose clock is skewed from the local one stamps
// mtimes the local clock cannot judge. On a calibrated filesystem an mtime
// more than RacyWindow in that clock's future (an archive or a cache
// restored from a machine whose clock was ahead) is trusted as well: a
// write made now would be stamped with the present, never with that mtime.
func StatTrustedNow(info os.FileInfo) bool {
	now := time.Now()
	if offset, ok := calibratedOffset(info); ok {
		now = now.Add(offset)
		if info.ModTime().After(now.Add(RacyWindow)) {
			return true
		}
	}
	return StatTrusted(info.ModTime(), now)
}

var fsClock struct {
	mu     sync.RWMutex
	ok     bool
	dev    uint64
	offset time.Duration
}

// CalibrateClock measures how far the clock of the filesystem holding dir
// is from the local clock, by stat-ing a file it writes there, so
// StatTrustedNow judges files on that filesystem against its own clock.
// Files on other filesystems keep the local clock. A missing or read-only
// dir leaves the previous calibration, or none; dir is never created.
func CalibrateClock(dir string) {
	if dir == "" {
		return
	}
	f, err := os.CreateTemp(dir, ".krit-clock-*")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	before := time.Now()
	_, werr := f.Write([]byte{0})
	cerr := f.Close()
	info, serr := os.Stat(name)
	if werr != nil || cerr != nil || serr != nil {
		return
	}
	dev, ok := deviceOf(info)
	if !ok {
		return
	}
	offset := info.ModTime().Sub(before)
	if offset > -RacyWindow/2 && offset < RacyWindow/2 {
		// Within a tick or two of the local clock: the filesystem shares it.
		offset = 0
	}
	fsClock.mu.Lock()
	fsClock.ok, fsClock.dev, fsClock.offset = true, dev, offset
	fsClock.mu.Unlock()
}

// calibratedOffset returns the measured clock offset of info's filesystem.
func calibratedOffset(info os.FileInfo) (time.Duration, bool) {
	dev, ok := deviceOf(info)
	if !ok {
		return 0, false
	}
	fsClock.mu.RLock()
	defer fsClock.mu.RUnlock()
	if !fsClock.ok || fsClock.dev != dev {
		return 0, false
	}
	return fsClock.offset, true
}

// resetClockCalibration forgets CalibrateClock's measurement (tests).
func resetClockCalibration() {
	fsClock.mu.Lock()
	fsClock.ok, fsClock.dev, fsClock.offset = false, 0, 0
	fsClock.mu.Unlock()
}
