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

// RacyContentToken returns "" when info, stat'd just now, is old enough for
// its size and mtime to identify path's content, and otherwise the content
// hash (or "unreadable"). Stat-keyed cache keys append it, so a racy file is
// keyed by content while every other file keeps its existing key.
// Directories have no content to hash and always return "".
//
// A stat that was racy once in this process stays keyed by content until the
// stat changes. Otherwise a key computed before a long step (a FIR run) and
// recomputed after it to detect edits during the step would differ only
// because the file aged out of the window.
func RacyContentToken(path string, info os.FileInfo) string {
	if info.IsDir() {
		return ""
	}
	stamp := racyStamp{size: info.Size(), modNano: info.ModTime().UnixNano()}
	if StatTrusted(info.ModTime(), racyNow()) {
		if seen, ok := racySeen.Load(path); !ok || seen.(racyStamp) != stamp {
			return ""
		}
	} else {
		racySeen.Store(path, stamp)
	}
	hx, err := Default().HashFile(path, nil)
	if err != nil {
		return "unreadable"
	}
	return hx
}

// racyStamp is the stat RacyContentToken last keyed a path by content under.
type racyStamp struct{ size, modNano int64 }

var (
	// racySeen maps a path to the racy stat it was keyed under; at most one
	// entry per path, for the life of the process.
	racySeen sync.Map
	// racyNow is the clock RacyContentToken judges stats against; tests
	// advance it to age a file out of the window.
	racyNow = time.Now
)
