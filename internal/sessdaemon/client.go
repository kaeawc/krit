package sessdaemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
)

// AnalyzeResult is the in-memory accumulation of an analyze stream.
// Callers that want true streaming should use AnalyzeStream directly.
type AnalyzeResult struct {
	Findings []Finding
	Summary  AnalyzeSummary
}

func Dial(socketPath string) (net.Conn, error) {
	return DialContext(context.Background(), socketPath)
}

func DialContext(ctx context.Context, socketPath string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", socketPath)
}

// AnalyzeStream sends an analyze request and invokes onFinding for
// each streamed finding. The terminal summary is returned. The
// connection is consumed and closed by this call.
func AnalyzeStream(conn net.Conn, params AnalyzeParams, onFinding func(Finding)) (AnalyzeSummary, error) {
	defer conn.Close()
	if err := writeRequest(conn, MethodAnalyze, params); err != nil {
		return AnalyzeSummary{}, err
	}
	dec := json.NewDecoder(bufio.NewReader(conn))
	var summary AnalyzeSummary
	var seenSummary bool
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return AnalyzeSummary{}, fmt.Errorf("decode stream: %w", err)
		}
		var probe struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return AnalyzeSummary{}, fmt.Errorf("decode kind: %w", err)
		}
		switch probe.Kind {
		case "finding":
			var rec AnalyzeStreamFinding
			if err := json.Unmarshal(raw, &rec); err != nil {
				return AnalyzeSummary{}, fmt.Errorf("decode finding: %w", err)
			}
			if onFinding != nil {
				onFinding(rec.Finding)
			}
		case "summary":
			var rec AnalyzeStreamSummary
			if err := json.Unmarshal(raw, &rec); err != nil {
				return AnalyzeSummary{}, fmt.Errorf("decode summary: %w", err)
			}
			summary = rec.Summary
			seenSummary = true
		}
	}
	if !seenSummary {
		return AnalyzeSummary{}, errors.New("sessdaemon: analyze stream ended without summary")
	}
	return summary, nil
}

// Analyze collects all findings into a slice and returns them alongside
// the summary.
func Analyze(socketPath string, params AnalyzeParams) (AnalyzeResult, error) {
	conn, err := Dial(socketPath)
	if err != nil {
		return AnalyzeResult{}, err
	}
	var findings []Finding
	sum, err := AnalyzeStream(conn, params, func(f Finding) {
		findings = append(findings, f)
	})
	if err != nil {
		return AnalyzeResult{}, err
	}
	return AnalyzeResult{Findings: findings, Summary: sum}, nil
}

func Health(socketPath string) (HealthResult, error) {
	conn, err := Dial(socketPath)
	if err != nil {
		return HealthResult{}, err
	}
	defer conn.Close()
	if err := writeRequest(conn, MethodHealth, nil); err != nil {
		return HealthResult{}, err
	}
	resp, err := readResponse(conn)
	if err != nil {
		return HealthResult{}, err
	}
	if resp.Error != nil {
		return HealthResult{}, fmt.Errorf("health: %s", resp.Error.Message)
	}
	var res HealthResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return HealthResult{}, fmt.Errorf("decode health: %w", err)
	}
	return res, nil
}

func Shutdown(socketPath string) error {
	conn, err := Dial(socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := writeRequest(conn, MethodShutdown, nil); err != nil {
		return err
	}
	resp, err := readResponse(conn)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("shutdown: %s", resp.Error.Message)
	}
	return nil
}

// writeRequest marshals and emits a single JSON-RPC request frame.
// Params is marshaled together with the envelope (one pass) by using
// an outer struct whose Params field is `any` rather than RawMessage.
// ID is a constant 1 — v1 connections carry exactly one request.
func writeRequest(w io.Writer, method string, params any) error {
	type outRequest struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params,omitempty"`
		ID      int    `json:"id"`
	}
	buf, err := json.Marshal(outRequest{
		JSONRPC: "2.0", Method: method, Params: params, ID: 1,
	})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	return writeFrame(w, buf)
}

func readResponse(r io.Reader) (Response, error) {
	buf, err := readFrame(r)
	if err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal(buf, &resp); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}
