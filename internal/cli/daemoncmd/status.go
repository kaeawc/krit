package daemoncmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kaeawc/krit/internal/sessdaemon"
)

// DaemonStatus is the JSON payload `daemon status --json` prints.
type DaemonStatus struct {
	Running       bool   `json:"running"`
	PID           int    `json:"pid,omitempty"`
	SocketPath    string `json:"socketPath,omitempty"`
	UptimeSeconds int64  `json:"uptimeSeconds,omitempty"`
	BinaryHash    string `json:"binaryHash,omitempty"`
	LastFlushUnix int64  `json:"lastFlushUnix,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	// StaleEntries counts left-over lifecycle artifacts that don't
	// correspond to a running daemon — a PID file pointing at a dead
	// process, or a socket file with no listener.
	StaleEntries int `json:"staleEntries,omitempty"`
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("daemon status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoFlag := addRepoFlag(fs)
	jsonFlag := fs.Bool("json", false, "emit status as a single JSON object")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	repo, err := resolveRepo(*repoFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "krit daemon status: %v\n", err)
		return 2
	}

	st := collectStatus(repo)

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			fmt.Fprintf(os.Stderr, "krit daemon status: encode: %v\n", err)
			return 1
		}
		return 0
	}
	printStatusText(os.Stdout, st)
	return 0
}

// collectStatus assembles a DaemonStatus for repo. Resolution order:
// PID file → health verb when reachable → stale-detection fallback.
func collectStatus(repo string) DaemonStatus {
	socket := sessdaemon.DefaultSocketPath(repo)
	st := DaemonStatus{SocketPath: socket}

	pid, perr := readPIDFile(pidFilePath(repo))
	switch {
	case perr == nil:
		st.PID = pid
	case errors.Is(perr, os.ErrNotExist):
		// no pid file
	default:
		st.LastError = perr.Error()
	}

	// Health dial is authoritative — if the daemon answers, trust it
	// over PID-file contents.
	socketExists := fileExists(socket)
	if socketExists {
		health, herr := sessdaemon.Health(socket)
		if herr == nil {
			st.Running = true
			if health.PID != 0 {
				st.PID = health.PID
			}
			st.UptimeSeconds = health.UptimeSeconds
			st.BinaryHash = health.BinaryHash
			st.LastFlushUnix = health.LastFlushUnix
			return st
		}
		st.StaleEntries++
		if st.LastError == "" {
			st.LastError = herr.Error()
		}
	}

	// Socket unreachable. If the PID file points at a live process,
	// report Running=true — the daemon may be in startup.
	if st.PID > 0 {
		if processAlive(st.PID) {
			st.Running = true
		} else {
			st.StaleEntries++
		}
	}
	return st
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printStatusText(w io.Writer, st DaemonStatus) {
	if !st.Running && st.StaleEntries == 0 && st.PID == 0 {
		fmt.Fprintln(w, "krit-daemon: not running")
		return
	}
	if !st.Running {
		fmt.Fprintln(w, "krit-daemon: not running (stale entries detected)")
	} else {
		fmt.Fprintln(w, "krit-daemon: running")
	}
	if st.PID != 0 {
		fmt.Fprintf(w, "  pid:           %d\n", st.PID)
	}
	if st.SocketPath != "" {
		fmt.Fprintf(w, "  socket:        %s\n", st.SocketPath)
	}
	if st.Running {
		fmt.Fprintf(w, "  uptime:        %s\n", time.Duration(st.UptimeSeconds)*time.Second)
	}
	if st.BinaryHash != "" {
		fmt.Fprintf(w, "  binary-hash:   %s\n", shortHash(st.BinaryHash))
	}
	if st.LastFlushUnix != 0 {
		fmt.Fprintf(w, "  last-flush:    %s\n", time.Unix(st.LastFlushUnix, 0).UTC().Format(time.RFC3339))
	}
	if st.StaleEntries > 0 {
		fmt.Fprintf(w, "  stale-entries: %d\n", st.StaleEntries)
	}
	if st.LastError != "" {
		fmt.Fprintf(w, "  last-error:    %s\n", st.LastError)
	}
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

func resolveRepo(repo string) (string, error) {
	if repo == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve cwd: %w", err)
		}
		repo = wd
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", fmt.Errorf("resolve repo path: %w", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return "", fmt.Errorf("repo is not a directory: %s", abs)
	}
	return abs, nil
}
