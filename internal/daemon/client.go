package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// dialVerified dials the unix socket and, before anything is sent over it,
// confirms the listener is a process of the current user. The socket is
// 0600 and fallback dirs are 0700, so this is defence in depth against a
// socket planted by another local user.
func dialVerified(socketPath string, timeout time.Duration) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := verifyPeer(conn, socketPath); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// ErrPeerUID matches (errors.Is) the error returned when the process
// listening on a socket runs as a different user than the caller.
var ErrPeerUID = errors.New("daemon socket peer runs as a different user")

// PeerUIDError is the concrete error behind ErrPeerUID.
type PeerUIDError struct {
	Path string
	UID  int
}

func (e *PeerUIDError) Error() string {
	return fmt.Sprintf("daemon at %s runs as uid %d, not %d; stop it as that user or use --no-daemon", e.Path, e.UID, os.Getuid())
}

// Is makes errors.Is(err, ErrPeerUID) match.
func (e *PeerUIDError) Is(target error) bool { return target == ErrPeerUID }

// Probe reports whether a daemon of the current user answers at socketPath.
// A nil error means reachable. Absence yields an ordinary error; a listener
// owned by another user yields an error matching ErrPeerUID, which callers
// that would otherwise spawn a daemon must not ignore.
func Probe(socketPath string) error {
	if socketPath == "" {
		return errors.New("daemon: empty socket path")
	}
	if _, err := os.Stat(socketPath); err != nil {
		return err
	}
	conn, err := dialVerified(socketPath, 200*time.Millisecond)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

// Available reports whether a daemon appears reachable at socketPath. It
// performs a short-timeout dial; absence is not an error from the caller's
// perspective, just "fall back to in-process".
func Available(socketPath string) bool { return Probe(socketPath) == nil }

// Call sends a single Request to the daemon at socketPath and decodes the
// Data field of the Response into out (which may be nil). It returns the
// daemon's error string as a Go error when OK=false.
func Call(socketPath, verb string, args any, out any) error {
	conn, err := dialVerified(socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon: dial %s: %w", socketPath, err)
	}
	defer conn.Close()

	var raw json.RawMessage
	if args != nil {
		buf, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("daemon: marshal args: %w", err)
		}
		raw = buf
	}
	req := Request{Verb: verb, Args: raw}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("daemon: marshal request: %w", err)
	}
	body = append(body, '\n')
	if _, err := conn.Write(body); err != nil {
		return fmt.Errorf("daemon: write: %w", err)
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("daemon: read: %w", err)
	}
	// Fast path for analyze-project's giant 30 MB envelope: a hand-
	// rolled byte scanner extracts Findings (json.RawMessage) and
	// Stats (small struct) without paying the ~150 ms two-pass
	// json.Unmarshal that the generic path costs on warm baselines.
	// Falls back transparently to json.Unmarshal when the envelope
	// shape doesn't match (different field order, whitespace, etc.).
	if apr, ok := out.(*AnalyzeProjectResult); ok {
		handled, daemonErr := ScanAnalyzeProjectResponse(line, apr)
		if handled {
			return daemonErr
		}
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("daemon: decode response: %w", err)
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if out == nil {
		return nil
	}
	if len(resp.Data) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}
