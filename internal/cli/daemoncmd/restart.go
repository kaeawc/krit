package daemoncmd

import (
	"flag"
	"fmt"
	"os"
)

func runRestart(args []string) int {
	fs := flag.NewFlagSet("daemon restart", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := addRepoFlag(fs)
	socketFlag := fs.String("socket", "", "socket path (defaults to <repo>/.krit/daemon.sock)")
	binaryFlag := fs.String("binary", "", "krit-daemon binary path (defaults to one next to krit)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	stopArgs := []string{}
	startArgs := []string{}
	if *repoFlag != "" {
		stopArgs = append(stopArgs, "--repo", *repoFlag)
		startArgs = append(startArgs, "--repo", *repoFlag)
	}
	if *socketFlag != "" {
		startArgs = append(startArgs, "--socket", *socketFlag)
	}
	if *binaryFlag != "" {
		startArgs = append(startArgs, "--binary", *binaryFlag)
	}

	// stop reports 0 even when the daemon wasn't running, and
	// ExitForceKill when SIGKILL had to take over — neither should
	// stop us from spawning a fresh daemon.
	if code := runStop(stopArgs); code != 0 && code != ExitForceKill {
		fmt.Fprintln(os.Stderr, "krit daemon restart: stop failed")
		return code
	}
	return runStart(startArgs)
}
