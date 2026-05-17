package lsp

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

func (s *Server) goToTestCodeActions(uri string, pos Position) []CodeAction {
	root := uriToPath(s.rootURI)
	if root == "" {
		return nil
	}
	currentFile, ok := s.getDocumentFlatFile(uri)
	if !ok || isLSPTestPath(uriToPath(uri)) {
		return nil
	}
	name := identifierAtPositionFlat(currentFile, pos)
	if name == "" {
		return nil
	}

	idx, err := buildWorkspaceCodeIndex(root)
	if err != nil {
		log.Printf("goToTest index error: %v", err)
		return nil
	}
	target, ok := symbolAtCurrentFile(idx, uriToPath(uri), name, currentFile, pos)
	if !ok {
		return nil
	}
	locations := testLocationsForSymbol(idx, target)
	if len(locations) == 0 {
		return nil
	}

	actions := make([]CodeAction, 0, len(locations))
	for _, loc := range locations {
		title := fmt.Sprintf("Go to test: %s", filepath.Base(uriToPath(loc.URI)))
		actions = append(actions, CodeAction{
			Title: title,
			Kind:  "refactor",
			Command: &Command{
				Title:     title,
				Command:   "krit.goToTest",
				Arguments: []interface{}{loc},
			},
		})
	}
	return actions
}

func buildWorkspaceCodeIndex(root string) (*scanner.CodeIndex, error) {
	paths, err := scanner.CollectKotlinFiles([]string{root}, nil)
	if err != nil {
		return nil, err
	}
	files := make([]*scanner.File, 0, len(paths))
	for _, path := range paths {
		file, err := scanner.ParseFile(context.Background(), path)
		if err != nil {
			continue
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, nil
	}
	return scanner.BuildIndex(files, 0), nil
}

func symbolAtCurrentFile(idx *scanner.CodeIndex, path, name string, currentFile *scanner.File, pos Position) (scanner.Symbol, bool) {
	if idx == nil {
		return scanner.Symbol{}, false
	}
	path = filepath.Clean(path)
	cursorOffset := filePositionToByteOffset(currentFile, pos)
	var fallback *scanner.Symbol
	for _, sym := range idx.SymbolsNamed(name) {
		if filepath.Clean(sym.File) != path || isLSPTestPath(sym.File) {
			continue
		}
		candidate := sym
		if sym.StartByte <= cursorOffset && cursorOffset <= sym.EndByte {
			return candidate, true
		}
		if fallback == nil {
			fallback = &candidate
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return scanner.Symbol{}, false
}

func testLocationsForSymbol(idx *scanner.CodeIndex, target scanner.Symbol) []Location {
	if idx == nil || target.Name == "" {
		return nil
	}
	testFiles := make(map[string]bool)
	for _, name := range referenceNamesForGoToTest(target) {
		for path := range idx.ReferenceFiles(name) {
			if isLSPTestPath(path) && filepath.Clean(path) != filepath.Clean(target.File) {
				testFiles[path] = true
			}
		}
	}
	if len(testFiles) == 0 {
		return nil
	}

	paths := make([]string, 0, len(testFiles))
	for path := range testFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	locations := make([]Location, 0, len(paths))
	for _, path := range paths {
		if loc, ok := bestTestDeclarationLocation(idx, path, target); ok {
			locations = append(locations, loc)
		}
	}
	return locations
}

func referenceNamesForGoToTest(sym scanner.Symbol) []string {
	names := []string{sym.Name}
	if sym.FQN != "" {
		names = append(names, sym.FQN)
	}
	if sym.Owner != "" {
		names = append(names, sym.Owner+"."+sym.Name)
	}
	return names
}

func bestTestDeclarationLocation(idx *scanner.CodeIndex, path string, target scanner.Symbol) (Location, bool) {
	var symbols []scanner.Symbol
	for _, sym := range idx.Symbols {
		if filepath.Clean(sym.File) == filepath.Clean(path) && isTestDeclarationKind(sym.Kind) {
			symbols = append(symbols, sym)
		}
	}
	if len(symbols) == 0 {
		return Location{}, false
	}

	targetTerms := testNameTerms(target)
	for _, sym := range symbols {
		if symbolNameContainsAny(sym.Name, targetTerms) {
			return symbolLocation(idx, sym), true
		}
	}
	return symbolLocation(idx, symbols[0]), true
}

func testNameTerms(sym scanner.Symbol) []string {
	var terms []string
	if sym.Name != "" {
		terms = append(terms, strings.ToLower(sym.Name))
	}
	if sym.Owner != "" {
		parts := strings.Split(sym.Owner, ".")
		terms = append(terms, strings.ToLower(parts[len(parts)-1]))
	}
	return terms
}

func symbolNameContainsAny(name string, terms []string) bool {
	lower := strings.ToLower(name)
	for _, term := range terms {
		if term != "" && strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func symbolLocation(idx *scanner.CodeIndex, sym scanner.Symbol) Location {
	start := Position{Line: 0, Character: 0}
	end := start
	if file := indexFileByPath(idx, sym.File); file != nil && sym.EndByte > sym.StartByte {
		content := string(file.Content)
		start = byteOffsetToPosition(content, sym.StartByte)
		end = byteOffsetToPosition(content, sym.EndByte)
		return Location{
			URI: pathToURI(sym.File),
			Range: Range{
				Start: start,
				End:   end,
			},
		}
	}
	if sym.Line > 0 {
		start.Line = uint32(sym.Line - 1)
		end = start
	}
	return Location{
		URI: pathToURI(sym.File),
		Range: Range{
			Start: start,
			End:   end,
		},
	}
}

func indexFileByPath(idx *scanner.CodeIndex, path string) *scanner.File {
	if idx == nil {
		return nil
	}
	clean := filepath.Clean(path)
	for _, file := range idx.Files {
		if file != nil && filepath.Clean(file.Path) == clean {
			return file
		}
	}
	return nil
}

func isTestDeclarationKind(kind string) bool {
	switch kind {
	case "class", "object", "interface", "function":
		return true
	default:
		return false
	}
}

func isLSPTestPath(path string) bool {
	normalized := filepath.ToSlash(path)
	for _, marker := range []string{"/test/", "/androidTest/", "/commonTest/", "/jvmTest/", "/testFixtures/"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
