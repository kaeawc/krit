package lsp

import (
	"encoding/json"

	"github.com/kaeawc/krit/internal/jsonrpc"
)

// JSON-RPC 2.0 types — aliases to the shared jsonrpc package.
type (
	Request      = jsonrpc.Request
	Response     = jsonrpc.Response
	Notification = jsonrpc.Notification
	RPCError     = jsonrpc.Error
)

// LSP Initialize types

// InitializeParams contains the parameters for the initialize request.
type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	RootURI               string             `json:"rootUri,omitempty"`
	RootPath              string             `json:"rootPath,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions json.RawMessage    `json:"initializationOptions,omitempty"`
}

// InitOptions contains options passed from the client during initialization.
type InitOptions struct {
	ConfigPath string `json:"configPath,omitempty"`
}

// DidChangeConfigurationParams contains the parameters for workspace/didChangeConfiguration.
type DidChangeConfigurationParams struct {
	Settings json.RawMessage `json:"settings"`
}

// ClientCapabilities describes the client's capabilities (minimal subset).
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities describes textDocument capabilities.
type TextDocumentClientCapabilities struct {
	PublishDiagnostics *PublishDiagnosticsCapability `json:"publishDiagnostics,omitempty"`
}

// PublishDiagnosticsCapability describes publishDiagnostics capabilities.
type PublishDiagnosticsCapability struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

// InitializeResult is the response to the initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

// ServerCapabilities describes the server's capabilities.
type ServerCapabilities struct {
	TextDocumentSync           *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	DiagnosticProvider         *DiagnosticOptions       `json:"diagnosticProvider,omitempty"`
	CodeActionProvider         bool                     `json:"codeActionProvider,omitempty"`
	CodeLensProvider           *CodeLensOptions         `json:"codeLensProvider,omitempty"`
	DocumentFormattingProvider bool                     `json:"documentFormattingProvider,omitempty"`
	HoverProvider              bool                     `json:"hoverProvider,omitempty"`
	DocumentSymbolProvider     bool                     `json:"documentSymbolProvider,omitempty"`
	DefinitionProvider         bool                     `json:"definitionProvider,omitempty"`
	ReferencesProvider         bool                     `json:"referencesProvider,omitempty"`
	RenameProvider             bool                     `json:"renameProvider,omitempty"`
	CompletionProvider         *CompletionOptions       `json:"completionProvider,omitempty"`
}

// TextDocumentSyncOptions describes how text documents are synced.
type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	// Change is the sync kind: 0=None, 1=Full, 2=Incremental.
	Change int `json:"change"`
}

// DiagnosticOptions describes diagnostic provider capabilities.
type DiagnosticOptions struct {
	InterFileDependencies bool   `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool   `json:"workspaceDiagnostics"`
	Identifier            string `json:"identifier,omitempty"`
}

// ServerInfo contains information about the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// LSP text document types

// DidOpenTextDocumentParams contains the parameters for didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentItem is an item to transfer a text document from client to server.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int32  `json:"version"`
	Text       string `json:"text"`
}

// DidChangeTextDocumentParams contains the parameters for didChange.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// VersionedTextDocumentIdentifier identifies a specific version of a text document.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int32  `json:"version"`
}

// TextDocumentContentChangeEvent describes a change to a text document.
// With full sync, RangeLength and Range are nil and Text contains the full content.
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

// DidCloseTextDocumentParams contains the parameters for didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// LSP Diagnostic types

// PublishDiagnosticsParams contains the parameters for publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic represents a diagnostic, such as a compiler error or warning.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"` // 1=Error, 2=Warning, 3=Info, 4=Hint
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// Range represents a range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a text document (0-based line and character).
type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// LSP Code Action types

// CodeActionParams contains the parameters for the textDocument/codeAction request.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeActionContext contains the diagnostics the code action was invoked for.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// CodeAction represents a code action (quick fix, refactor, etc.).
type CodeAction struct {
	Title       string          `json:"title"`
	Kind        string          `json:"kind"` // "quickfix"
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit  `json:"edit,omitempty"`
	Data        *CodeActionData `json:"data,omitempty"`
}

// CodeActionData carries extra client-facing metadata for a code action.
type CodeActionData struct {
	Preview *CodeActionPreview `json:"preview,omitempty"`
}

// CodeActionPreview describes a proposed fix preview payload.
type CodeActionPreview struct {
	FixLevel string `json:"fixLevel"`
	Diff     string `json:"diff"`
}

// CodeLensParams contains the parameters for the textDocument/codeLens request.
type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// CodeLens represents inline metadata shown above source ranges.
type CodeLens struct {
	Range   Range    `json:"range"`
	Command *Command `json:"command,omitempty"`
	Data    any      `json:"data,omitempty"`
}

// CodeLensOptions describes code lens provider capabilities.
type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// WorkspaceEdit represents changes to multiple resources.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

// TextEdit represents a text edit applicable to a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// Command represents an executable editor command attached to a code lens.
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// DocumentFormattingParams contains the parameters for textDocument/formatting.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

// FormattingOptions describes formatting options.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// HoverParams contains the parameters for textDocument/hover.
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Hover is the result of a hover request.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent represents a string value with a kind (markdown or plaintext).
type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown" or "plaintext"
	Value string `json:"value"`
}

// DocumentSymbolParams contains the parameters for textDocument/documentSymbol.
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DocumentSymbol represents a symbol in a document (class, function, property, etc.).
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolKind constants per LSP specification.
const (
	SymbolKindClass    = 5
	SymbolKindFunction = 12
	SymbolKindProperty = 7
)

// LSP Definition types

// DefinitionParams contains the parameters for textDocument/definition.
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Location represents a location inside a resource.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LSP References types

// ReferenceParams contains the parameters for textDocument/references.
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

// ReferenceContext contains additional information about the context of a reference request.
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// LSP Rename types

// RenameParams contains the parameters for textDocument/rename.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// LSP Completion types

// CompletionParams contains the parameters for textDocument/completion.
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"` // 1=Text, 6=Variable, 14=Keyword
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// CompletionList represents a collection of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionOptions describes completion provider capabilities.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CompletionItemKind constants per LSP specification.
const (
	CompletionKindText    = 1
	CompletionKindKeyword = 14
	CompletionKindClass   = 7
)
