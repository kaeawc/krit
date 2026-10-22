package daemoncmd

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// readArgv is processArgv, replaceable in tests that need a victim process
// to present a daemon argv the OS would not let a stand-in shell carry.
var readArgv = processArgv

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
	argv, err := readArgv(pid)
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
//
// The effective root is computed with the same flag.FlagSet semantics the
// real commands use (last occurrence wins, parsing stops at "--" or the first
// positional argument, value flags consume the next argument). Any argument
// the real command would reject leaves the process unverified.
func argvIsDaemonFor(argv []string, repo string) bool {
	if len(argv) == 0 {
		return false
	}
	base := strings.TrimSuffix(filepath.Base(argv[0]), ".exe")
	var root string
	var ok bool
	switch {
	case base == "krit-daemon":
		root, ok = effectiveDaemonRepo(argv[1:])
	case strings.HasPrefix(base, "krit") && len(argv) > 1 && argv[1] == "serve":
		root, ok = effectiveServeRoot(argv[2:])
	default:
		return false
	}
	return ok && sameRoot(root, repo)
}

// effectiveDaemonRepo mirrors the flag set in cmd/krit-daemon/main.go.
func effectiveDaemonRepo(args []string) (string, bool) {
	fs := flag.NewFlagSet("krit-daemon", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	version := fs.Bool("version", false, "")
	repo := fs.String("repo", "", "")
	fs.String("socket", "", "")
	verbose := fs.Bool("verbose", false, "")
	fs.BoolVar(verbose, "v", false, "")
	fs.Bool("strict-verify", false, "")
	fs.Duration("idle-timeout", 0, "")
	fs.String("client-binary-hash", "", "")
	if err := fs.Parse(args); err != nil || *version || fs.NArg() > 0 {
		return "", false
	}
	return *repo, *repo != ""
}

// effectiveServeRoot mirrors the flag set in internal/cli/serve/serve.go.
func effectiveServeRoot(args []string) (string, bool) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "")
	fs.String("socket", "", "")
	stop := fs.Bool("stop", false, "")
	fs.Duration("idle-timeout", 30*time.Minute, "")
	fs.Int64("max-parse-bytes", 0, "")
	fs.Bool("no-watcher", false, "")
	fs.String("watch-backend", "auto", "")
	fs.Bool("strict-verify", false, "")
	if err := fs.Parse(args); err != nil || *stop || fs.NArg() > 0 {
		return "", false
	}
	return *root, true
}
