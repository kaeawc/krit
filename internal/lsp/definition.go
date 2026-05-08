package lsp

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// handleDefinition finds the declaration of the symbol under the cursor.
//
// The oracle FQN index is preferred when available — it can jump across
// files and resolve symbols imported from libraries. When the oracle is
// offline (the daemon hasn't warmed up, or no workspace index has been
// loaded yet) the handler falls back to the single-file textual walker so
// users are no worse off than before milestone 5.
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

	if idx := s.waitForOracleIndex(500 * time.Millisecond); idx != nil {
		if locs := lookupDefinitionByName(idx, name); len(locs) > 0 {
			s.sendResponse(req.ID, definitionResult(locs), nil)
			return
		}
	}

	loc := findDeclarationFlat(file, name, uri)
	if loc == nil {
		s.sendResponse(req.ID, nil, nil)
		return
	}
	s.sendResponse(req.ID, loc, nil)
}

// lookupDefinitionByName returns LSP Locations for every declaration the
// oracle index recognises by the given simple name. DeclLocations without
// source positions still resolve to the declaring file at Range{0,0}, which
// is the best we can do until krit-types emits per-decl line/column.
func lookupDefinitionByName(idx *oracle.Index, name string) []Location {
	candidates := idx.FindDeclarationBySimpleName(name)
	if len(candidates) == 0 {
		return nil
	}
	out := make([]Location, 0, len(candidates))
	for _, decl := range candidates {
		if decl == nil || (decl.File == "" && decl.JARPath == "") {
			continue
		}
		out = append(out, declLocation(decl))
	}
	return out
}

// declLocation converts an oracle DeclLocation into an LSP Location. Positions
// are 1-based in the oracle and 0-based in LSP; zero values map to the start
// of the file, which is the documented behaviour when krit-types has not yet
// emitted a per-decl source position.
func declLocation(decl *oracle.DeclLocation) Location {
	line := decl.Line
	if line > 0 {
		line--
	}
	col := decl.Column
	if col > 0 {
		col--
	}
	pos := Position{Line: uint32(line), Character: uint32(col)}
	uri := pathToURI(decl.File)
	if decl.JARPath != "" {
		uri = BuildJARURI(JARRef{JARPath: decl.JARPath, FQN: decl.FQN})
	}
	return Location{
		URI: uri,
		Range: Range{
			Start: pos,
			End:   pos,
		},
	}
}

// definitionResult collapses to a single Location when there is exactly one
// candidate, matching the LSP convention that single-result definition uses
// the bare object form.
func definitionResult(locs []Location) interface{} {
	if len(locs) == 1 {
		return locs[0]
	}
	return locs
}

func identifierAtPositionFlat(file *scanner.File, pos Position) string {
	offset := filePositionToByteOffset(file, pos)
	if node := flatIdentifierNodeAtOffset(file, offset); node != 0 {
		return file.FlatNodeText(node)
	}
	return identifierAtFlatColumn(file, pos)
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

// handleReferences finds every occurrence of the symbol under the cursor.
//
// When the oracle index is available the result is a workspace-wide list
// pulled from FindReferencesByFQN; the legacy single-file walker still acts
// as the fallback when the daemon is offline.
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

	if idx := s.waitForOracleIndex(500 * time.Millisecond); idx != nil {
		if locs := workspaceReferencesByName(idx, name, params.Context.IncludeDeclaration, len(name)); locs != nil {
			s.sendResponse(req.ID, locs, nil)
			return
		}
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

// workspaceReferencesByName aggregates oracle references for every FQN whose
// simple name matches name. Returns nil when no FQN candidate is recognised
// so the handler can drop to the textual fallback. nameLen is used to size
// the returned ranges; the oracle stores reference positions but not spans.
func workspaceReferencesByName(idx *oracle.Index, name string, includeDecl bool, nameLen int) []Location {
	candidates := idx.FindDeclarationBySimpleName(name)
	if len(candidates) == 0 {
		return nil
	}
	var out []Location
	for _, decl := range candidates {
		if decl == nil {
			continue
		}
		for _, ref := range idx.FindReferencesByFQN(decl.FQN) {
			if !includeDecl && ref.IsDeclaration {
				continue
			}
			if ref.File == "" {
				continue
			}
			out = append(out, refLocation(ref, nameLen))
		}
	}
	if out == nil {
		out = []Location{}
	}
	return out
}

// refLocation converts an oracle ReferenceLocation into an LSP Location,
// adjusting from 1-based to 0-based and synthesising an end column from
// nameLen.
func refLocation(ref oracle.ReferenceLocation, nameLen int) Location {
	line := ref.Line
	if line > 0 {
		line--
	}
	col := ref.Column
	if col > 0 {
		col--
	}
	start := Position{Line: uint32(line), Character: uint32(col)}
	end := Position{Line: uint32(line), Character: uint32(col + nameLen)}
	return Location{
		URI: pathToURI(ref.File),
		Range: Range{
			Start: start,
			End:   end,
		},
	}
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

// handleRename produces a WorkspaceEdit that renames every occurrence of the
// symbol under the cursor.
//
// With the oracle index loaded, rename targets every reference across the
// workspace. The new name is validated against Kotlin identifier rules
// before the edit is emitted. When the oracle is unavailable the rename
// degrades to a single-file walker AND a window/showMessage warning so the
// user knows the edit only touched the open file — silent partial renames
// are worse than a failed rename.
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

	newName := strings.TrimSpace(params.NewName)
	if newName == "" {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "new name cannot be empty"})
		return
	}
	if !isKotlinIdentifier(newName) {
		s.sendResponse(req.ID, nil, &RPCError{Code: -32602, Message: "new name is not a valid Kotlin identifier"})
		return
	}

	if idx := s.waitForOracleIndex(500 * time.Millisecond); idx != nil {
		if changes := workspaceRenameByName(idx, name, newName); len(changes) > 0 {
			s.sendResponse(req.ID, WorkspaceEdit{Changes: changes}, nil)
			return
		}
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
			NewText: newName,
		}
	}

	wsEdit := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			uri: edits,
		},
	}
	s.sendResponse(req.ID, wsEdit, nil)
	s.sendNotification("window/showMessage", ShowMessageParams{
		Type:    MessageTypeWarning,
		Message: "Rename limited to the current file: workspace index unavailable. Public symbols may need manual updates in other files.",
	})
}

// workspaceRenameByName builds the per-file edit groups for a workspace
// rename. It returns nil when no FQN candidate is recognised so the handler
// can drop to the textual fallback.
func workspaceRenameByName(idx *oracle.Index, name, newName string) map[string][]TextEdit {
	candidates := idx.FindDeclarationBySimpleName(name)
	if len(candidates) == 0 {
		return nil
	}
	changes := map[string][]TextEdit{}
	for _, decl := range candidates {
		if decl == nil {
			continue
		}
		for _, ref := range idx.FindReferencesByFQN(decl.FQN) {
			if ref.File == "" || ref.Line == 0 {
				continue
			}
			loc := refLocation(ref, len(name))
			changes[loc.URI] = append(changes[loc.URI], TextEdit{
				Range:   loc.Range,
				NewText: newName,
			})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return changes
}

func isKotlinIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		switch {
		case ch == '_':
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case i > 0 && ch >= '0' && ch <= '9':
		default:
			return false
		}
	}
	return true
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
	names := make([]string, 0, len(api.Registry))
	for _, r := range api.Registry {
		names = append(names, r.ID)
	}
	return names
}
