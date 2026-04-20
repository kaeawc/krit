package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/jsonrpc"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// debounceDelay is the duration to wait after a didChange before analyzing.
// Exported as a variable so tests can override it.
var debounceDelay = 100 * time.Millisecond

// Server implements a minimal LSP server over stdio using JSON-RPC 2.0.
type Server struct {
	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex // protects writes

	// LSP state
	initialized bool
	shutdown    bool
	rootURI     string
	configPath  string // explicit config path from initializationOptions

	// Document tracking
	docsMu sync.Mutex           // protects docs map
	docs   map[string]*Document // URI -> Document

	// Analysis: pipeline-driven single-file analyzer shared with MCP + CLI.
	analyzer *pipeline.SingleFileAnalyzer
	cfg      *config.Config

	// Verbose gates informational log output.
	Verbose bool
}

// logInfo logs an informational message gated behind s.Verbose.
func (s *Server) logInfo(format string, args ...interface{}) {
	if s.Verbose {
		log.Printf(format, args...)
	}
}

// Document tracks an open text document.
type Document struct {
	URI      string
	Content  []byte
	Version  int32
	Findings scanner.FindingColumns
	File     *scanner.File
	debounce *time.Timer // debounce timer for didChange
}

// NewServer creates a new LSP server reading from reader and writing to writer.
func NewServer(reader *bufio.Reader, writer io.Writer) *Server {
	return &Server{
		reader: reader,
		writer: writer,
		docs:   make(map[string]*Document),
	}
}

// Run reads and dispatches LSP messages until EOF or exit.
func (s *Server) Run() {
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

		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("invalid JSON-RPC message: %v", err)
			continue
		}

		s.handleMessage(&req)
	}
}

// sendResponse sends a JSON-RPC response via the shared transport.
func (s *Server) sendResponse(id interface{}, result interface{}, rpcErr *RPCError) {
	jsonrpc.SendResponse(s.writer, &s.mu, id, result, rpcErr)
}

// sendNotification sends a JSON-RPC notification (no ID).
func (s *Server) sendNotification(method string, params interface{}) {
	jsonrpc.SendNotification(s.writer, &s.mu, method, params)
}

// writeMessage serializes and writes a JSON-RPC message with Content-Length framing.
func (s *Server) writeMessage(msg interface{}) {
	jsonrpc.WriteMessage(s.writer, &s.mu, msg)
}

// handleMessage dispatches a JSON-RPC request or notification.
func (s *Server) handleMessage(req *Request) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		s.handleInitialized(req)
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		s.handleExit()
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "textDocument/codeAction":
		s.handleCodeAction(req)
	case "textDocument/codeLens":
		s.handleCodeLens(req)
	case "textDocument/formatting":
		s.handleFormatting(req)
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(req)
	case "textDocument/definition":
		s.handleDefinition(req)
	case "textDocument/references":
		s.handleReferences(req)
	case "textDocument/rename":
		s.handleRename(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	case "workspace/didChangeConfiguration":
		s.handleDidChangeConfiguration(req)
	default:
		if req.ID != nil {
			// Unknown request (not notification) -- return MethodNotFound
			s.sendResponse(req.ID, nil, &RPCError{
				Code:    -32601,
				Message: "method not found: " + req.Method,
			})
		}
		// Unknown notifications are silently ignored per LSP spec
	}
}

// handleInitialize responds with server capabilities.
func (s *Server) handleInitialize(req *Request) {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Printf("initialize params error: %v", err)
		}
	}

	s.rootURI = params.RootURI
	s.logInfo("initialize: rootURI=%s", s.rootURI)

	// Parse initializationOptions for config path
	if params.InitializationOptions != nil {
		var opts InitOptions
		if err := json.Unmarshal(params.InitializationOptions, &opts); err == nil && opts.ConfigPath != "" {
			s.configPath = opts.ConfigPath
			s.logInfo("initialize: configPath=%s", s.configPath)
		}
	}

	// Load configuration and build dispatcher
	s.loadConfigAndBuildDispatcher()

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // Full sync
			},
			// Server uses push model (publishDiagnostics notifications on
			// didOpen/didChange). Do not advertise DiagnosticProvider —
			// that would cause 3.17+ clients to pull via textDocument/diagnostic,
			// which this server does not implement.
			CodeActionProvider:         true,
			CodeLensProvider:           &CodeLensOptions{},
			DocumentFormattingProvider: true,
			HoverProvider:              true,
			DocumentSymbolProvider:     true,
			DefinitionProvider:         true,
			ReferencesProvider:         true,
			RenameProvider:             true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{"@", "\""},
			},
		},
		ServerInfo: &ServerInfo{
			Name:    "krit-lsp",
			Version: "0.0.1",
		},
	}

	s.sendResponse(req.ID, result, nil)
}

// handleInitialized is called after the client confirms initialization.
func (s *Server) handleInitialized(req *Request) {
	s.initialized = true
	s.logInfo("initialized: server ready")
}

// handleShutdown prepares for exit.
func (s *Server) handleShutdown(req *Request) {
	s.shutdown = true
	s.logInfo("shutdown requested")
	s.sendResponse(req.ID, nil, nil)
}

// handleExit terminates the server.
func (s *Server) handleExit() {
	s.logInfo("exit")
	if s.shutdown {
		exitFunc(0)
	} else {
		exitFunc(1)
	}
}

// exitFunc is os.Exit by default, but can be overridden for testing.
var exitFunc = osExit

func osExit(code int) {
	os.Exit(code)
}

// handleDidOpen processes a newly opened document.
func (s *Server) handleDidOpen(req *Request) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("didOpen params error: %v", err)
		return
	}

	uri := params.TextDocument.URI
	content := []byte(params.TextDocument.Text)

	s.docsMu.Lock()
	s.docs[uri] = &Document{
		URI:     uri,
		Content: content,
		Version: params.TextDocument.Version,
	}
	s.docsMu.Unlock()

	s.logInfo("didOpen: %s (version %d, %d bytes)", uri, params.TextDocument.Version, len(content))

	// didOpen fires analysis immediately (no debounce)
	s.analyzeAndPublish(uri, content)
}

// handleDidChange processes document changes (full sync) with debounce.
func (s *Server) handleDidChange(req *Request) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("didChange params error: %v", err)
		return
	}

	uri := params.TextDocument.URI

	// With full sync, the last content change contains the entire document
	if len(params.ContentChanges) == 0 {
		return
	}
	content := []byte(params.ContentChanges[len(params.ContentChanges)-1].Text)

	s.docsMu.Lock()
	doc, exists := s.docs[uri]
	if !exists {
		doc = &Document{URI: uri}
		s.docs[uri] = doc
	}
	doc.Content = content
	doc.Version = params.TextDocument.Version
	doc.File = nil

	// Cancel previous debounce timer if any
	if doc.debounce != nil {
		doc.debounce.Stop()
	}

	// Start debounce timer
	doc.debounce = time.AfterFunc(debounceDelay, func() {
		s.docsMu.Lock()
		currentDoc := s.docs[uri]
		s.docsMu.Unlock()
		if currentDoc != nil {
			s.analyzeAndPublish(uri, currentDoc.Content)
		}
	})
	s.docsMu.Unlock()

	s.logInfo("didChange: %s (version %d, debounced)", uri, params.TextDocument.Version)
}

// handleDidClose clears diagnostics for a closed document.
func (s *Server) handleDidClose(req *Request) {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("didClose params error: %v", err)
		return
	}

	uri := params.TextDocument.URI

	s.docsMu.Lock()
	if doc, ok := s.docs[uri]; ok {
		if doc.debounce != nil {
			doc.debounce.Stop()
		}
	}
	delete(s.docs, uri)
	s.docsMu.Unlock()

	s.logInfo("didClose: %s", uri)

	// Clear diagnostics for the closed file
	s.publishDiagnostics(uri, nil)
}

// loadConfigAndBuildDispatcher loads krit.yml from workspace root (or explicit path)
// and builds the rule dispatcher with the resulting active rules.
func (s *Server) loadConfigAndBuildDispatcher() {
	// Determine config path
	configPath := s.configPath
	if configPath == "" && s.rootURI != "" {
		rootPath := uriToPath(s.rootURI)
		// Check for krit.yml or .krit.yml in the workspace root
		for _, name := range []string{"krit.yml", ".krit.yml"} {
			candidate := filepath.Join(rootPath, name)
			if _, err := os.Stat(candidate); err == nil {
				configPath = candidate
				break
			}
		}
	}

	// Load config if found
	if configPath != "" {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Printf("config load error (%s): %v", configPath, err)
		} else {
			s.cfg = cfg
			rules.ApplyConfig(cfg)
			s.logInfo("config loaded: %s", configPath)
		}
	}

	// Delegate rule discovery and dispatcher construction to the
	// pipeline package so the LSP, MCP, and CLI entry points share one
	// source of truth for the active rule set. The SingleFileAnalyzer
	// wraps the pipeline Parse → Dispatch path for single-buffer
	// analysis (didOpen / didChange / formatting); cross-file and
	// module-aware rules are skipped because the LSP only sees one
	// buffer at a time.
	s.analyzer = pipeline.NewSingleFileAnalyzer(nil, nil)
	s.logInfo("dispatcher: %d active rules", len(s.analyzer.ActiveRules))
}

// handleDidChangeConfiguration reloads config and rebuilds the dispatcher.
func (s *Server) handleDidChangeConfiguration(req *Request) {
	s.logInfo("workspace/didChangeConfiguration: reloading config")
	s.loadConfigAndBuildDispatcher()

	// Re-analyze all open documents with the new config
	s.docsMu.Lock()
	uris := make([]string, 0, len(s.docs))
	contents := make([][]byte, 0, len(s.docs))
	for uri, doc := range s.docs {
		uris = append(uris, uri)
		contents = append(contents, doc.Content)
	}
	s.docsMu.Unlock()

	for i, uri := range uris {
		s.analyzeAndPublish(uri, contents[i])
	}
}

// getDocumentFlatFile returns the cached flat parse for a document, reparsing on demand if needed.
func (s *Server) getDocumentFlatFile(uri string) (*scanner.File, bool) {
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	if !ok {
		s.docsMu.Unlock()
		return nil, false
	}
	file := doc.File
	content := doc.Content
	s.docsMu.Unlock()

	if file != nil && file.FlatTree != nil {
		return file, true
	}

	parsed, err := pipeline.ParseSingle(context.Background(), uriToPath(uri), content)
	if err != nil {
		log.Printf("parse error for %s: %v", uri, err)
		return nil, false
	}

	s.docsMu.Lock()
	if doc, ok := s.docs[uri]; ok {
		doc.File = parsed
	}
	s.docsMu.Unlock()
	return parsed, true
}

// analyzeAndPublish parses the content and publishes diagnostics.
func (s *Server) analyzeAndPublish(uri string, content []byte) {
	if s.analyzer == nil {
		return
	}

	// Only analyze Kotlin files
	path := uriToPath(uri)
	if !strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".kts") {
		return
	}

	file, err := pipeline.ParseSingle(context.Background(), path, content)
	if err != nil {
		log.Printf("parse error for %s: %v", uri, err)
		s.publishDiagnostics(uri, nil)
		return
	}

	// Run per-file rules through the shared pipeline analyzer.
	columns := s.analyzer.AnalyzeFileColumns(file)

	// Store findings and cached tree alongside the document for code actions
	s.docsMu.Lock()
	if doc, ok := s.docs[uri]; ok {
		doc.Findings = columns
		doc.File = file
	}
	s.docsMu.Unlock()

	// Convert to LSP diagnostics
	diagnostics := FindingColumnsToDiagnostics(&columns)

	s.publishDiagnostics(uri, diagnostics)
}

// publishDiagnostics sends a textDocument/publishDiagnostics notification.
func (s *Server) publishDiagnostics(uri string, diagnostics []Diagnostic) {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

// handleCodeAction returns quick-fix code actions for findings with auto-fixes.
func (s *Server) handleCodeAction(req *Request) {
	var params CodeActionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("codeAction params error: %v", err)
		s.sendResponse(req.ID, []CodeAction{}, nil)
		return
	}

	uri := params.TextDocument.URI
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	var docContent string
	var docFindings scanner.FindingColumns
	if ok {
		docContent = string(doc.Content)
		docFindings = doc.Findings
	}
	s.docsMu.Unlock()
	if !ok {
		s.sendResponse(req.ID, []CodeAction{}, nil)
		return
	}

	actions := findingColumnsToCodeActions(uri, docContent, &docFindings)
	s.sendResponse(req.ID, actions, nil)
}

// findingColumnsToCodeActions converts fixable columnar findings to LSP code actions.
func findingColumnsToCodeActions(uri string, content string, columns *scanner.FindingColumns) []CodeAction {
	var actions []CodeAction
	if columns == nil {
		return []CodeAction{}
	}
	for row := 0; row < columns.Len(); row++ {
		fix := columns.FixAt(row)
		if fix == nil {
			continue
		}

		var startPos, endPos Position
		var newText string

		if fix.ByteMode {
			startPos = byteOffsetToPosition(content, fix.StartByte)
			endPos = byteOffsetToPosition(content, fix.EndByte)
			newText = fix.Replacement
		} else if fix.StartLine > 0 && fix.EndLine > 0 {
			lines := strings.Split(content, "\n")
			startLine := fix.StartLine - 1 // 0-based
			endLine := fix.EndLine         // exclusive
			if startLine < 0 {
				startLine = 0
			}
			if endLine > len(lines) {
				endLine = len(lines)
			}
			startPos = Position{Line: uint32(startLine), Character: 0}
			if endLine < len(lines) {
				endPos = Position{Line: uint32(endLine), Character: 0}
			} else {
				endPos = byteOffsetToPosition(content, len(content))
			}
			newText = fix.Replacement
		} else {
			continue
		}

		ruleName := columns.RuleAt(row)
		diag := rowToDiagnostic(columns, row)
		action := CodeAction{
			Title:       fmt.Sprintf("Fix: %s", ruleName),
			Kind:        "quickfix",
			Diagnostics: []Diagnostic{diag},
			Edit: &WorkspaceEdit{
				Changes: map[string][]TextEdit{
					uri: {
						{
							Range: Range{
								Start: startPos,
								End:   endPos,
							},
							NewText: newText,
						},
					},
				},
			},
		}
		if preview := buildQuickFixPreview(uri, content, ruleName, fix); preview != nil {
			action.Data = &CodeActionData{Preview: preview}
		}
		actions = append(actions, action)
	}
	if actions == nil {
		actions = []CodeAction{}
	}
	return actions
}

// handleFormatting applies krit auto-fixes to the document and returns text edits.
func (s *Server) handleFormatting(req *Request) {
	var params DocumentFormattingParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("formatting params error: %v", err)
		s.sendResponse(req.ID, []TextEdit{}, nil)
		return
	}

	uri := params.TextDocument.URI
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	var content []byte
	var columns scanner.FindingColumns
	if ok {
		content = doc.Content
		columns = doc.Findings
	}
	s.docsMu.Unlock()

	if !ok {
		s.sendResponse(req.ID, []TextEdit{}, nil)
		return
	}

	// If we don't have findings yet, run analysis now
	if columns.Len() == 0 && s.analyzer != nil {
		path := uriToPath(uri)
		if strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts") {
			file := doc.File
			if file == nil {
				var err error
				file, err = pipeline.ParseSingle(context.Background(), path, content)
				if err == nil {
					s.docsMu.Lock()
					if currentDoc, ok := s.docs[uri]; ok {
						currentDoc.File = file
					}
					s.docsMu.Unlock()
				}
			}
			if file != nil {
				columns = s.analyzer.AnalyzeFileColumns(file)
			}
		}
	}

	// Collect fixable findings and build text edits
	contentStr := string(content)
	var edits []TextEdit
	for row := 0; row < columns.Len(); row++ {
		fix := columns.FixAt(row)
		if fix == nil {
			continue
		}

		var startPos, endPos Position
		var newText string

		if fix.ByteMode {
			startPos = byteOffsetToPosition(contentStr, fix.StartByte)
			endPos = byteOffsetToPosition(contentStr, fix.EndByte)
			newText = fix.Replacement
		} else if fix.StartLine > 0 && fix.EndLine > 0 {
			lines := strings.Split(contentStr, "\n")
			startLine := fix.StartLine - 1
			endLine := fix.EndLine
			if startLine < 0 {
				startLine = 0
			}
			if endLine > len(lines) {
				endLine = len(lines)
			}
			startPos = Position{Line: uint32(startLine), Character: 0}
			if endLine < len(lines) {
				endPos = Position{Line: uint32(endLine), Character: 0}
			} else {
				endPos = byteOffsetToPosition(contentStr, len(contentStr))
			}
			newText = fix.Replacement
		} else {
			continue
		}

		edits = append(edits, TextEdit{
			Range: Range{
				Start: startPos,
				End:   endPos,
			},
			NewText: newText,
		})
	}

	if edits == nil {
		edits = []TextEdit{}
	}
	s.sendResponse(req.ID, edits, nil)
}

// handleHover returns rule information when hovering over a line with a finding.
func (s *Server) handleHover(req *Request) {
	var params HoverParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("hover params error: %v", err)
		s.sendResponse(req.ID, nil, nil)
		return
	}

	uri := params.TextDocument.URI
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	var columns scanner.FindingColumns
	if ok {
		columns = doc.Findings
	}
	s.docsMu.Unlock()

	if !ok || columns.Len() == 0 {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	// Find findings on the hovered line (LSP position is 0-based, findings are 1-based)
	hoverLine := int(params.Position.Line) + 1
	var matched []int
	for row := 0; row < columns.Len(); row++ {
		if columns.LineAt(row) == hoverLine {
			matched = append(matched, row)
		}
	}

	if len(matched) == 0 {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: formatHoverColumns(&columns, matched),
		},
	}
	s.sendResponse(req.ID, hover, nil)
}

// handleDocumentSymbol returns the symbol outline for a Kotlin file using the flat tree.
func (s *Server) handleDocumentSymbol(req *Request) {
	var params DocumentSymbolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("documentSymbol params error: %v", err)
		s.sendResponse(req.ID, []DocumentSymbol{}, nil)
		return
	}

	uri := params.TextDocument.URI
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	var file *scanner.File
	if ok {
		file = doc.File
	}
	s.docsMu.Unlock()
	if !ok || file == nil {
		s.sendResponse(req.ID, []DocumentSymbol{}, nil)
		return
	}

	symbols := extractSymbolsFlat(file)
	if symbols == nil {
		symbols = []DocumentSymbol{}
	}
	s.sendResponse(req.ID, symbols, nil)
}

// extractSymbolsFlat walks the flat tree and extracts class, function, and property declarations.
func extractSymbolsFlat(file *scanner.File) []DocumentSymbol {
	var symbols []DocumentSymbol
	if file == nil || file.FlatTree == nil {
		return nil
	}
	file.FlatForEachChild(0, func(child uint32) {
		sym := flatNodeToSymbol(file, child)
		if sym != nil {
			symbols = append(symbols, *sym)
		}
	})
	return symbols
}

// nodeToSymbol converts a tree-sitter node to a DocumentSymbol if it's a recognized declaration.
func flatNodeToSymbol(file *scanner.File, idx uint32) *DocumentSymbol {
	nodeType := file.FlatType(idx)
	var kind int
	switch nodeType {
	case "class_declaration", "object_declaration":
		kind = SymbolKindClass
	case "function_declaration":
		kind = SymbolKindFunction
	case "property_declaration":
		kind = SymbolKindProperty
	default:
		return nil
	}

	name := ""
	var nameNode uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "simple_identifier", "type_identifier":
			name = file.FlatNodeText(child)
			nameNode = child
		case "variable_declaration":
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" {
					name = file.FlatNodeText(gc)
					nameNode = gc
					break
				}
			}
		}
		if name != "" {
			break
		}
	}
	if name == "" {
		return nil
	}

	content := string(file.Content)
	nodeRange := Range{
		Start: byteOffsetToPosition(content, int(file.FlatStartByte(idx))),
		End:   byteOffsetToPosition(content, int(file.FlatEndByte(idx))),
	}
	selectionRange := Range{
		Start: byteOffsetToPosition(content, int(file.FlatStartByte(nameNode))),
		End:   byteOffsetToPosition(content, int(file.FlatEndByte(nameNode))),
	}

	sym := &DocumentSymbol{
		Name:           name,
		Kind:           kind,
		Range:          nodeRange,
		SelectionRange: selectionRange,
	}

	if kind == SymbolKindClass {
		file.FlatForEachChild(idx, func(child uint32) {
			if file.FlatType(child) == "class_body" || file.FlatType(child) == "enum_class_body" {
				sym.Children = extractSymbolsFlatFromNode(file, child)
			}
		})
	}

	return sym
}

func extractSymbolsFlatFromNode(file *scanner.File, idx uint32) []DocumentSymbol {
	var symbols []DocumentSymbol
	file.FlatForEachChild(idx, func(child uint32) {
		if sym := flatNodeToSymbol(file, child); sym != nil {
			symbols = append(symbols, *sym)
		}
	})
	return symbols
}

// byteOffsetToPosition converts a byte offset in content to an LSP Position (0-based line/character).
func byteOffsetToPosition(content string, offset int) Position {
	line := 0
	col := 0
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return Position{Line: uint32(line), Character: uint32(col)}
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		// Fallback: strip the file:// prefix
		return strings.TrimPrefix(uri, "file://")
	}
	if u.Scheme == "file" {
		return u.Path
	}
	return uri
}
