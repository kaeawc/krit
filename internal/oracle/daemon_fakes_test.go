package oracle

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// ---------------------------------------------------------------------------
// FakeDaemonProcess — a TCP-based fake JVM daemon for integration testing
// ---------------------------------------------------------------------------

// FakeDaemonProcess simulates a running krit-types JVM daemon that listens
// on a TCP port and responds to JSON-RPC requests. Tests configure the
// Responses map to return canned replies for each method.
type FakeDaemonProcess struct {
	PID       int
	Port      int
	Alive     bool
	Responses map[string]string // method → JSON result value (the "result" field)
	listener  net.Listener
	mu        sync.Mutex
	conns     []net.Conn
	closed    bool
}

// NewFakeDaemon starts a TCP listener on a random port that serves JSON-RPC
// responses from the Responses map. The returned fake uses os.Getpid() as
// its PID (guaranteed alive). Call Close() when done.
func NewFakeDaemon(t *testing.T) *FakeDaemonProcess {
	t.Helper()

	listener := listenLocalTCP(t)

	port := listener.Addr().(*net.TCPAddr).Port

	f := &FakeDaemonProcess{
		PID:       currentPID(),
		Port:      port,
		Alive:     true,
		Responses: make(map[string]string),
		listener:  listener,
	}

	// Default responses for common methods
	f.Responses["ping"] = `{"ok": true, "uptime": 42}`
	f.Responses["analyzeAll"] = `{"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}`
	f.Responses["analyze"] = `{"version": 1, "kotlinVersion": "2.1.0", "files": {}, "dependencies": {}}`
	f.Responses["rebuild"] = `{}`
	f.Responses["shutdown"] = `{}`
	f.Responses["checkpoint"] = `{"ok": true}`

	// Accept connections in the background
	go f.acceptLoop(t)

	return f
}

func (f *FakeDaemonProcess) acceptLoop(t *testing.T) {
	t.Helper()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return // listener closed
		}
		f.mu.Lock()
		f.conns = append(f.conns, conn)
		f.mu.Unlock()
		go f.handleConn(t, conn)
	}
}

func (f *FakeDaemonProcess) handleConn(t *testing.T, conn net.Conn) {
	t.Helper()
	defer conn.Close()

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	for sc.Scan() {
		var req daemonRequest
		if err := json.Unmarshal([]byte(sc.Text()), &req); err != nil {
			// Send error response
			resp := fmt.Sprintf(`{"id": 0, "error": "invalid JSON: %s"}`, err.Error())
			conn.Write([]byte(resp + "\n"))
			continue
		}

		f.mu.Lock()
		resultJSON, ok := f.Responses[req.Method]
		f.mu.Unlock()

		if ok {
			resp := fmt.Sprintf(`{"id": %d, "result": %s}`, req.ID, resultJSON)
			conn.Write([]byte(resp + "\n"))
		} else {
			resp := fmt.Sprintf(`{"id": %d, "error": "unknown method: %s"}`, req.ID, req.Method)
			conn.Write([]byte(resp + "\n"))
		}

		if req.Method == "shutdown" {
			return
		}
	}
}

// Close shuts down the fake daemon's listener and all connections.
func (f *FakeDaemonProcess) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	f.Alive = false
	f.listener.Close()
	for _, c := range f.conns {
		c.Close()
	}
}

// ConnectDaemon creates a Daemon struct connected to this fake daemon via TCP.
func (f *FakeDaemonProcess) ConnectDaemon(t *testing.T) *Daemon {
	t.Helper()

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", f.Port))
	if err != nil {
		t.Fatalf("FakeDaemon.ConnectDaemon: dial: %v", err)
	}

	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)

	return &Daemon{
		stdin:   conn,
		stdout:  reader,
		conn:    conn,
		port:    f.Port,
		nextID:  1,
		started: true,
		shared:  true,
	}
}

// currentPID returns the current process PID — always alive.
func currentPID() int {
	return os.Getpid()
}

func listenLocalTCP(t *testing.T) net.Listener {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) ||
			strings.Contains(err.Error(), "operation not permitted") ||
			strings.Contains(err.Error(), "permission denied") {
			t.Skipf("local TCP bind unavailable in this environment: %v", err)
		}
		t.Fatalf("listen: %v", err)
	}

	return listener
}
