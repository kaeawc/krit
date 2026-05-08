package lsp

import (
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

// oracleIndex returns the current oracle FQN index, or nil if none has been
// loaded yet.
func (s *Server) oracleIndex() *oracle.Index {
	if s == nil {
		return nil
	}
	return s.oracleIdx.Load()
}

// SetOracleIndex installs a new index. Tests use this directly; production
// callers will route through the workspace-init path in a later milestone.
func (s *Server) SetOracleIndex(idx *oracle.Index) {
	s.oracleIdx.Store(idx)
}

// oracleHoverSection returns the markdown block describing the symbol at pos
// using the oracle index, or "" when the index is unavailable, the document
// is not parsed, or no identifier sits under the cursor.
func (s *Server) oracleHoverSection(uri string, pos Position) string {
	idx := s.waitForOracleIndex(500 * time.Millisecond)
	if idx == nil {
		return ""
	}
	file, ok := s.getDocumentFlatFile(uri)
	if !ok {
		return ""
	}
	name := identifierAtPositionFlat(file, pos)
	if name == "" {
		return ""
	}
	return formatSymbolHover(idx, name)
}

// oracleIndexHolder wraps atomic.Pointer[oracle.Index] so it can live on the
// Server struct without requiring callers to spell out the generic type.
type oracleIndexHolder = atomic.Pointer[oracle.Index]
