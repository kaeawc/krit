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
)

// Handler runs a single verb. It receives the raw JSON args and returns a
// JSON-marshalable result or an error. Handlers may be invoked concurrently;
// the verb implementation is responsible for any internal locking.
type Handler func(ctx context.Context, args json.RawMessage) (any, error)

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
}

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

	l, err := net.Listen("unix", s.socketPath)
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

	s.wg.Add(1)
	go s.acceptLoop(cctx)

	// Trigger Stop when parent context cancels.
	go func() {
		<-cctx.Done()
		s.Stop()
	}()

	return nil
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
			fmt.Fprintf(os.Stderr, "krit daemon: accept: %v\n", err)
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
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 {
			if err == nil || errors.Is(err, io.EOF) {
				return
			}
			writeResponse(conn, errorResponse(err.Error()))
			return
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(conn, errorResponse("invalid request: "+err.Error()))
			continue
		}
		resp := s.dispatch(ctx, req)
		if !writeResponse(conn, resp) {
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

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	s.mu.RLock()
	h, ok := s.handlers[req.Verb]
	s.mu.RUnlock()
	if !ok {
		return errorResponse(fmt.Sprintf("unknown verb %q", req.Verb))
	}
	result, err := h(ctx, req.Args)
	if err != nil {
		return errorResponse(err.Error())
	}
	data, err := json.Marshal(result)
	if err != nil {
		return errorResponse("marshal result: " + err.Error())
	}
	return Response{OK: true, Data: data}
}

func errorResponse(msg string) Response { return Response{OK: false, Error: msg} }

func writeResponse(w io.Writer, resp Response) bool {
	buf, err := json.Marshal(resp)
	if err != nil {
		return false
	}
	buf = append(buf, '\n')
	_, err = w.Write(buf)
	return err == nil
}
