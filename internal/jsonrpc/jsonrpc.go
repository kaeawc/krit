// Package jsonrpc implements JSON-RPC 2.0 message transport shared by the
// LSP and MCP servers. LSP uses Content-Length-header framing
// (ReadMessage/WriteMessage); MCP-over-stdio uses newline-delimited JSON
// per the MCP spec (ReadMessageNDJSON/WriteMessageNDJSON).
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/logger"
)

// pkgLog routes write/marshal failures from the framed transport.
// Package-level because WriteMessage / SendResponse are stateless
// top-level helpers shared by LSP and MCP — neither owner has a clean
// place to thread a logger through. Tests swap via SetLogger to assert
// on emitted records.
var pkgLog logger.Logger = logger.New(logger.Config{Format: logger.FormatText, Level: slog.LevelInfo})

// SetLogger replaces the package-level Logger. Intended for tests.
func SetLogger(l logger.Logger) { pkgLog = l }

// Request is a JSON-RPC 2.0 request or notification from the client.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response from the server.
//
// Per JSON-RPC 2.0, a response MUST contain either `result` or `error`, never
// both and never neither. The `omitempty` on Result was a bug — it dropped the
// field when Result was nil, producing responses with neither field, which
// modern LSP clients reject. Use successResponse / errorResponse via
// SendResponse instead of constructing Response directly for new code.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

type successResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result"`
}

type errorResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Error   *Error      `json:"error"`
}

// Notification is a JSON-RPC 2.0 notification from the server (no ID).
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ReadMessage reads one Content-Length-framed JSON-RPC message from r.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	var contentLength int

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %q", val)
			}
			contentLength = n
		}
		// Ignore other headers (e.g., Content-Type)
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(r, body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	return body, nil
}

// WriteMessage serializes msg as JSON and writes it with Content-Length
// framing. The write is protected by mu.
func WriteMessage(w io.Writer, mu *sync.Mutex, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		pkgLog.Error("marshal error", "err", err)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, err = io.WriteString(w, header)
	if err != nil {
		pkgLog.Error("write header error", "err", err)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		pkgLog.Error("write body error", "err", err)
		return
	}
}

// SendResponse sends a JSON-RPC 2.0 success or error response.
//
// A non-nil rpcErr produces an error response (no `result` field). Otherwise
// produces a success response with a `result` field that is always emitted —
// `null` if result is nil. Never produces a response with both or neither.
func SendResponse(w io.Writer, mu *sync.Mutex, id interface{}, result interface{}, rpcErr *Error) {
	if rpcErr != nil {
		WriteMessage(w, mu, errorResponse{JSONRPC: "2.0", ID: id, Error: rpcErr})
		return
	}
	WriteMessage(w, mu, successResponse{JSONRPC: "2.0", ID: id, Result: result})
}

// ReadMessageNDJSON reads one newline-delimited JSON-RPC message from r.
// Per the MCP stdio transport spec, each message is a single line of JSON
// terminated by \n, with no embedded newlines and no Content-Length header.
// Blank lines are skipped so a stray CRLF between messages does not error.
func ReadMessageNDJSON(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// Return any trailing partial line so callers can still
			// process a final message that lacked a newline before EOF.
			if err == io.EOF && len(line) > 0 {
				trimmed := trimNDJSON(line)
				if len(trimmed) > 0 {
					return trimmed, nil
				}
			}
			return nil, err
		}
		trimmed := trimNDJSON(line)
		if len(trimmed) == 0 {
			continue
		}
		return trimmed, nil
	}
}

func trimNDJSON(line []byte) []byte {
	// Strip trailing \r\n / \n and any surrounding whitespace.
	return []byte(strings.TrimSpace(string(line)))
}

// WriteMessageNDJSON serializes msg as JSON and writes it as a single line
// terminated by \n. The write is protected by mu.
func WriteMessageNDJSON(w io.Writer, mu *sync.Mutex, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		pkgLog.Error("marshal error", "err", err)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if _, err := w.Write(data); err != nil {
		pkgLog.Error("write body error", "err", err)
		return
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		pkgLog.Error("write newline error", "err", err)
		return
	}
}

// SendResponseNDJSON is the NDJSON counterpart of SendResponse.
func SendResponseNDJSON(w io.Writer, mu *sync.Mutex, id interface{}, result interface{}, rpcErr *Error) {
	if rpcErr != nil {
		WriteMessageNDJSON(w, mu, errorResponse{JSONRPC: "2.0", ID: id, Error: rpcErr})
		return
	}
	WriteMessageNDJSON(w, mu, successResponse{JSONRPC: "2.0", ID: id, Result: result})
}

// SendNotification sends a JSON-RPC 2.0 notification (no ID).
func SendNotification(w io.Writer, mu *sync.Mutex, method string, params interface{}) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	WriteMessage(w, mu, notif)
}
