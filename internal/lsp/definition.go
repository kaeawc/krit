package lsp

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// handleDefinition finds the declaration of the symbol under the cursor.
func (s *Server) handleDefinition(req *Request) {
	var params DefinitionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("definition params error: %v", err)
		s.sendResponse(req.ID, nil, nil)
		return
	}

	uri := params.TextDocument.URI
	file, ok := s.getDocumentFlatFile(uri)
	if !ok {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	name := identifierAtPositionFlat(file, params.Position)
	if name == "" {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	loc := findDeclarationFlat(file, name, uri)
	if loc == nil {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	s.sendResponse(req.ID, loc, nil)
}

func identifierAtPositionFlat(file *scanner.File, pos Position) string {
	offset := flatByteOffsetAtPosition(file, pos)
	if node := flatIdentifierNodeAtOffset(file, offset); node != 0 {
		return file.FlatNodeText(node)
	}
	return identifierAtFlatColumn(file, pos)
}

func flatByteOffsetAtPosition(file *scanner.File, pos Position) int {
	if file == nil {
		return 0
	}
	return file.LineOffset(int(pos.Line)) + int(pos.Character)
}

func flatNodeAtPosition(file *scanner.File, pos Position) uint32 {
	offset := flatByteOffsetAtPosition(file, pos)
	if file == nil {
		return 0
	}
	end := offset + 1
	if end > len(file.Content) {
		end = len(file.Content)
	}
	if idx, ok := file.FlatNamedDescendantForByteRange(uint32(offset), uint32(end)); ok {
		return idx
	}
	return 0
}

func flatIdentifierNodeAtOffset(file *scanner.File, offset int) uint32 {
	if file == nil || file.FlatTree == nil {
		return 0
	}
	var best uint32
	bestSpan := -1
	file.FlatWalkAllNodes(0, func(idx uint32) {
		t := file.FlatType(idx)
		if t != "simple_identifier" && t != "type_identifier" {
			return
		}
		start := int(file.FlatStartByte(idx))
		end := int(file.FlatEndByte(idx))
		if offset < start || offset >= end {
			return
		}
		span := end - start
		if best == 0 || bestSpan < 0 || span < bestSpan {
			best = idx
			bestSpan = span
		}
	})
	return best
}

func findDeclarationFlat(file *scanner.File, name string, uri string) *Location {
	var result *Location
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if result != nil {
			return
		}
		switch file.FlatType(idx) {
		case "class_declaration", "object_declaration", "function_declaration":
			if declName := extractDeclNameFlat(file, idx); declName == name {
				result = flatNodeToLocation(file, idx, uri, name)
			}
		case "property_declaration":
			if declName := extractPropertyNameFlat(file, idx); declName == name {
				result = flatNodeToLocation(file, idx, uri, name)
			}
		}
	})
	if result != nil {
		return result
	}

	file.FlatWalkAllNodes(0, func(idx uint32) {
		if result != nil {
			return
		}
		t := file.FlatType(idx)
		if t != "simple_identifier" && t != "type_identifier" {
			return
		}
		if file.FlatNodeText(idx) != name {
			return
		}
		for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
			switch file.FlatType(parent) {
			case "class_declaration", "object_declaration", "function_declaration", "property_declaration":
				result = flatNodeToLocation(file, parent, uri, name)
				return
			}
		}
	})
	return result
}

func identifierAtFlatColumn(file *scanner.File, pos Position) string {
	if file == nil || int(pos.Line) >= len(file.Lines) {
		return ""
	}
	line := file.Lines[pos.Line]
	col := int(pos.Character)
	if col < 0 {
		return ""
	}
	if col > len(line) {
		col = len(line)
	}
	start := col
	for start > 0 && isIdentifierChar(line[start-1]) {
		start--
	}
	end := col
	for end < len(line) && isIdentifierChar(line[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return line[start:end]
}

func isIdentifierChar(b byte) bool {
	return b == '_' || b == '$' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func extractDeclNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if t := file.FlatType(child); t == "simple_identifier" || t == "type_identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

func extractPropertyNameFlat(file *scanner.File, idx uint32) string {
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" {
					return file.FlatNodeText(gc)
				}
			}
		}
	}
	return ""
}

func flatNodeToLocation(file *scanner.File, idx uint32, uri string, name string) *Location {
	content := string(file.Content)
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		if (file.FlatType(child) == "simple_identifier" || file.FlatType(child) == "type_identifier") &&
			file.FlatNodeText(child) == name {
			return &Location{
				URI: uri,
				Range: Range{
					Start: byteOffsetToPosition(content, int(file.FlatStartByte(child))),
					End:   byteOffsetToPosition(content, int(file.FlatEndByte(child))),
				},
			}
		}
		if file.FlatType(child) == "variable_declaration" {
			for j := 0; j < file.FlatChildCount(child); j++ {
				gc := file.FlatChild(child, j)
				if file.FlatType(gc) == "simple_identifier" && file.FlatNodeText(gc) == name {
					return &Location{
						URI: uri,
						Range: Range{
							Start: byteOffsetToPosition(content, int(file.FlatStartByte(gc))),
							End:   byteOffsetToPosition(content, int(file.FlatEndByte(gc))),
						},
					}
				}
			}
		}
	}
	return &Location{
		URI: uri,
		Range: Range{
			Start: byteOffsetToPosition(content, int(file.FlatStartByte(idx))),
			End:   byteOffsetToPosition(content, int(file.FlatEndByte(idx))),
		},
	}
}

// handleReferences finds all occurrences of the symbol under cursor in the current file.
func (s *Server) handleReferences(req *Request) {
	var params ReferenceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("references params error: %v", err)
		s.sendResponse(req.ID, []Location{}, nil)
		return
	}

	uri := params.TextDocument.URI
	file, ok := s.getDocumentFlatFile(uri)
	if !ok {
		s.sendResponse(req.ID, []Location{}, nil)
		return
	}

	name := identifierAtPositionFlat(file, params.Position)
	if name == "" {
		s.sendResponse(req.ID, []Location{}, nil)
		return
	}

	locations := findAllIdentifiersFlat(file, name, uri)
	if locations == nil {
		locations = []Location{}
	}

	if !params.Context.IncludeDeclaration {
		declLoc := findDeclarationFlat(file, name, uri)
		if declLoc != nil {
			filtered := make([]Location, 0, len(locations))
			for _, loc := range locations {
				if loc.Range.Start != declLoc.Range.Start || loc.Range.End != declLoc.Range.End {
					filtered = append(filtered, loc)
				}
			}
			locations = filtered
		}
	}

	s.sendResponse(req.ID, locations, nil)
}

func findAllIdentifiersFlat(file *scanner.File, name string, uri string) []Location {
	var locs []Location
	content := string(file.Content)
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if t := file.FlatType(idx); (t == "simple_identifier" || t == "type_identifier") && file.FlatNodeText(idx) == name {
			locs = append(locs, Location{
				URI: uri,
				Range: Range{
					Start: byteOffsetToPosition(content, int(file.FlatStartByte(idx))),
					End:   byteOffsetToPosition(content, int(file.FlatEndByte(idx))),
				},
			})
		}
	})
	return locs
}

// handleRename renames all occurrences of the symbol under cursor in the current file.
func (s *Server) handleRename(req *Request) {
	var params RenameParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("rename params error: %v", err)
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "invalid params"})
		return
	}

	uri := params.TextDocument.URI
	file, ok := s.getDocumentFlatFile(uri)
	if !ok {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "document not open"})
		return
	}

	name := identifierAtPositionFlat(file, params.Position)
	if name == "" {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "no symbol at position"})
		return
	}

	if strings.TrimSpace(params.NewName) == "" {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "new name cannot be empty"})
		return
	}

	locations := findAllIdentifiersFlat(file, name, uri)
	if len(locations) == 0 {
		s.sendResponse(req.ID, nil, nil)
		return
	}

	edits := make([]TextEdit, len(locations))
	for i, loc := range locations {
		edits[i] = TextEdit{
			Range:   loc.Range,
			NewText: params.NewName,
		}
	}

	wsEdit := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			uri: edits,
		},
	}
	s.sendResponse(req.ID, wsEdit, nil)
}

// handleCompletion provides basic completions for annotations and suppression rule names.
func (s *Server) handleCompletion(req *Request) {
	var params CompletionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Printf("completion params error: %v", err)
		s.sendResponse(req.ID, CompletionList{Items: []CompletionItem{}}, nil)
		return
	}

	uri := params.TextDocument.URI
	s.docsMu.Lock()
	doc, ok := s.docs[uri]
	var content []byte
	if ok {
		content = doc.Content
	}
	s.docsMu.Unlock()

	if !ok {
		s.sendResponse(req.ID, CompletionList{Items: []CompletionItem{}}, nil)
		return
	}

	line := getLineAtPosition(content, params.Position)
	items := computeCompletions(line, params.Position)

	s.sendResponse(req.ID, CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// getLineAtPosition returns the text of the line at the given position.
func getLineAtPosition(content []byte, pos Position) string {
	lines := strings.Split(string(content), "\n")
	if int(pos.Line) >= len(lines) {
		return ""
	}
	return lines[pos.Line]
}

// computeCompletions returns completion items based on the line context.
func computeCompletions(line string, pos Position) []CompletionItem {
	col := int(pos.Character)
	if col > len(line) {
		col = len(line)
	}
	prefix := line[:col]

	if idx := strings.LastIndex(prefix, "@Suppress(\""); idx >= 0 {
		afterQuote := prefix[idx+len("@Suppress(\""):]
		if strings.Contains(afterQuote, "\"") {
			return nil
		}
		return ruleNameCompletions(afterQuote)
	}
	if idx := strings.LastIndex(prefix, "@SuppressWarnings(\""); idx >= 0 {
		afterQuote := prefix[idx+len("@SuppressWarnings(\""):]
		if strings.Contains(afterQuote, "\"") {
			return nil
		}
		return ruleNameCompletions(afterQuote)
	}

	trimmed := strings.TrimSpace(prefix)
	if strings.HasSuffix(trimmed, "@") || (strings.Contains(trimmed, "@") && !strings.Contains(trimmed[strings.LastIndex(trimmed, "@"):], "(")) {
		return annotationCompletions()
	}

	return nil
}

func annotationCompletions() []CompletionItem {
	annotations := []struct {
		label  string
		detail string
		insert string
	}{
		{"Suppress", "Suppress warnings", "Suppress(\"$1\")"},
		{"SuppressWarnings", "Suppress warnings (Java-style)", "SuppressWarnings(\"$1\")"},
		{"JvmStatic", "JVM static method", "JvmStatic"},
		{"JvmField", "JVM field access", "JvmField"},
		{"JvmOverloads", "Generate overloaded methods", "JvmOverloads"},
		{"JvmName", "Custom JVM name", "JvmName(\"$1\")"},
		{"Composable", "Jetpack Compose composable", "Composable"},
		{"Deprecated", "Mark as deprecated", "Deprecated(\"$1\")"},
		{"Throws", "Declare thrown exceptions", "Throws($1::class)"},
		{"Serializable", "Mark as serializable", "Serializable"},
		{"Parcelize", "Android Parcelable generation", "Parcelize"},
		{"Keep", "Prevent ProGuard removal", "Keep"},
		{"VisibleForTesting", "Visible for testing only", "VisibleForTesting"},
	}

	items := make([]CompletionItem, len(annotations))
	for i, a := range annotations {
		items[i] = CompletionItem{
			Label:      a.label,
			Kind:       CompletionKindClass,
			Detail:     a.detail,
			InsertText: a.insert,
		}
	}
	return items
}

func ruleNameCompletions(typed string) []CompletionItem {
	var items []CompletionItem
	lower := strings.ToLower(typed)
	for _, r := range registeredRuleNames() {
		if typed == "" || strings.Contains(strings.ToLower(r), lower) {
			items = append(items, CompletionItem{
				Label:  r,
				Kind:   CompletionKindText,
				Detail: "krit rule",
			})
		}
	}
	if items == nil {
		items = []CompletionItem{}
	}
	return items
}

func registeredRuleNames() []string {
	var names []string
	for _, r := range rules.Registry {
		names = append(names, r.Name())
	}
	return names
}
