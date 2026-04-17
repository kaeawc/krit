package lsp

import (
	"encoding/json"
	"log"
)

// handleCodeLens wires up textDocument/codeLens so clients can discover the
// capability before function-level metric lenses are implemented.
func (s *Server) handleCodeLens(req *Request) {
	var params CodeLensParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("codeLens params error: %v", err)
		s.sendResponse(req.ID, []CodeLens{}, nil)
		return
	}

	s.sendResponse(req.ID, []CodeLens{}, nil)
}
