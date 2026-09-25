package oracle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/hashutil"
)

// JarIdentity cheaply identifies the jar artifact currently at jarPath.
func JarIdentity(jarPath string) string {
	info, err := os.Stat(jarPath)
	if err != nil {
		return "missing"
	}
	return hashutil.HashHex([]byte(fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())))[:8]
}

// JarPathTag distinguishes jars with the same filename in different installs.
func JarPathTag(jarPath string) string {
	path, err := filepath.Abs(jarPath)
	if err != nil {
		path = jarPath
	}
	return hashutil.HashHex([]byte(filepath.Clean(path)))[:8]
}
