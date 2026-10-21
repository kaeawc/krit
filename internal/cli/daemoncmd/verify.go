package daemoncmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/daemon"
)

// verifyDaemonPID reports whether pid is this repo's krit daemon. The pid
// file lives inside the (possibly untrusted) repository, so it is never
// enough on its own to justify signalling a process. The daemon is asked
// first over its socket; if nothing answers (or an older daemon does not
// report its pid), the OS process table is consulted.
func verifyDaemonPID(repo string, pid int) (bool, string) {
	socket := daemon.LocateSocketPath(repo)
	if fileExists(socket) {
		var st daemon.StatusResult
		if err := daemon.Call(socket, daemon.VerbStatus, nil, &st); err == nil && st.PID != 0 {
			if st.PID != pid {
				return false, fmt.Sprintf("the daemon answering on the socket is pid %d", st.PID)
			}
			if !sameRoot(st.Root, repo) {
				return false, fmt.Sprintf("the daemon with that pid serves %s", st.Root)
			}
			return true, ""
		}
	}
	argv, err := processArgv(pid)
	if err != nil {
		return false, fmt.Sprintf("cannot inspect the process: %v", err)
	}
	if !argvIsDaemonFor(argv, repo) {
		return false, "its command line is not a krit daemon for this root"
	}
	return true, ""
}

func sameRoot(a, b string) bool {
	if a == "" || b == "" || !filepath.IsAbs(a) {
		return false
	}
	return daemon.NormalizeRoot(a) == daemon.NormalizeRoot(b)
}

// argvIsDaemonFor matches the argv krit uses to start a daemon:
// `krit-daemon --repo <root> ...` (daemon start) or
// `krit serve --root <root> ...` (auto-started / serve).
func argvIsDaemonFor(argv []string, repo string) bool {
	if len(argv) == 0 {
		return false
	}
	base := strings.TrimSuffix(filepath.Base(argv[0]), ".exe")
	var flagName string
	switch {
	case base == "krit-daemon":
		flagName = "--repo"
	case strings.HasPrefix(base, "krit") && len(argv) > 1 && argv[1] == "serve":
		flagName = "--root"
	default:
		return false
	}
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		if a == flagName && i+1 < len(argv) {
			return sameRoot(argv[i+1], repo)
		}
		if v, ok := strings.CutPrefix(a, flagName+"="); ok {
			return sameRoot(v, repo)
		}
	}
	return false
}
