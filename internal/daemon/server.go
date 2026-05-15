package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/diag"
)

// Handler runs a single verb. It receives the raw JSON args and returns a
// JSON-marshalable result or an error. Handlers may be invoked concurrently;
// the verb implementation is responsible for any internal locking.
//
// A handler may return a value that implements RawResponseWriter to skip
// the default json.Marshal envelope and stream the response directly into
// the connection. This is the streaming path issue #60 added so the
// analyze-project verb avoids buffering the multi-megabyte findings JSON
// in memory.
type Handler func(ctx context.Context, args json.RawMessage) (any, error)

// RawResponseWriter is the optional interface a Handler result can
// implement to bypass the json.Marshal + Response envelope path. The
// dispatch loop hands the request context plus connection writer; the
// implementation must write a complete newline-terminated Response
// envelope. WriteResponseLine is the canonical helper for the
// fallback/error case.
type RawResponseWriter interface {
	WriteRawResponse(ctx context.Context, w io.Writer) error
}

// Server is a long-lived process that accepts daemon Requests on a Unix
// socket and dispatches each to a registered Handler.
type Server struct {
	socketPath string

	mu       sync.RWMutex
	handlers map[string]Handler

	listener net.Listener

	wg     sync.WaitGroup
	cancel context.CancelFunc

	stopOnce sync.Once
	stopped  chan struct{}

	// Reporter, when non-nil, receives accept-loop warnings. Nil falls
	// back to a default warnings-only stderr Reporter so library code
	// never panics.
	Reporter *diag.Reporter

	// IdleTimeout, when > 0, makes the server self-stop after no
	// requests have been seen for the given duration. Updated on every
	// dispatch. Zero (the default) disables auto-shutdown.
	IdleTimeout time.Duration
	// lastActivity holds time.Now().UnixNano() of the most recent
	// dispatched request. Read by the idle watchdog, written under the
	// dispatch hot path — atomic to avoid contention.
	lastActivity atomic.Int64

	// SocketWatchdogInterval overrides the cadence of the
	// socket-presence check. Zero (the default) uses
	// socketWatchdogDefaultInterval. Exposed so tests can speed up
	// the loop without waiting for the production interval.
	SocketWatchdogInterval time.Duration
	// SocketWatchdogDisabled, when true, suppresses the periodic
	// os.Stat on the socket. Tests that drive a server through manual
	// lifecycle calls set this so they don't race against the
	// watchdog calling Stop.
	SocketWatchdogDisabled bool
}

// reporter returns Server.Reporter, or a default stderr Reporter when nil.
func (s *Server) reporter() *diag.Reporter {
	if s.Reporter != nil {
		return s.Reporter
	}
	return defaultReporter
}

var defaultReporter = &diag.Reporter{Warning: os.Stderr}

// NewServer returns a Server bound to socketPath. The socket file is
// created with mode 0600 when Start is called.
func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]Handler),
		stopped:    make(chan struct{}),
	}
}

// Register attaches a Handler for verb. Registering the same verb twice
// replaces the prior handler.
func (s *Server) Register(verb string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[verb] = h
}

// SocketPath returns the configured socket path.
func (s *Server) SocketPath() string { return s.socketPath }

// Start begins listening. It returns once the listener is ready (or an
// error if it could not bind). Connection accept and dispatch run in
// background goroutines until Stop is called or the listener errors.
func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		return fmt.Errorf("daemon: prepare socket dir: %w", err)
	}
	// Clean up any stale socket from a previous crashed run. A live daemon
	// would refuse to bind anyway; an orphaned inode just blocks us.
	_ = os.Remove(s.socketPath)

	l, err := (&net.ListenConfig{}).Listen(ctx, "unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("daemon: listen %s: %w", s.socketPath, err)
	}
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		_ = l.Close()
		return fmt.Errorf("daemon: chmod socket: %w", err)
	}

	s.listener = l

	cctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.lastActivity.Store(time.Now().UnixNano())
	s.wg.Add(1)
	go s.acceptLoop(cctx)

	// Trigger Stop when parent context cancels.
	go func() {
		<-cctx.Done()
		s.Stop()
	}()

	if s.IdleTimeout > 0 {
		s.wg.Add(1)
		go s.idleWatchdog(cctx)
	}

	// socketWatchdog catches the phantom-socket failure mode: the
	// dirent at s.socketPath has been unlinked (rm -rf .krit, errant
	// test cleanup, IDE/Spotlight artifact) while the daemon process
	// is still alive and bound to the now-orphan inode. New
	// connections fail at connect(2) with ENOENT and clients silently
	// fall back to in-process forever. Stop on detection so the next
	// CLI invocation hits "no daemon, spawn fresh" rather than
	// "daemon socket missing, give up".
	s.wg.Add(1)
	go s.socketWatchdog(cctx)

	return nil
}

// socketWatchdog polls every socketWatchdogInterval for the socket
// dirent and triggers Stop the first time os.Stat fails. The poll
// cost is one os.Stat every few seconds — well below the budget for
// a long-lived daemon process, and the alternative (clients silently
// timing out forever) is worse. Skips the check while
// SocketWatchdogDisabled is set so unit tests can opt out without a
// custom mock.
func (s *Server) socketWatchdog(ctx context.Context) {
	defer s.wg.Done()
	if s.SocketWatchdogDisabled || s.socketPath == "" {
		return
	}
	tick := s.SocketWatchdogInterval
	if tick <= 0 {
		tick = socketWatchdogDefaultInterval
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := os.Stat(s.socketPath); err != nil {
				s.reporter().Warnf("krit daemon: socket %s disappeared (%v); exiting so the next CLI invocation can respawn\n", s.socketPath, err)
				go s.Stop()
				return
			}
		}
	}
}

// socketWatchdogDefaultInterval is the production cadence for the
// socket-presence check. 5s balances detection latency (a single CLI
// fallback at most) against the cost of one stat call per daemon
// lifetime.
const socketWatchdogDefaultInterval = 5 * time.Second

// idleWatchdog calls Stop when no request has been dispatched for
// IdleTimeout. Polls at IdleTimeout/4 (clamped to a minimum of one
// second) so the worst-case overshoot is roughly 25 percent.
func (s *Server) idleWatchdog(ctx context.Context) {
	defer s.wg.Done()
	tick := s.IdleTimeout / 4
	if tick < time.Second {
		tick = time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			last := time.Unix(0, s.lastActivity.Load())
			if time.Since(last) >= s.IdleTimeout {
				s.reporter().Warnf("krit daemon: idle for %s, exiting\n", s.IdleTimeout)
				go s.Stop()
				return
			}
		}
	}
}

// Wait blocks until Stop has completed and all in-flight handlers finish.
func (s *Server) Wait() {
	<-s.stopped
}

// Stop closes the listener and waits for in-flight connections to drain.
// Safe to call multiple times.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.listener != nil {
			_ = s.listener.Close()
		}
		_ = os.Remove(s.socketPath)
		s.wg.Wait()
		close(s.stopped)
	})
}

func (s *Server) acceptLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			s.reporter().Warnf("krit daemon: accept: %v\n", err)
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serveConn(ctx, conn)
		}()
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	// Wrap the write side too so dispatch handlers that issue many
	// small Write calls (the streaming analyze-project path emits
	// envelope head/body/tail in 3+ chunks) coalesce into one
	// syscall per response.
	writer := bufio.NewWriter(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 {
			if err == nil || errors.Is(err, io.EOF) {
				return
			}
			writeResponse(writer, errorResponse(err.Error()))
			_ = writer.Flush()
			return
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(writer, errorResponse("invalid request: "+err.Error()))
			if flushErr := writer.Flush(); flushErr != nil {
				return
			}
			continue
		}
		ok := s.dispatchAndWrite(ctx, req, writer)
		if flushErr := writer.Flush(); flushErr != nil {
			return
		}
		if !ok {
			return
		}
		if req.Verb == VerbShutdown {
			// Trigger graceful shutdown without blocking on Stop's wg.Wait
			// (this goroutine is itself tracked by that WaitGroup).
			if s.cancel != nil {
				s.cancel()
			}
			return
		}
		if err != nil {
			// Non-empty line plus a read error (e.g. EOF after the request)
			// — the request was processed; close the connection.
			return
		}
	}
}

// dispatchAndWrite resolves the verb and writes the response directly
// into w. Returns false when the connection should be closed (write
// error). When the handler result implements RawResponseWriter the
// streaming path is taken; otherwise the standard json.Marshal +
// Response envelope is written via writeResponse.
func (s *Server) dispatchAndWrite(ctx context.Context, req Request, w io.Writer) bool {
	s.lastActivity.Store(time.Now().UnixNano())
	s.mu.RLock()
	h, ok := s.handlers[req.Verb]
	s.mu.RUnlock()
	if !ok {
		return writeResponse(w, errorResponse(fmt.Sprintf("unknown verb %q", req.Verb)))
	}
	result, err := h(ctx, req.Args)
	if err != nil {
		return writeResponse(w, errorResponse(err.Error()))
	}
	if rw, ok := result.(RawResponseWriter); ok {
		return rw.WriteRawResponse(ctx, w) == nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return writeResponse(w, errorResponse("marshal result: "+err.Error()))
	}
	return writeResponse(w, Response{OK: true, Data: data})
}

func errorResponse(msg string) Response { return Response{OK: false, Error: msg} }

// WriteErrorResponseLine emits a `{"ok":false,"error":...}\n` line
// using the daemon's wire format. Exported so RawResponseWriter
// implementations can fall back to the standard error envelope
// without re-implementing the line-delimited shape.
func WriteErrorResponseLine(w io.Writer, msg string) error {
	buf, err := json.Marshal(errorResponse(msg))
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	_, err = w.Write(buf)
	return err
}

func writeResponse(w io.Writer, resp Response) bool {
	buf, err := json.Marshal(resp)
	if err != nil {
		return false
	}
	buf = append(buf, '\n')
	_, err = w.Write(buf)
	return err == nil
}
