//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package buildid

import "os"

func fileInode(os.FileInfo) uint64 { return 0 }
