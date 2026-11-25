// Package srcjar turns generated-source jars (.srcjar) into source roots.
//
// Bazel hands generated code (KSP, kapt, Wire/protobuf, SQLDelight, view
// binding) to compilers as srcjars: zip archives of .kt and .java files. The
// Kotlin compiler only reads sources from the file system, so, like
// rules_kotlin's builder, krit unpacks each srcjar once and then treats the
// directory as a generated source root: compiled as FIR and oracle context,
// never reported on.
//
// The directory is named after the SHA-256 of the archive's bytes under
// $KRIT_HOME/srcjars (default ~/.krit/srcjars), so an unchanged srcjar maps
// to the same root on every run (keeping FIR caches warm) and a changed one
// to a new root. Hermetic runners such as Bazel actions set KRIT_HOME to a
// directory they own.
package srcjar

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
)

// Extension is the file extension of a generated-source jar.
const Extension = ".srcjar"

// maxEntryBytes caps one extracted source file; generated sources are far
// smaller, and the cap stops a malformed archive from filling the disk.
const maxEntryBytes = 64 << 20

// Root is the directory srcjars are unpacked under: $KRIT_HOME/srcjars, or
// ~/.krit/srcjars.
func Root() (string, error) {
	dir, err := fsutil.UserStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "srcjars"), nil
}

var (
	memoMu sync.Mutex
	// memo maps a srcjar's path, size and modification time to its source
	// root, so the several model loads of one scan hash each archive once.
	memo = map[string]string{}
)

// Extract unpacks the .kt and .java entries of the srcjar at jarPath into a
// content-addressed directory under root and returns that directory. A
// directory that already exists for the same content is reused as is.
func Extract(jarPath, root string) (string, error) {
	info, err := os.Stat(jarPath)
	if err != nil {
		return "", fmt.Errorf("srcjar %s: %w", jarPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("srcjar %s: is a directory", jarPath)
	}
	abs, err := filepath.Abs(jarPath)
	if err != nil {
		abs = jarPath
	}
	memoKey := fmt.Sprintf("%s\x00%s\x00%d\x00%d", root, abs, info.Size(), info.ModTime().UnixNano())
	memoMu.Lock()
	dir, ok := memo[memoKey]
	memoMu.Unlock()
	if ok {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	sum, err := fileSHA256(jarPath)
	if err != nil {
		return "", fmt.Errorf("srcjar %s: %w", jarPath, err)
	}
	dir = filepath.Join(root, sum)
	if _, err := os.Stat(dir); err != nil {
		if err := unpack(jarPath, root, dir); err != nil {
			return "", fmt.Errorf("srcjar %s: %w", jarPath, err)
		}
	}
	memoMu.Lock()
	memo[memoKey] = dir
	memoMu.Unlock()
	return dir, nil
}

// ExtractAll extracts every srcjar under Root and returns the source roots in
// input order, without duplicates. A srcjar that cannot be read is reported
// in warnings and skipped: its types then resolve as unknown, as they do
// without the srcjar.
func ExtractAll(jars []string) (dirs []string, warnings []string) {
	if len(jars) == 0 {
		return nil, nil
	}
	root, err := Root()
	if err != nil {
		return nil, []string{fmt.Sprintf("generated srcjars: no krit home directory (%v); set KRIT_HOME", err)}
	}
	seen := map[string]bool{}
	for _, jar := range jars {
		if strings.TrimSpace(jar) == "" {
			continue
		}
		dir, err := Extract(jar, root)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	return dirs, warnings
}

func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// unpack writes the archive's sources into a fresh sibling of dir and renames
// it into place, so a concurrent or interrupted run never sees a partial root.
func unpack(jarPath, root, dir string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	tmp, err := os.MkdirTemp(root, filepath.Base(dir)+".tmp-")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(tmp)
		}
	}()
	for _, entry := range zr.File {
		rel, ok := sourceEntryPath(entry)
		if !ok {
			continue
		}
		if err := writeEntry(entry, filepath.Join(tmp, filepath.FromSlash(rel))); err != nil {
			return fmt.Errorf("%s: %w", entry.Name, err)
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another run published the same content first.
		if _, statErr := os.Stat(dir); statErr == nil {
			return nil
		}
		return err
	}
	committed = true
	return nil
}

// sourceEntryPath returns the slash-separated relative path an entry is
// extracted to. Only regular .kt and .java files are extracted; entries whose
// name is absolute or climbs out of the root are skipped.
func sourceEntryPath(entry *zip.File) (string, bool) {
	if !entry.Mode().IsRegular() {
		return "", false
	}
	name := strings.ReplaceAll(entry.Name, `\`, "/")
	if ext := path.Ext(name); ext != ".kt" && ext != ".java" {
		return "", false
	}
	if strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':') {
		return "", false
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func writeEntry(entry *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := entry.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// A duplicate entry name: the first one wins, as in javac.
			return nil
		}
		return err
	}
	n, copyErr := io.Copy(out, io.LimitReader(rc, maxEntryBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if n > maxEntryBytes {
		return fmt.Errorf("larger than %d bytes", maxEntryBytes)
	}
	return closeErr
}
