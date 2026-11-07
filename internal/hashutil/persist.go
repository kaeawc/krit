package hashutil

import (
	"encoding/gob"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// racyWindow is how recent a file's modification time may be, relative to when a memo was saved,
// before its saved hash is not trusted. A file changed within this window of the save could have
// the same size and a timestamp the filesystem has not yet told apart; git's index treats such
// "racy" entries the same way.
const racyWindow = 2 * time.Second

// persistedMemoVersion changes when the on-disk layout does.
const persistedMemoVersion = 1

type persistedMemo struct {
	Version int
	// Hasher names the algorithm behind Hex; a memo saved under another one is ignored.
	Hasher  string
	SavedAt int64 // UnixNano
	Paths   []string
	Sizes   []int64
	MTimes  []int64
	Hex     []string
}

// LoadPersisted adds the hashes saved by SavePersisted to m, except entries too close to the save
// time to trust. A missing or unreadable file adds nothing. It returns how many entries it added.
func (m *Memo) LoadPersisted(path string) int {
	if m == nil {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	saved, ok := decodePersisted(f)
	if !ok || saved.Version != persistedMemoVersion || saved.Hasher != HasherName() ||
		len(saved.Paths) != len(saved.Hex) || len(saved.Paths) != len(saved.Sizes) || len(saved.Paths) != len(saved.MTimes) {
		return 0
	}
	trustBefore := saved.SavedAt - int64(racyWindow)
	added := 0
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range saved.Paths {
		if saved.MTimes[i] >= trustBefore || len(saved.Hex[i]) != 64 {
			continue
		}
		if _, exists := m.entries[p]; exists {
			continue
		}
		m.entries[p] = memoEntry{size: saved.Sizes[i], modNano: saved.MTimes[i], hex: saved.Hex[i], raw: rawOf(saved.Hex[i])}
		added++
	}
	return added
}

// decodePersisted reads a memo file, treating any decode failure or panic on corrupt input as a miss.
func decodePersisted(f *os.File) (saved persistedMemo, ok bool) {
	defer func() {
		if recover() != nil {
			saved, ok = persistedMemo{}, false
		}
	}()
	if gob.NewDecoder(f).Decode(&saved) != nil {
		return persistedMemo{}, false
	}
	return saved, true
}

// SavePersisted writes the memo for LoadPersisted, replacing the file atomically.
func (m *Memo) SavePersisted(path string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	snapshot := make(map[string]memoEntry, len(m.entries))
	for p, e := range m.entries {
		snapshot[p] = e
	}
	m.mu.Unlock()
	saved := persistedMemo{Version: persistedMemoVersion, Hasher: HasherName(), SavedAt: time.Now().UnixNano()}
	for p, e := range snapshot {
		// Entries for deleted files would otherwise stay in the memo forever.
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		saved.Paths = append(saved.Paths, p)
		saved.Sizes = append(saved.Sizes, e.size)
		saved.MTimes = append(saved.MTimes, e.modNano)
		saved.Hex = append(saved.Hex, e.hex)
	}
	return writePersisted(path, saved)
}

// writePersisted replaces path with the encoded memo, leaving no temp file behind on failure.
func writePersisted(path string, saved persistedMemo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.")
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(tmp).Encode(&saved); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
