package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/kaeawc/krit/internal/jsonrpc"
	"github.com/kaeawc/krit/internal/logger"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

const protocolVersion = "2024-11-05"

// Server implements an MCP server over stdio using JSON-RPC 2.0.
type Server struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex // protects writes

	// analyzer wraps the pipeline Parse → Dispatch path for single-buffer
	// analysis (analyze, suggest_fixes, inspect_types tools). Path-based
	// tools (analyze_project) still reuse analyzer.Dispatcher for the
	// per-file dispatch loop because DispatchPhase expects the full
	// IndexResult state they don't need.
	analyzer *pipeline.SingleFileAnalyzer

	// workspace memoizes parsed files across tool invocations so repeated
	// calls on identical content (a common pattern when a client chains
	// analyze → fix → inspect) don't re-run tree-sitter every time.
	workspace *pipeline.WorkspaceState

	// Verbose gates informational log output.
	Verbose bool

	// log routes lifecycle/error messages. NewServer sets a stderr
	// text-handler at Info level; SetLogger lets tests inject a
	// logger.Capture to assert on emitted records.
	log logger.Logger
}

// logInfo logs an informational message gated behind s.Verbose. Preserves
// the printf-style call shape; one short string per record fits the
// existing call sites without restructuring them.
func (s *Server) logInfo(format string, args ...interface{}) {
	if s.Verbose {
		s.log.Info(fmt.Sprintf(format, args...))
	}
}

// SetLogger overrides the default Logger. Intended for tests to inject
// logger.NewCapture so emitted records are observable.
func (s *Server) SetLogger(l logger.Logger) { s.log = l }

// NewServer creates a new MCP server reading from reader and writing to writer.
func NewServer(reader *bufio.Reader, writer io.Writer) *Server {
	return &Server{
		reader:    reader,
		writer:    writer,
		workspace: pipeline.NewWorkspaceState(""),
		log:       logger.New(logger.Config{Format: logger.FormatText, Level: slog.LevelInfo}),
	}
}

// Run reads and dispatches MCP messages until EOF or exit.
func (s *Server) Run() {
	s.buildDispatcher()

	for {
		msg, err := jsonrpc.ReadMessage(s.reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.logInfo("EOF on stdin, exiting")
				return
			}
			s.log.Error("read error", "err", err)
			return
		}

		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			s.log.Warn("invalid JSON-RPC message", "err", err)
			continue
		}

		s.handleMessage(&req)
	}
}

// buildDispatcher constructs the shared single-file analyzer. Delegates
// rule discovery and dispatcher construction to the pipeline package so
// the LSP, MCP, and CLI entry points share one source of truth for the
// active rule set.
func (s *Server) buildDispatcher() {
	s.analyzer = pipeline.NewSingleFileAnalyzer(nil, nil)
	s.logInfo("dispatcher: %d active rules", len(s.analyzer.ActiveRules))
}

// sendResponse sends a JSON-RPC 2.0 response via the shared transport.
func (s *Server) sendResponse(id interface{}, result interface{}, rpcErr *RPCError) {
	jsonrpc.SendResponse(s.writer, &s.mu, id, result, rpcErr)
}

// handleMessage dispatches a JSON-RPC request or notification.
func (s *Server) handleMessage(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		s.logInfo("initialized: server ready")
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourcesRead(req)
	case "prompts/list":
		s.handlePromptsList(req)
	case "prompts/get":
		s.handlePromptsGet(req)
	default:
		if req.ID != nil {
			s.sendResponse(req.ID, nil, &RPCError{
				Code:    -32601,
				Message: "method not found: " + req.Method,
			})
		}
	}
}

// handleInitialize responds with server capabilities.
func (s *Server) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: ServerCaps{
			Tools:     &ToolsCap{},
			Resources: &ResourcesCap{},
			Prompts:   &PromptsCap{},
		},
		ServerInfo: ServerInfo{
			Name:    "krit-mcp",
			Version: "0.0.1",
		},
	}
	s.sendResponse(req.ID, result, nil)
}

// handleToolsList returns the list of available tools.
func (s *Server) handleToolsList(req *Request) {
	result := ToolsListResult{
		Tools: toolDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handleToolsCall dispatches a tool call.
func (s *Server) handleToolsCall(req *Request) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendResponse(req.ID, nil, &RPCError{
			Code:    -32602,
			Message: "invalid params: " + err.Error(),
		})
		return
	}

	var result ToolResult
	switch params.Name {
	case "analyze":
		result = s.toolAnalyze(params.Arguments)
	case "fix":
		result = s.toolFix(params.Arguments)
	case "rules":
		result = s.toolRules(params.Arguments)
	case "metrics":
		result = s.toolMetrics(params.Arguments)
	case "symbols":
		result = s.toolSymbols(params.Arguments)
	case "types":
		result = s.toolTypes(params.Arguments)
	case "structure":
		result = s.toolStructure(params.Arguments)
	default:
		result = ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "unknown tool: " + params.Name}},
			IsError: true,
		}
	}
	s.sendResponse(req.ID, result, nil)
}

// handleResourcesList returns the list of available resources.
func (s *Server) handleResourcesList(req *Request) {
	result := ResourcesListResult{
		Resources: resourceDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handleResourcesRead returns the content of a resource.
func (s *Server) handleResourcesRead(req *Request) {
	var params ResourceReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendResponse(req.ID, nil, &RPCError{
			Code:    -32602,
			Message: "invalid params: " + err.Error(),
		})
		return
	}

	content, mimeType, err := readResource(params.URI)
	if err != nil {
		s.sendResponse(req.ID, nil, &RPCError{
			Code:    -32602,
			Message: err.Error(),
		})
		return
	}

	result := ResourceReadResult{
		Contents: []ResourceContent{{
			URI:      params.URI,
			MimeType: mimeType,
			Text:     content,
		}},
	}
	s.sendResponse(req.ID, result, nil)
}

// handlePromptsList returns the list of available prompts.
func (s *Server) handlePromptsList(req *Request) {
	result := PromptsListResult{
		Prompts: promptDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handlePromptsGet returns a prompt with its messages.
func (s *Server) handlePromptsGet(req *Request) {
	var params PromptGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendResponse(req.ID, nil, &RPCError{
			Code:    -32602,
			Message: "invalid params: " + err.Error(),
		})
		return
	}

	result, rpcErr := s.getPrompt(params.Name, params.Arguments)
	if rpcErr != nil {
		s.sendResponse(req.ID, nil, rpcErr)
		return
	}
	s.sendResponse(req.ID, result, nil)
}

// parseAndAnalyzeColumns parses Kotlin code (via the workspace cache so
// repeated calls on identical content don't re-parse) and runs the
// per-file dispatcher.
func (s *Server) parseAndAnalyzeColumns(code string, path string) (scanner.FindingColumns, error) {
	file, err := s.workspace.ParseFile(context.Background(), path, []byte(code))
	if err != nil {
		return scanner.FindingColumns{}, err
	}
	return s.analyzer.AnalyzeFileColumns(file), nil
}

// parseKotlinCode parses Kotlin source code string into a scanner.File via
// the shared pipeline.ParseSingle helper.
func parseKotlinCode(code string, path string) (*scanner.File, error) {
	return pipeline.ParseSingle(context.Background(), path, []byte(code))
}
