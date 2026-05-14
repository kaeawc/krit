// Package sessdaemon implements the long-lived per-repo daemon that
// owns one scan.Session and serves analyze/health/shutdown verbs over
// a Unix socket. See issue #201 for the scope.
//
// Wire format: each request is a 4-byte big-endian length prefix
// followed by a JSON-RPC 2.0 frame. Single-verb responses (health,
// shutdown) use the same length-prefixed envelope. The analyze
// response is a stream of newline-delimited JSON records terminated by
// connection close — see Server.handleAnalyze.
package sessdaemon

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	MethodAnalyze  = "analyze"
	MethodHealth   = "health"
	MethodShutdown = "shutdown"
)

// maxFrameBytes caps inbound request frames so a malformed length
// prefix can't make us allocate gigabytes. Analyze response NDJSON
// streams and is not bounded.
const maxFrameBytes = 32 * 1024 * 1024

// Request is the wire JSON-RPC 2.0 request envelope.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// Response is the wire JSON-RPC 2.0 response envelope.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

// Error mirrors the JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInternal       = -32603
)

// AnalyzeParams is the params struct for the analyze method. Flags is
// kept opaque in v1; future commits will replace json.RawMessage with
// a structured shape once the CLI/daemon flag contract stabilises.
type AnalyzeParams struct {
	Paths      []string        `json:"paths"`
	Flags      json.RawMessage `json:"flags,omitempty"`
	BinaryHash string          `json:"binaryHash,omitempty"`
}

// AnalyzeStreamFinding is one element of the analyze response stream.
// Zero or more appear, followed by exactly one AnalyzeStreamSummary
// before EOF.
type AnalyzeStreamFinding struct {
	Kind    string  `json:"kind"` // "finding"
	Finding Finding `json:"finding"`
}

// AnalyzeStreamSummary is the terminal NDJSON record for an analyze
// response.
type AnalyzeStreamSummary struct {
	Kind    string         `json:"kind"` // "summary"
	Summary AnalyzeSummary `json:"summary"`
}

// AnalyzeSummary carries per-call totals.
type AnalyzeSummary struct {
	FilesScanned      int   `json:"filesScanned"`
	FindingsCount     int   `json:"findingsCount"`
	ParseHits         int64 `json:"parseHits"`
	ParseMisses       int64 `json:"parseMisses"`
	FindingsBundleHit bool  `json:"findingsBundleHit"`
	DurationMs        int64 `json:"durationMs"`
}

// Finding is the wire-stable shape of one finding row.
type Finding struct {
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Col        int     `json:"col"`
	StartByte  int     `json:"startByte"`
	EndByte    int     `json:"endByte"`
	RuleSet    string  `json:"ruleSet,omitempty"`
	Rule       string  `json:"rule"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Confidence float64 `json:"confidence,omitempty"`
}

// HealthResult is the result payload returned by the health verb.
type HealthResult struct {
	OK            bool   `json:"ok"`
	PID           int    `json:"pid"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	ParsedEntries int    `json:"parsedEntries"`
	RequestCount  int64  `json:"requestCount"`
	BinaryHash    string `json:"binaryHash,omitempty"`
	LastFlushUnix int64  `json:"lastFlushUnix"`
}

// ShutdownResult is the ack body for the shutdown verb.
type ShutdownResult struct {
	OK bool `json:"ok"`
}

func readFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 {
		return nil, errors.New("sessdaemon: zero-length frame")
	}
	if int(n) > maxFrameBytes {
		return nil, fmt.Errorf("sessdaemon: frame too large (%d > %d)", n, maxFrameBytes)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeFrame(w io.Writer, payload []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

func writeResponse(w io.Writer, resp Response) error {
	resp.JSONRPC = "2.0"
	buf, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return writeFrame(w, buf)
}

func writeError(w io.Writer, id json.RawMessage, code int, msg string) error {
	return writeResponse(w, Response{ID: id, Error: &Error{Code: code, Message: msg}})
}
