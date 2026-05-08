package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

var codeLensDecisionRe = regexp.MustCompile(`\b(if|else\s+if|when|for|while|catch)\b|&&|\|\||\?:`)

// handleCodeLens returns per-function inline metrics for the requested document.
func (s *Server) handleCodeLens(req *Request) {
	var params CodeLensParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("codeLens params error: %v", err)
		s.sendResponse(req.ID, []CodeLens{}, nil)
		return
	}

	uri := params.TextDocument.URI
	file, ok := s.getDocumentFlatFile(uri)
	if !ok || file == nil {
		s.sendResponse(req.ID, []CodeLens{}, nil)
		return
	}

	files := s.openParsedFiles()
	idx := scanner.BuildIndex(files, runtime.NumCPU())
	lenses := buildFunctionCodeLenses(uri, file, idx)
	s.sendResponse(req.ID, lenses, nil)
}

func (s *Server) openParsedFiles() []*scanner.File {
	s.docsMu.Lock()
	docs := make([]*Document, 0, len(s.docs))
	for _, doc := range s.docs {
		docs = append(docs, doc)
	}
	s.docsMu.Unlock()

	files := make([]*scanner.File, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if doc.File != nil && doc.File.FlatTree != nil {
			files = append(files, doc.File)
			continue
		}
		parsed, err := s.workspace.ParseFile(context.Background(), uriToPath(doc.URI), doc.Content)
		if err != nil {
			continue
		}
		s.docsMu.Lock()
		if current, ok := s.docs[doc.URI]; ok {
			current.File = parsed
		}
		s.docsMu.Unlock()
		files = append(files, parsed)
	}
	return files
}

func buildFunctionCodeLenses(uri string, file *scanner.File, idx *scanner.CodeIndex) []CodeLens {
	if file == nil || idx == nil {
		return []CodeLens{}
	}
	symbols := functionsInFile(idx, file.Path)
	lenses := make([]CodeLens, 0, len(symbols))
	for i, sym := range symbols {
		endLine := len(file.Lines) + 1
		if i+1 < len(symbols) && symbols[i+1].Line > sym.Line {
			endLine = symbols[i+1].Line
		}
		complexity := functionComplexity(file.Lines, sym.Line, endLine)
		consumers := consumerFileCount(idx, sym)
		title := fmt.Sprintf("complexity=%d | %d consumers", complexity, consumers)
		lenses = append(lenses, CodeLens{
			Range: Range{
				Start: Position{Line: uint32(maxInt(sym.Line-1, 0)), Character: 0},
				End:   Position{Line: uint32(maxInt(sym.Line-1, 0)), Character: 0},
			},
			Command: &Command{
				Title:   title,
				Command: "krit.showReferences",
				Arguments: []interface{}{
					map[string]interface{}{
						"uri":    uri,
						"symbol": symbolName(sym),
						"file":   filepath.ToSlash(file.Path),
						"line":   sym.Line,
					},
				},
			},
			Data: map[string]interface{}{
				"complexity": complexity,
				"consumers":  consumers,
				"symbol":     symbolName(sym),
			},
		})
	}
	return lenses
}

func functionsInFile(idx *scanner.CodeIndex, path string) []scanner.Symbol {
	out := make([]scanner.Symbol, 0)
	for _, sym := range idx.Symbols {
		if sym.File == path && sym.Kind == "function" {
			out = append(out, sym)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func consumerFileCount(idx *scanner.CodeIndex, sym scanner.Symbol) int {
	files := make(map[string]bool)
	for _, name := range []string{sym.Name, sym.FQN} {
		if name == "" {
			continue
		}
		for file := range idx.ReferenceFiles(name) {
			if file != sym.File {
				files[file] = true
			}
		}
	}
	return len(files)
}

func functionComplexity(lines []string, startLine, endLine int) int {
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= startLine || endLine > len(lines)+1 {
		endLine = len(lines) + 1
	}
	complexity := 1
	for _, line := range lines[startLine-1 : endLine-1] {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		complexity += len(codeLensDecisionRe.FindAllString(trimmed, -1))
	}
	return complexity
}

func symbolName(sym scanner.Symbol) string {
	if sym.FQN != "" {
		return sym.FQN
	}
	if sym.Owner != "" {
		return sym.Owner + "." + sym.Name
	}
	return sym.Name
}
