// Command krit-daemon is the long-lived per-repo analysis process. It is
// a thin shim around the in-tree krit-serve daemon (internal/cli/serve),
// which owns the resident WorkspaceState, parse cache, analysis cache,
// and oracle JVM and serves the analyze-project / analyze-buffer /
// analyze-buffers verbs the CLI's daemonclient speaks.
//
// Historical note: an earlier scaffold under internal/sessdaemon
// implemented a parallel JSON-RPC server, but its wire format
// (length-prefixed frames) was incompatible with the CLI's
// line-delimited daemon protocol — see issue #247. The shim retires
// that parallel implementation by routing every flag through to
// serve.Run.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/cli/serve"
	"github.com/kaeawc/krit/internal/daemon"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	repoFlag := flag.String("repo", "", "repository root the daemon serves (required)")
	socketFlag := flag.String("socket", "", "socket path (defaults to <repo>/.krit/daemon.sock)")
	verboseFlag := flag.Bool("verbose", false, "log lifecycle events to stderr")
	flag.BoolVar(verboseFlag, "v", false, "alias for --verbose")
	// --strict-verify reruns every analyze in-process from cold caches
	// and fails the response on divergence. Doubles per-request cost;
	// intended for alpha-period correctness hunts, not steady-state
	// production load. See issue #202.
	strictVerifyFlag := flag.Bool("strict-verify", false,
		"compare every analyze response against a fresh in-process baseline; fail on divergence (off by default)")
	idleTimeoutFlag := flag.Duration("idle-timeout", 0,
		"exit after this duration of no requests (e.g. 30m); 0 disables auto-shutdown")
	// --client-binary-hash is the SHA-256 hex of the krit binary the
	// spawning CLI used. When non-empty, the daemon advertises this
	// over the status verb so the CLI's binary-hash handshake compares
	// against its own (matching) hash. When empty, the daemon falls
	// back to hashing the sibling krit binary, then to hashing itself
	// (which always mismatches and disables the handshake).
	clientHashFlag := flag.String("client-binary-hash", "",
		"SHA-256 hex of the krit binary the CLI is running; advertised via the status verb")
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
		fmt.Fprintf(os.Stderr, "krit-daemon: resolve --repo: %v\n", err)
		os.Exit(1)
	}
	if info, err := os.Stat(repo); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "krit-daemon: --repo is not a directory: %s\n", repo)
		os.Exit(1)
	}

	// flock(2) single-instance enforcement (issue #208). serve.Run
	// itself doesn't hold the lock, so we acquire it here and release
	// on exit. The kernel releases the fd if the process is killed.
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
		fmt.Fprintf(os.Stderr, "krit-daemon: acquire lock: %v\n", err)
		os.Exit(1)
	}
	defer lock.Close() //nolint:errcheck // best effort on shutdown

	// Translate krit-daemon's flag surface to serve.Run's. The flag
	// names changed when we collapsed the two daemon implementations.
	serveArgs := []string{"--root", repo}
	if *socketFlag != "" {
		serveArgs = append(serveArgs, "--socket", *socketFlag)
	}
	if *idleTimeoutFlag > 0 {
		serveArgs = append(serveArgs, "--idle-timeout", idleTimeoutFlag.String())
	}
	if *strictVerifyFlag {
		serveArgs = append(serveArgs, "--strict-verify")
	}

	if *verboseFlag {
		fmt.Fprintf(os.Stderr, "krit-daemon: starting in-tree serve.Run with args %v\n", serveArgs)
	}

	// serve.Version is the version reported by the status verb. Mirror
	// the krit-daemon binary's version so clients can detect upgrades.
	serve.Version = version

	// The binary-hash handshake compares the CLI's krit binary hash to
	// what the daemon advertises. With krit and krit-daemon as separate
	// binaries the handshake would always mismatch if we hashed our own
	// executable. Resolution order:
	//   1. --client-binary-hash flag (the spawning CLI passed its hash)
	//   2. hash of the sibling "krit" binary next to krit-daemon
	//   3. empty (disables the handshake)
	switch {
	case *clientHashFlag != "":
		serve.BinaryHashOverride = *clientHashFlag
	default:
		serve.BinaryHashOverride = hashSiblingKritBinary()
	}

	os.Exit(serve.Run(serveArgs))
}

// hashSiblingKritBinary returns the SHA-256 hex digest of the krit
// binary that lives next to this krit-daemon binary, or "" when no
// sibling exists. The CLI hashes its own executable; matching that
// requires the daemon to hash the same file, not its own (different)
// binary. Returns "" silently on any I/O error — the handshake then
// treats the daemon as "no opinion" and accepts any CLI hash.
func hashSiblingKritBinary() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	sibling := filepath.Join(filepath.Dir(exe), "krit")
	f, err := os.Open(sibling)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
