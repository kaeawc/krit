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
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
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
func SendResponse(w io.Writer, mu *sync.Mutex, id interface{}, result interface{}, rpcErr *Error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}
	WriteMessage(w, mu, resp)
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
