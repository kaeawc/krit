package lsp

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/oracle"
)

// JARContentParams is the request payload for the krit/jarContent custom
// LSP method. Clients call this when a navigation result returns a
// krit-jar:// URI and the client needs the document body to render.
type JARContentParams struct {
	URI string `json:"uri"`
}

// JARContentResult is the response for krit/jarContent. Languages other
// than Kotlin are unsupported; the field is included so future expansions
// (Java decompile, e.g.) don't require a protocol bump.
type JARContentResult struct {
	URI      string `json:"uri"`
	Language string `json:"languageId"`
	Text     string `json:"text"`
}

// JARLookup is the optional bridge from the LSP server into oracle
// declaration data. When unset, krit/jarContent falls back to an
// unresolved-classpath placeholder. Wiring this up is part of the
// navigation-handler oracle integration milestone — providing the seam
// here lets that work proceed without changing the protocol.
type JARLookup func(fqn string) *oracle.Class

// SetJARLookup installs (or replaces) the lookup. Safe to call before or
// after the first krit/jarContent request.
func (s *Server) SetJARLookup(fn JARLookup) {
	s.jarMu.Lock()
	defer s.jarMu.Unlock()
	s.jarLookup = fn
	// Reset the cache so a previously-installed signature stub renderer
	// doesn't keep returning the old placeholder.
	s.jarCache = nil
}

// jarCacheLocked returns the active cache, creating it on first use. Caller
// must hold s.jarMu.
func (s *Server) jarCacheLocked() *oracle.DecompileCache {
	if s.jarCache != nil {
		return s.jarCache
	}
	root := filepath.Join(s.jarCacheRoot(), "stub")
	s.jarCache = oracle.NewDecompileCache(root, s.jarDecompilerLocked(nil))
	return s.jarCache
}

func (s *Server) installDaemonDecompiler(d *oracle.Daemon) {
	s.jarMu.Lock()
	defer s.jarMu.Unlock()
	s.jarCache = oracle.NewDecompileCache(filepath.Join(s.jarCacheRoot(), "kaa"), s.jarDecompilerLocked(d))
}

func (s *Server) jarDecompilerLocked(d *oracle.Daemon) oracle.Decompiler {
	stub := &oracle.SignatureStubDecompiler{Lookup: func(fqn string) *oracle.Class {
		if s.jarLookup == nil {
			return nil
		}
		return s.jarLookup(fqn)
	}}
	if d == nil {
		return stub
	}
	return oracle.FallbackDecompiler{
		Primary:  oracle.DaemonDecompiler{Daemon: d},
		Fallback: stub,
	}
}

// jarCacheRoot resolves where decompile output is persisted on disk. When
// no project root has been advertised by the client we fall back to the
// system temp dir so tests don't litter the working tree.
func (s *Server) jarCacheRoot() string {
	if root := uriToPath(s.rootURI); root != "" {
		return filepath.Join(root, ".krit", "jar-decompile")
	}
	return filepath.Join(os.TempDir(), "krit-jar-decompile")
}

func (s *Server) handleJARContent(req *Request) {
	var params JARContentParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("jarContent params error: %v", err)
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "invalid params"})
		return
	}
	ref, err := ParseJARURI(params.URI)
	if err != nil {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: err.Error()})
		return
	}

	s.jarMu.Lock()
	cache := s.jarCacheLocked()
	s.jarMu.Unlock()

	text, err := cache.Get(ref.JARPath, ref.FQN)
	if err != nil {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32603, Message: err.Error()})
		return
	}
	if cache.JARMissing(ref.JARPath) {
		s.logInfo("jarContent: jar not found on disk: %s", ref.JARPath)
	}
	s.sendResponse(req.ID, JARContentResult{
		URI:      params.URI,
		Language: "kotlin",
		Text:     text,
	}, nil)
}

func (s *Server) handleJARDidOpen(params DidOpenTextDocumentParams) {
	uri := params.TextDocument.URI
	text, err := s.jarText(uri)
	if err != nil {
		s.log.Warn("jar didOpen failed", "uri", uri, "err", err)
		text = params.TextDocument.Text
	}

	s.docsMu.Lock()
	s.docs[uri] = &Document{
		URI:     uri,
		Content: []byte(text),
		Version: params.TextDocument.Version,
	}
	s.docsMu.Unlock()

	s.logInfo("didOpen jar: %s (version %d, %d bytes)", uri, params.TextDocument.Version, len(text))
}

func (s *Server) jarText(uri string) (string, error) {
	ref, err := ParseJARURI(uri)
	if err != nil {
		return "", err
	}
	s.jarMu.Lock()
	cache := s.jarCacheLocked()
	s.jarMu.Unlock()
	return cache.Get(ref.JARPath, ref.FQN)
}
