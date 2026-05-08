package mcp

import (
	"encoding/json"
	"runtime"
	"sync"

	"github.com/kaeawc/krit/internal/scanner"
)

// symbolsArgs are the arguments for the symbols tool.
type symbolsArgs struct {
	Operation string `json:"operation"`

	// operation=outline
	Code string `json:"code"`
	Path string `json:"path"`

	// operation=references
	Name         string   `json:"name"`
	ProjectPaths []string `json:"project_paths"`
	IncludeJava  bool     `json:"include_java"`
	IncludeXML   bool     `json:"include_xml"`
}

func (s *Server) toolSymbols(arguments json.RawMessage) ToolResult {
	var args symbolsArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	op := args.Operation
	if op == "" {
		op = opSymbolsReferences
	}

	switch op {
	case opSymbolsOutline:
		return s.symbolsOutline(args)
	case opSymbolsReferences:
		return s.symbolsReferences(args)
	default:
		return errorResult("unknown operation: " + op + "; valid: " + formatList(symbolsOperations))
	}
}

// symbolsOutline returns the declarations in a parsed file.
func (s *Server) symbolsOutline(args symbolsArgs) ToolResult {
	if args.Code == "" {
		return errorResult("'code' argument is required for operation=outline")
	}

	path := args.Path
	if path == "" {
		path = "input.kt"
	}

	file, err := parseKotlinCode(args.Code, path)
	if err != nil {
		return errorResult(err.Error())
	}

	index := scanner.BuildIndex([]*scanner.File{file}, 1)

	type symbolJSON struct {
		Name       string `json:"name"`
		Kind       string `json:"kind"`
		Line       int    `json:"line"`
		Visibility string `json:"visibility,omitempty"`
		Owner      string `json:"owner,omitempty"`
		FQN        string `json:"fqn,omitempty"`
		Signature  string `json:"signature,omitempty"`
	}

	syms := make([]symbolJSON, 0, len(index.Symbols))
	for _, sym := range index.Symbols {
		syms = append(syms, symbolJSON{
			Name:       sym.Name,
			Kind:       sym.Kind,
			Line:       sym.Line,
			Visibility: sym.Visibility,
			Owner:      sym.Owner,
			FQN:        sym.FQN,
			Signature:  sym.Signature,
		})
	}

	type outlineResult struct {
		Path    string       `json:"path"`
		Total   int          `json:"total"`
		Symbols []symbolJSON `json:"symbols"`
	}

	return jsonResult(outlineResult{
		Path:    path,
		Total:   len(syms),
		Symbols: syms,
	})
}

// symbolsReferences searches for symbol references across project files.
func (s *Server) symbolsReferences(args symbolsArgs) ToolResult {
	if args.Name == "" {
		return errorResult("'name' argument is required for operation=references")
	}
	if len(args.ProjectPaths) == 0 {
		return errorResult("'project_paths' argument is required for operation=references")
	}

	var filePaths []string
	for _, p := range args.ProjectPaths {
		ktFiles, err := scanner.CollectKotlinFiles([]string{p}, nil)
		if err != nil {
			return errorResult("collecting Kotlin files: " + err.Error())
		}
		filePaths = append(filePaths, ktFiles...)

		if args.IncludeJava {
			javaFiles, err := scanner.CollectJavaFiles([]string{p}, nil)
			if err != nil {
				return errorResult("collecting Java files: " + err.Error())
			}
			filePaths = append(filePaths, javaFiles...)
		}

		if args.IncludeXML {
			xmlFiles, err := collectXMLFiles(p)
			if err != nil {
				return errorResult("collecting XML files: " + err.Error())
			}
			filePaths = append(filePaths, xmlFiles...)
		}
	}

	refs := scanFilesForSymbolParallel(filePaths, args.Name)
	if refs == nil {
		refs = []refMatch{}
	}
	return jsonResult(refs)
}

// scanFilesForSymbolParallel fans the per-file substring grep across worker
// goroutines. Each match list is independent; results are concatenated in
// the input file order via per-worker chunks.
func scanFilesForSymbolParallel(paths []string, name string) []refMatch {
	if len(paths) == 0 {
		return nil
	}
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers > len(paths) {
		workers = len(paths)
	}

	type chunk struct {
		idx     int
		matches []refMatch
	}

	jobs := make(chan int, len(paths))
	results := make(chan chunk, len(paths))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				m, err := searchFileForSymbol(paths[i], name)
				if err != nil {
					continue
				}
				if len(m) > 0 {
					results <- chunk{idx: i, matches: m}
				}
			}
		}()
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	chunks := make([]chunk, 0)
	for c := range results {
		chunks = append(chunks, c)
	}
	// Stable file order
	for i := 1; i < len(chunks); i++ {
		for j := i; j > 0 && chunks[j].idx < chunks[j-1].idx; j-- {
			chunks[j], chunks[j-1] = chunks[j-1], chunks[j]
		}
	}
	var refs []refMatch
	for _, c := range chunks {
		refs = append(refs, c.matches...)
	}
	return refs
}
