// Package daemoncmd implements the `krit daemon` lifecycle
// subcommands. Each operates on a PID file at
// <repoDir>/.krit/daemon.pid and the per-repo socket path published
// by internal/daemon (the line-delimited JSON-RPC server hosted by
// internal/cli/serve and the cmd/krit-daemon shim).
package daemoncmd

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Run dispatches one of the daemon lifecycle subcommands. Returns the
// process exit code: 0 on success, 1 on user-visible failure, 2 on
// usage error, 75 on a forced (SIGKILL) stop.
func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "start":
		return runStart(rest)
	case "stop":
		return runStop(rest)
	case "status":
		return runStatus(rest)
	case "restart":
		return runRestart(rest)
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "krit daemon: unknown subcommand %q\n", sub)
		usage(os.Stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: krit daemon <start|stop|status|restart> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  start    Spawn the krit-daemon process and wait until ready.")
	fmt.Fprintln(w, "  stop     Send SIGTERM, wait up to 5s, then SIGKILL.")
	fmt.Fprintln(w, "  status   Print PID, uptime, socket, last-flush. Add --json for tooling.")
	fmt.Fprintln(w, "  restart  Stop the daemon (force if necessary) then start a fresh one.")
}

func addRepoFlag(fs *flag.FlagSet) *string {
	return fs.String("repo", "", "repository root the daemon serves (defaults to cwd)")
}
