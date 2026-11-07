package scan

import (
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/hashutil"
)

// hashMemoPath is where the content-hash memo lives, next to the analysis cache file. KRIT_NO_HASH_PERSIST=1
// turns the persisted memo off.
func hashMemoPath(cacheFilePath string) string {
	if cacheFilePath == "" || os.Getenv("KRIT_NO_HASH_PERSIST") == "1" {
		return ""
	}
	return filepath.Join(filepath.Dir(cacheFilePath), "hash-memo.gob")
}

// loadPersistedHashes seeds the run's hash memo with the previous run's hashes, so unchanged files are
// not read again. Entries for files modified close to the save time are left out (see hashutil).
func loadPersistedHashes(cacheFilePath string) {
	if path := hashMemoPath(cacheFilePath); path != "" {
		hashutil.Default().LoadPersisted(path)
	}
}

// savePersistedHashes stores the run's hash memo for the next run.
func savePersistedHashes(cacheFilePath string) {
	if path := hashMemoPath(cacheFilePath); path != "" {
		_ = hashutil.Default().SavePersisted(path)
	}
}
