# SharedJsonRpcLayer

**Cluster:** [core-infra](README.md) · **Status:** shipped ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Extracts the duplicated Content-Length HTTP-style message framing
used by both the LSP server and the MCP server into a shared
`internal/jsonrpc/` package. Both servers import this package instead
of each maintaining their own copy.

## Current cost

The LSP protocol and the MCP protocol both use the same
`Content-Length: <n>\r\n\r\n<json>` wire format. The framing
implementation — `readMessage()`, `writeMessage()`, and
`sendResponse()` — is written independently in each server:

- `internal/lsp/server.go` lines ~90–170
- `internal/mcp/server.go` lines ~76–121

The implementations are functionally identical (~75 lines each) but
have diverged in small ways: error handling, buffer sizes, and logging
calls. When one is updated (e.g., to handle partial reads or add
observability), the other is not. A third entry point (e.g., a DAP
server) would introduce a third copy.

Relevant files:
- `internal/lsp/server.go:readMessage`, `writeMessage`, `sendResponse`
- `internal/mcp/server.go:readMessage`, `writeMessage`, `sendResponse`

## Proposed design

```go
// internal/jsonrpc/jsonrpc.go

package jsonrpc

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
)

type Message struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id,omitempty"`
    Method  string          `json:"method,omitempty"`
    Params  json.RawMessage `json:"params,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *ResponseError  `json:"error,omitempty"`
}

type ResponseError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// ReadMessage reads one Content-Length-framed JSON message from r.
func ReadMessage(r *bufio.Reader) (*Message, error)

// WriteMessage writes one Content-Length-framed JSON message to w.
func WriteMessage(w io.Writer, msg *Message) error

// Respond sends a success response.
func Respond(w io.Writer, id any, result any) error

// RespondError sends an error response.
func RespondError(w io.Writer, id any, code int, msg string) error
```

Both `internal/lsp/server.go` and `internal/mcp/server.go` replace
their local framing functions with calls to this package. The package
gets its own unit tests covering partial reads, large payloads, and
malformed headers.

## Migration path

1. Create `internal/jsonrpc/jsonrpc.go` with the functions above,
   ported from `lsp/server.go` (which has slightly better error
   handling).
2. Write unit tests for `internal/jsonrpc/` covering the edge cases
   neither server currently tests.
3. Replace the inline implementations in `lsp/server.go` with calls
   to the shared package.
4. Replace the inline implementations in `mcp/server.go` with calls
   to the shared package.
5. Verify the LSP and MCP integration tests pass.

## Acceptance criteria

- No `Content-Length` string literal appears outside
  `internal/jsonrpc/`.
- `internal/jsonrpc/` has ≥ 90% test coverage.
- LSP and MCP integration tests pass without modification.
- A hypothetical third JSON-RPC server would import the package with
  no copy-paste.

## Vibe detector evidence (2026-04-16)

The vibe-detector audit independently flagged this as a Medium-severity
red flag:

- `readMessage()`, `writeMessage()`, and `Run()` loop are nearly
  identical in `lsp/server.go:91-116` and `mcp/server.go:76-100`.
- Both use mutex-protected writer with the same pattern.
- Both `cmd/krit-lsp/main.go` and `cmd/krit-mcp/main.go` are
  near-identical ~33-line programs (version flag, logging setup,
  stdin/stdout reader/writer — only difference is
  `lsp.NewServer` vs `mcp.NewServer`).

This concept already covers the right solution. Priority: Medium
(independent of other core-infra items, can land first).

## Links

- Depends on: nothing (land independently of other items)
- Related: `internal/lsp/server.go`, `internal/mcp/server.go`
