package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"

	"github.com/kaeawc/krit/internal/jsonrpc"
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

	// Verbose gates informational log output.
	Verbose bool
}

// logInfo logs an informational message gated behind s.Verbose.
func (s *Server) logInfo(format string, args ...interface{}) {
	if s.Verbose {
		log.Printf(format, args...)
	}
}

// NewServer creates a new MCP server reading from reader and writing to writer.
func NewServer(reader *bufio.Reader, writer io.Writer) *Server {
	return &Server{
		reader: reader,
		writer: writer,
	}
}

// Run reads and dispatches MCP messages until EOF or exit.
func (s *Server) Run() {
	s.buildDispatcher()

	for {
		msg, err := jsonrpc.ReadMessage(s.reader)
		if err != nil {
			if err == io.EOF {
				s.logInfo("EOF on stdin, exiting")
				return
			}
			log.Printf("read error: %v", err)
			return
		}

		var req MCPRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("invalid JSON-RPC message: %v", err)
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

// writeMessage serializes and writes a JSON-RPC message with Content-Length framing.
func (s *Server) writeMessage(msg interface{}) {
	jsonrpc.WriteMessage(s.writer, &s.mu, msg)
}

// handleMessage dispatches a JSON-RPC request or notification.
func (s *Server) handleMessage(req *MCPRequest) {
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
func (s *Server) handleInitialize(req *MCPRequest) {
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
func (s *Server) handleToolsList(req *MCPRequest) {
	result := ToolsListResult{
		Tools: toolDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handleToolsCall dispatches a tool call.
func (s *Server) handleToolsCall(req *MCPRequest) {
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
	case "suggest_fixes":
		result = s.toolSuggestFixes(params.Arguments)
	case "explain_rule":
		result = s.toolExplainRule(params.Arguments)
	case "inspect_types":
		result = s.toolInspectTypes(params.Arguments)
	case "find_references":
		result = s.toolFindReferences(params.Arguments)
	case "analyze_project":
		result = s.toolAnalyzeProject(params.Arguments)
	case "analyze_android":
		result = s.toolAnalyzeAndroid(params.Arguments)
	case "inspect_modules":
		result = s.toolInspectModules(params.Arguments)
	default:
		result = ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "unknown tool: " + params.Name}},
			IsError: true,
		}
	}
	s.sendResponse(req.ID, result, nil)
}

// handleResourcesList returns the list of available resources.
func (s *Server) handleResourcesList(req *MCPRequest) {
	result := ResourcesListResult{
		Resources: resourceDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handleResourcesRead returns the content of a resource.
func (s *Server) handleResourcesRead(req *MCPRequest) {
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
func (s *Server) handlePromptsList(req *MCPRequest) {
	result := PromptsListResult{
		Prompts: promptDefinitions(),
	}
	s.sendResponse(req.ID, result, nil)
}

// handlePromptsGet returns a prompt with its messages.
func (s *Server) handlePromptsGet(req *MCPRequest) {
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

// parseAndAnalyzeColumns parses Kotlin code and runs the dispatcher
// through pipeline.SingleFileAnalyzer, returning findings in columnar form.
func (s *Server) parseAndAnalyzeColumns(code string, path string) (scanner.FindingColumns, error) {
	columns, _, err := s.analyzer.AnalyzeBufferColumns(context.Background(), path, []byte(code))
	return columns, err
}

// parseAndAnalyze parses Kotlin code and runs the dispatcher, returning
// compatibility Finding values.
func (s *Server) parseAndAnalyze(code string, path string) ([]scanner.Finding, error) {
	columns, err := s.parseAndAnalyzeColumns(code, path)
	if err != nil {
		return nil, err
	}
	return columns.Findings(), nil
}

// parseKotlinCode parses Kotlin source code string into a scanner.File via
// the shared pipeline.ParseSingle helper.
func parseKotlinCode(code string, path string) (*scanner.File, error) {
	return pipeline.ParseSingle(context.Background(), path, []byte(code))
}

// exitFunc is os.Exit by default, but can be overridden for testing.
var exitFunc = osExit

func osExit(code int) {
	os.Exit(code)
}
