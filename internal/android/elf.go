package android

import (
	"bytes"
	"io"
	"os"
)

// elfMagic is the first four bytes of an ELF object file.
var elfMagic = []byte{0x7f, 'E', 'L', 'F'}

// IsELFFile reports whether path, after following symlinks, is a regular file that starts with the ELF magic number.
// UnsafeNativeCodeLocation decides with it, and the res-dir cache key reads assets through it too, so the two agree.
func IsELFFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(elfMagic))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return bytes.Equal(head, elfMagic)
}
