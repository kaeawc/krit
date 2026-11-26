//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package hashutil

import "os"

func deviceOf(os.FileInfo) (uint64, bool) { return 0, false }
