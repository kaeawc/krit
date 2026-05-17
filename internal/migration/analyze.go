package migration

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

type Options struct {
	Root    string
	Library string
	From    string
	To      string
	Map     Map
}

type Suggestion struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Symbol    string `json:"symbol"`
	Current   string `json:"current"`
	Suggested string `json:"suggested"`
	Reason    string `json:"reason"`
}

type Report struct {
	Library     string       `json:"library"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	Suggestions []Suggestion `json:"suggestions"`
}

func Analyze(opts Options) (Report, error) {
	entries, err := opts.Map.Select(opts.Library, opts.From, opts.To)
	if err != nil {
		return Report{}, err
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	kotlinPaths, javaPaths, err := scanner.CollectKotlinAndJavaFiles(context.Background(), []string{root}, nil)
	if err != nil {
		return Report{}, err
	}
	kotlinFiles, errs := scanner.ScanFiles(context.Background(), kotlinPaths, runtime.NumCPU())
	if len(errs) > 0 {
		return Report{}, errs[0]
	}
	javaFiles, errs := scanner.ScanJavaFiles(context.Background(), javaPaths, runtime.NumCPU())
	if len(errs) > 0 {
		return Report{}, errs[0]
	}
	index := scanner.BuildIndex(kotlinFiles, runtime.NumCPU(), javaFiles...)
	files := make(map[string]*scanner.File, len(kotlinFiles)+len(javaFiles))
	for _, file := range append(kotlinFiles, javaFiles...) {
		if file != nil {
			files[file.Path] = file
		}
	}

	seen := make(map[string]bool)
	report := Report{Library: opts.Map.Library, From: opts.From, To: opts.To}
	for _, ref := range index.References {
		if ref.InComment {
			continue
		}
		file := files[ref.File]
		if file == nil || ref.Line <= 0 || ref.Line > len(file.Lines) {
			continue
		}
		line := file.Lines[ref.Line-1]
		for _, entry := range entries {
			if !referenceMatches(ref.Name, line, entry.Symbol) {
				continue
			}
			current := strings.TrimSpace(line)
			suggested := suggestLine(current, entry.Symbol, entry.Replacement)
			key := ref.File + "\x00" + entry.Symbol + "\x00" + suggested + "\x00" + current
			if seen[key] {
				continue
			}
			seen[key] = true
			report.Suggestions = append(report.Suggestions, Suggestion{
				File:      relPath(root, ref.File),
				Line:      ref.Line,
				Column:    columnFor(line, ref.Name),
				Symbol:    entry.Symbol,
				Current:   current,
				Suggested: suggested,
				Reason:    entryReason(opts.Map.Library, opts.To, entry),
			})
		}
	}
	sort.Slice(report.Suggestions, func(i, j int) bool {
		if report.Suggestions[i].File != report.Suggestions[j].File {
			return report.Suggestions[i].File < report.Suggestions[j].File
		}
		if report.Suggestions[i].Line != report.Suggestions[j].Line {
			return report.Suggestions[i].Line < report.Suggestions[j].Line
		}
		return report.Suggestions[i].Symbol < report.Suggestions[j].Symbol
	})
	return report, nil
}

func referenceMatches(name, line, symbol string) bool {
	owner := ownerSimpleName(symbol)
	call := simpleName(symbol)
	if name == owner {
		return true
	}
	if name == call && owner != "" && strings.Contains(line, owner) {
		return true
	}
	return false
}

func suggestLine(current, symbol, replacement string) string {
	suggested := current
	if owner := ownerFQN(symbol); owner != "" {
		if replacementOwner := ownerFQN(replacement); replacementOwner != "" && strings.Contains(suggested, owner) {
			suggested = strings.ReplaceAll(suggested, owner, replacementOwner)
			return suggested
		}
	}
	if owner := ownerSimpleName(symbol); owner != "" {
		if replacementOwner := ownerSimpleName(replacement); replacementOwner != "" {
			suggested = strings.ReplaceAll(suggested, owner, replacementOwner)
		}
	}
	if call := simpleName(symbol); call != "" {
		if replacementCall := simpleName(replacement); replacementCall != "" {
			suggested = strings.ReplaceAll(suggested, call, replacementCall)
		}
	}
	return suggested
}

func entryReason(library, to string, entry Entry) string {
	if strings.TrimSpace(entry.Reason) != "" {
		return entry.Reason
	}
	return library + " " + to + " migration map marks " + entry.Symbol + " for replacement."
}

func ownerFQN(symbol string) string {
	if i := strings.LastIndex(symbol, "."); i >= 0 {
		return symbol[:i]
	}
	return ""
}

func ownerSimpleName(symbol string) string {
	parts := strings.Split(symbol, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2]
}

func simpleName(symbol string) string {
	if i := strings.LastIndex(symbol, "."); i >= 0 {
		return symbol[i+1:]
	}
	return symbol
}

func columnFor(line, token string) int {
	if token == "" {
		return 1
	}
	if i := strings.Index(line, token); i >= 0 {
		return i + 1
	}
	return 1
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
