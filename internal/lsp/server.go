package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/jsonrpc"
	"github.com/kaeawc/krit/internal/logger"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// debounceDelay waits long enough to coalesce normal typing bursts while
// keeping diagnostics responsive after a brief pause. Exported as a variable so
// tests can override it.
var debounceDelay = 100 * time.Millisecond

// oracleDebounceDelay is the longer window used for oracle re-analysis,
// since re-running the oracle is heavier than re-running diagnostics.
var oracleDebounceDelay = 400 * time.Millisecond

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
	resolver typeinfer.TypeResolver
	cfg      *config.Config

	// workspace is the content-addressed parse cache shared across LSP
	// requests on the same buffer.
	workspace *pipeline.WorkspaceState

	// Verbose gates informational log output.
	Verbose bool

	// log routes lifecycle/error messages. NewServer sets a stderr
	// text-handler at Info level; SetLogger lets tests inject a
	// logger.Capture to assert on emitted records.
	log logger.Logger

	// oracleIdx points at the current oracle FQN index, when one has been
	// loaded. Reads use atomic.Pointer.Load so navigation handlers can
	// race with refreshes from didChange. nil means oracle is unavailable
	// — handlers fall back to the single-file textual walkers.
	oracleIdx         oracleIndexHolder
	indexMu           sync.Mutex
	indexer           WorkspaceIndexer
	userSetIndexer    bool
	indexCancel       context.CancelFunc
	indexReady        chan struct{}
	indexOnInitialize bool
	useOracleDaemon   bool
	workDoneProgress  bool
	initClasspath     []string
	oracleDaemon      *oracle.Daemon

	// jarCache decompiles JAR-resident declarations on demand. nil until
	// the first krit/jarContent request triggers lazy initialisation, so
	// servers that never see a JAR-scoped goto-def pay nothing.
	jarMu     sync.Mutex
	jarCache  *oracle.DecompileCache
	jarLookup JARLookup

	// OracleRefresh, when set, is invoked on a separate longer-debounced
	// timer after each didChange. Implementations send a per-file re-analyze
	// request to the krit-types daemon and apply the result to the FQN
	// reverse index. Must be set before Run() — there is no synchronization
	// against concurrent reassignment.
	OracleRefresh func(uri string, content []byte)
}

// logInfo logs an informational message gated behind s.Verbose. Preserves
// the printf-style call shape so existing call sites don't have to be
// restructured into key=value pairs.
func (s *Server) logInfo(format string, args ...interface{}) {
	if s.Verbose {
		s.log.Info(fmt.Sprintf(format, args...))
	}
}

// SetLogger overrides the default Logger. Intended for tests to inject
// logger.NewCapture so emitted records are observable.
func (s *Server) SetLogger(l logger.Logger) { s.log = l }

func (s *Server) SetWorkspaceIndexer(indexer WorkspaceIndexer) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.indexer = indexer
	s.userSetIndexer = true
}

// Document tracks an open text document.
type Document struct {
	URI            string
	Content        []byte
	Version        int32
	Findings       scanner.FindingColumns
	File           *scanner.File
	debounce       *time.Timer // debounce timer for diagnostic re-analysis
	oracleDebounce *time.Timer // debounce timer for oracle re-analysis
}

// NewServer creates a new LSP server reading from reader and writing to writer.
func NewServer(reader *bufio.Reader, writer io.Writer) *Server {
	return &Server{
		reader:            reader,
		writer:            writer,
		docs:              make(map[string]*Document),
		workspace:         pipeline.NewWorkspaceState(""),
		log:               logger.New(logger.Config{Format: logger.FormatText, Level: slog.LevelInfo}),
		indexOnInitialize: true,
		useOracleDaemon:   true,
		workDoneProgress:  true,
	}
}

// Run reads and dispatches LSP messages until EOF or exit.
func (s *Server) Run() {
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

// sendResponse sends a JSON-RPC response via the shared transport.
//
// Notifications (JSON-RPC 2.0 messages with no `id` member) MUST NOT receive
// a response. The dispatcher routes notifications through the same per-method
// handlers as requests, and several of those handlers unconditionally call
// sendResponse with req.ID — which is nil for notifications. Gating the
// emission here keeps every handler honest without each one needing to repeat
// the nil-id check, and avoids emitting a response with `"id":null` that
// strict LSP clients reject.
func (s *Server) sendResponse(id interface{}, result interface{}, rpcErr *RPCError) {
	if id == nil {
		return
	}
	jsonrpc.SendResponse(s.writer, &s.mu, id, result, rpcErr)
}

// sendNotification sends a JSON-RPC notification (no ID).
func (s *Server) sendNotification(method string, params interface{}) {
	jsonrpc.SendNotification(s.writer, &s.mu, method, params)
}

// handleMessage dispatches a JSON-RPC request or notification.
//
// Two protocol gates run before any per-method dispatch:
//
//   - Post-shutdown: once handleShutdown has set s.shutdown, the LSP spec
//     requires the server to respond to every request other than `exit` with
//     InvalidRequest (-32600). Notifications received after shutdown are
//     silently dropped.
//   - Pre-initialize: before the client's `initialize` request has been
//     answered, every method other than `initialize`, `initialized`, and
//     `exit` must be rejected with ServerNotInitialized (-32002). This
//     prevents document handlers (didOpen / codeAction / ...) from running
//     with a nil analyzer or unloaded config and matches the conformance
//     behaviour modern LSP clients assume.
//
// Both gates only emit an error response when req.ID is non-nil; notifications
// stay silent per JSON-RPC 2.0.
func (s *Server) handleMessage(req *Request) {
	if s.shutdown && req.Method != "exit" {
		if req.ID != nil {
			s.sendResponse(req.ID, nil, &RPCError{
				Code:    -32600,
				Message: "server is shutting down",
			})
		}
		return
	}
	if !s.initialized && !isInitializationExempt(req.Method) {
		if req.ID != nil {
			s.sendResponse(req.ID, nil, &RPCError{
				Code:    -32002,
				Message: "server not initialized",
			})
		}
		return
	}
	if s.handleLifecycleMessage(req) {
		return
	}
	if s.handleTextDocumentMessage(req) {
		return
	}
	if s.handleWorkspaceMessage(req) {
		return
	}
	if req.ID != nil {
		s.sendResponse(req.ID, nil, &RPCError{
			Code:    -32601,
			Message: "method not found: " + req.Method,
		})
	}
}

// isInitializationExempt reports whether method may be processed before the
// client has sent `initialized`. The LSP spec allows only initialize, the
// initialized notification itself, and exit; everything else must be rejected
// with ServerNotInitialized.
func isInitializationExempt(method string) bool {
	switch method {
	case "initialize", "initialized", "exit":
		return true
	default:
		return false
	}
}

func (s *Server) handleLifecycleMessage(req *Request) bool {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		s.handleInitialized(req)
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		s.handleExit()
	default:
		return false
	}
	return true
}

func (s *Server) handleTextDocumentMessage(req *Request) bool {
	switch req.Method {
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
	default:
		return false
	}
	return true
}

func (s *Server) handleWorkspaceMessage(req *Request) bool {
	switch req.Method {
	case "workspace/executeCommand":
		s.handleExecuteCommand(req)
	case "workspace/didChangeConfiguration":
		s.handleDidChangeConfiguration(req)
	case "krit/jarContent":
		s.handleJARContent(req)
	default:
		return false
	}
	return true
}

// handleInitialize responds with server capabilities.
func (s *Server) handleInitialize(req *Request) {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.log.Warn("initialize params error", "err", err)
		}
	}

	s.rootURI = params.RootURI
	s.logInfo("initialize: rootURI=%s", s.rootURI)
	if params.Capabilities.Window != nil {
		s.workDoneProgress = params.Capabilities.Window.WorkDoneProgress
	}

	// Parse initializationOptions for config path and workspace indexing.
	s.indexOnInitialize = true
	if params.InitializationOptions != nil {
		var opts InitOptions
		if err := json.Unmarshal(params.InitializationOptions, &opts); err == nil {
			if opts.ConfigPath != "" {
				s.configPath = opts.ConfigPath
				s.logInfo("initialize: configPath=%s", s.configPath)
			}
			if opts.IndexOnInitialize != nil {
				s.indexOnInitialize = *opts.IndexOnInitialize
			}
			if len(opts.Classpath) > 0 {
				s.initClasspath = append([]string(nil), opts.Classpath...)
			}
			if opts.UseOracleDaemon != nil {
				s.useOracleDaemon = *opts.UseOracleDaemon
			}
		}
	}

	// Load configuration and build dispatcher
	s.loadConfigAndBuildDispatcher()
	s.configureWorkspaceIndexer()

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
	s.startWorkspaceIndex(params)
}

// handleInitialized is called after the client confirms initialization.
func (s *Server) handleInitialized(_ *Request) {
	s.initialized = true
	s.logInfo("initialized: server ready")
}

// handleShutdown prepares for exit.
func (s *Server) handleShutdown(req *Request) {
	s.shutdown = true
	s.cancelWorkspaceIndex()
	s.releaseOracleDaemon()
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
		s.log.Warn("didOpen params error", "err", err)
		return
	}

	uri := params.TextDocument.URI
	if IsJARURI(uri) {
		s.handleJARDidOpen(params)
		return
	}
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
		s.log.Warn("didChange params error", "err", err)
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

	// Navigation queries during this window read the pre-edit index — stale
	// by up to oracleDebounceDelay, acceptable because the user just typed.
	if s.OracleRefresh != nil {
		if doc.oracleDebounce != nil {
			doc.oracleDebounce.Stop()
		}
		doc.oracleDebounce = time.AfterFunc(oracleDebounceDelay, func() {
			s.docsMu.Lock()
			currentDoc := s.docs[uri]
			var content []byte
			if currentDoc != nil {
				content = currentDoc.Content
			}
			s.docsMu.Unlock()
			if currentDoc != nil {
				s.OracleRefresh(uri, content)
			}
		})
	}
	s.docsMu.Unlock()

	s.logInfo("didChange: %s (version %d, debounced)", uri, params.TextDocument.Version)
}

// handleDidClose clears diagnostics for a closed document.
func (s *Server) handleDidClose(req *Request) {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Warn("didClose params error", "err", err)
		return
	}

	uri := params.TextDocument.URI

	s.docsMu.Lock()
	if doc, ok := s.docs[uri]; ok {
		if doc.debounce != nil {
			doc.debounce.Stop()
		}
		if doc.oracleDebounce != nil {
			doc.oracleDebounce.Stop()
		}
	}
	delete(s.docs, uri)
	s.docsMu.Unlock()

	// Free the cached parse for this URI; the buffer is no longer
	// open and a future re-open will hit a clean cache.
	s.workspace.Invalidate(uriToPath(uri))

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
		for _, name := range config.Filenames {
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
			s.log.Warn("config load error", "path", configPath, "err", err)
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
	// Wire up a source-level type resolver so type-aware rules
	// (NeedsResolver capability) receive a non-nil ctx.Resolver via the
	// dispatcher. The LSP only sees one buffer at a time, so
	// analyzeAndPublish re-indexes each parsed file into this resolver
	// before dispatch. Without this wiring, type-aware rules silently
	// degrade to their fallback heuristics — matching the CLI behaviour
	// required by the TypeResolutionService acceptance criteria.
	s.resolver = typeinfer.NewResolver()
	s.analyzer = pipeline.NewSingleFileAnalyzer(nil, s.resolver)
	s.logInfo("dispatcher: %d active rules", len(s.analyzer.ActiveRules))
}

// handleDidChangeConfiguration reloads config and rebuilds the dispatcher.
func (s *Server) handleDidChangeConfiguration(_ *Request) {
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

	parsed, err := s.workspace.ParseFile(context.Background(), uriToPath(uri), content)
	if err != nil {
		s.log.Warn("parse error", "uri", uri, "err", err)
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

	file, err := s.workspace.ParseFile(context.Background(), path, content)
	if err != nil {
		s.log.Warn("parse error", "uri", uri, "err", err)
		s.publishDiagnostics(uri, nil)
		return
	}

	// Index the current buffer into the type resolver so rules
	// declaring NeedsResolver see up-to-date scope / class / import
	// data. Single-file indexing is cheap; full-workspace indexing is
	// intentionally skipped because the LSP only holds one buffer at a
	// time.
	if indexer, ok := s.resolver.(interface {
		IndexFilesParallel([]*scanner.File, int)
	}); ok {
		indexer.IndexFilesParallel([]*scanner.File{file}, 1)
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
		s.log.Warn("codeAction params error", "err", err)
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
	actions = append(actions, s.goToTestCodeActions(uri, params.Range.Start)...)
	s.sendResponse(req.ID, actions, nil)
}

func (s *Server) handleExecuteCommand(req *Request) {
	var params ExecuteCommandParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Warn("executeCommand params error", "err", err)
		s.sendResponse(req.ID, nil, nil)
		return
	}
	if params.Command != "krit.goToTest" || len(params.Arguments) == 0 {
		s.sendResponse(req.ID, nil, nil)
		return
	}
	var loc Location
	if err := json.Unmarshal(params.Arguments[0], &loc); err != nil || loc.URI == "" {
		s.sendResponse(req.ID, nil, nil)
		return
	}
	s.sendResponse(req.ID, loc, nil)
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

		startPos, endPos, newText, ok := resolveFixPositions(content, fix)
		if !ok {
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
			action.Description = fmt.Sprintf("```diff\n%s```", preview.Diff)
		}
		actions = append(actions, action)
	}
	if actions == nil {
		actions = []CodeAction{}
	}
	return actions
}

// resolveFixPositions converts a fix's coordinates to LSP start/end positions
// and replacement text. Returns ok=false when the fix has no usable coordinates.
//
// Rules emit Replacement strings with bare "\n" line terminators. When the
// underlying document uses CRLF (common on Windows checkouts), returning the
// LF text verbatim writes a mixed-ending file the next time the client
// applies the edit — the same class of corruption fixed in
// internal/fixer/fixer.go via detectLineEnding. Rewrite the replacement to
// match the document's existing convention so TextEdit.newText stays
// internally consistent with the lines it surrounds.
func resolveFixPositions(contentStr string, fix *scanner.Fix) (startPos, endPos Position, newText string, ok bool) {
	sep := detectDocumentLineEnding(contentStr)
	if fix.ByteMode {
		return byteOffsetToPosition(contentStr, fix.StartByte),
			byteOffsetToPosition(contentStr, fix.EndByte),
			normalizeLineEndings(fix.Replacement, sep),
			true
	}
	if fix.StartLine > 0 && fix.EndLine > 0 {
		lines := strings.Split(contentStr, "\n")
		startLine := fix.StartLine - 1
		if startLine < 0 {
			startLine = 0
		}
		endLine := fix.EndLine
		if endLine > len(lines) {
			endLine = len(lines)
		}
		start := Position{Line: uint32(startLine), Character: 0}
		var end Position
		if endLine < len(lines) {
			end = Position{Line: uint32(endLine), Character: 0}
		} else {
			end = byteOffsetToPosition(contentStr, len(contentStr))
		}
		return start, end, normalizeLineEndings(fix.Replacement, sep), true
	}
	return Position{}, Position{}, "", false
}

// detectDocumentLineEnding mirrors internal/fixer.detectLineEnding: returns
// "\r\n" when the first newline in the document is preceded by a carriage
// return, otherwise "\n". The fixer helper is unexported and lives in a
// different package, so we keep this LSP-local copy rather than widening the
// fixer API just to share one branch.
func detectDocumentLineEnding(s string) string {
	i := strings.IndexByte(s, '\n')
	if i > 0 && s[i-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

// normalizeLineEndings rewrites replacement to use sep as its line terminator.
// Replacement strings come from rule code with bare "\n"; on CRLF documents
// the dispatcher would otherwise emit LF-only edits that corrupt the buffer
// when applied. Strips any stray trailing "\r" left on each split line so the
// rejoined output is uniform rather than mixed.
func normalizeLineEndings(replacement, sep string) string {
	if sep == "\n" || !strings.Contains(replacement, "\n") {
		return replacement
	}
	parts := strings.Split(replacement, "\n")
	for i, p := range parts {
		parts[i] = strings.TrimSuffix(p, "\r")
	}
	return strings.Join(parts, sep)
}

// formattingColumns returns the cached or freshly-computed findings for a
// Kotlin document, along with its content. ok is false when the document is
// not tracked.
func (s *Server) formattingColumns(uri string) (content []byte, columns scanner.FindingColumns, ok bool) {
	s.docsMu.Lock()
	doc, exists := s.docs[uri]
	if exists {
		content = doc.Content
		columns = doc.Findings
	}
	s.docsMu.Unlock()

	if !exists {
		return nil, scanner.FindingColumns{}, false
	}

	if columns.Len() == 0 && s.analyzer != nil {
		path := uriToPath(uri)
		if strings.HasSuffix(path, ".kt") || strings.HasSuffix(path, ".kts") {
			file := doc.File
			if file == nil {
				var err error
				file, err = s.workspace.ParseFile(context.Background(), path, content)
				if err == nil {
					s.docsMu.Lock()
					if currentDoc, ok2 := s.docs[uri]; ok2 {
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
	return content, columns, true
}

// handleFormatting applies krit auto-fixes to the document and returns text edits.
func (s *Server) handleFormatting(req *Request) {
	var params DocumentFormattingParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Warn("formatting params error", "err", err)
		s.sendResponse(req.ID, []TextEdit{}, nil)
		return
	}

	uri := params.TextDocument.URI
	content, columns, ok := s.formattingColumns(uri)
	if !ok {
		s.sendResponse(req.ID, []TextEdit{}, nil)
		return
	}

	contentStr := string(content)
	var edits []TextEdit
	for row := 0; row < columns.Len(); row++ {
		fix := columns.FixAt(row)
		if fix == nil {
			continue
		}
		startPos, endPos, newText, resolved := resolveFixPositions(contentStr, fix)
		if !resolved {
			continue
		}
		edits = append(edits, TextEdit{
			Range:   Range{Start: startPos, End: endPos},
			NewText: newText,
		})
	}

	if edits == nil {
		edits = []TextEdit{}
	}
	s.sendResponse(req.ID, edits, nil)
}

// handleHover composes hover markdown that combines krit rule findings on
// the hovered line with type and declaration metadata pulled from the oracle
// index. When the oracle is unavailable the hover degrades to rule findings
// only — the same behaviour as before milestone 5.
func (s *Server) handleHover(req *Request) {
	var params HoverParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Warn("hover params error", "err", err)
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
	if !ok {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	var sections []string
	matched := hoverFindingRows(&columns, doc.Content, params.Position)
	if len(matched) > 0 {
		sections = append(sections, formatHoverColumns(&columns, matched, s.cfg))
	}

	if section := s.oracleHoverSection(uri, params.Position); section != "" {
		sections = append(sections, section)
	}

	if len(sections) == 0 {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: strings.Join(sections, "\n\n---\n\n"),
		},
	}
	s.sendResponse(req.ID, hover, nil)
}

func hoverFindingRows(columns *scanner.FindingColumns, content []byte, pos Position) []int {
	if columns == nil {
		return nil
	}
	hoverOffset := positionToByteOffset(content, pos)
	hoverLine := int(pos.Line) + 1
	var matched []int
	for row := 0; row < columns.Len(); row++ {
		startByte := columns.StartByteAt(row)
		endByte := columns.EndByteAt(row)
		if endByte > startByte {
			if hoverOffset >= startByte && hoverOffset < endByte {
				matched = append(matched, row)
			}
			continue
		}
		if columns.LineAt(row) == hoverLine {
			matched = append(matched, row)
		}
	}
	return matched
}

func positionToByteOffset(content []byte, pos Position) int {
	line := uint32(0)
	col := uint32(0)
	for i, b := range content {
		if line == pos.Line && col == pos.Character {
			return i
		}
		if b == '\n' {
			if line == pos.Line {
				return i
			}
			line++
			col = 0
			continue
		}
		col++
	}
	return len(content)
}

// handleDocumentSymbol returns the symbol outline for a Kotlin file using the flat tree.
func (s *Server) handleDocumentSymbol(req *Request) {
	var params DocumentSymbolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.log.Warn("documentSymbol params error", "err", err)
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

// pathToURI converts a filesystem path to a file:// URI. Paths that already
// look like a URI are returned unchanged.
func pathToURI(path string) string {
	if strings.HasPrefix(path, "file://") || strings.Contains(path, "://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return "file://" + path
	}
	return "file:///" + path
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
