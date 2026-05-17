package codemod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/kotlin"
)

type Edit struct {
	File        string
	StartByte   int
	EndByte     int
	Replacement string
}

type Result struct {
	Matches       int
	FilesMatched  int
	EditsApplied  int
	FilesModified int
	Edits         []Edit
}

func Run(ctx context.Context, root string, recipe Recipe, apply bool) (Result, error) {
	paths, err := sourceFiles(root, recipe.Language)
	if err != nil {
		return Result{}, err
	}
	var all []Edit
	matchedFiles := make(map[string]bool)
	for _, path := range paths {
		edits, err := EditsForFile(ctx, path, recipe)
		if err != nil {
			return Result{}, err
		}
		if len(edits) > 0 {
			matchedFiles[path] = true
			all = append(all, edits...)
		}
	}
	result := Result{Matches: len(all), FilesMatched: len(matchedFiles), Edits: all}
	if apply {
		applied, modified, err := ApplyEdits(all)
		if err != nil {
			return result, err
		}
		result.EditsApplied = applied
		result.FilesModified = modified
	}
	return result, nil
}

func EditsForFile(ctx context.Context, path string, recipe Recipe) ([]Edit, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lang := languageFor(recipe.Language)
	if lang == nil {
		return nil, fmt.Errorf("unsupported language %q", recipe.Language)
	}
	query, err := sitter.NewQuery([]byte(recipe.Match), lang)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)
	var edits []Edit
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		filtered := cursor.FilterPredicates(match, content)
		if filtered == nil {
			continue
		}
		edit, ok := editForMatch(path, content, query, filtered, recipe.Replacement)
		if ok {
			edits = append(edits, edit)
		}
	}
	return dedupeAndDropOverlapping(edits), nil
}

func editForMatch(path string, content []byte, query *sitter.Query, match *sitter.QueryMatch, replacement string) (Edit, bool) {
	captures := make(map[string]*sitter.Node)
	var nodes []*sitter.Node
	for _, capture := range match.Captures {
		name := query.CaptureNameForId(capture.Index)
		captures[name] = capture.Node
		nodes = append(nodes, capture.Node)
	}
	if len(nodes) == 0 {
		return Edit{}, false
	}
	target := captures["match"]
	if target == nil {
		target = lowestCommonAncestor(nodes)
	}
	if target == nil {
		return Edit{}, false
	}
	return Edit{
		File:        path,
		StartByte:   int(target.StartByte()),
		EndByte:     int(target.EndByte()),
		Replacement: interpolate(replacement, captures, content),
	}, true
}

var interpolationRe = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func interpolate(template string, captures map[string]*sitter.Node, content []byte) string {
	return interpolationRe.ReplaceAllStringFunc(template, func(token string) string {
		m := interpolationRe.FindStringSubmatch(token)
		if len(m) != 2 {
			return token
		}
		node := captures[m[1]]
		if node == nil {
			return ""
		}
		return node.Content(content)
	})
}

func lowestCommonAncestor(nodes []*sitter.Node) *sitter.Node {
	if len(nodes) == 0 {
		return nil
	}
	current := nodes[0]
	for current != nil && !current.IsNull() {
		ok := true
		for _, node := range nodes[1:] {
			if !nodeHasAncestorOrSelf(node, current) {
				ok = false
				break
			}
		}
		if ok {
			return current
		}
		current = current.Parent()
	}
	return nil
}

func nodeHasAncestorOrSelf(node, ancestor *sitter.Node) bool {
	for node != nil && !node.IsNull() {
		if node.Equal(ancestor) {
			return true
		}
		node = node.Parent()
	}
	return false
}

func dedupeAndDropOverlapping(edits []Edit) []Edit {
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].File != edits[j].File {
			return edits[i].File < edits[j].File
		}
		if edits[i].StartByte != edits[j].StartByte {
			return edits[i].StartByte < edits[j].StartByte
		}
		return edits[i].EndByte < edits[j].EndByte
	})
	var out []Edit
	for _, edit := range edits {
		if edit.StartByte >= edit.EndByte {
			continue
		}
		if len(out) > 0 {
			prev := out[len(out)-1]
			if prev.File == edit.File && edit.StartByte < prev.EndByte {
				continue
			}
			if prev.File == edit.File && prev.StartByte == edit.StartByte && prev.EndByte == edit.EndByte {
				continue
			}
		}
		out = append(out, edit)
	}
	return out
}

func ApplyEdits(edits []Edit) (applied int, filesModified int, err error) {
	byFile := make(map[string][]Edit)
	for _, edit := range edits {
		byFile[edit.File] = append(byFile[edit.File], edit)
	}
	for path, fileEdits := range byFile {
		content, err := os.ReadFile(path)
		if err != nil {
			return applied, filesModified, err
		}
		sort.Slice(fileEdits, func(i, j int) bool { return fileEdits[i].StartByte > fileEdits[j].StartByte })
		buf := append([]byte(nil), content...)
		for _, edit := range fileEdits {
			if edit.StartByte < 0 || edit.EndByte > len(buf) || edit.StartByte > edit.EndByte {
				continue
			}
			buf = append(buf[:edit.StartByte], append([]byte(edit.Replacement), buf[edit.EndByte:]...)...)
			applied++
		}
		if string(buf) == string(content) {
			continue
		}
		if err := os.WriteFile(path, buf, 0644); err != nil {
			return applied, filesModified, err
		}
		filesModified++
	}
	return applied, filesModified, nil
}

func sourceFiles(root, language string) ([]string, error) {
	var suffix string
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "kotlin":
		suffix = ".kt"
	case "java":
		suffix = ".java"
	default:
		return nil, fmt.Errorf("unsupported language %q", language)
	}
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".gradle", ".idea", ".kotlin", "build", "out", "target", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, suffix) {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func languageFor(language string) *sitter.Language {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "kotlin":
		return kotlin.GetLanguage()
	case "java":
		return java.GetLanguage()
	default:
		return nil
	}
}
