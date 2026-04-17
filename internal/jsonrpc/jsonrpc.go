// Package jsonrpc implements Content-Length-framed JSON-RPC 2.0 message
// transport shared by the LSP and MCP servers.
package jsonrpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
)

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
		log.Printf("marshal error: %v", err)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, err = io.WriteString(w, header)
	if err != nil {
		log.Printf("write header error: %v", err)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		log.Printf("write body error: %v", err)
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

// SendNotification sends a JSON-RPC 2.0 notification (no ID).
func SendNotification(w io.Writer, mu *sync.Mutex, method string, params interface{}) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	WriteMessage(w, mu, notif)
}
