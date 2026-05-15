// Command krit-daemon is the long-lived per-repo analysis process. One
// daemon owns one scan.Session and serves analyze/health/shutdown verbs
// over a Unix socket. See internal/sessdaemon for the wire protocol.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/sessdaemon"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	repoFlag := flag.String("repo", "", "repository root the daemon serves (required)")
	socketFlag := flag.String("socket", "", "socket path (defaults to <repo>/.krit/daemon.sock)")
	verboseFlag := flag.Bool("verbose", false, "log lifecycle events to stderr")
	flag.BoolVar(verboseFlag, "v", false, "alias for --verbose")
	strictVerifyFlag := flag.Bool("strict-verify", true,
		"run an in-process baseline alongside every analyze and fail on divergence (issue #202; on by default during alpha)")
	idleTimeoutFlag := flag.Duration("idle-timeout", 0,
		"exit after this duration of no requests (e.g. 30m); 0 disables auto-shutdown")
	flag.Parse()

	if *versionFlag {
		fmt.Println("krit-daemon", version)
		return
	}

	if *repoFlag == "" {
		fmt.Fprintln(os.Stderr, "krit-daemon: --repo is required")
		flag.Usage()
		os.Exit(2)
	}

	repo, err := filepath.Abs(*repoFlag)
	if err != nil {
		log.Fatalf("krit-daemon: resolve --repo: %v", err)
	}
	if info, err := os.Stat(repo); err != nil || !info.IsDir() {
		log.Fatalf("krit-daemon: --repo is not a directory: %s", repo)
	}

	lock, err := daemon.AcquireRepoLock(repo)
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyHeld) {
			if pid := daemon.ReadPIDFile(repo); pid > 0 {
				fmt.Fprintf(os.Stderr, "krit-daemon: already running, PID %d\n", pid)
			} else {
				fmt.Fprintln(os.Stderr, "krit-daemon: already running (PID unknown)")
			}
			os.Exit(2)
		}
		log.Fatalf("krit-daemon: acquire lock: %v", err)
	}
	defer lock.Close() //nolint:errcheck // best effort on shutdown

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := sessdaemon.NewServer(ctx, sessdaemon.Options{
		RepoDir:      repo,
		SocketPath:   *socketFlag,
		StrictVerify: *strictVerifyFlag,
		IdleTimeout:  *idleTimeoutFlag,
	})
	if err != nil {
		log.Fatalf("krit-daemon: %v", err)
	}

	if err := srv.Start(ctx); err != nil {
		log.Fatalf("krit-daemon: start: %v", err)
	}
	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "krit-daemon listening on %s\n", srv.SocketPath())
	}

	// Stop on SIGINT / SIGTERM. The shutdown verb path also drives
	// Stop, so signals just give an operator override.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		srv.Stop()
	}()

	srv.Wait()
}
