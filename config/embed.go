// Package config embeds krit's shipped configuration — the default rule
// config, the onboarding profiles, and the controversial-rule registry — so
// a released binary carries them without a source checkout.
package config

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
)

// DefaultConfigName is default-krit.yml's path within FS.
const DefaultConfigName = "default-krit.yml"

// FS holds default-krit.yml, profiles/*.yml, and onboarding/*.json, at the
// same relative paths they have under the repo's config/ directory.
//
//go:embed default-krit.yml profiles/*.yml onboarding/*.json
var FS embed.FS

// DefaultConfig returns the embedded default-krit.yml.
func DefaultConfig() []byte {
	data, err := FS.ReadFile(DefaultConfigName)
	if err != nil {
		panic("embedded " + DefaultConfigName + " missing: " + err.Error())
	}
	return data
}

// ContentHash is a hex SHA-256 over every embedded file's path and content,
// so a directory written by WriteTree can be keyed to exactly this content.
func ContentHash() string {
	h := sha256.New()
	_ = walkFiles(func(path string, data []byte) error {
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))
}

// WriteTree writes every embedded file under dir, mirroring the repo's
// config/ layout (dir/default-krit.yml, dir/profiles/<name>.yml, ...).
func WriteTree(dir string) error {
	return walkFiles(func(path string, data []byte) error {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return fsutil.WriteFileAtomic(target, data, 0o644)
	})
}

// walkFiles visits each embedded file in lexical path order.
func walkFiles(visit func(path string, data []byte) error) error {
	return fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := FS.ReadFile(path)
		if err != nil {
			return err
		}
		return visit(path, data)
	})
}
