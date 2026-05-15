package sessdaemon

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

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cli/scan"
)

// DefaultSocketName is the relative path under <repoDir>/.krit where
// the daemon binds when no explicit socket path is provided.
const DefaultSocketName = ".krit/daemon.sock"

func DefaultSocketPath(repoDir string) string {
	return filepath.Join(repoDir, DefaultSocketName)
}

// writeBufSize is the analyze-response writer buffer. Default 4 KB
// triggers a syscall every ~20 findings on multi-MB streams; 64 KB
// keeps the syscall count down without paying meaningful idle memory.
const writeBufSize = 64 * 1024

// Server is a long-lived per-repo daemon process. It owns one
// scan.Session that the analyze verb drives, and serializes all
// verb dispatch behind a single mutex on the session.
type Server struct {
	socketPath   string
	repoDir      string
	binaryHash   string
	pid          int
	strictVerify bool

	session *scan.Session

	// oracle owns lazy construction + crash recovery of the resident
	// krit-types JVM. The Daemon handle itself lives on session.OracleDaemon
	// (see #207) so Session.Close cleans it up on shutdown without an
	// extra hook here.
	oracle oracleDaemonState

	listener net.Listener
	startAt  time.Time

	mu sync.Mutex // serializes analyze; analyze handler holds it for full call

	requestCount atomic.Int64
	lastFlush    atomic.Int64

	// flushInterval is the period between resident-cache flushes; tests
	// override the default before Start.
	flushInterval time.Duration
	flushWG       sync.WaitGroup

	stopOnce sync.Once
	stopped  chan struct{}
	cancel   context.CancelFunc
}

// Options configures NewServer. RepoDir is required; everything else
// has sensible defaults.
type Options struct {
	RepoDir    string
	SocketPath string // overrides DefaultSocketPath
	BinaryHash string // optional; surfaced in health
	// StrictVerify, when true, makes every analyze request also run a
	// fresh in-process baseline (a cold scan.Session against the same
	// paths) and compare the two sets of findings via daemon.Compare.
	// Any divergence is fatal to the response and is written to
	// `${repoDir}/.krit/daemon-divergence-NNNN.log`. This is the
	// correctness oracle issue #202 added; on by default during alpha,
	// opt-in post-stabilization.
	StrictVerify bool
}

// NewServer constructs a Server with a fresh scan.Session for opts.RepoDir.
// Start binds the socket and begins dispatching; Stop drains the session.
func NewServer(ctx context.Context, opts Options) (*Server, error) {
	if opts.RepoDir == "" {
		return nil, errors.New("sessdaemon: RepoDir is required")
	}
	sess, err := scan.NewSession(ctx, opts.RepoDir, nil)
	if err != nil {
		return nil, fmt.Errorf("sessdaemon: new session: %w", err)
	}
	cacheDir, cacheFilePath := cache.ResolveCacheDir("", []string{opts.RepoDir})
	if cacheDir != "" && cacheFilePath != "" {
		sess.AnalysisCache = cache.Load(cacheFilePath)
		sess.AnalysisCacheFilePath = cacheFilePath
		sess.AnalysisCache.MarkFlushed()
	}
	socket := opts.SocketPath
	if socket == "" {
		socket = DefaultSocketPath(opts.RepoDir)
	}
	return &Server{
		socketPath:    socket,
		repoDir:       opts.RepoDir,
		binaryHash:    opts.BinaryHash,
		pid:           os.Getpid(),
		strictVerify:  opts.StrictVerify,
		session:       sess,
		oracle:        oracleDaemonState{starter: defaultOracleStarter{}},
		stopped:       make(chan struct{}),
		flushInterval: defaultFlushInterval,
	}, nil
}

func (s *Server) SocketPath() string { return s.socketPath }

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		return fmt.Errorf("sessdaemon: prepare socket dir: %w", err)
	}
	// Reap any stale socket from a crashed previous run. A live daemon
	// would refuse to rebind anyway; an orphan inode just blocks us.
	_ = os.Remove(s.socketPath)

	l, err := (&net.ListenConfig{}).Listen(ctx, "unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("sessdaemon: listen %s: %w", s.socketPath, err)
	}
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		_ = l.Close()
		return fmt.Errorf("sessdaemon: chmod socket: %w", err)
	}

	s.listener = l
	s.startAt = time.Now()
	cctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go s.acceptLoop(cctx)
	if s.flushInterval > 0 && s.session != nil && s.session.AnalysisCache != nil && s.session.AnalysisCacheFilePath != "" {
		s.flushWG.Add(1)
		go s.flushLoop(cctx)
	}
	go func() {
		<-cctx.Done()
		s.Stop()
	}()
	return nil
}

func (s *Server) Wait() { <-s.stopped }

// Stop closes the listener, drains the session, and signals waiters. Safe
// to call multiple times.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.listener != nil {
			_ = s.listener.Close()
		}
		_ = os.Remove(s.socketPath)
		// Drain the flush goroutine before the final Save so they
		// never race on the cache file.
		s.flushWG.Wait()
		if s.session != nil {
			s.flushAnalysisCache()
			_ = s.session.Close()
		}
		close(s.stopped)
	})
}

// flushAnalysisCache persists the resident *cache.Cache when it has
// been mutated since the prior flush. The flag is cleared *before*
// Save so an UpdateEntryColumns that lands during the I/O is still
// observed on the next tick.
func (s *Server) flushAnalysisCache() {
	if s == nil || s.session == nil {
		return
	}
	c := s.session.AnalysisCache
	if c == nil || s.session.AnalysisCacheFilePath == "" {
		return
	}
	if !c.MutatedSinceFlush() {
		return
	}
	c.MarkFlushed()
	if err := c.Save(s.session.AnalysisCacheFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "krit-daemon: cache flush: %v\n", err)
		return
	}
	s.lastFlush.Store(time.Now().Unix())
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return
			}
			fmt.Fprintf(os.Stderr, "krit-daemon: accept: %v\n", err)
			return
		}
		go s.serveConn(ctx, conn)
	}
}

// serveConn handles one connection lifecycle: read one request frame,
// dispatch, then close. v1 is intentionally one-request-per-connection
// because the analyze response stream terminates by close.
func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriterSize(conn, writeBufSize)
	defer writer.Flush() //nolint:errcheck // best effort on close

	frame, err := readFrame(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = writeError(writer, nil, ErrCodeParse, err.Error())
		}
		return
	}

	var req Request
	if err := json.Unmarshal(frame, &req); err != nil {
		_ = writeError(writer, nil, ErrCodeParse, "decode request: "+err.Error())
		return
	}
	if req.JSONRPC != "2.0" {
		_ = writeError(writer, req.ID, ErrCodeInvalidRequest, "jsonrpc must be \"2.0\"")
		return
	}
	s.requestCount.Add(1)

	switch req.Method {
	case MethodAnalyze:
		s.handleAnalyze(ctx, writer, req)
	case MethodHealth:
		s.handleHealth(writer, req)
	case MethodShutdown:
		s.handleShutdown(writer, req)
	default:
		_ = writeError(writer, req.ID, ErrCodeMethodNotFound,
			fmt.Sprintf("unknown method %q", req.Method))
	}
}

func (s *Server) handleHealth(w io.Writer, req Request) {
	res := HealthResult{
		OK:            true,
		PID:           s.pid,
		UptimeSeconds: int64(time.Since(s.startAt).Seconds()),
		RequestCount:  s.requestCount.Load(),
		BinaryHash:    s.binaryHash,
		LastFlushUnix: s.lastFlush.Load(),
	}
	data, err := json.Marshal(res)
	if err != nil {
		_ = writeError(w, req.ID, ErrCodeInternal, "marshal health: "+err.Error())
		return
	}
	_ = writeResponse(w, Response{ID: req.ID, Result: data})
}

// handleShutdown acks the request and then schedules a Stop. Flushing
// the writer before scheduling Stop ensures the ack frame reaches the
// client before the listener tears down.
func (s *Server) handleShutdown(w io.Writer, req Request) {
	data, err := json.Marshal(ShutdownResult{OK: true})
	if err != nil {
		_ = writeError(w, req.ID, ErrCodeInternal, "marshal shutdown: "+err.Error())
		return
	}
	_ = writeResponse(w, Response{ID: req.ID, Result: data})
	if bw, ok := w.(*bufio.Writer); ok {
		_ = bw.Flush()
	}
	go s.Stop()
}
