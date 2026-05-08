package mcp

import (
	"encoding/json"

	"github.com/kaeawc/krit/internal/metrics"
)

type metricsArgs struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Rule      string `json:"rule"`
	Since     string `json:"since"`
}

func (s *Server) toolMetrics(arguments json.RawMessage) ToolResult {
	var args metricsArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	op := args.Operation
	if op == "" {
		op = "query"
	}
	if op != "query" {
		return errorResult("unknown operation: " + op + "; valid: query")
	}
	if args.Rule == "" {
		return errorResult("'rule' argument is required")
	}
	path := args.Path
	if path == "" {
		path = ".krit/metrics.jsonl"
	}
	since, err := metrics.ParseSince(args.Since)
	if err != nil {
		return errorResult(err.Error())
	}
	rows, err := metrics.Query(metrics.QueryOptions{Path: path, Rule: args.Rule, Since: since})
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(rows)
}
