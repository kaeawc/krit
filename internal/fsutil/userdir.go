package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserKritDir returns the path to the per-user Krit cache directory,
// creating it with mode 0700 if it does not exist. With KRIT_HOME set it is
// $KRIT_HOME/user-cache, so relocating the per-user directory also moves
// these files. Otherwise the directory lives under os.UserCacheDir when
// available so it is private to the calling user; on platforms where
// UserCacheDir fails (e.g. unset HOME and XDG_CACHE_HOME on Unix) it falls
// back to a uid-tagged subdir of os.TempDir.
//
// Krit's JVM helper logs and one-shot cache files live here so a
// malicious user on a multi-user host cannot pre-place a symlink at a
// shared /tmp path and redirect Krit's writes — the 0700 directory
// keeps the path off-limits.
func UserKritDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv(HomeEnv)); home != "" {
		base := filepath.Join(absOrSelf(home), "user-cache")
		if err := os.MkdirAll(base, 0o700); err != nil {
			return "", fmt.Errorf("create krit user dir %s: %w", base, err)
		}
		return base, nil
	}
	base, err := os.UserCacheDir()
	shared := false
	if err != nil || base == "" {
		base = filepath.Join(os.TempDir(), fmt.Sprintf("krit-%d", os.Getuid()))
		shared = true // predictable name in a shared dir: validate ownership and parent
	} else {
		base = filepath.Join(base, "krit")
	}
	if shared {
		if err := EnsurePrivateDir(base, true); err != nil {
			return "", fmt.Errorf("create krit user dir %s: %w", base, err)
		}
		return base, nil
	}
	// The per-user cache dir is not plantable by other users, so a
	// symlinked ~/.cache/krit (caches on another disk) stays supported.
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create krit user dir %s: %w", base, err)
	}
	return base, nil
}

// CreateUserKritFile creates (or truncates) name under UserKritDir
// with mode 0600. Returns the open file plus its absolute path so
// callers can surface the location to the user. Convenience wrapper
// around UserKritDir + os.OpenFile.
func CreateUserKritFile(name string) (*os.File, string, error) {
	dir, err := UserKritDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, path, err
	}
	return f, path, nil
}

// firCacheDirEnv is krit-fir's override for its per-user cache, which holds
// binary snapshots.
const firCacheDirEnv = "KRIT_FIR_CACHE_DIR"

// JVMEnv is the environment for a krit-fir JVM, or nil to inherit krit's.
// With KRIT_HOME set, krit-fir's per-user cache moves to UserKritDir, since
// the JVM otherwise derives it from user.home or XDG_CACHE_HOME. An explicit
// KRIT_FIR_CACHE_DIR is left alone.
func JVMEnv() []string {
	if strings.TrimSpace(os.Getenv(HomeEnv)) == "" || os.Getenv(firCacheDirEnv) != "" {
		return nil
	}
	dir, err := UserKritDir()
	if err != nil {
		return nil
	}
	return append(os.Environ(), firCacheDirEnv+"="+dir)
}
